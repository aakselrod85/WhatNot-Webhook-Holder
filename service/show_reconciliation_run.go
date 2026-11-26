package service

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/SaloEater/WhatNot-Webhook-Holder/cache"
	"github.com/SaloEater/WhatNot-Webhook-Holder/entity"
	"github.com/SaloEater/WhatNot-Webhook-Holder/reconciliation"
)

const reconciliationRunningTTL = 10 * time.Minute

type ReconReportShow struct {
	Id    string `json:"id"`
	Title string `json:"title"`
}

type ReconToAdd struct {
	Team     string  `json:"team"`
	Username string  `json:"username"`
	Price    float64 `json:"price"`
}

type ReconExtraTeam struct {
	Team     string `json:"team"`
	Customer string `json:"customer"`
	EventId  int64  `json:"event_id"`
}

type ReconBreak struct {
	WnName     string           `json:"wn_name"`
	BreakId    int64            `json:"break_id"`
	BreakName  string           `json:"break_name"`
	MatchedBy  string           `json:"matched_by"`
	Matched    int              `json:"matched"`
	ToFix      int              `json:"to_fix"`
	ToFixPrice int              `json:"to_fix_price"`
	ToAdd      []ReconToAdd     `json:"to_add"` // orders with no free event, added to the break as new events
	Extra      []ReconExtraTeam `json:"extra"`
}

type ReconUnplaced struct {
	OrderId  string `json:"order_id"`
	Title    string `json:"title"`
	Username string `json:"username"`
}

type ReconExtraGiveaway struct {
	BreakId  int64  `json:"break_id"`
	Customer string `json:"customer"`
	EventId  int64  `json:"event_id"`
}

// ReconGiveawayItem is one Whatnot giveaway order and where this run placed it.
type ReconGiveawayItem struct {
	OrderId   string `json:"order_id"`
	Title     string `json:"title"`
	Username  string `json:"username"`
	CreatedAt string `json:"created_at"`
	BreakId   int64  `json:"break_id"` // 0 when unplaced
	BreakName string `json:"break_name"`
	PlacedBy  string `json:"placed_by"` // auto | manual | none
	Status    string `json:"status"`    // matched | missing | unplaced
}

type ReconGiveaways struct {
	ToAdd    int                  `json:"to_add"`
	Items    []ReconGiveawayItem  `json:"items"`
	Unplaced []ReconUnplaced      `json:"unplaced"`
	Extra    []ReconExtraGiveaway `json:"extra"`
}

type ReconUnparsed struct {
	Title    string `json:"title"`
	Username string `json:"username"`
}

type ReconReport struct {
	Show          ReconReportShow `json:"show"`
	OrdersScanned int             `json:"orders_scanned"`
	Blocking      int             `json:"blocking"`
	Breaks        []ReconBreak    `json:"breaks"`
	Giveaways     ReconGiveaways  `json:"giveaways"`
	Unparsed      []ReconUnparsed `json:"unparsed"`
}

// reconWrite is one planned DB write, computed during the dry run.
type reconWrite struct {
	update *entity.Event // existing event with customer/price replaced
	create *entity.Event // new giveaway event
}

