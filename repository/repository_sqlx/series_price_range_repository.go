package repository_sqlx

import (
	"github.com/SaloEater/WhatNot-Webhook-Holder/entity"
	"github.com/jmoiron/sqlx"
)

type SeriesPriceRangeRepository struct {
	DB *sqlx.DB
}

func (r *SeriesPriceRangeRepository) ListBySeries(seriesId int64) ([]*entity.SeriesPriceRange, error) {
	rows := []*entity.SeriesPriceRange{}
	err := r.DB.Unsafe().Select(&rows, `SELECT * FROM series_price_ranges WHERE series_id = $1 ORDER BY price_from ASC, id ASC`, seriesId)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *SeriesPriceRangeRepository) Create(w *entity.SeriesPriceRange) (int64, error) {
	var id int64
	err := r.DB.QueryRow(
		`INSERT INTO series_price_ranges (series_id, price_from, price_to, count) VALUES ($1, $2, $3, $4) RETURNING id`,
		w.SeriesId, w.PriceFrom, w.PriceTo, w.Count,
	).Scan(&id)
	return id, err
}

func (r *SeriesPriceRangeRepository) Update(w *entity.SeriesPriceRange) error {
	_, err := r.DB.Exec(
		`UPDATE series_price_ranges SET price_from = $1, price_to = $2, count = $3 WHERE id = $4`,
		w.PriceFrom, w.PriceTo, w.Count, w.Id,
	)
	return err
}

func (r *SeriesPriceRangeRepository) Delete(id int64) error {
	_, err := r.DB.Exec(`DELETE FROM series_price_ranges WHERE id=$1`, id)
	return err
}
