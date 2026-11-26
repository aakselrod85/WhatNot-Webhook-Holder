package service

import (
	"fmt"

	"github.com/SaloEater/WhatNot-Webhook-Holder/cache"
	"github.com/SaloEater/WhatNot-Webhook-Holder/entity"
)

type SetStreamWhatnotShowRequest struct {
	StreamId      int64   `json:"stream_id"`
	WhatnotShowId *string `json:"whatnot_show_id"`
	// WhatnotShowName is the fallback title when Whatnot returns none.
	WhatnotShowName *string `json:"whatnot_show_name"`
}

type SetStreamWhatnotShowResponse struct {
	Success bool `json:"success"`
}

// SetStreamWhatnotShow binds the show to one Whatnot show of its channel (a null or empty id unbinds). Binding reads
// the Whatnot show fresh from Whatnot, so a show bound late is not stale; when that fails the bind fails.
func (s *Service) SetStreamWhatnotShow(r *SetStreamWhatnotShowRequest) (*SetStreamWhatnotShowResponse, error) {
	stream, err := s.StreamRepositorier.Get(r.StreamId)
	if err != nil {
		return nil, err
	}
	if r.WhatnotShowId != nil && *r.WhatnotShowId == "" {
		r.WhatnotShowId = nil
	}

	if r.WhatnotShowId == nil {
		err = s.StreamRepositorier.SetWhatnotShow(stream.Id, nil, nil)
	} else {
		other, gerr := s.StreamRepositorier.GetByWhatnotShowId(stream.ChannelId, *r.WhatnotShowId)
		if gerr != nil {
			return nil, gerr
		}
		if other != nil && other.Id != stream.Id {
			return nil, fmt.Errorf("whatnot show %s is already bound to stream %d", *r.WhatnotShowId, other.Id)
		}
		client, cerr := s.whatnotClientForChannel(stream.ChannelId)
		if cerr != nil {
			return nil, cerr
		}
		wn, werr := client.GetShow(*r.WhatnotShowId)
		if werr != nil {
			return nil, werr
		}
		content := whatnotContentFromAPI(wn)
		if content.Title == "" && r.WhatnotShowName != nil {
			content.Title = *r.WhatnotShowName
		}
		err = s.StreamRepositorier.SetWhatnotShow(stream.Id, r.WhatnotShowId, content)
	}
	if err != nil {
		return &SetStreamWhatnotShowResponse{}, err
	}

	// The verification was against the previous show, so it no longer holds. is_ended stays true.
	sameAsBefore := (stream.WhatnotShowId == nil && r.WhatnotShowId == nil) ||
		(stream.WhatnotShowId != nil && r.WhatnotShowId != nil && *stream.WhatnotShowId == *r.WhatnotShowId)
	if (stream.Status == entity.StreamStatusVerified || stream.Status == entity.StreamStatusFailedToVerify) && !sameAsBefore {
		if err := s.SetStreamStatus(stream, entity.StreamStatusEnded); err != nil {
			return &SetStreamWhatnotShowResponse{}, err
		}
	}
	s.StreamCache.Delete(cache.IdToKey(r.StreamId))
	// Bound: the message is created/updated once the show is live; unbound: its message is removed.
	s.syncDiscordShowAsync(stream.Id)
	return &SetStreamWhatnotShowResponse{Success: true}, nil
}
