package repository_sqlx

import (
	"fmt"

	"github.com/SaloEater/WhatNot-Webhook-Holder/entity"
	"github.com/jmoiron/sqlx"
)

// EventRepository works on mobwebapp's break_spots table (the second connection pool). A holder "event" is a spot:
// team is label, index is position, customer is customer_name. customer_id and whatnot_order_id are never set.
type EventRepository struct {
	DB *sqlx.DB
}

// eventCols maps break_spots onto entity.Event. The price is numeric and may be null.
const eventCols = `e.id, e.break_id, e."position" AS "index", e.customer_name AS customer, COALESCE(e.price, 0) AS price,
	e.label AS team, e.is_giveaway, e.note, e.quantity, (e.deleted_at IS NOT NULL) AS is_deleted, e.giveaway_type`

const eventUpdateSQL = `UPDATE break_spots SET
		break_id = :break_id,
		"position" = :index,
		customer_name = :customer,
		price = :price,
		label = :team,
		is_giveaway = :is_giveaway,
		note = :note,
		quantity = :quantity,
		giveaway_type = :giveaway_type
	WHERE id = :id`

func (r *EventRepository) GetAll(ids []int64) ([]*entity.Event, error) {
	query, args, err := sqlx.Named(`SELECT `+eventCols+` FROM break_spots e WHERE e.id IN (:ids)`, map[string]interface{}{"ids": ids})
	if err != nil {
		return nil, err
	}
	query, args, err = sqlx.In(query, args...)
	if err != nil {
		return nil, err
	}
	query = r.DB.Rebind(query)

	events := make([]*entity.Event, len(ids))
	err = r.DB.Unsafe().Select(&events, query, args...)
	return events, err
}

func (r *EventRepository) GetAllByStream(streamId int64) ([]*entity.Event, error) {
	events := []*entity.Event{}
	err := r.DB.Unsafe().Select(&events, `SELECT `+eventCols+` FROM break_spots e
INNER JOIN show_breaks b ON b.id = e.break_id
WHERE b.show_id = $1 AND b.deleted_at IS NULL AND e.deleted_at IS NULL`, streamId)
	return events, err
}

func (r *EventRepository) GetAllByBreak(breakId int64) ([]*entity.Event, error) {
	events := []*entity.Event{}
	err := r.DB.Unsafe().Select(&events, `SELECT `+eventCols+` FROM break_spots e WHERE e.break_id = $1 AND e.deleted_at IS NULL`, breakId)
	return events, err
}

func (r *EventRepository) Get(id int64) (*entity.Event, error) {
	var event entity.Event
	err := r.DB.Unsafe().Get(&event, `SELECT `+eventCols+` FROM break_spots e WHERE e.id = $1 AND e.deleted_at IS NULL`, id)
	return &event, err
}

func (r *EventRepository) Delete(id int64) error {
	_, err := r.DB.Exec(`UPDATE break_spots SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`, id)
	return err
}

func (r *EventRepository) Update(event *entity.Event) error {
	_, err := r.DB.NamedExec(eventUpdateSQL, event)
	return err
}

func (r *EventRepository) Create(event *entity.Event) (int64, error) {
	var id int64
	rows, err := r.DB.NamedQuery(`INSERT INTO break_spots (
		break_id,
		"position",
		customer_name,
		price,
		label,
		is_giveaway,
		note,
		quantity,
		giveaway_type
	) VALUES (
		:break_id,
		(SELECT COALESCE(MAX("position"), 0) + 1 FROM break_spots WHERE break_id = :break_id),
		:customer,
		:price,
		:team,
		:is_giveaway,
		:note,
		:quantity,
		:giveaway_type
	) RETURNING (id)`, event)
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

func (r *EventRepository) GetAllChildren(eventId int64) ([]*entity.Event, error) {
	events := []*entity.Event{}
	err := r.DB.Unsafe().Select(&events, `
		SELECT `+eventCols+` FROM break_spots e WHERE e.break_id IN (
		    SELECT break_id FROM break_spots WHERE id = $1
		) AND e.deleted_at IS NULL
	`, eventId)
	return events, err
}

func (r *EventRepository) UpdateAll(events []*entity.Event) error {
	tx, err := r.DB.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, event := range events {
		if _, err = tx.NamedExec(eventUpdateSQL, event); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *EventRepository) Move(id int64, newIndex int) error {
	tx, err := r.DB.Beginx()
	defer func() {
		errRb := tx.Rollback()
		if errRb != nil {
			fmt.Println("trying to cancel committed transaction")
		}
	}()
	query := `
WITH oldRecord AS (
    SELECT "position" as oldIndex, break_id as breakId FROM break_spots WHERE id = :id
) UPDATE break_spots
SET
    "position" = CASE
            WHEN :new_index > oldRecord.oldIndex THEN
                CASE
                    WHEN "position" <= :new_index AND "position" > oldRecord.oldIndex THEN "position" - 1
                    ELSE "position"
                END
            WHEN :new_index < oldRecord.oldIndex THEN
                CASE
                    WHEN "position" >= :new_index AND "position" < oldRecord.oldIndex THEN "position" + 1
                    ELSE "position"
                END
            ELSE "position"
        END
FROM oldRecord
WHERE break_id = oldRecord.breakId;
`
	args := map[string]interface{}{
		"id":        id,
		"new_index": newIndex,
	}
	_, err = tx.NamedExec(query, args)
	if err != nil {
		return err
	}

	_, err = tx.NamedExec(`UPDATE break_spots SET "position" = :new_index WHERE id = :id;`, args)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *EventRepository) GetAvailableByChannelIDAndTeam(channelID int64, team string) (*entity.Event, error) {
	var event entity.Event
	err := r.DB.Unsafe().Get(&event, `
		SELECT `+eventCols+`
		FROM channels c
		INNER JOIN shows s ON s.id = c.active_show_id
		INNER JOIN break_spots e ON e.break_id = s.active_break_id
		WHERE c.id = $1 AND e.label = $2 AND e.deleted_at IS NULL
		LIMIT 1
	`, channelID, team)
	return &event, err
}

func (r *EventRepository) GetLastIndex(breakId int64) (int, error) {
	var index int
	err := r.DB.Get(&index, `SELECT COALESCE(MAX("position"), 0) FROM break_spots WHERE break_id = $1 AND label != '' AND customer_name != ''`, breakId)
	return index, err
}

func (r *EventRepository) BeginTx() (*sqlx.Tx, error) {
	return r.DB.Beginx()
}

func (r *EventRepository) UpdateTx(tx *sqlx.Tx, event *entity.Event) error {
	_, err := tx.NamedExec(eventUpdateSQL, event)
	return err
}

func (r *EventRepository) CreateTx(tx *sqlx.Tx, event *entity.Event) (int64, error) {
	var id int64
	query, args, err := sqlx.Named(`INSERT INTO break_spots (
		break_id, "position", customer_name, price, label, is_giveaway, note, quantity, giveaway_type
	) VALUES (
		:break_id, (SELECT COALESCE(MAX("position"), 0) + 1 FROM break_spots WHERE break_id = :break_id), :customer, :price, :team, :is_giveaway, :note, :quantity, :giveaway_type
	) RETURNING id`, event)
	if err != nil {
		return 0, err
	}
	err = tx.Get(&id, tx.Rebind(query), args...)
	return id, err
}
