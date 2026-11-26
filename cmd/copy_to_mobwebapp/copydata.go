package main

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

type srcChannel struct {
	ID           int64         `db:"id"`
	Name         string        `db:"name"`
	HighBidTeam  string        `db:"default_high_bid_team"`
	HighBidFloor int           `db:"default_high_bid_floor"`
	APIKey       string        `db:"whatnot_api_key"`
	Secret       string        `db:"whatnot_webhook_secret"`
	ActiveStream sql.NullInt64 `db:"active_stream_id"`
}

type srcShow struct {
	OldID       int64         `db:"old_id"`
	NewID       int64         `db:"new_id"`
	ChannelID   int64         `db:"channel_id"`
	Title       string        `db:"title"`
	Status      string        `db:"status"`
	Deleted     bool          `db:"deleted"`
	CreatedAt   time.Time     `db:"created_at"`
	ActiveBreak sql.NullInt64 `db:"active_break"`
	WID         string        `db:"wid"`
	WTitle      string        `db:"wtitle"` // the Whatnot show's title; copied only where Bound
	Description string        `db:"description"`
	Thumbnail   string        `db:"thumbnail_url"`
	Visibility  string        `db:"visibility"`
	Scheduled   sql.NullTime  `db:"sched"`
	Started     sql.NullTime  `db:"started_at"`
	Ended       sql.NullTime  `db:"ended_at"`
	Bound       bool          // keeps whatnot_show_id (decided in Go)
	BreakStatus string        // 'done' / 'open' for its breaks
}

type srcBreak struct {
	NewID        int64         `db:"id"`
	ShowNewID    int64         `db:"show_id"`
	Number       int           `db:"number"`
	Title        string        `db:"title"`
	HighBidTeam  string        `db:"high_bid_team"`
	GiveawayTeam string        `db:"giveaway_team"`
	HighBidFloor int           `db:"high_bid_floor"`
	SeriesID     sql.NullInt64 `db:"series_id"`
	Started      sql.NullTime  `db:"started"`
	Ended        sql.NullTime  `db:"ended"`
	Deleted      bool          `db:"deleted"`
}

type srcSpot struct {
	NewID        int64          `db:"id"`
	BreakNewID   int64          `db:"break_id"`
	Position     int            `db:"position"`
	Label        string         `db:"label"`
	Customer     string         `db:"customer"`
	Price        sql.NullString `db:"price"`
	IsGiveaway   bool           `db:"is_giveaway"`
	GiveawayType int            `db:"giveaway_type"`
	Note         string         `db:"note"`
	Quantity     int            `db:"quantity"`
	Deleted      bool           `db:"deleted"`
}

type copyData struct {
	Channels []srcChannel
	Shows    []*srcShow
	Breaks   []srcBreak
	Spots    []srcSpot
	// Notes for the report.
	SharedWhatnot   []string // Whatnot show ids with several streams: what was done
	NullIndexSpots  int
	IsEndedMismatch int
}

// overStatuses are the show statuses that mean the show is over (the same set the holder repository derives is_ended from).
var overStatuses = map[string]bool{"ended": true, "verified": true, "failed_to_verify": true, "cancelled": true}

var allowedShowStatus = map[string]bool{"scheduled": true, "live": true, "ended": true, "cancelled": true, "verified": true, "failed_to_verify": true}

