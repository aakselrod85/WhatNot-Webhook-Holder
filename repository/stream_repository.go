package repository

import (
	"time"

	"github.com/SaloEater/WhatNot-Webhook-Holder/entity"
)

type StreamRepositorier interface {
	GetAll() ([]*entity.Stream, error)
	Get(int64) (*entity.Stream, error)
	Delete(int64) error
	// SetStatus writes only the status (the holder's words: planned, live, ...).
	SetStatus(id int64, status string) error
	// SetActiveBreak writes only active_break_id.
	SetActiveBreak(id int64, breakId *int64) error
	GetUsernames(int64) ([]string, error)
	GetAllByChannelId(int64) ([]*entity.Stream, error)
	GetStats(int64) (*entity.StreamStatistic, error)
	GetEnriched(int64) (*entity.StreamEnriched, error)
	GetByChannelIdPaginated(channelId int64, limit int64, offset int64) ([]*entity.Stream, error)
	CountByChannelId(channelId int64) (int64, error)
	// GetPriorBuyers returns the lower-cased usernames that bought (non-giveaway) in the channel's streams created
	// before `before`, excluding stream `excludeStreamId`.
	GetPriorBuyers(channelId int64, excludeStreamId int64, before time.Time) ([]string, error)
	// GetByWhatnotShowId returns the live show of the channel bound to the Whatnot show; nil, nil when none is.
	GetByWhatnotShowId(channelId int64, whatnotShowId string) (*entity.Stream, error)
	// SetWhatnotShow binds the show to a Whatnot show (content non-nil) or unbinds it (whatnotShowId and content nil).
	SetWhatnotShow(streamId int64, whatnotShowId *string, content *entity.WhatnotShowContent) error
	// UpdateWhatnotContent overwrites the bound Whatnot show's content fields on the show.
	UpdateWhatnotContent(streamId int64, content *entity.WhatnotShowContent) error
	// GetWhatnotShow returns the show's Whatnot-bound content (title = the show's own); nil, nil when the show is
	// deleted or has no Whatnot show bound.
	GetWhatnotShow(id int64) (*entity.WhatnotShow, error)
	// SetTimes stores non-nil started_at / ended_at; nil ones keep the stored value.
	SetTimes(streamId int64, startedAt *time.Time, endedAt *time.Time) error
}