// RunShowReconciliation compares the Whatnot orders of the stream's show with the stream's events.
// Error handling: a failed run still leaves a finished row (status failed); it is returned together
// with the error. A dry run only stores its report on the reconciliation row: events and the stream
// status stay untouched.
func (s *Service) RunShowReconciliation(streamId int64, trigger string, dryRun bool) (*entity.ShowReconciliation, error) {
	stream, err := s.StreamRepositorier.Get(streamId)
	if err != nil {
		return nil, err
	}
	if stream.WhatnotShowId == nil {
		return nil, fmt.Errorf("stream %d has no whatnot show", streamId)
	}
	whatnotShowId := *stream.WhatnotShowId

	id, started, err := s.StartReconciliation(streamId, whatnotShowId, trigger, time.Now().Add(-reconciliationRunningTTL))
	if err != nil {
		return nil, err
	}
	if !started {
		return s.GetReconciliationByStream(streamId, whatnotShowId)
	}

	status, report, runErr := s.safeReconcile(id, stream, dryRun)
	errMsg := ""
	if runErr != nil {
		status = entity.ReconciliationStatusFailed
		errMsg = runErr.Error()
	}
	reportJSON, _ := json.Marshal(report)
	if report == nil {
		reportJSON = []byte("{}")
	}
	if err := s.FinishReconciliation(id, status, errMsg, reportJSON); err != nil {
		return nil, err
	}
	if status == entity.ReconciliationStatusApplied {
		if err := s.SetStreamStatus(stream, entity.StreamStatusVerified); err != nil {
			fmt.Println(fmt.Sprintf("reconciliation %d: set stream %d verified: %s", id, streamId, err.Error()))
		}
	}
	// A real run that ends needs_manual or failed moves an ended stream to failed_to_verify and pings once.
	// Already failed_to_verify stays (no ping); verified is never downgraded; a dry run changes nothing.
	alert := false
	if !dryRun && (status == entity.ReconciliationStatusNeedsManual || status == entity.ReconciliationStatusFailed) {
		if fresh, err := s.StreamRepositorier.Get(streamId); err != nil {
			fmt.Println(fmt.Sprintf("reconciliation %d: reload stream %d: %s", id, streamId, err.Error()))
		} else if fresh.Status == entity.StreamStatusEnded {
			if err := s.setStreamStatusNoSync(fresh, entity.StreamStatusFailedToVerify); err != nil {
				fmt.Println(fmt.Sprintf("reconciliation %d: set stream %d failed_to_verify: %s", id, streamId, err.Error()))
			} else {
				alert = true
			}
		}
	}
	// Covers needs_manual and failed; the alert variant syncs the message first, then pings, under one lock.
	s.syncDiscordShowAsyncAlert(streamId, alert)
	row, err := s.GetReconciliation(id)
	if err != nil {
		return nil, err
	}
	fmt.Println(fmt.Sprintf("reconciliation %d (stream %d, %s): %s %s", id, streamId, trigger, status, errMsg))
	return row, runErr
}

func (s *Service) safeReconcile(id int64, stream *entity.Stream, dryRun bool) (status string, report *ReconReport, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return s.reconcile(id, stream, dryRun)
}

