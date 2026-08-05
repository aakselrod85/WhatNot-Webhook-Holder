package repository_sqlx

import (
	"fmt"
	"github.com/SaloEater/WhatNot-Webhook-Holder/entity"
	"github.com/jmoiron/sqlx"
)

type StreamRepository struct {
	DB *sqlx.DB
}

func (r *StreamRepository) GetAll() ([]*entity.Stream, error) {
	days := []*entity.Stream{}
	err := r.DB.Unsafe().Select(&days, `SELECT * FROM stream WHERE is_deleted = false`)
	return days, err
}

func (r *StreamRepository) GetAllByChannelId(channelId int64) ([]*entity.Stream, error) {
	days := []*entity.Stream{}
	err := r.DB.Unsafe().Select(&days, `SELECT * FROM stream WHERE is_deleted = false AND channel_id = $1`, channelId)
	return days, err
}

func (r *StreamRepository) GetByChannelIdPaginated(channelId int64, limit int64, offset int64) ([]*entity.Stream, error) {
	days := []*entity.Stream{}
	err := r.DB.Unsafe().Select(&days, `SELECT * FROM stream WHERE is_deleted = false AND channel_id = $1 ORDER BY created_at DESC, id DESC LIMIT $2 OFFSET $3`, channelId, limit, offset)
	return days, err
}

func (r *StreamRepository) CountByChannelId(channelId int64) (int64, error) {
	var count int64
	err := r.DB.Get(&count, `SELECT COUNT(*) FROM stream WHERE is_deleted = false AND channel_id = $1`, channelId)
	if err != nil {
		return 0, fmt.Errorf("CountByChannelId: %w", err)
	}
	return count, nil
}

func (r *StreamRepository) Get(id int64) (*entity.Stream, error) {
	var day entity.Stream
	err := r.DB.Unsafe().Get(&day, `SELECT * FROM stream where id = $1 AND is_deleted = false`, id)
	return &day, err
}

func (r *StreamRepository) Delete(id int64) error {
	_, err := r.DB.Exec(`
WITH updatedStream AS (UPDATE stream SET is_deleted = TRUE WHERE id = $1),
breakIDs AS (SELECT id FROM break WHERE day_id=$1),
updatedBreaks AS (UPDATE break SET is_deleted = true WHERE id IN (SELECT * FROM breakIDs)),
updatedEvents AS (UPDATE event SET is_deleted = true WHERE break_id IN (SELECT * FROM breakIDs))
SELECT TRUE

`, id)
	return err
}

func (r *StreamRepository) Update(day *entity.Stream) error {
	_, err := r.DB.NamedExec(`UPDATE stream SET
	  	name = :name,
		created_at = :created_at,
		is_ended = :is_ended,
		active_break_id = :active_break_id
	WHERE id = :id`, day)

	return err
}

func (r *StreamRepository) Create(day *entity.Stream) (int64, error) {
	var id int64
	rows, err := r.DB.NamedQuery(`INSERT INTO stream (
		name, created_at, is_deleted, channel_id
	) VALUES (
	  	:name,
		:created_at,
		:is_deleted,
		:channel_id
	) RETURNING (id)`, day)
	if err != nil {
		return id, err
	}

	defer rows.Close()

	if rows.Next() {
		if err = rows.Scan(&id); err != nil {
			return 0, err
		}
	} else {
		return 0, fmt.Errorf("no rows returned after INSERT")
	}
	return id, err
}

func (r *StreamRepository) GetUsernames(id int64) ([]string, error) {
	usernames := []string{}
	err := r.DB.Select(&usernames, `
SELECT DISTINCT customer from event
WHERE is_deleted IS FALSE
GROUP BY customer
HAVING COUNT(*) > 10
`)

	return usernames, err
}

func (r *StreamRepository) GetStats(id int64) (*entity.StreamStatistic, error) {
	stats := entity.StreamStatistic{}
	err := r.DB.Get(&stats, `
		SELECT s.name as name, COUNT(DISTINCT b.id) as breaks_amount, SUM(e.price) as sold_for, COUNT(DISTINCT e.customer) as customers_amount, c.name as channel_name
		FROM stream s
		INNER JOIN public.break b on s.id = b.day_id
		INNER JOIN public.event e on b.id = e.break_id
		INNER JOIN public.channel c on c.id = s.channel_id
		WHERE s.id = $1 AND e.customer != ''
		GROUP BY s.id, c.id
	`, id)

	if err != nil {
		return nil, err
	}

	err = r.DB.Select(&stats.BigCustomers, `
		SELECT e.customer
		FROM stream s
		INNER JOIN public.break b on s.id = b.day_id
		INNER JOIN public.event e on b.id = e.break_id
		WHERE s.id = $1 AND e.customer != ''
		GROUP BY e.customer
		HAVING COUNT(*) > 20
	`, id)

	if err != nil {
		return nil, err
	}

	err = r.DB.Select(&stats.LuckyGoblins, `
		SELECT e.customer
		FROM stream s
		INNER JOIN public.break b on s.id = b.day_id
		INNER JOIN public.event e on b.id = e.break_id
		WHERE s.id = $1 AND e.is_giveaway = TRUE AND e.customer != ''
		GROUP BY e.customer
		HAVING COUNT(*) > 3
	`, id)

	return &stats, err
}

func (r *StreamRepository) GetEnriched(id int64) (*entity.StreamEnriched, error) {
	enriched := entity.StreamEnriched{Stream: &entity.Stream{}}
	err := r.DB.Get(&enriched, `
		SELECT s.*, c.name as channel_name
		FROM stream s
		INNER JOIN channel c on s.channel_id = c.id
		WHERE s.id = $1
	`, id)

	return &enriched, err
}
