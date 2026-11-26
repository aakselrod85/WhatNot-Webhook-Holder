package main

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// knownMobFKs is every foreign key that points at channels, shows, show_breaks or break_spots in mobwebapp as of
// migration 20261126000019. The script checks the catalog against this list and stops when a later migration added
// another one, because the wipe has to decide what to do with it.
var knownMobFKs = map[string]bool{
	"break_cards.break_id->show_breaks": true, "break_hits.break_id->show_breaks": true,
	"break_spots.break_id->show_breaks": true, "break_templates.channel_id->channels": true,
	"build_log.show_id->shows": true, "build_slots.break_id->show_breaks": true,
	"build_slots.show_id->shows": true, "planned_breaks.break_id->show_breaks": true,
	"planned_shows.channel_id->channels": true, "planned_shows.show_id->shows": true,
	"show_breaks.show_id->shows": true, "shows.channel_id->channels": true,
}

type fkRef struct {
	Table, Column, Target string // Table is already quoted
	Raw                   string
}

func mobFKs(mtx q) ([]fkRef, error) {
	var rows []struct {
		Tbl    string `db:"tbl"`
		Raw    string `db:"raw"`
		Col    string `db:"col"`
		Target string `db:"target"`
		N      int    `db:"n"`
	}
	err := mtx.Select(&rows, `SELECT c.conrelid::regclass::text AS tbl, c.conrelid::regclass::text AS raw,
			(SELECT a.attname FROM pg_attribute a WHERE a.attrelid = c.conrelid AND a.attnum = c.conkey[1]) AS col,
			c.confrelid::regclass::text AS target, cardinality(c.conkey) AS n
		FROM pg_constraint c
		WHERE c.contype = 'f' AND c.confrelid = ANY (ARRAY['channels', 'shows', 'show_breaks', 'break_spots']::regclass[])
		ORDER BY 1, 3`)
	if err != nil {
		return nil, err
	}
	var out []fkRef
	for _, r := range rows {
		key := r.Raw + "." + r.Col + "->" + r.Target
		if r.N != 1 || !knownMobFKs[key] {
			return nil, fmt.Errorf("mobwebapp has a foreign key this script does not know: %s; decide what the wipe does with it and add it to the script", key)
		}
		out = append(out, fkRef{Table: r.Tbl, Column: pq.QuoteIdentifier(r.Col), Target: r.Target, Raw: key})
	}
	if len(out) != len(knownMobFKs) {
		return nil, fmt.Errorf("expected %d foreign keys into the shared tables, found %d (a table of the plan is missing?)", len(knownMobFKs), len(out))
	}
	return out, nil
}

type mobChannel struct {
	ID         int64  `db:"id"`
	Key        string `db:"key"`
	Name       string `db:"name"`
	ManualHits bool   `db:"manual_hits"`
	Sort       int    `db:"sort"`
}

func exec(mtx *sqlx.Tx, what string, query string, args ...interface{}) (int64, error) {
	res, err := mtx.Exec(query, args...)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", what, err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func (a *app) phase2(mtx *sqlx.Tx, hq q, tr *Translation, d *copyData) (bool, error) {
	fks, err := mobFKs(mtx)
	if err != nil {
		return false, err
	}

	// keep_* temp tables: the ids this script owns. Everything else in the three tables is "what mobwebapp has today".
	for _, t := range []string{"keep_shows", "keep_breaks", "keep_spots"} {
		if _, err := mtx.Exec(`CREATE TEMP TABLE ` + t + ` (id int PRIMARY KEY) ON COMMIT DROP`); err != nil {
			return false, err
		}
	}
	for name, ids := range map[string][]int64{"keep_shows": tr.keepList(tr.Stream), "keep_breaks": tr.keepList(tr.Break), "keep_spots": tr.keepList(tr.Event)} {
		ids := ids
		name := name
		if err := chunks(len(ids), func(lo, hi int) error {
			_, err := mtx.Exec(`INSERT INTO `+name+` SELECT unnest($1::int[])`, pq.Array(ids[lo:hi]))
			return err
		}); err != nil {
			return false, err
		}
	}

	// Stop rule: a mobwebapp show that would be wiped but shares a Whatnot show id with the holder.
	if err := a.stopCheck(mtx, hq); err != nil {
		return false, err
	}

	if err := a.copyChannels(mtx, fks, d); err != nil {
		return false, err
	}
	if err := a.wipe(mtx); err != nil {
		return false, err
	}
	if err := a.insertRows(mtx, d); err != nil {
		return false, err
	}

	// Channels' live state: active show, translated; null when that stream is not copied.
	var nulled []string
	for _, c := range d.Channels {
		var active sql.NullInt64
		if c.ActiveStream.Valid {
			if nid, ok := tr.Stream[c.ActiveStream.Int64]; ok {
				active = sql.NullInt64{Int64: nid, Valid: true}
			} else {
				nulled = append(nulled, fmt.Sprintf("channel %d (stream %d not copied)", c.ID, c.ActiveStream.Int64))
			}
		}
		if _, err := exec(mtx, "channel settings", `UPDATE channels SET active_show_id = $2, default_high_bid_team = $3,
			default_high_bid_floor = $4, whatnot_api_key = $5, whatnot_webhook_secret = $6 WHERE id = $1`,
			c.ID, active, c.HighBidTeam, c.HighBidFloor, c.APIKey, c.Secret); err != nil {
			return false, err
		}
	}
	if len(nulled) > 0 {
		fmt.Println("active_show_id set to null for:", strings.Join(nulled, "; "))
	}
	if a.execute {
		if _, err := mtx.Exec(`SELECT setval(pg_get_serial_sequence('channels', 'id'), (SELECT MAX(id) FROM channels))`); err != nil {
			return false, err
		}
	}

	res, err := a.verify(hq, mtx, tr)
	if err != nil {
		return false, err
	}
	res.print(false)
	return res.OK(), nil
}

