package repository_sqlx

import (
	"fmt"
	"github.com/SaloEater/WhatNot-Webhook-Holder/entity"
	"github.com/jmoiron/sqlx"
)

type EventRepository struct {
	DB *sqlx.DB
}

func (r *EventRepository) GetAll(ids []int64) ([]*entity.Event, error) {
	query, args, err := sqlx.Named(`SELECT * FROM event WHERE id IN (:ids)`, map[string]interface{}{"ids": ids})
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
	err := r.DB.Unsafe().Select(&events, `SELECT e.* FROM event e
INNER JOIN break b ON b.id = e.break_id
WHERE b.day_id = $1 AND b.is_deleted = false AND e.is_deleted = false`, streamId)
	return events, err
}

func (r *EventRepository) GetAllByBreak(breakId int64) ([]*entity.Event, error) {
	events := []*entity.Event{}
	err := r.DB.Unsafe().Select(&events, `SELECT * FROM event WHERE break_id = $1 AND is_deleted = false`, breakId)
	return events, err
}

func (r *EventRepository) Get(id int64) (*entity.Event, error) {
	var event entity.Event
	err := r.DB.Unsafe().Get(&event, `SELECT * FROM event where id = $1 AND is_deleted = false`, id)
	return &event, err
}

func (r *EventRepository) Delete(id int64) error {
	_, err := r.DB.Exec(`UPDATE event SET is_deleted = true WHERE id = $1`, id)
	return err
}

func (r *EventRepository) Update(event *entity.Event) error {
	_, err := r.DB.NamedExec(`UPDATE event SET
		break_id = :break_id,
		index = :index,
		customer = :customer,
		price = :price,
		team = :team,
		is_giveaway = :is_giveaway,
		note = :note,
		quantity = :quantity,
		giveaway_type = :giveaway_type
	WHERE id = :id`, event)

	return err
}

func (r *EventRepository) Create(event *entity.Event) (int64, error) {
	var id int64
	rows, err := r.DB.NamedQuery(`INSERT INTO event (
		break_id,
		index,
		customer,
		price,
		team,
		is_giveaway,
		note,
		quantity,
        is_deleted,
	   	giveaway_type
	) VALUES (
		:break_id,
		(SELECT COALESCE(MAX(index), 0) + 1 FROM event WHERE break_id = :break_id),
		:customer,
		:price,
		:team,
		:is_giveaway,
		:note,
		:quantity,
		:is_deleted,
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
		SELECT * FROM event WHERE break_id IN (
		    SELECT break_id FROM event WHERE id = $1
		) AND is_deleted = false
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
		_, err = r.DB.NamedExec(`UPDATE event SET
			break_id = :break_id,
			index = :index,
			customer = :customer,
			price = :price,
			team = :team,
			is_giveaway = :is_giveaway,
			note = :note,
			quantity = :quantity,
			giveaway_type = :giveaway_type
		WHERE id = :id;`, event)
		if err != nil {
			return err
		}
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
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
    SELECT index as oldIndex, break_id as breakId FROM event WHERE id = :id
) UPDATE event
SET
    index = CASE
            WHEN :new_index > oldRecord.oldIndex THEN
                CASE
                    WHEN index <= :new_index AND index > oldRecord.oldIndex THEN index - 1
                    ELSE index
                END
            WHEN :new_index < oldRecord.oldIndex THEN
                CASE
                    WHEN index >= :new_index AND index < oldRecord.oldIndex THEN index + 1
                    ELSE index
                END
            ELSE index
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

	_, err = tx.NamedExec(`UPDATE event SET index = :new_index WHERE id = :id;`, args)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *EventRepository) GetAvailableByChannelIDAndTeam(channelID int64, team string) (*entity.Event, error) {
	var event entity.Event
	err := r.DB.Unsafe().Get(&event, `
		WITH selectedStream AS (
			SELECT active_stream_id FROM channel WHERE id = $1
		), selectedBreak AS (
			SELECT active_break_id FROM stream INNER JOIN selectedStream s ON s.active_stream_id = stream.id
		) SELECT * FROM event INNER JOIN selectedBreak b ON event.break_id = b.active_break_id WHERE team = $2 AND is_deleted = false LIMIT 1
	`, channelID, team)
	return &event, err
}

func (r *EventRepository) GetLastIndex(breakId int64) (int, error) {
	var index int
	err := r.DB.Get(&index, `SELECT COALESCE(MAX(index), 0) FROM event WHERE break_id = $1 AND team != '' AND customer != ''`, breakId)
	return index, err
}

func (r *EventRepository) BeginTx() (*sqlx.Tx, error) {
	return r.DB.Beginx()
}

func (r *EventRepository) UpdateTx(tx *sqlx.Tx, event *entity.Event) error {
	_, err := tx.NamedExec(`UPDATE event SET
		break_id = :break_id,
		index = :index,
		customer = :customer,
		price = :price,
		team = :team,
		is_giveaway = :is_giveaway,
		note = :note,
		quantity = :quantity,
		giveaway_type = :giveaway_type
	WHERE id = :id`, event)
	return err
}

func (r *EventRepository) CreateTx(tx *sqlx.Tx, event *entity.Event) (int64, error) {
	var id int64
	query, args, err := sqlx.Named(`INSERT INTO event (
		break_id, index, customer, price, team, is_giveaway, note, quantity, is_deleted, giveaway_type
	) VALUES (
		:break_id, (SELECT COALESCE(MAX(index), 0) + 1 FROM event WHERE break_id = :break_id), :customer, :price, :team, :is_giveaway, :note, :quantity, :is_deleted, :giveaway_type
	) RETURNING id`, event)
	if err != nil {
		return 0, err
	}
	err = tx.Get(&id, tx.Rebind(query), args...)
	return id, err
}
