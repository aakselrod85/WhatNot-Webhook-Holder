package repository_sqlx

import (
	"fmt"
	"github.com/SaloEater/WhatNot-Webhook-Holder/entity"
	"github.com/jmoiron/sqlx"
)

type SeriesRepository struct {
	DB *sqlx.DB
}

func (r *SeriesRepository) Create(name string, totalCards int64, defaultPrice string) (int64, error) {
	var id int64
	err := r.DB.QueryRow(
		`INSERT INTO series (name, total_cards, default_price) VALUES ($1, $2, $3) RETURNING id`,
		name, totalCards, defaultPrice,
	).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (r *SeriesRepository) Get(id int64) (*entity.Series, error) {
	var s entity.Series
	err := r.DB.Unsafe().Get(&s, `SELECT * FROM series WHERE id = $1 AND is_deleted = false`, id)
	return &s, err
}

func (r *SeriesRepository) GetList() ([]*entity.Series, error) {
	series := []*entity.Series{}
	err := r.DB.Unsafe().Select(&series, `SELECT * FROM series WHERE is_deleted = false ORDER BY created_at DESC`)
	return series, err
}

func (r *SeriesRepository) GetListPaginated(limit int64, offset int64) ([]*entity.Series, error) {
	series := []*entity.Series{}
	err := r.DB.Unsafe().Select(&series, `SELECT * FROM series WHERE is_deleted = false ORDER BY created_at DESC, id DESC LIMIT $1 OFFSET $2`, limit, offset)
	return series, err
}

func (r *SeriesRepository) Update(id int64, name string, usedCards int64, defaultPrice string, totalCards int64) error {
	_, err := r.DB.Exec(
		`UPDATE series SET name = $1, used_cards = $2, default_price = $3, total_cards = $4 WHERE id = $5`,
		name, usedCards, defaultPrice, totalCards, id,
	)
	return err
}

func (r *SeriesRepository) Close(id int64) error {
	_, err := r.DB.Exec(`UPDATE series SET status = 'closed' WHERE id = $1`, id)
	return err
}

func (r *SeriesRepository) Delete(id int64) error {
	_, err := r.DB.Exec(`UPDATE series SET is_deleted = true WHERE id = $1`, id)
	return err
}

func (r *SeriesRepository) CountOpen() (int, error) {
	var count int
	err := r.DB.QueryRow(`SELECT COUNT(*) FROM series WHERE status = 'open' AND is_deleted = false`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("CountOpen: %w", err)
	}
	return count, nil
}

func (r *SeriesRepository) CountActive() (int64, error) {
	var count int64
	err := r.DB.QueryRow(`SELECT COUNT(*) FROM series WHERE is_deleted = false`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("CountActive: %w", err)
	}
	return count, nil
}