func (a *app) stopCheck(mtx *sqlx.Tx, hq q) error {
	var existing []struct {
		ID      int64          `db:"id"`
		Channel int64          `db:"channel_id"`
		Title   string         `db:"title"`
		Status  string         `db:"status"`
		WID     sql.NullString `db:"whatnot_show_id"`
	}
	if err := mtx.Select(&existing, `SELECT id, channel_id, title, status, whatnot_show_id FROM shows
		WHERE id NOT IN (SELECT id FROM keep_shows) ORDER BY id`); err != nil {
		return err
	}
	var wids []string
	for _, s := range existing {
		if s.WID.Valid {
			wids = append(wids, s.WID.String)
		}
	}
	var clash []string
	if len(wids) > 0 {
		if err := hq.Select(&clash, `SELECT DISTINCT show_id FROM whatnot_show WHERE show_id = ANY($1)`, pq.Array(wids)); err != nil {
			return err
		}
	}
	fmt.Printf("mobwebapp shows that the wipe removes: %d\n", len(existing))
	for _, s := range existing {
		fmt.Printf("  show %d channel %d %q status=%s whatnot_show_id=%q\n", s.ID, s.Channel, s.Title, s.Status, s.WID.String)
	}
	if len(clash) > 0 {
		return fmt.Errorf("STOP: mobwebapp show(s) with a Whatnot show id that also exists in the holder's whatnot_show: %s", strings.Join(clash, ", "))
	}
	return nil
}

