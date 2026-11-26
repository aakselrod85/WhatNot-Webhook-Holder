package repository_sqlx

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/SaloEater/WhatNot-Webhook-Holder/entity"
	"github.com/jmoiron/sqlx"
)

// StreamRepository works on mobwebapp's shows table (the second connection pool). A holder "stream" is a show.
// Statuses: the holder's "planned" is the show's "scheduled"; the others are the same words.
type StreamRepository struct {
	DB *sqlx.DB
}

// streamCols maps shows onto entity.Stream; is_ended is derived from the status.
const streamCols = `s.id, s.title AS name, s.channel_id, s.created_at, (s.deleted_at IS NOT NULL) AS is_deleted,
	(s.status IN ('ended', 'failed_to_verify', 'verified')) AS is_ended, s.active_break_id, s.whatnot_show_id,
	NULLIF(s.whatnot_show_title, '') AS whatnot_show_name,
	CASE WHEN s.status = 'scheduled' THEN 'planned' ELSE s.status END AS status`

func statusToDB(status string) string {
	if status == entity.StreamStatusPlanned {
		return "scheduled"
	}
	return status
}

func (r *StreamRepository) GetAll() ([]*entity.Stream, error) {
	days := []*entity.Stream{}
	err := r.DB.Unsafe().Select(&days, `SELECT `+streamCols+` FROM shows s WHERE s.deleted_at IS NULL`)
	return days, err
}

func (r *StreamRepository) GetAllByChannelId(channelId int64) ([]*entity.Stream, error) {
	days := []*entity.Stream{}
	err := r.DB.Unsafe().Select(&days, `SELECT `+streamCols+` FROM shows s WHERE s.deleted_at IS NULL AND s.channel_id = $1`, channelId)
	return days, err
}

func (r *StreamRepository) GetByChannelIdPaginated(channelId int64, limit int64, offset int64) ([]*entity.Stream, error) {
	days := []*entity.Stream{}
	err := r.DB.Unsafe().Select(&days, `SELECT `+streamCols+` FROM shows s WHERE s.deleted_at IS NULL AND s.channel_id = $1 ORDER BY s.created_at DESC, s.id DESC LIMIT $2 OFFSET $3`, channelId, limit, offset)
	return days, err
}

func (r *StreamRepository) CountByChannelId(channelId int64) (int64, error) {
	var count int64
	err := r.DB.Get(&count, `SELECT COUNT(*) FROM shows WHERE deleted_at IS NULL AND channel_id = $1`, channelId)
	if err != nil {
		return 0, fmt.Errorf("CountByChannelId: %w", err)
	}
	return count, nil
}

func (r *StreamRepository) Get(id int64) (*entity.Stream, error) {
	var day entity.Stream
	err := r.DB.Unsafe().Get(&day, `SELECT `+streamCols+` FROM shows s WHERE s.id = $1 AND s.deleted_at IS NULL`, id)
	return &day, err
}

func (r *StreamRepository) GetByWhatnotShowId(channelId int64, whatnotShowId string) (*entity.Stream, error) {
	var day entity.Stream
	err := r.DB.Unsafe().Get(&day, `SELECT `+streamCols+` FROM shows s WHERE s.channel_id = $1 AND s.whatnot_show_id = $2 AND s.deleted_at IS NULL`, channelId, whatnotShowId)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &day, nil
}

