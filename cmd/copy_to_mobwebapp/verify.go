package main

import (
	"database/sql"
	"fmt"
	"sort"

	"github.com/lib/pq"
)

type showSums struct {
	ID       int64  `db:"id"`
	Breaks   int    `db:"breaks"`
	Events   int    `db:"events"`
	Deleted  int    `db:"deleted"`
	Sum      string `db:"sum"`
	ChannelN int64  `db:"channel_id"`
}

type verifyResult struct {
	Shows, Breaks, Spots int
	PerChannel           map[int64][3]int
	Mismatches           []string
	MismatchCount        int
}

func (r *verifyResult) OK() bool { return r.MismatchCount == 0 }

func (r *verifyResult) print(final bool) {
	fmt.Printf("verification: %d shows, %d breaks, %d spots on the mobwebapp side; %d mismatches\n", r.Shows, r.Breaks, r.Spots, r.MismatchCount)
	for _, m := range r.Mismatches {
		fmt.Println("  MISMATCH", m)
	}
}

// verify compares, per copied show, the number of breaks, number of events (and how many of them deleted) and the
// sum of prices (rounded to cents per event, which is what numeric(10,2) stores) between the holder's old tables
// and mobwebapp. hq is the holder (transaction or pool), mq mobwebapp (transaction or pool).
func (a *app) verify(hq q, mq q, tr *Translation) (*verifyResult, error) {
	res := &verifyResult{PerChannel: map[int64][3]int{}}
	var src []showSums
	if err := hq.Select(&src, `SELECT ts.new_id AS id, count(DISTINCT b.id)::int AS breaks, count(e.id)::int AS events,
			count(e.id) FILTER (WHERE COALESCE(e.is_deleted, false))::int AS deleted,
			COALESCE(sum(round(e.price::numeric, 2)), 0)::numeric(20,2)::text AS sum, 0 AS channel_id
		FROM copy_translation ts
		LEFT JOIN break b ON b.day_id = ts.old_id
		LEFT JOIN event e ON e.break_id = b.id
		WHERE ts.entity = 'stream' GROUP BY ts.new_id ORDER BY ts.new_id`); err != nil {
		return nil, err
	}
	var ids []int64
	for _, s := range src {
		ids = append(ids, s.ID)
	}
	var tgt []showSums
	if err := mq.Select(&tgt, `SELECT s.id, count(DISTINCT b.id)::int AS breaks, count(sp.id)::int AS events,
			count(sp.id) FILTER (WHERE sp.deleted_at IS NOT NULL)::int AS deleted,
			COALESCE(sum(sp.price), 0)::numeric(20,2)::text AS sum, s.channel_id
		FROM shows s LEFT JOIN show_breaks b ON b.show_id = s.id LEFT JOIN break_spots sp ON sp.break_id = b.id
		WHERE s.id = ANY($1) GROUP BY s.id ORDER BY s.id`, pq.Array(ids)); err != nil {
		return nil, err
	}
	tmap := map[int64]showSums{}
	for _, t := range tgt {
		tmap[t.ID] = t
	}
	note := func(format string, args ...interface{}) {
		res.MismatchCount++
		if len(res.Mismatches) < 10 {
			res.Mismatches = append(res.Mismatches, fmt.Sprintf(format, args...))
		}
	}
	for _, s := range src {
		t, ok := tmap[s.ID]
		if !ok {
			note("show %d missing in mobwebapp", s.ID)
			continue
		}
		if s.Breaks != t.Breaks || s.Events != t.Events || s.Deleted != t.Deleted || s.Sum != t.Sum {
			note("show %d: source breaks=%d events=%d deleted=%d sum=%s; target breaks=%d events=%d deleted=%d sum=%s",
				s.ID, s.Breaks, s.Events, s.Deleted, s.Sum, t.Breaks, t.Events, t.Deleted, t.Sum)
		}
		c := res.PerChannel[t.ChannelN]
		c[0]++
		c[1] += t.Breaks
		c[2] += t.Events
		res.PerChannel[t.ChannelN] = c
		res.Shows++
		res.Breaks += t.Breaks
		res.Spots += t.Events
	}
	// The bound Whatnot show's id and title must be what the holder's pick chose (unbound shows keep an empty title).
	var wsrc []struct {
		ID    int64  `db:"id"`
		Wid   string `db:"wid"`
		Title string `db:"wtitle"`
	}
	if err := hq.Select(&wsrc, `WITH `+pickWhatnotShow+`
		SELECT ts.new_id AS id, p.wid, COALESCE(p.wtitle, '') AS wtitle
		FROM copy_translation ts JOIN pick p ON p.stream_id = ts.old_id WHERE ts.entity = 'stream'`); err != nil {
		return nil, err
	}
	var wtgt []struct {
		ID    int64          `db:"id"`
		Wid   sql.NullString `db:"wid"`
		Title string         `db:"wtitle"`
	}
	if err := mq.Select(&wtgt, `SELECT id, whatnot_show_id AS wid, whatnot_show_title AS wtitle FROM shows WHERE id = ANY($1)`, pq.Array(ids)); err != nil {
		return nil, err
	}
	wmap := map[int64]struct {
		Wid   sql.NullString
		Title string
	}{}
	for _, t := range wtgt {
		wmap[t.ID] = struct {
			Wid   sql.NullString
			Title string
		}{t.Wid, t.Title}
	}
	for _, s := range wsrc {
		t, ok := wmap[s.ID]
		switch {
		case !ok:
		case t.Wid.Valid && (t.Wid.String != s.Wid || t.Title != s.Title):
			note("show %d: whatnot show %q title %q; target %q title %q", s.ID, s.Wid, s.Title, t.Wid.String, t.Title)
		case !t.Wid.Valid && t.Title != "":
			note("show %d: unbound but whatnot_show_title is %q", s.ID, t.Title)
		}
	}
	// Totals must match the translation too (a row that exists in no show would hide above).
	if res.Breaks != len(tr.Break) || res.Spots != len(tr.Event) || res.Shows != len(tr.Stream) {
		note("totals: translation has %d streams/%d breaks/%d events, mobwebapp shows %d/%d/%d", len(tr.Stream), len(tr.Break), len(tr.Event), res.Shows, res.Breaks, res.Spots)
	}
	return res, nil
}

