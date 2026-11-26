package repository_sqlx

import (
	"database/sql"
	"time"

	"github.com/SaloEater/WhatNot-Webhook-Holder/entity"
	"github.com/jmoiron/sqlx"
)

// BreakRepository works on mobwebapp's show_breaks table (the second connection pool).
type BreakRepository struct {
	DB *sqlx.DB
}

// breakCols maps show_breaks onto entity.Break. A null started_at / ended_at is the zero time, as the holder's
// date columns stored it. number, status and whatnot_ref are never written.
const breakCols = `b.id, b.show_id AS day_id, b.title AS name, b.high_bid_team, b.giveaway_team, b.high_bid_floor,
	COALESCE(b.started_at, '0001-01-01 00:00:00+00') AS start_date,
	COALESCE(b.ended_at, '0001-01-01 00:00:00+00') AS end_date,
	(b.deleted_at IS NOT NULL) AS is_deleted, b.series_id, b.number`

// nullTime stores the zero time as NULL.
func nullTime(t time.Time) sql.NullTime {
	return sql.NullTime{Time: t, Valid: !t.IsZero()}
}

func (r *BreakRepository) Get(id int64) (*entity.Break, error) {
	var dayBreak entity.Break
	err := r.DB.Unsafe().Get(&dayBreak, `SELECT `+breakCols+` FROM show_breaks b WHERE b.id = $1 AND b.deleted_at IS NULL`, id)
	return &dayBreak, err
}

func (r *BreakRepository) Delete(id int64) error {
	_, err := r.DB.Exec(`
WITH updatedBreak AS (UPDATE show_breaks SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL),
updatedEvent AS (UPDATE break_spots SET deleted_at = now() WHERE break_id = $1 AND deleted_at IS NULL)
SELECT TRUE
`, id)
	return err
}

func (r *BreakRepository) Update(dayBreak *entity.Break) error {
	_, err := r.DB.Exec(`UPDATE show_breaks SET
		started_at = $2,
		ended_at = $3,
		high_bid_team = $4,
		giveaway_team = $5,
		high_bid_floor = $6
	WHERE id = $1`, dayBreak.Id, nullTime(dayBreak.StartDate), nullTime(dayBreak.EndDate),
		dayBreak.HighBidTeam, dayBreak.GiveawayTeam, dayBreak.HighBidFloor)

	return err
}

func (r *BreakRepository) Rename(id int64, name string) error {
	_, err := r.DB.Exec(`UPDATE show_breaks SET title = $2 WHERE id = $1`, id, name)
	return err
}

func (r *BreakRepository) GetBreaksByStreamId(dayId int64) ([]*entity.Break, error) {
	breaks := []*entity.Break{}
	err := r.DB.Unsafe().Select(&breaks, `SELECT `+breakCols+` FROM show_breaks b WHERE b.show_id = $1 AND b.deleted_at IS NULL ORDER BY b.number, b.id`, dayId)
	return breaks, err
}

func (r *BreakRepository) SetSeries(breakID int64, seriesID *int64) error {
	_, err := r.DB.Exec(`UPDATE show_breaks SET series_id = $1 WHERE id = $2`, seriesID, breakID)
	return err
}

// GetActiveSeriesIdByChannel returns the series of the channel's active break; nil, nil when the channel has no
// active show, no active break, or the break has no series. The break must not be deleted.
func (r *BreakRepository) GetActiveSeriesIdByChannel(channelId int64) (*int64, error) {
	var seriesId sql.NullInt64
	err := r.DB.Get(&seriesId, `
SELECT b.series_id
FROM channels c
JOIN shows s ON s.id = c.active_show_id
JOIN show_breaks b ON b.id = s.active_break_id AND b.show_id = s.id AND b.deleted_at IS NULL
WHERE c.id = $1`, channelId)
	if err == sql.ErrNoRows || (err == nil && !seriesId.Valid) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &seriesId.Int64, nil
}