func (a *app) copyChannels(mtx *sqlx.Tx, fks []fkRef, d *copyData) error {
	var existing []mobChannel
	if err := mtx.Select(&existing, `SELECT id, key, name, manual_hits, sort FROM channels ORDER BY id`); err != nil {
		return err
	}
	byKey := map[string]mobChannel{}
	byID := map[int64]mobChannel{}
	for _, c := range existing {
		byKey[c.Key] = c
		byID[c.ID] = c
	}
	mappedKey := map[string]int64{}
	for _, m := range a.chanMap {
		mappedKey[m.Key] = m.ID
	}
	var pending []chanMapping
	for _, m := range a.chanMap {
		c, ok := byKey[m.Key]
		if !ok {
			return fmt.Errorf("channel-map: mobwebapp has no channel with key %q", m.Key)
		}
		if c.ID != m.ID {
			pending = append(pending, m)
		}
	}
	for _, m := range pending {
		if occ, ok := byID[m.ID]; ok {
			if _, isMapped := mappedKey[occ.Key]; !isMapped {
				return fmt.Errorf("channel id %d (needed for %q) is taken by the unmapped channel %q", m.ID, m.Key, occ.Key)
			}
		}
	}
	holderHas := map[int64]bool{}
	for _, c := range d.Channels {
		holderHas[c.ID] = true
	}
	for _, m := range a.chanMap {
		if !holderHas[m.ID] {
			fmt.Printf("WARNING: holder has no live channel %d for mobwebapp key %q\n", m.ID, m.Key)
		}
	}

	var chanFKs []fkRef
	for _, f := range fks {
		if f.Target == "channels" {
			chanFKs = append(chanFKs, f)
		}
	}
	repoint := func(from, to int64) error {
		for _, f := range chanFKs {
			n, err := exec(mtx, "repoint "+f.Table, `UPDATE `+f.Table+` SET `+f.Column+` = $1 WHERE `+f.Column+` = $2`, to, from)
			if err != nil {
				return err
			}
			if n > 0 {
				fmt.Printf("  %s.%s: %d rows %d -> %d\n", f.Table, f.Column, n, from, to)
			}
		}
		return nil
	}
	const tmpBase = 1000000
	if len(pending) == 0 {
		fmt.Println("renumber: the three channels already have their holder ids")
	}
	for _, m := range pending { // A: out of the way
		c := byKey[m.Key]
		fmt.Printf("renumber %q: %d -> %d (via a temporary row)\n", c.Key, c.ID, m.ID)
		if _, err := exec(mtx, "tmp channel", `INSERT INTO channels (id, key, name, manual_hits, sort) VALUES ($1, $2, $3, $4, $5)`,
			tmpBase+m.ID, c.Key+"__moving", c.Name, c.ManualHits, c.Sort); err != nil {
			return err
		}
		if err := repoint(c.ID, tmpBase+m.ID); err != nil {
			return err
		}
		if _, err := exec(mtx, "delete old channel", `DELETE FROM channels WHERE id = $1`, c.ID); err != nil {
			return err
		}
	}
	for _, m := range pending { // B: into place
		c := byKey[m.Key]
		if _, err := exec(mtx, "final channel", `INSERT INTO channels (id, key, name, manual_hits, sort) VALUES ($1, $2, $3, $4, $5)`,
			m.ID, c.Key, c.Name, c.ManualHits, c.Sort); err != nil {
			return err
		}
		if err := repoint(tmpBase+m.ID, m.ID); err != nil {
			return err
		}
		if _, err := exec(mtx, "delete tmp channel", `DELETE FROM channels WHERE id = $1`, tmpBase+m.ID); err != nil {
			return err
		}
	}

	// The other holder channels, with their holder ids.
	var maxSort int
	if err := mtx.Get(&maxSort, `SELECT COALESCE(MAX(sort), 0) FROM channels`); err != nil {
		return err
	}
	finalByID := map[int64]bool{}
	for _, c := range existing {
		finalByID[c.ID] = true
	}
	for _, m := range pending {
		finalByID[m.ID] = true
	}
	var added []int64
	for _, c := range d.Channels {
		if finalByID[c.ID] {
			if occ, ok := byID[c.ID]; ok && c.ID < tmpBase {
				_, mapped := mappedKey[occ.Key]
				if !mapped && occ.Key != fmt.Sprintf("holder_%d", c.ID) {
					return fmt.Errorf("mobwebapp channel %d (%q) is not one of the mapped channels and not holder_%d; refusing to overwrite its holder settings", c.ID, occ.Key, c.ID)
				}
			}
			continue
		}
		maxSort++
		name := strings.TrimSpace(c.Name)
		if name == "" {
			name = fmt.Sprintf("Channel %d", c.ID)
		}
		if _, err := exec(mtx, "new channel", `INSERT INTO channels (id, key, name, manual_hits, sort) VALUES ($1, $2, $3, false, $4)`,
			c.ID, fmt.Sprintf("holder_%d", c.ID), name, maxSort); err != nil {
			return err
		}
		added = append(added, c.ID)
	}
	fmt.Printf("channels added: %v\n", added)
	return nil
}

func (a *app) wipe(mtx *sqlx.Tx) error {
	fmt.Println("wipe (rows whose ids are not in the translation table):")
	steps := []struct{ what, sql string }{
		{"break_spots deleted", `DELETE FROM break_spots WHERE id NOT IN (SELECT id FROM keep_spots)`},
		{"break_hits deleted", `DELETE FROM break_hits WHERE break_id NOT IN (SELECT id FROM keep_breaks)`},
		{"break_cards deleted (card list of a wiped break; the inventory items stay)", `DELETE FROM break_cards WHERE break_id NOT IN (SELECT id FROM keep_breaks)`},
		{"build_slots.break_id set to null", `UPDATE build_slots SET break_id = NULL WHERE break_id IS NOT NULL AND break_id NOT IN (SELECT id FROM keep_breaks)`},
		{"build_slots.show_id set to null", `UPDATE build_slots SET show_id = NULL WHERE show_id IS NOT NULL AND show_id NOT IN (SELECT id FROM keep_shows)`},
		{"build_log.show_id set to null", `UPDATE build_log SET show_id = NULL WHERE show_id IS NOT NULL AND show_id NOT IN (SELECT id FROM keep_shows)`},
		{"planned_breaks.break_id set to null", `UPDATE planned_breaks SET break_id = NULL WHERE break_id IS NOT NULL AND break_id NOT IN (SELECT id FROM keep_breaks)`},
		{"planned_shows.show_id set to null", `UPDATE planned_shows SET show_id = NULL WHERE show_id IS NOT NULL AND show_id NOT IN (SELECT id FROM keep_shows)`},
		{"show_breaks deleted", `DELETE FROM show_breaks WHERE id NOT IN (SELECT id FROM keep_breaks)`},
		{"shows deleted", `DELETE FROM shows WHERE id NOT IN (SELECT id FROM keep_shows)`},
	}
	for _, s := range steps {
		n, err := exec(mtx, s.what, s.sql)
		if err != nil {
			return err
		}
		fmt.Printf("  %-80s %d\n", s.what, n)
	}
	return nil
}

