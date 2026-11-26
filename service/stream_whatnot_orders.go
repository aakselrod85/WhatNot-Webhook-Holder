package service

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
)

type StreamWhatnotOrdersRequest struct {
	StreamId int64 `json:"stream_id"`
	// ReconciliationId, when set, returns that run's stored snapshot instead of calling Whatnot.
	ReconciliationId int64 `json:"reconciliation_id"`
}

type WhatnotBreakOrder struct {
	OrderId   string  `json:"order_id"`
	Team      string  `json:"team"`
	Username  string  `json:"username"`
	Price     float64 `json:"price"`
	CreatedAt string  `json:"created_at"`
}

type WhatnotBreakOrders struct {
	Name   string              `json:"name"`
	Orders []WhatnotBreakOrder `json:"orders"`
}

type WhatnotGiveawayOrder struct {
	OrderId   string `json:"order_id"`
	Title     string `json:"title"`
	Username  string `json:"username"`
	CreatedAt string `json:"created_at"`
}

type WhatnotUnparsedOrder struct {
	OrderId   string  `json:"order_id"`
	Title     string  `json:"title"`
	Username  string  `json:"username"`
	Price     float64 `json:"price"`
	CreatedAt string  `json:"created_at"`
}

type StreamWhatnotOrdersResponse struct {
	Show          WhatnotShow            `json:"show"`
	OrdersScanned int                    `json:"orders_scanned"`
	Breaks        []WhatnotBreakOrders   `json:"breaks"`
	Giveaways     []WhatnotGiveawayOrder `json:"giveaways"`
	Unparsed      []WhatnotUnparsedOrder `json:"unparsed"`
}

type ordersBreakGroup struct {
	name   string
	first  time.Time
	orders []WhatnotBreakOrder
	times  []time.Time
}

func (s *Service) StreamWhatnotOrders(r *StreamWhatnotOrdersRequest) (*StreamWhatnotOrdersResponse, error) {
	if r.ReconciliationId != 0 {
		row, err := s.GetReconciliation(r.ReconciliationId)
		if err != nil {
			return nil, err
		}
		if row.StreamId != r.StreamId {
			return nil, errors.New("reconciliation belongs to another stream")
		}
		stream, err := s.StreamRepositorier.Get(r.StreamId)
		if err != nil {
			return nil, err
		}
		if stream.WhatnotShowId == nil || *stream.WhatnotShowId != row.ShowId {
			return nil, errors.New("reconciliation belongs to a different show than the one linked to this stream")
		}
		snapshot := &StreamWhatnotOrdersResponse{}
		if err := json.Unmarshal(row.Orders, snapshot); err != nil {
			return nil, err
		}
		if snapshot.Breaks == nil {
			snapshot.Breaks = []WhatnotBreakOrders{}
		}
		if snapshot.Giveaways == nil {
			snapshot.Giveaways = []WhatnotGiveawayOrder{}
		}
		if snapshot.Unparsed == nil {
			snapshot.Unparsed = []WhatnotUnparsedOrder{}
		}
		return snapshot, nil
	}
	stream, err := s.StreamRepositorier.Get(r.StreamId)
	if err != nil {
		return nil, err
	}
	if stream.WhatnotShowId == nil {
		return nil, errors.New("stream has no whatnot show")
	}
	client, err := s.whatnotClientForChannel(stream.ChannelId)
	if err != nil {
		return nil, err
	}
	show, err := client.GetShow(*stream.WhatnotShowId)
	if err != nil {
		return nil, err
	}
	notBefore := stream.CreatedAt.Add(-24 * time.Hour)
	if show.ScheduledStartAt != nil {
		notBefore = show.ScheduledStartAt.Add(-3 * time.Hour)
	}
	orders, err := client.ListOrdersForShow(show.Id, notBefore)
	if err != nil {
		return nil, err
	}

	groups := map[string]*ordersBreakGroup{}
	var ordered []*ordersBreakGroup
	giveaways := []WhatnotGiveawayOrder{}
	unparsed := []WhatnotUnparsedOrder{}
	scanned := 0

	for _, o := range orders {
		if o.Status == "cancelled" || o.Status == "failed" {
			continue
		}
		scanned++
		items, err := client.ListOrderItems(o.Id)
		if err != nil {
			return nil, err
		}
		createdAt := o.CreatedAt.UTC().Format(time.RFC3339Nano)
		if o.TransactionType == "giveaway" {
			title := ""
			if len(items) > 0 {
				title = items[0].Title
			}
			giveaways = append(giveaways, WhatnotGiveawayOrder{OrderId: o.Id, Title: title, Username: o.Username, CreatedAt: createdAt})
			continue
		}
		for _, it := range items {
			price := it.UnitPrice * float64(it.Quantity)
			idx := strings.LastIndex(it.Title, " - ")
			if idx < 0 {
				unparsed = append(unparsed, WhatnotUnparsedOrder{OrderId: o.Id, Title: it.Title, Username: o.Username, Price: price, CreatedAt: createdAt})
				continue
			}
			name := it.Title[:idx]
			g := groups[name]
			if g == nil {
				g = &ordersBreakGroup{name: name, first: o.CreatedAt}
				groups[name] = g
				ordered = append(ordered, g)
			}
			if o.CreatedAt.Before(g.first) {
				g.first = o.CreatedAt
			}
			g.orders = append(g.orders, WhatnotBreakOrder{
				OrderId:   o.Id,
				Team:      it.Title[idx+len(" - "):],
				Username:  o.Username,
				Price:     price,
				CreatedAt: createdAt,
			})
			g.times = append(g.times, o.CreatedAt)
		}
	}

	for _, g := range ordered {
		idx := make([]int, len(g.orders))
		for i := range idx {
			idx[i] = i
		}
		sort.SliceStable(idx, func(a, b int) bool { return g.times[idx[a]].Before(g.times[idx[b]]) })
		sorted := make([]WhatnotBreakOrder, len(idx))
		for i, j := range idx {
			sorted[i] = g.orders[j]
		}
		g.orders = sorted
	}
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].first.Before(ordered[j].first) })

	breaks := make([]WhatnotBreakOrders, len(ordered))
	for i, g := range ordered {
		breaks[i] = WhatnotBreakOrders{Name: g.name, Orders: g.orders}
	}

	return &StreamWhatnotOrdersResponse{
		Show:          WhatnotShow{Id: show.Id, Title: show.Title, Status: show.Status, ScheduledStartAt: show.ScheduledStartAt, EndedAt: show.EndedAt},
		OrdersScanned: scanned,
		Breaks:        breaks,
		Giveaways:     giveaways,
		Unparsed:      unparsed,
	}, nil
}
