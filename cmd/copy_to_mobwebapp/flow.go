package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// Translation maps holder ids to mobwebapp ids, per entity (the copy_translation table in the holder's database).
type Translation struct {
	Stream, Break, Event map[int64]int64
}

func (t *Translation) keepList(m map[int64]int64) []int64 {
	out := make([]int64, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

func loadTranslation(hq q) (*Translation, error) {
	var rows []struct {
		Entity string `db:"entity"`
		Old    int64  `db:"old_id"`
		New    int64  `db:"new_id"`
	}
	if err := hq.Select(&rows, `SELECT entity, old_id, new_id FROM copy_translation`); err != nil {
		return nil, err
	}
	t := &Translation{map[int64]int64{}, map[int64]int64{}, map[int64]int64{}}
	for _, r := range rows {
		switch r.Entity {
		case "stream":
			t.Stream[r.Old] = r.New
		case "break":
			t.Break[r.Old] = r.New
		case "event":
			t.Event[r.Old] = r.New
		}
	}
	return t, nil
}

// run is the whole script. Dry run: one holder transaction and one mobwebapp transaction run every phase for real
// and are rolled back at the end. Execute: three committed transactions (holder translate, mobwebapp copy, holder
// rewrite), each safe to repeat. Returns false on a verification mismatch.
func (a *app) run(ctx context.Context) (bool, error) {
	htx, err := a.holder.Beginx()
	if err != nil {
		return false, err
	}
	defer htx.Rollback()
	if _, err := htx.Exec(`SET LOCAL lock_timeout = '` + lockTimeout + `'`); err != nil {
		return false, err
	}
	if err := a.ensureHolderTables(htx); err != nil {
		return false, err
	}

	done, err := stepDone(htx, stepRewrite)
	if err != nil {
		return false, err
	}
	if done {
		fmt.Println("\nThe copy was already completed earlier (step log: " + stepRewrite + "). Nothing is changed; mobwebapp is not touched.")
		tr, err := loadTranslation(htx)
		if err != nil {
			return false, err
		}
		htx.Rollback()
		return a.report(a.holder, a.mob, tr)
	}

	// Phase 1: translation rows (holder side), ids allocated from mobwebapp's sequences.
	fmt.Println("\n== Phase 1: translation table (holder) ==")
	tr, err := a.ensureTranslation(htx)
	if err != nil {
		return false, err
	}
	var hq q = htx
	if a.execute {
		if err := htx.Commit(); err != nil {
			return false, err
		}
		hq = a.holder
	}
	fmt.Printf("translation: %d streams, %d breaks, %d events\n", len(tr.Stream), len(tr.Break), len(tr.Event))
	if a.execute && a.stopAfter == 1 {
		fmt.Println("stopping after phase 1 (--stop-after-phase)")
		return true, nil
	}

	// Phase 2: mobwebapp, one transaction.
	fmt.Println("\n== Phase 2: mobwebapp (channels, wipe, copy) ==")
	mtx, err := a.mob.Beginx()
	if err != nil {
		return false, err
	}
	defer mtx.Rollback()
	data, err := loadCopyData(hq, tr)
	if err != nil {
		return false, err
	}
	ok, err := a.phase2(mtx, hq, tr, data)
	if err != nil {
		return false, err
	}
	if !ok {
		fmt.Println("verification failed inside the mobwebapp transaction: rolled back, nothing was written there")
		return false, nil
	}
	if a.execute {
		if err := mtx.Commit(); err != nil {
			return false, err
		}
	}

	if a.execute && a.stopAfter == 2 {
		fmt.Println("stopping after phase 2 (--stop-after-phase)")
		return true, nil
	}

	// Phase 3: holder side id rewrite, move aside, FK drops, step log; one transaction.
	fmt.Println("\n== Phase 3: holder rewrite ==")
	var h3 *sqlx.Tx
	if a.execute {
		if h3, err = a.holder.Beginx(); err != nil {
			return false, err
		}
		defer h3.Rollback()
		if _, err := h3.Exec(`SET LOCAL lock_timeout = '` + lockTimeout + `'`); err != nil {
			return false, err
		}
	} else {
		h3 = htx
	}
	if err := a.phase3(h3, tr); err != nil {
		return false, err
	}
	if a.execute {
		if err := h3.Commit(); err != nil {
			return false, err
		}
		return a.report(a.holder, a.mob, tr)
	}
	return a.report(htx, mtx, tr)
}

func (a *app) ensureHolderTables(htx *sqlx.Tx) error {
	var exists bool
	if err := htx.Get(&exists, `SELECT to_regclass('public.copy_translation') IS NOT NULL AND to_regclass('public.copy_step_log') IS NOT NULL`); err != nil {
		return err
	}
	if exists {
		return nil
	}
	temp := ""
	if !a.execute {
		temp = "TEMP "
		fmt.Println("(dry run: the translation and step-log tables are created as temporary tables)")
	}
	_, err := htx.Exec(`CREATE ` + temp + `TABLE IF NOT EXISTS copy_translation (
		entity text NOT NULL, old_id bigint NOT NULL, new_id bigint NOT NULL,
		PRIMARY KEY (entity, old_id), UNIQUE (entity, new_id))`)
	if err != nil {
		return err
	}
	_, err = htx.Exec(`CREATE ` + temp + `TABLE IF NOT EXISTS copy_step_log (
		step text PRIMARY KEY, done_at timestamptz NOT NULL DEFAULT now(), info text NOT NULL DEFAULT '')`)
	return err
}

func stepDone(hq q, step string) (bool, error) {
	var n int
	err := hq.Get(&n, `SELECT count(*) FROM copy_step_log WHERE step = $1`, step)
	return n > 0, err
}

// stream is one holder stream that has a Whatnot show, with the date that decides whether it is copied.
type stream struct {
	ID        int64         `db:"id"`
	ChannelID int64         `db:"channel_id"`
	Deleted   bool          `db:"deleted"`
	Date      time.Time     `db:"sdate"`
	Created   time.Time     `db:"created"`
	Channel   sql.NullInt64 `db:"chan_ok"`
}

const pickWhatnotShow = `
pick AS (
	SELECT DISTINCT ON (sws.stream_id) sws.stream_id, w.show_id AS wid, w.title AS wtitle, w.description, w.thumbnail_url, w.visibility,
		w.scheduled_date, w.started_at, w.ended_at
	FROM stream_whatnot_show sws JOIN whatnot_show w ON w.id = sws.whatnot_show_id
	ORDER BY sws.stream_id, COALESCE(w.started_at, w.scheduled_date) DESC NULLS LAST, w.id DESC
)`

// ensureTranslation creates the translation rows. First time: the streams of the cutoff window (recorded in the
// step log). Later runs: the streams already in the table, plus any break or event they have gained since.
func (a *app) ensureTranslation(htx *sqlx.Tx) (*Translation, error) {
	logged, err := stepDone(htx, stepTranslate)
	if err != nil {
		return nil, err
	}
	if !logged {
		var streams []stream
		err := htx.Select(&streams, `WITH `+pickWhatnotShow+`
			SELECT s.id, s.channel_id, COALESCE(s.is_deleted, false) AS deleted,
				COALESCE(p.started_at, p.scheduled_date, s.created_at::timestamp AT TIME ZONE 'UTC') AS sdate,
				s.created_at::timestamp AT TIME ZONE 'UTC' AS created,
				(SELECT c.id FROM channel c WHERE c.id = s.channel_id AND NOT COALESCE(c.is_deleted, false)) AS chan_ok
			FROM stream s JOIN pick p ON p.stream_id = s.id ORDER BY s.id`)
		if err != nil {
			return nil, err
		}
		var ids []int64
		for _, s := range streams {
			if s.Date.Before(a.since) || !s.Channel.Valid {
				continue
			}
			ids = append(ids, s.ID)
		}
		newIDs, err := a.alloc("shows", len(ids))
		if err != nil {
			return nil, err
		}
		if err := insertTranslation(htx, "stream", ids, newIDs); err != nil {
			return nil, err
		}
		if _, err := htx.Exec(`INSERT INTO copy_step_log (step, info) VALUES ($1, $2)`, stepTranslate,
			fmt.Sprintf("since=%s streams=%d", a.since.Format(time.RFC3339), len(ids))); err != nil {
			return nil, err
		}
		fmt.Printf("%d streams enter the copy (of %d with a Whatnot show)\n", len(ids), len(streams))
	} else {
		fmt.Println("translation step already logged: the cutoff is not evaluated again; only new breaks/events of known streams are added")
	}

	var breaks []int64
	if err := htx.Select(&breaks, `SELECT b.id FROM break b
		JOIN copy_translation ts ON ts.entity = 'stream' AND ts.old_id = b.day_id
		LEFT JOIN copy_translation tb ON tb.entity = 'break' AND tb.old_id = b.id
		WHERE tb.old_id IS NULL ORDER BY b.id`); err != nil {
		return nil, err
	}
	newIDs, err := a.alloc("show_breaks", len(breaks))
	if err != nil {
		return nil, err
	}
	if err := insertTranslation(htx, "break", breaks, newIDs); err != nil {
		return nil, err
	}

	var events []int64
	if err := htx.Select(&events, `SELECT e.id FROM event e
		JOIN copy_translation tb ON tb.entity = 'break' AND tb.old_id = e.break_id
		LEFT JOIN copy_translation te ON te.entity = 'event' AND te.old_id = e.id
		WHERE te.old_id IS NULL ORDER BY e.id`); err != nil {
		return nil, err
	}
	newIDs, err = a.alloc("break_spots", len(events))
	if err != nil {
		return nil, err
	}
	if err := insertTranslation(htx, "event", events, newIDs); err != nil {
		return nil, err
	}
	fmt.Printf("new translation rows this run: %d breaks, %d events\n", len(breaks), len(events))
	return loadTranslation(htx)
}

// alloc returns n new ids for a mobwebapp table, ascending. Execute: nextval of its sequence (the sequence is not
// rolled back, so ids are never reused even if a later step fails). Dry run: numbers above the table's maximum
// and the sequence's last value, without touching the sequence.
func (a *app) alloc(table string, n int) ([]int64, error) {
	ids := make([]int64, 0, n)
	if n == 0 {
		return ids, nil
	}
	if a.execute {
		if err := a.mob.Select(&ids, `SELECT nextval(pg_get_serial_sequence($1, 'id')) FROM generate_series(1, $2)`, table, n); err != nil {
			return nil, err
		}
		sortInt64(ids)
		return ids, nil
	}
	var base int64
	if err := a.mob.Get(&base, `SELECT GREATEST(
		(SELECT COALESCE(MAX(id), 0) FROM `+pq.QuoteIdentifier(table)+`),
		(SELECT COALESCE(last_value, 0) FROM pg_sequences WHERE schemaname = 'public' AND sequencename = $1))`, table+"_id_seq"); err != nil {
		return nil, err
	}
	for i := 1; i <= n; i++ {
		ids = append(ids, base+int64(i))
	}
	return ids, nil
}

func insertTranslation(htx *sqlx.Tx, entity string, old, new []int64) error {
	return chunks(len(old), func(lo, hi int) error {
		_, err := htx.Exec(`INSERT INTO copy_translation (entity, old_id, new_id)
			SELECT $1, o, n FROM unnest($2::bigint[], $3::bigint[]) AS t(o, n) ON CONFLICT DO NOTHING`,
			entity, pq.Array(old[lo:hi]), pq.Array(new[lo:hi]))
		return err
	})
}