func (a *app) insertRows(mtx *sqlx.Tx, d *copyData) error {
	// shows
	n := int64(0)
	err := chunks(len(d.Shows), func(lo, hi int) error {
		part := d.Shows[lo:hi]
		var ids, chans []int64
		var titles, statuses, descs, thumbs, vis, wtitles []string
		var sched, started, ended, created, deleted []sql.NullString
		var wid []sql.NullString
		var active []sql.NullInt64
		for _, s := range part {
			ids = append(ids, s.NewID)
			chans = append(chans, s.ChannelID)
			titles = append(titles, s.Title)
			statuses = append(statuses, s.Status)
			descs = append(descs, s.Description)
			thumbs = append(thumbs, s.Thumbnail)
			vis = append(vis, s.Visibility)
			if s.Bound {
				wtitles = append(wtitles, s.WTitle)
			} else {
				wtitles = append(wtitles, "")
			}
			sched = append(sched, nullStr(s.Scheduled))
			started = append(started, nullStr(s.Started))
			ended = append(ended, nullStr(s.Ended))
			created = append(created, sql.NullString{String: s.CreatedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"), Valid: true})
			if s.Deleted {
				deleted = append(deleted, sql.NullString{String: "now", Valid: true})
			} else {
				deleted = append(deleted, sql.NullString{})
			}
			if s.Bound {
				wid = append(wid, sql.NullString{String: s.WID, Valid: true})
			} else {
				wid = append(wid, sql.NullString{})
			}
			active = append(active, s.ActiveBreak)
		}
		res, err := mtx.Exec(`INSERT INTO shows (id, channel_id, title, status, scheduled_at, started_at, ended_at, whatnot_show_id,
				created_at, active_break_id, description, thumbnail_url, visibility, whatnot_show_title, deleted_at)
			SELECT id, channel_id, title, status, sched, started, ended, wid, created, active, descr, thumb, vis, wtitle,
				CASE WHEN del IS NULL THEN NULL ELSE now() END
			FROM unnest($1::int[], $2::int[], $3::text[], $4::text[], $5::timestamptz[], $6::timestamptz[], $7::timestamptz[],
				$8::text[], $9::timestamptz[], $10::int[], $11::text[], $12::text[], $13::text[], $14::text[], $15::text[])
				AS t(id, channel_id, title, status, sched, started, ended, wid, created, active, descr, thumb, vis, del, wtitle)
			ON CONFLICT (id) DO NOTHING`,
			pq.Array(ids), pq.Array(chans), pq.Array(titles), pq.Array(statuses), pq.GenericArray{A: sched}, pq.GenericArray{A: started},
			pq.GenericArray{A: ended}, pq.GenericArray{A: wid}, pq.GenericArray{A: created}, pq.GenericArray{A: active},
			pq.Array(descs), pq.Array(thumbs), pq.Array(vis), pq.GenericArray{A: deleted}, pq.Array(wtitles))
		if err != nil {
			return fmt.Errorf("insert shows: %w", err)
		}
		c, _ := res.RowsAffected()
		n += c
		return nil
	})
	if err != nil {
		return err
	}
	fmt.Printf("shows inserted: %d (of %d; the rest were already there)\n", n, len(d.Shows))

	showCreated := map[int64]string{}
	for _, s := range d.Shows {
		showCreated[s.NewID] = s.CreatedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00")
	}

	// breaks
	n = 0
	err = chunks(len(d.Breaks), func(lo, hi int) error {
		var ids, shows, nums, floors []int64
		var titles, statuses, hb, gt, created []string
		var series []sql.NullInt64
		var started, ended, deleted []sql.NullString
		showStatus := map[int64]string{}
		for _, s := range d.Shows {
			showStatus[s.NewID] = s.BreakStatus
		}
		for _, b := range d.Breaks[lo:hi] {
			ids = append(ids, b.NewID)
			shows = append(shows, b.ShowNewID)
			nums = append(nums, int64(b.Number))
			floors = append(floors, int64(b.HighBidFloor))
			titles = append(titles, b.Title)
			statuses = append(statuses, showStatus[b.ShowNewID])
			hb = append(hb, b.HighBidTeam)
			gt = append(gt, b.GiveawayTeam)
			created = append(created, showCreated[b.ShowNewID])
			series = append(series, b.SeriesID)
			started = append(started, nullStr(b.Started))
			ended = append(ended, nullStr(b.Ended))
			deleted = append(deleted, delMark(b.Deleted))
		}
		res, err := mtx.Exec(`INSERT INTO show_breaks (id, show_id, number, title, status, created_at, high_bid_team, giveaway_team,
				high_bid_floor, started_at, ended_at, series_id, deleted_at)
			SELECT id, show_id, number, title, status, created::timestamptz, hb, gt, floor, started, ended, series,
				CASE WHEN del IS NULL THEN NULL ELSE now() END
			FROM unnest($1::int[], $2::int[], $3::int[], $4::text[], $5::text[], $6::text[], $7::text[], $8::text[], $9::int[],
				$10::timestamptz[], $11::timestamptz[], $12::bigint[], $13::text[])
				AS t(id, show_id, number, title, status, created, hb, gt, floor, started, ended, series, del)
			ON CONFLICT (id) DO NOTHING`,
			pq.Array(ids), pq.Array(shows), pq.Array(nums), pq.Array(titles), pq.Array(statuses), pq.Array(created), pq.Array(hb),
			pq.Array(gt), pq.Array(floors), pq.GenericArray{A: started}, pq.GenericArray{A: ended}, pq.GenericArray{A: series},
			pq.GenericArray{A: deleted})
		if err != nil {
			return fmt.Errorf("insert show_breaks: %w", err)
		}
		c, _ := res.RowsAffected()
		n += c
		return nil
	})
	if err != nil {
		return err
	}
	fmt.Printf("show_breaks inserted: %d (of %d)\n", n, len(d.Breaks))

	breakShow := map[int64]int64{}
	for _, b := range d.Breaks {
		breakShow[b.NewID] = b.ShowNewID
	}
	// spots
	n = 0
	err = chunks(len(d.Spots), func(lo, hi int) error {
		var ids, breaks, pos, gtype, qty []int64
		var labels, cust, notes, created []string
		var price, deleted []sql.NullString
		var giveaway []bool
		for _, s := range d.Spots[lo:hi] {
			ids = append(ids, s.NewID)
			breaks = append(breaks, s.BreakNewID)
			pos = append(pos, int64(s.Position))
			gtype = append(gtype, int64(s.GiveawayType))
			qty = append(qty, int64(s.Quantity))
			labels = append(labels, s.Label)
			cust = append(cust, s.Customer)
			notes = append(notes, s.Note)
			created = append(created, showCreated[breakShow[s.BreakNewID]])
			price = append(price, s.Price)
			giveaway = append(giveaway, s.IsGiveaway)
			deleted = append(deleted, delMark(s.Deleted))
		}
		res, err := mtx.Exec(`INSERT INTO break_spots (id, break_id, label, price, created_at, position, is_giveaway, giveaway_type,
				note, quantity, customer_name, deleted_at)
			SELECT id, break_id, label, price::numeric(10,2), created::timestamptz, pos, giveaway, gtype, note, qty, cust,
				CASE WHEN del IS NULL THEN NULL ELSE now() END
			FROM unnest($1::int[], $2::int[], $3::text[], $4::text[], $5::text[], $6::int[], $7::bool[], $8::smallint[],
				$9::text[], $10::int[], $11::text[], $12::text[])
				AS t(id, break_id, label, price, created, pos, giveaway, gtype, note, qty, cust, del)
			ON CONFLICT (id) DO NOTHING`,
			pq.Array(ids), pq.Array(breaks), pq.Array(labels), pq.GenericArray{A: price}, pq.Array(created), pq.Array(pos),
			pq.Array(giveaway), pq.Array(gtype), pq.Array(notes), pq.Array(qty), pq.Array(cust), pq.GenericArray{A: deleted})
		if err != nil {
			return fmt.Errorf("insert break_spots: %w", err)
		}
		c, _ := res.RowsAffected()
		n += c
		return nil
	})
	if err != nil {
		return err
	}
	fmt.Printf("break_spots inserted: %d (of %d)\n", n, len(d.Spots))
	return nil
}

func delMark(deleted bool) sql.NullString {
	if deleted {
		return sql.NullString{String: "x", Valid: true}
	}
	return sql.NullString{}
}
