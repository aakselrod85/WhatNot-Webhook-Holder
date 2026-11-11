package repository

import (
	"github.com/SaloEater/WhatNot-Webhook-Holder/entity"
	"github.com/jmoiron/sqlx"
)

type EventRepositorier interface {
	GetAllByBreak(int64) ([]*entity.Event, error)
	// GetAllByStream returns the non-deleted events of the stream's non-deleted breaks in one query.
	GetAllByStream(streamId int64) ([]*entity.Event, error)
	Get(int64) (*entity.Event, error)
	Delete(int64) error
	Update(*entity.Event) error
	Create(*entity.Event) (int64, error)
	GetAllChildren(int64) ([]*entity.Event, error)
	UpdateAll([]*entity.Event) error
	Move(int64, int) error
	GetAll([]int64) ([]*entity.Event, error)
	GetAvailableByChannelIDAndTeam(int64, string) (*entity.Event, error)
	GetLastIndex(int64) (int, error)
	BeginTx() (*sqlx.Tx, error)
	UpdateTx(*sqlx.Tx, *entity.Event) error
	// CreateTx inserts with the event's own Index (unlike Create, which appends at MAX(index)+1).
	CreateTx(*sqlx.Tx, *entity.Event) (int64, error)
}
