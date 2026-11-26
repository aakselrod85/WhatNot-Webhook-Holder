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
		series_id, name, team, price, rotation, url, thumbnail,
		version, art_x, art_y, art_w, art_h, label_x, label_y, label_w, label_h, label_text
	) VALUES (
		:series_id,
		:name,
		:team,
		:price,
		:rotation,
		:url,
		:thumbnail,
		:version, :art_x, :art_y, :art_w, :art_h, :label_x, :label_y, :label_w, :label_h, :label_text
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
	_, err := r.DB.Exec(`UPDATE photo SET url = $1, version = -1,
		art_x = 0, art_y = 0, art_w = 0, art_h = 0,
		label_x = 0, label_y = 0, label_w = 0, label_h = 0, label_text = '' WHERE id = $2`, url, id)
	return err
}

func (r *PhotoRepository) UpdateBounds(id int64, version int64, art entity.Bounds, label entity.Bounds, labelText string) error {
	_, err := r.DB.Exec(`UPDATE photo SET version = $1,
		art_x = $2, art_y = $3, art_w = $4, art_h = $5,
		label_x = $6, label_y = $7, label_w = $8, label_h = $9, label_text = $10 WHERE id = $11`,
		version, art.X, art.Y, art.W, art.H, label.X, label.Y, label.W, label.H, labelText, id)
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

// GetUnsoldBySeriesId is the holder-side half of the channel board query: the caller finds the series of the
// channel's active break in mobwebapp's database first (BreakRepository.GetActiveSeriesIdByChannel).
func (r *PhotoRepository) GetUnsoldBySeriesId(seriesId int64, withSold bool) ([]*entity.Photo, error) {
	photos := []*entity.Photo{}
	err := r.DB.Select(&photos, `
SELECT p.*
FROM photo p
JOIN series s ON s.id = p.series_id
WHERE s.id = $1
  AND ($2 OR p.is_sold = false)
  AND p.is_deleted = false
  AND s.is_deleted = false
`, seriesId, withSold)
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