func (s *Service) reconcile(id int64, stream *entity.Stream, dryRun bool) (string, *ReconReport, error) {
	row, err := s.GetReconciliation(id)
	if err != nil {
		return "", nil, err
	}
	var manual ReconManualMap
	if err := json.Unmarshal(row.ManualMap, &manual); err != nil {
		return "", nil, err
	}

	orders, err := s.StreamWhatnotOrders(&StreamWhatnotOrdersRequest{StreamId: stream.Id})
	if err != nil {
		return "", nil, err
	}
	ordersJSON, err := json.Marshal(orders)
	if err != nil {
		return "", nil, err
	}
	if err := s.SetReconciliationOrders(id, ordersJSON); err != nil {
		return "", nil, err
	}

	// Nothing to compare against is not a pass: it must not end as "applied" and verify the stream.
	if orders.OrdersScanned == 0 {
		return "", nil, fmt.Errorf("no orders found for show %q (status: %s), nothing to reconcile", orders.Show.Title, orders.Show.Status)
	}

	dbBreaks, err := s.BreakRepositorier.GetBreaksByStreamId(stream.Id)
	if err != nil {
		return "", nil, err
	}
	sort.SliceStable(dbBreaks, func(a, b int) bool { return dbBreaks[a].Number < dbBreaks[b].Number })
	eventsByBreak := map[int64][]*entity.Event{}
	for _, b := range dbBreaks {
		evs, err := s.EventRepositorier.GetAllByBreak(b.Id)
		if err != nil {
			return "", nil, err
		}
		eventsByBreak[b.Id] = evs
	}

	// ---- dry run ----
	report := &ReconReport{
		Show:          ReconReportShow{Id: orders.Show.Id, Title: orders.Show.Title},
		OrdersScanned: orders.OrdersScanned,
		Breaks:        make([]ReconBreak, len(orders.Breaks)),
		Giveaways:     ReconGiveaways{Items: []ReconGiveawayItem{}, Unplaced: []ReconUnplaced{}, Extra: []ReconExtraGiveaway{}},
		Unparsed:      make([]ReconUnparsed, len(orders.Unparsed)),
	}
	for i, u := range orders.Unparsed {
		report.Unparsed[i] = ReconUnparsed{Title: u.Title, Username: u.Username}
	}

	rb := make([]reconciliation.DBBreak, len(dbBreaks))
	knownBreak := map[int64]bool{}
	for i, b := range dbBreaks {
		rb[i] = reconciliation.DBBreak{Id: b.Id, Name: b.Name}
		knownBreak[b.Id] = true
	}
	wnNames := make([]string, len(orders.Breaks))
	for i, b := range orders.Breaks {
		wnNames[i] = b.Name
	}
	matches := reconciliation.MatchBreaksWithOverrides(wnNames, rb, manual.Breaks)
	wnToBreak := map[string]int64{}

	var writes []reconWrite
	touched := map[int64]bool{}
	eventById := map[int64]*entity.Event{}
	for _, evs := range eventsByBreak {
		for _, e := range evs {
			eventById[e.Id] = e
		}
	}

	for i, wb := range orders.Breaks {
		m := matches[i]
		rbk := ReconBreak{WnName: wb.Name, BreakId: m.BreakId, BreakName: m.BreakName, MatchedBy: m.By, ToAdd: []ReconToAdd{}, Extra: []ReconExtraTeam{}}
		if m.By == reconciliation.MatchedByNone {
			report.Blocking++
			report.Breaks[i] = rbk
			continue
		}
		if m.By == reconciliation.MatchedBySkip {
			report.Breaks[i] = rbk
			continue
		}
		wnToBreak[wb.Name] = m.BreakId

		ords := make([]reconciliation.Order, len(wb.Orders))
		for j, o := range wb.Orders {
			ords[j] = reconciliation.Order{OrderId: o.OrderId, Team: o.Team, Username: o.Username, Price: o.Price}
		}
		evs := make([]reconciliation.Event, 0, len(eventsByBreak[m.BreakId]))
		for _, e := range eventsByBreak[m.BreakId] {
			evs = append(evs, reconciliation.Event{Id: e.Id, Team: e.Team, Customer: e.Customer, Price: float64(e.Price), IsGiveaway: e.IsGiveaway})
		}
		res := reconciliation.CompareTeams(ords, evs)
		rbk.Matched = res.Matched
		for _, f := range res.Fixes {
			if f.Status == reconciliation.StatusPriceMismatch {
				rbk.ToFixPrice++
			} else {
				rbk.ToFix++
			}
			upd := *eventById[f.Event.Id]
			upd.Customer = f.Order.Username
			upd.Price = float32(f.Order.Price)
			writes = append(writes, reconWrite{update: &upd})
			touched[m.BreakId] = true
		}
		for _, o := range res.ToAdd {
			rbk.ToAdd = append(rbk.ToAdd, ReconToAdd{Team: o.Team, Username: o.Username, Price: o.Price})
			writes = append(writes, reconWrite{create: &entity.Event{
				BreakId: m.BreakId, Customer: o.Username, Price: float32(o.Price), Team: o.Team,
			}})
			touched[m.BreakId] = true
		}
		for _, e := range res.Extra {
			rbk.Extra = append(rbk.Extra, ReconExtraTeam{Team: e.Team, Customer: e.Customer, EventId: e.Id})
		}
		report.Breaks[i] = rbk
	}

	// giveaways
	var teamOrders []reconciliation.TeamOrder
	for _, wb := range orders.Breaks {
		for _, o := range wb.Orders {
			teamOrders = append(teamOrders, reconciliation.TeamOrder{WnBreak: wb.Name, CreatedAt: parseOrderTime(o.CreatedAt)})
		}
	}
	gws := make([]reconciliation.Giveaway, len(orders.Giveaways))
	for i, g := range orders.Giveaways {
		gws[i] = reconciliation.Giveaway{OrderId: g.OrderId, Title: g.Title, Username: g.Username, CreatedAt: parseOrderTime(g.CreatedAt)}
	}
	var others []time.Time
	for _, u := range orders.Unparsed {
		others = append(others, parseOrderTime(u.CreatedAt))
	}
	placement := reconciliation.BucketGiveaways(teamOrders, gws, others)
	placedByBreak := map[int64][]reconciliation.Giveaway{}
	breakNames := map[int64]string{}
	for _, b := range dbBreaks {
		breakNames[b.Id] = b.Name
	}
	items := make([]ReconGiveawayItem, len(gws))
	itemByOrder := map[string]*ReconGiveawayItem{}
	for i, g := range gws {
		items[i] = ReconGiveawayItem{OrderId: g.OrderId, Title: g.Title, Username: g.Username, CreatedAt: orders.Giveaways[i].CreatedAt, PlacedBy: "none", Status: "unplaced"}
		itemByOrder[g.OrderId] = &items[i]
		if over, has := manual.Giveaways[g.OrderId]; has && knownBreak[over] {
			placedByBreak[over] = append(placedByBreak[over], g)
			items[i].BreakId, items[i].BreakName, items[i].PlacedBy, items[i].Status = over, breakNames[over], "manual", "matched"
			continue
		}
		breakId, ok := wnToBreak[placement[i]]
		if placement[i] == "" || !ok {
			report.Giveaways.Unplaced = append(report.Giveaways.Unplaced, ReconUnplaced{OrderId: g.OrderId, Title: g.Title, Username: g.Username})
			report.Blocking++
			continue
		}
		placedByBreak[breakId] = append(placedByBreak[breakId], g)
		items[i].BreakId, items[i].BreakName, items[i].PlacedBy, items[i].Status = breakId, breakNames[breakId], "auto", "matched"
	}
	for _, b := range dbBreaks {
		evs := make([]reconciliation.Event, 0)
		for _, e := range eventsByBreak[b.Id] {
			if e.IsGiveaway {
				evs = append(evs, reconciliation.Event{Id: e.Id, Customer: e.Customer, IsGiveaway: true})
			}
		}
		diff := reconciliation.DiffGiveaways(placedByBreak[b.Id], evs)
		for _, e := range diff.Extra {
			report.Giveaways.Extra = append(report.Giveaways.Extra, ReconExtraGiveaway{BreakId: b.Id, Customer: e.Customer, EventId: e.Id})
		}
		for _, g := range diff.Missing {
			itemByOrder[g.OrderId].Status = "missing"
			gt := entity.GiveawayType(entity.GiveawayTypePack)
			if reconciliation.IsSlabTitle(g.Title) {
				gt = entity.GiveawayType(entity.GiveawayTypeSlab)
			}
			writes = append(writes, reconWrite{create: &entity.Event{
				BreakId: b.Id, Customer: g.Username, IsGiveaway: true, Quantity: 1, GiveawayType: gt,
			}})
			touched[b.Id] = true
			report.Giveaways.ToAdd++
		}
	}

	sort.SliceStable(items, func(a, b int) bool {
		return parseOrderTime(items[a].CreatedAt).Before(parseOrderTime(items[b].CreatedAt))
	})
	report.Giveaways.Items = items

	// ---- decide ----
	if report.Blocking > 0 {
		return entity.ReconciliationStatusNeedsManual, report, nil
	}
	if dryRun {
		return entity.ReconciliationStatusDryRun, report, nil
	}

	// ---- apply, one transaction ----
	if len(writes) > 0 {
		tx, err := s.EventRepositorier.BeginTx()
		if err != nil {
			return "", report, err
		}
		for _, w := range writes {
			if w.update != nil {
				err = s.EventRepositorier.UpdateTx(tx, w.update)
			} else {
				_, err = s.EventRepositorier.CreateTx(tx, w.create)
			}
			if err != nil {
				tx.Rollback()
				return "", report, err
			}
		}
		if err := tx.Commit(); err != nil {
			return "", report, err
		}
	}
	for breakId := range touched {
		s.EventsCache.Delete(cache.IdToKey(breakId))
	}
	return entity.ReconciliationStatusApplied, report, nil
}

func parseOrderTime(v string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, v)
	if err != nil {
		return time.Time{}
	}
	return t
}