// GetWhatnotShow returns the show's Whatnot-bound content for the Discord card; nil, nil when the show is deleted or
// has no Whatnot show bound. Title is the Whatnot title when stored, else the show's own.
func (r *StreamRepository) GetWhatnotShow(id int64) (*entity.WhatnotShow, error) {
	var show entity.WhatnotShow
	err := r.DB.Unsafe().Get(&show, `SELECT s.id, s.channel_id, s.whatnot_show_id AS show_id, COALESCE(NULLIF(s.whatnot_show_title, ''), s.title) AS title,
	COALESCE(s.description, '') AS description, COALESCE(s.thumbnail_url, '') AS thumbnail_url,
	COALESCE(s.visibility, '') AS visibility, s.scheduled_at AS scheduled_date, s.started_at, s.ended_at
FROM shows s WHERE s.id = $1 AND s.deleted_at IS NULL AND s.whatnot_show_id IS NOT NULL AND s.whatnot_show_id <> ''`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &show, nil
}

func (r *StreamRepository) Delete(id int64) error {
	_, err := r.DB.Exec(`
WITH updatedStream AS (UPDATE shows SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL),
breakIDs AS (SELECT id FROM show_breaks WHERE show_id = $1),
updatedBreaks AS (UPDATE show_breaks SET deleted_at = now() WHERE id IN (SELECT * FROM breakIDs) AND deleted_at IS NULL),
updatedEvents AS (UPDATE break_spots SET deleted_at = now() WHERE break_id IN (SELECT * FROM breakIDs) AND deleted_at IS NULL)
SELECT TRUE
`, id)
	return err
}

func (r *StreamRepository) SetStatus(id int64, status string) error {
	_, err := r.DB.Exec(`UPDATE shows SET status = $2 WHERE id = $1`, id, statusToDB(status))
	return err
}

func (r *StreamRepository) SetActiveBreak(id int64, breakId *int64) error {
	_, err := r.DB.Exec(`UPDATE shows SET active_break_id = $2 WHERE id = $1`, id, breakId)
	return err
}

func (r *StreamRepository) SetWhatnotShow(streamId int64, whatnotShowId *string, c *entity.WhatnotShowContent) error {
	if c == nil {
		_, err := r.DB.Exec(`UPDATE shows SET whatnot_show_id = NULL, whatnot_show_title = '', description = '', thumbnail_url = '', visibility = '' WHERE id = $1`, streamId)
		return err
	}
	_, err := r.DB.Exec(`UPDATE shows SET whatnot_show_id = $2, description = $3, thumbnail_url = $4, visibility = $5,
		scheduled_at = COALESCE($6, scheduled_at), ended_at = COALESCE($7, ended_at), whatnot_show_title = $8 WHERE id = $1`,
		streamId, whatnotShowId, c.Description, c.ThumbnailUrl, c.Visibility, c.ScheduledAt, c.EndedAt, c.Title)
	return err
}

func (r *StreamRepository) UpdateWhatnotContent(streamId int64, c *entity.WhatnotShowContent) error {
	_, err := r.DB.Exec(`UPDATE shows SET description = $2, thumbnail_url = $3, visibility = $4,
		scheduled_at = COALESCE($5, scheduled_at), whatnot_show_title = COALESCE(NULLIF($6, ''), whatnot_show_title) WHERE id = $1`,
		streamId, c.Description, c.ThumbnailUrl, c.Visibility, c.ScheduledAt, c.Title)
	return err
}

func (r *StreamRepository) SetTimes(streamId int64, startedAt *time.Time, endedAt *time.Time) error {
	_, err := r.DB.Exec(`UPDATE shows SET started_at = COALESCE($2, started_at), ended_at = COALESCE($3, ended_at) WHERE id = $1`,
		streamId, startedAt, endedAt)
	return err
}

func (r *StreamRepository) GetUsernames(id int64) ([]string, error) {
	usernames := []string{}
	err := r.DB.Select(&usernames, `
SELECT DISTINCT customer_name from break_spots
WHERE deleted_at IS NULL
GROUP BY customer_name
HAVING COUNT(*) > 10
`)

	return usernames, err
}

func (r *StreamRepository) GetStats(id int64) (*entity.StreamStatistic, error) {
	stats := entity.StreamStatistic{}
	// price is numeric: the sum is rounded to a whole number for sold_for (an int).
	err := r.DB.Get(&stats, `
		SELECT s.title as name, COUNT(DISTINCT b.id) as breaks_amount, ROUND(COALESCE(SUM(e.price), 0))::int as sold_for, COUNT(DISTINCT e.customer_name) as customers_amount, c.name as channel_name
		FROM shows s
		INNER JOIN show_breaks b on s.id = b.show_id
		INNER JOIN break_spots e on b.id = e.break_id
		INNER JOIN channels c on c.id = s.channel_id
		WHERE s.id = $1 AND e.customer_name != ''
		GROUP BY s.id, c.id
	`, id)

	if err != nil {
		return nil, err
	}

	err = r.DB.Select(&stats.BigCustomers, `
		SELECT e.customer_name
		FROM shows s
		INNER JOIN show_breaks b on s.id = b.show_id
		INNER JOIN break_spots e on b.id = e.break_id
		WHERE s.id = $1 AND e.customer_name != ''
		GROUP BY e.customer_name
		HAVING COUNT(*) > 20
	`, id)

	if err != nil {
		return nil, err
	}

	err = r.DB.Select(&stats.LuckyGoblins, `
		SELECT e.customer_name
		FROM shows s
		INNER JOIN show_breaks b on s.id = b.show_id
		INNER JOIN break_spots e on b.id = e.break_id
		WHERE s.id = $1 AND e.is_giveaway = TRUE AND e.customer_name != ''
		GROUP BY e.customer_name
		HAVING COUNT(*) > 3
	`, id)

	return &stats, err
}

func (r *StreamRepository) GetEnriched(id int64) (*entity.StreamEnriched, error) {
	enriched := entity.StreamEnriched{Stream: &entity.Stream{}}
	err := r.DB.Get(&enriched, `
		SELECT `+streamCols+`, c.name as channel_name
		FROM shows s
		INNER JOIN channels c on s.channel_id = c.id
		WHERE s.id = $1
	`, id)

	return &enriched, err
}

func (r *StreamRepository) GetPriorBuyers(channelId int64, excludeStreamId int64, before time.Time) ([]string, error) {
	buyers := []string{}
	err := r.DB.Select(&buyers, `
		SELECT DISTINCT lower(trim(e.customer_name))
		FROM break_spots e
		INNER JOIN show_breaks b ON b.id = e.break_id
		INNER JOIN shows s ON s.id = b.show_id
		WHERE s.channel_id = $1 AND s.id <> $2 AND s.created_at < $3
		  AND s.deleted_at IS NULL AND b.deleted_at IS NULL AND e.deleted_at IS NULL
		  AND e.is_giveaway IS NOT TRUE
		  AND trim(COALESCE(e.customer_name, '')) NOT IN ('', '?')
	`, channelId, excludeStreamId, before)
	return buyers, err
}
