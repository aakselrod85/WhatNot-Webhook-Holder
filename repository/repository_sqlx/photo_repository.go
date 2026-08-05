package repository_sqlx

import (
	"fmt"

	"github.com/SaloEater/WhatNot-Webhook-Holder/entity"
	"github.com/jmoiron/sqlx"
)

type PhotoRepository struct {
	DB *sqlx.DB
}

func (r *PhotoRepository) Create(p *entity.Photo) (int64, error) {
	var id int64
	rows, err := r.DB.NamedQuery(`INSERT INTO photo (
		series_id, name, team, price, rotation, url, thumbnail
	) VALUES (
		:series_id,
		:name,
		:team,
		:price,
		:rotation,
		:url,
		:thumbnail
	) RETURNING (id)`, p)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	if rows.Next() {
		if err = rows.Scan(&id); err != nil {
			return 0, err
		}
	} else {
		return 0, fmt.Errorf("no rows returned after INSERT")
	}
	return id, nil
}

func (r *PhotoRepository) GetById(id int64) (*entity.Photo, error) {
	p := &entity.Photo{}
	err := r.DB.Unsafe().Get(p, `SELECT * FROM photo WHERE id = $1`, id)
	return p, err
}

func (r *PhotoRepository) UpdateUrl(id int64, url string) error {
	_, err := r.DB.Exec(`UPDATE photo SET url = $1 WHERE id = $2`, url, id)
	return err
}

func (r *PhotoRepository) UpdateRotation(id int64, rotation int64) error {
	_, err := r.DB.Exec(`UPDATE photo SET rotation = $1 WHERE id = $2`, rotation, id)
	return err
}

func (r *PhotoRepository) UpdateThumbnail(id int64, thumbnail string) error {
	_, err := r.DB.Exec(`UPDATE photo SET thumbnail = $1 WHERE id = $2`, thumbnail, id)
	return err
}

func (r *PhotoRepository) GetBySeriesId(seriesId int64) ([]*entity.Photo, error) {
	photos := []*entity.Photo{}
	err := r.DB.Unsafe().Select(&photos, `SELECT * FROM photo WHERE series_id = $1`, seriesId)
	return photos, err
}

func (r *PhotoRepository) GetUnsoldByChannelActiveStream(channelId int64, withSold bool) ([]*entity.Photo, error) {
	photos := []*entity.Photo{}
	err := r.DB.Select(&photos, `
SELECT p.*
FROM photo p
JOIN series s ON s.id = p.series_id
JOIN break b ON b.series_id = s.id AND b.is_deleted = false
JOIN stream st ON st.id = b.day_id AND st.active_break_id = b.id
JOIN channel c ON c.active_stream_id = st.id
WHERE c.id = $1
  AND ($2 OR p.is_sold = false)
  AND p.is_deleted = false
  AND s.is_deleted = false
`, channelId, withSold)
	return photos, err
}

func (r *PhotoRepository) Update(id int64, name, team string, price int64) error {
	_, err := r.DB.Exec(`UPDATE photo SET name = $1, team = $2, price = $3 WHERE id = $4`, name, team, price, id)
	return err
}

func (r *PhotoRepository) MarkSold(id int64, sold bool) error {
	_, err := r.DB.Exec(`UPDATE photo SET is_sold = $1 WHERE id = $2`, sold, id)
	return err
}

func (r *PhotoRepository) Delete(id int64) error {
	_, err := r.DB.Exec(`UPDATE photo SET is_deleted = true WHERE id = $1`, id)
	return err
}

func (r *PhotoRepository) Restore(id int64) error {
	_, err := r.DB.Exec(`UPDATE photo SET is_deleted = false WHERE id = $1`, id)
	return err
}

func (r *PhotoRepository) GetPricesBySeriesId(seriesId int64) ([]*entity.SeriesTeamTotal, error) {
	totals := []*entity.SeriesTeamTotal{}
	err := r.DB.Select(&totals, `
		SELECT team,
		       COALESCE(SUM(price) FILTER (WHERE is_sold = false), 0) AS price_left,
		       SUM(price) AS total_price
		FROM photo
		WHERE series_id = $1
		  AND is_deleted = false
		GROUP BY team
		ORDER BY team
	`, seriesId)
	return totals, err
}

func (r *PhotoRepository) CountUnsoldBySeriesId(seriesId int64) (int64, error) {
	var count int64
	err := r.DB.QueryRow(`
		SELECT COUNT(*)
		FROM photo
		WHERE series_id = $1
		  AND is_sold = false
		  AND is_deleted = false
	`, seriesId).Scan(&count)
	return count, err
}

func (r *PhotoRepository) CountSoldBySeriesId(seriesId int64) (int64, error) {
	var count int64
	err := r.DB.QueryRow(`
		SELECT COUNT(*)
		FROM photo
		WHERE series_id = $1
		  AND is_sold = true
		  AND is_deleted = false
	`, seriesId).Scan(&count)
	return count, err
}