// report prints the end-of-run summary and returns false on a mismatch.
func (a *app) report(hq q, mq q, tr *Translation) (bool, error) {
	fmt.Println("\n== Report ==")
	res, err := a.verify(hq, mq, tr)
	if err != nil {
		return false, err
	}
	fmt.Println("copied, per channel (shows / breaks / spots):")
	var chans []int64
	for c := range res.PerChannel {
		chans = append(chans, c)
	}
	sort.Slice(chans, func(i, j int) bool { return chans[i] < chans[j] })
	for _, c := range chans {
		v := res.PerChannel[c]
		fmt.Printf("  channel %d: %d / %d / %d\n", c, v[0], v[1], v[2])
	}
	res.print(true)

	// Skipped streams.
	var skip struct {
		Total        int `db:"total"`
		NoWhatnot    int `db:"no_whatnot"`
		NoWhatnotNew int `db:"no_whatnot_new"`
		WithShow     int `db:"with_show"`
	}
	if err := hq.Get(&skip, `SELECT count(*)::int AS total,
			count(*) FILTER (WHERE NOT EXISTS (SELECT 1 FROM stream_whatnot_show x WHERE x.stream_id = s.id))::int AS no_whatnot,
			count(*) FILTER (WHERE NOT EXISTS (SELECT 1 FROM stream_whatnot_show x WHERE x.stream_id = s.id)
				AND s.created_at::timestamp AT TIME ZONE 'UTC' >= $1)::int AS no_whatnot_new,
			count(*) FILTER (WHERE EXISTS (SELECT 1 FROM stream_whatnot_show x WHERE x.stream_id = s.id))::int AS with_show
		FROM stream s`, a.since); err != nil {
		return false, err
	}
	var notCopiedWithShow, olderOrChannel int
	if err := hq.Get(&notCopiedWithShow, `SELECT count(*)::int FROM stream s WHERE EXISTS (SELECT 1 FROM stream_whatnot_show x WHERE x.stream_id = s.id)
		AND s.id NOT IN (SELECT old_id FROM copy_translation WHERE entity = 'stream')`); err != nil {
		return false, err
	}
	olderOrChannel = notCopiedWithShow
	fmt.Printf("streams in the holder: %d\n  copied: %d\n  skipped, no Whatnot show: %d (of which dated in the window: %d)\n  skipped, has a Whatnot show but is older than the cutoff or on a channel that is not copied: %d\n",
		skip.Total, len(tr.Stream), skip.NoWhatnot, skip.NoWhatnotNew, olderOrChannel)
	return res.OK(), a.jsonScan(hq)
}