func loadCopyData(hq q, tr *Translation) (*copyData, error) {
	d := &copyData{}
	if err := hq.Select(&d.Channels, `SELECT id, COALESCE(name, '') AS name, default_high_bid_team, default_high_bid_floor,
		whatnot_api_key, whatnot_webhook_secret, active_stream_id FROM channel WHERE NOT COALESCE(is_deleted, false) ORDER BY id`); err != nil {
		return nil, err
	}
	if err := hq.Select(&d.Shows, `WITH `+pickWhatnotShow+`
		SELECT s.id AS old_id, ts.new_id, s.channel_id, COALESCE(s.name, '') AS title, s.status,
			COALESCE(s.is_deleted, false) AS deleted, s.created_at::timestamp AT TIME ZONE 'UTC' AS created_at,
			tb.new_id AS active_break, p.wid, COALESCE(p.wtitle, '') AS wtitle, p.description, p.thumbnail_url, p.visibility,
			p.scheduled_date AS sched, p.started_at, p.ended_at
		FROM stream s
		JOIN copy_translation ts ON ts.entity = 'stream' AND ts.old_id = s.id
		JOIN pick p ON p.stream_id = s.id
		LEFT JOIN copy_translation tb ON tb.entity = 'break' AND tb.old_id = s.active_break_id
		ORDER BY s.id`); err != nil {
		return nil, err
	}
	if len(d.Shows) != len(tr.Stream) {
		return nil, fmt.Errorf("%d translated streams but %d loaded (a stream lost its Whatnot show link?)", len(tr.Stream), len(d.Shows))
	}

	// Status, binding.
	byWID := map[string][]*srcShow{}
	showStatus := map[int64]string{}
	for _, s := range d.Shows {
		if s.Status == "planned" {
			s.Status = "scheduled"
		}
		if !allowedShowStatus[s.Status] {
			return nil, fmt.Errorf("stream %d has status %q, which mobwebapp's shows_status_check does not allow", s.OldID, s.Status)
		}
		s.BreakStatus = "open"
		if overStatuses[s.Status] {
			s.BreakStatus = "done"
		}
		showStatus[s.NewID] = s.BreakStatus
		byWID[s.WID] = append(byWID[s.WID], s)
	}
	// One Whatnot show, several streams: the newest live stream (highest id) keeps whatnot_show_id; the others get
	// null. Deleted streams never keep it, unless every stream of that Whatnot show is deleted (then the newest one does).
	var wids []string
	for w := range byWID {
		wids = append(wids, w)
	}
	sort.Strings(wids)
	for _, w := range wids {
		list := byWID[w]
		var keep *srcShow
		for _, s := range list { // list is ordered by old id ascending
			if !s.Deleted {
				keep = s
			}
		}
		if keep == nil {
			keep = list[len(list)-1]
		}
		keep.Bound = true
		if len(list) > 1 {
			var others []string
			for _, s := range list {
				if s != keep {
					others = append(others, fmt.Sprintf("%d", s.OldID))
				}
			}
			d.SharedWhatnot = append(d.SharedWhatnot, fmt.Sprintf("whatnot %s: stream %d keeps it; streams %s copied without it", w, keep.OldID, strings.Join(others, ",")))
		}
	}

	if err := hq.Select(&d.Breaks, `SELECT tb.new_id AS id, ts.new_id AS show_id,
			row_number() OVER (PARTITION BY b.day_id ORDER BY b.id)::int AS number, COALESCE(b.name, '') AS title,
			b.high_bid_team, b.giveaway_team, b.high_bid_floor, b.series_id,
			b.start_date AT TIME ZONE 'UTC' AS started, b.end_date AT TIME ZONE 'UTC' AS ended,
			COALESCE(b.is_deleted, false) AS deleted
		FROM break b
		JOIN copy_translation ts ON ts.entity = 'stream' AND ts.old_id = b.day_id
		JOIN copy_translation tb ON tb.entity = 'break' AND tb.old_id = b.id
		ORDER BY b.id`); err != nil {
		return nil, err
	}
	if len(d.Breaks) != len(tr.Break) {
		return nil, fmt.Errorf("%d translated breaks but %d loaded", len(tr.Break), len(d.Breaks))
	}
	if err := hq.Select(&d.Spots, `SELECT te.new_id AS id, tb.new_id AS break_id, COALESCE(e."index", 0) AS "position",
			COALESCE(e.team, '') AS label, COALESCE(e.customer, '') AS customer, round(e.price::numeric, 2)::text AS price,
			COALESCE(e.is_giveaway, false) AS is_giveaway, COALESCE(e.giveaway_type, 0) AS giveaway_type,
			COALESCE(e.note, '') AS note, COALESCE(e.quantity, 1) AS quantity, COALESCE(e.is_deleted, false) AS deleted
		FROM event e
		JOIN copy_translation te ON te.entity = 'event' AND te.old_id = e.id
		JOIN copy_translation tb ON tb.entity = 'break' AND tb.old_id = e.break_id
		ORDER BY e.id`); err != nil {
		return nil, err
	}
	if len(d.Spots) != len(tr.Event) {
		return nil, fmt.Errorf("%d translated events but %d loaded", len(tr.Event), len(d.Spots))
	}
	// Facts for the report.
	if err := hq.Get(&d.NullIndexSpots, `SELECT count(*) FROM event e JOIN copy_translation te ON te.entity = 'event' AND te.old_id = e.id WHERE e."index" IS NULL`); err != nil {
		return nil, err
	}
	if err := hq.Get(&d.IsEndedMismatch, `SELECT count(*) FROM stream s JOIN copy_translation ts ON ts.entity = 'stream' AND ts.old_id = s.id
		WHERE s.is_ended <> (s.status IN ('ended', 'failed_to_verify', 'verified'))`); err != nil {
		return nil, err
	}
	_ = showStatus
	return d, nil
}

func nullStr(t sql.NullTime) sql.NullString {
	if !t.Valid {
		return sql.NullString{}
	}
	return sql.NullString{String: t.Time.UTC().Format(time.RFC3339Nano), Valid: true}
}

func sortInt64(a []int64) { sort.Slice(a, func(i, j int) bool { return a[i] < a[j] }) }

// chunks calls f over [lo, hi) windows of n items.
func chunks(n int, f func(lo, hi int) error) error {
	const size = 4000
	for lo := 0; lo < n; lo += size {
		hi := lo + size
		if hi > n {
			hi = n
		}
		if err := f(lo, hi); err != nil {
			return err
		}
	}
	return nil
}
