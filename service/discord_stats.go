package service

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/SaloEater/WhatNot-Webhook-Holder/discord"
	"github.com/SaloEater/WhatNot-Webhook-Holder/entity"
)

// The Stats view of a show card: totals of the show's first stream plus one break at a time (◀ ▶ buttons).
// The selected break travels in the view string as "stats:<break id>"; plain "stats" = the latest break.

const discordStatsViewPrefix = entity.DiscordViewStats + ":"

// parseDiscordView splits a stored/clicked view into the base view and the selected break id (0 = none).
func parseDiscordView(view string) (string, int64) {
	if strings.HasPrefix(view, discordStatsViewPrefix) {
		id, _ := strconv.ParseInt(strings.TrimPrefix(view, discordStatsViewPrefix), 10, 64)
		return entity.DiscordViewStats, id
	}
	return view, 0
}

type buyerTotal struct {
	Name   string
	Orders int
	Money  float64
}

// statsBlock is the part shared by the total and the per-break stats.
type statsBlock struct {
	Money     float64
	Buyers    int
	New       int
	Old       int
	Slots     int
	Giveaways int
	Average   float64
	Top       []buyerTotal // at most 3
	Above     []buyerTotal // spent more than Average
}

type breakStats struct {
	Id   int64
	Name string
	statsBlock
	FromPrevious []string // ready to print; nil for the first break
}

type showStatsReport struct {
	Total        statsBlock
	Duration     time.Duration // 0 = unknown
	LuckyGoblins []string
	Breaks       []breakStats // in break order
	Selected     int          // index into Breaks; -1 when there are none
}

func customerKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func isBuyerEvent(e *entity.Event) bool {
	c := strings.TrimSpace(e.Customer)
	return !e.IsGiveaway && c != "" && c != entity.NoCustomer
}

// buildShowStats computes the Stats view for the show from its first stream. nil when the show has no stream.
func (s *Service) buildShowStats(show *entity.WhatnotShow, streams []*entity.Stream, selectedBreakId int64) (*showStatsReport, error) {
	if len(streams) == 0 {
		return nil, nil
	}
	stream := streams[0]
	breaks, err := s.BreakRepositorier.GetBreaksByStreamId(stream.Id)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(breaks, func(i, j int) bool {
		if !breaks[i].StartDate.Equal(breaks[j].StartDate) {
			return breaks[i].StartDate.Before(breaks[j].StartDate)
		}
		return breaks[i].Id < breaks[j].Id
	})
	// All events in one query, then grouped by break.
	streamEvents, err := s.EventRepositorier.GetAllByStream(stream.Id)
	if err != nil {
		return nil, err
	}
	byBreak := map[int64][]*entity.Event{}
	for _, e := range streamEvents {
		byBreak[e.BreakId] = append(byBreak[e.BreakId], e)
	}
	events := make([][]*entity.Event, len(breaks))
	for i, b := range breaks {
		events[i] = byBreak[b.Id]
	}
	prior, err := s.StreamRepositorier.GetPriorBuyers(stream.ChannelId, stream.Id, stream.CreatedAt)
	if err != nil {
		return nil, err
	}
	priorSet := make(map[string]bool, len(prior))
	for _, p := range prior {
		priorSet[p] = true
	}

	report := &showStatsReport{Selected: -1}
	var all []*entity.Event
	for _, evs := range events {
		all = append(all, evs...)
	}
	report.Total = computeStatsBlock(all, priorSet)
	report.LuckyGoblins = luckyGoblins(all)
	report.Duration = showDuration(show, breaks)

	// streak[key] = number of consecutive breaks, up to the current one, the buyer bought in.
	streak := map[string]int{}
	var prevBuyers map[string]bool
	for i, b := range breaks {
		bs := breakStats{Id: b.Id, Name: b.Name, statsBlock: computeStatsBlock(events[i], priorSet)}
		if bs.Name == "" {
			bs.Name = fmt.Sprintf("Break %d", i+1)
		}
		cur := map[string]string{} // key -> display name
		for _, e := range events[i] {
			if isBuyerEvent(e) {
				if _, ok := cur[customerKey(e.Customer)]; !ok {
					cur[customerKey(e.Customer)] = strings.TrimSpace(e.Customer)
				}
			}
		}
		newStreak := map[string]int{}
		for k := range cur {
			newStreak[k] = streak[k] + 1
		}
		if i > 0 {
			type rep struct {
				name string
				n    int
			}
			var repeat []rep
			for k, name := range cur {
				if prevBuyers[k] {
					repeat = append(repeat, rep{name, newStreak[k]})
				}
			}
			sort.Slice(repeat, func(a, c int) bool {
				if repeat[a].n != repeat[c].n {
					return repeat[a].n > repeat[c].n
				}
				return strings.ToLower(repeat[a].name) < strings.ToLower(repeat[c].name)
			})
			bs.FromPrevious = []string{}
			for _, r := range repeat {
				if r.n >= 3 {
					bs.FromPrevious = append(bs.FromPrevious, fmt.Sprintf("%s (%d breaks in a row)", r.name, r.n))
				} else {
					bs.FromPrevious = append(bs.FromPrevious, r.name)
				}
			}
		}
		streak = newStreak
		prevBuyers = map[string]bool{}
		for k := range cur {
			prevBuyers[k] = true
		}
		report.Breaks = append(report.Breaks, bs)
	}

	// Total stats is the default page; an unknown/deleted break also falls back to it.
	for i, b := range report.Breaks {
		if b.Id == selectedBreakId {
			report.Selected = i
		}
	}
	return report, nil
}

func computeStatsBlock(events []*entity.Event, prior map[string]bool) statsBlock {
	var block statsBlock
	byBuyer := map[string]*buyerTotal{}
	var order []string
	for _, e := range events {
		if e.IsGiveaway {
			if strings.TrimSpace(e.Customer) != "" {
				block.Giveaways++
			}
			continue
		}
		if !isBuyerEvent(e) {
			// A sold slot with an unknown buyer ("?") still counts as money and a slot, not as a buyer.
			if strings.TrimSpace(e.Customer) == entity.NoCustomer {
				block.Slots++
				block.Money += float64(e.Price)
			}
			continue
		}
		k := customerKey(e.Customer)
		bt, ok := byBuyer[k]
		if !ok {
			bt = &buyerTotal{Name: strings.TrimSpace(e.Customer)}
			byBuyer[k] = bt
			order = append(order, k)
		}
		bt.Orders++
		bt.Money += float64(e.Price)
		block.Slots++
		block.Money += float64(e.Price)
	}
	block.Buyers = len(byBuyer)
	if block.Buyers == 0 {
		return block
	}
	for k := range byBuyer {
		if prior[k] {
			block.Old++
		} else {
			block.New++
		}
	}
	var buyersMoney float64
	for _, bt := range byBuyer {
		buyersMoney += bt.Money
	}
	block.Average = buyersMoney / float64(block.Buyers)

	list := make([]buyerTotal, 0, len(order))
	for _, k := range order {
		list = append(list, *byBuyer[k])
	}
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].Money != list[j].Money {
			return list[i].Money > list[j].Money
		}
		return list[i].Orders > list[j].Orders
	})
	block.Top = list[:min(3, len(list))]
	for _, b := range list {
		if b.Money > block.Average+1e-9 {
			block.Above = append(block.Above, b)
		}
	}
	return block
}

// luckyGoblins are the giveaway winners with 3 or more wins in the stream, most wins first.
func luckyGoblins(events []*entity.Event) []string {
	wins := map[string]int{}
	names := map[string]string{}
	for _, e := range events {
		c := strings.TrimSpace(e.Customer)
		if !e.IsGiveaway || c == "" || c == entity.NoCustomer {
			continue
		}
		k := customerKey(c)
		wins[k]++
		if _, ok := names[k]; !ok {
			names[k] = c
		}
	}
	var keys []string
	for k, n := range wins {
		if n >= 3 {
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		if wins[keys[i]] != wins[keys[j]] {
			return wins[keys[i]] > wins[keys[j]]
		}
		return keys[i] < keys[j]
	})
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = fmt.Sprintf("%s (%d wins)", names[k], wins[k])
	}
	return out
}

// showDuration uses the Whatnot show times (until now while live), else the span of the breaks.
func showDuration(show *entity.WhatnotShow, breaks []*entity.Break) time.Duration {
	if show != nil && show.StartedAt != nil {
		end := time.Now()
		if show.EndedAt != nil {
			end = *show.EndedAt
		}
		if end.After(*show.StartedAt) {
			return end.Sub(*show.StartedAt)
		}
	}
	var first, last time.Time
	for _, b := range breaks {
		if !b.StartDate.IsZero() && (first.IsZero() || b.StartDate.Before(first)) {
			first = b.StartDate
		}
		if !b.EndDate.IsZero() && b.EndDate.After(last) {
			last = b.EndDate
		}
	}
	if !first.IsZero() && last.After(first) {
		return last.Sub(first)
	}
	return 0
}

func formatMoney(v float64) string {
	if math.Abs(v-math.Round(v)) < 0.005 {
		return fmt.Sprintf("$%d", int64(math.Round(v)))
	}
	return fmt.Sprintf("$%.2f", v)
}

func formatBuyer(b buyerTotal) string {
	word := "orders"
	if b.Orders == 1 {
		word = "order"
	}
	return fmt.Sprintf("%s (%d %s, %s)", b.Name, b.Orders, word, formatMoney(b.Money))
}

// statsSummaryFields are the short numbers as inline fields: Discord shows them in columns, 3 per row.
func statsSummaryFields(b statsBlock) []discord.EmbedField {
	return []discord.EmbedField{
		{Name: "Total money", Value: formatMoney(b.Money), Inline: true},
		{Name: "Buyers", Value: fmt.Sprintf("%d (new %d · old %d)", b.Buyers, b.New, b.Old), Inline: true},
		{Name: "Slots sold", Value: fmt.Sprint(b.Slots), Inline: true},
		{Name: "Giveaways", Value: fmt.Sprint(b.Giveaways), Inline: true},
	}
}

// headerField is a full-width section title with an invisible body.
func headerField(title string) discord.EmbedField {
	return discord.EmbedField{Name: title, Value: "​"}
}

// listFields puts the items into one field, continuing in "(cont.)" fields when the 1024 limit is reached.
func listFields(name string, items []string, sep string) []discord.EmbedField {
	if len(items) == 0 {
		return []discord.EmbedField{{Name: name, Value: "-"}}
	}
	var fields []discord.EmbedField
	var cur strings.Builder
	flush := func() {
		title := name
		if len(fields) > 0 {
			title = name + " (cont.)"
		}
		fields = append(fields, discord.EmbedField{Name: title, Value: cur.String()})
		cur.Reset()
	}
	for _, it := range items {
		it = truncateRunes(it, discordMaxFieldValue)
		if cur.Len() > 0 && len([]rune(cur.String()))+len([]rune(sep))+len([]rune(it)) > discordMaxFieldValue {
			flush()
		}
		if cur.Len() > 0 {
			cur.WriteString(sep)
		}
		cur.WriteString(it)
	}
	flush()
	return fields
}

func buyerStrings(list []buyerTotal) []string {
	out := make([]string, len(list))
	for i, b := range list {
		out[i] = formatBuyer(b)
	}
	return out
}

// renderStatsReport renders one page: Total stats (Selected = -1) or the selected break.
func renderStatsReport(r *showStatsReport) []discord.EmbedField {
	if r.Selected >= 0 && r.Selected < len(r.Breaks) {
		return renderBreakPage(r)
	}
	fields := []discord.EmbedField{headerField("🔥 Total stats 🔥")}
	fields = append(fields, statsSummaryFields(r.Total)...)
	duration := "-"
	if r.Duration > 0 {
		duration = formatDiscordDuration(r.Duration)
	}
	fields = append(fields, discord.EmbedField{Name: "Show duration", Value: duration, Inline: true})
	if r.Total.Buyers > 0 {
		fields = append(fields, listFields("Top-3 buyers", buyerStrings(r.Total.Top), "\n")...)
		fields = append(fields, listFields("Above average ("+formatMoney(r.Total.Average)+")", buyerStrings(r.Total.Above), "\n")...)
	}
	if len(r.LuckyGoblins) > 0 {
		fields = append(fields, listFields("Lucky giveaway goblins", r.LuckyGoblins, ", ")...)
	}
	return fields
}

func renderBreakPage(r *showStatsReport) []discord.EmbedField {
	b := r.Breaks[r.Selected]
	title := fmt.Sprintf("⚡ %s ⚡ (%d/%d)", b.Name, r.Selected+1, len(r.Breaks))
	fields := []discord.EmbedField{headerField(title)}
	fields = append(fields, statsSummaryFields(b.statsBlock)...)
	if b.Buyers > 0 {
		fields = append(fields, listFields("Top-3 buyers", buyerStrings(b.Top), "\n")...)
		fields = append(fields, listFields("Above average ("+formatMoney(b.Average)+")", buyerStrings(b.Above), "\n")...)
	}
	if b.FromPrevious != nil {
		fields = append(fields, listFields("Buyers from the previous break", b.FromPrevious, ", ")...)
	}
	return fields
}

// Stats pages: page 0 is Total stats, page i+1 is Breaks[i]. A page is addressed by its break id, 0 = Total.

func (r *showStatsReport) pageBreakId(page int) int64 {
	if page <= 0 || page > len(r.Breaks) {
		return 0
	}
	return r.Breaks[page-1].Id
}

func (r *showStatsReport) pageLabel(page int) string {
	if page <= 0 {
		return "Total stats"
	}
	return fmt.Sprintf("Break %d/%d", page, len(r.Breaks))
}

// discordMaxSelectOptions is Discord's limit of options in one select menu.
const discordMaxSelectOptions = 25

// renderStatsNav is the ◀ page ▶ row plus the page select menu; nil when there are no breaks to page to.
func renderStatsNav(r *showStatsReport, prefix string, showId int64) []discord.ActionRow {
	if r == nil || len(r.Breaks) == 0 {
		return nil
	}
	pages := len(r.Breaks) + 1
	cur := r.Selected + 1
	prev, next := max(cur-1, 0), min(cur+1, pages-1)
	arrows := discord.ActionRow{Type: discord.ComponentTypeActionRow, Components: []discord.Component{
		{Type: discord.ComponentTypeButton, Style: discord.ButtonStyleSecondary, Label: "◀",
			CustomId: fmt.Sprintf("%s:brk:%d:%d:p", prefix, r.pageBreakId(prev), showId), Disabled: cur == 0},
		{Type: discord.ComponentTypeButton, Style: discord.ButtonStyleSecondary, Label: r.pageLabel(cur),
			CustomId: fmt.Sprintf("%s:noop:%d", prefix, showId), Disabled: true},
		{Type: discord.ComponentTypeButton, Style: discord.ButtonStyleSecondary, Label: "▶",
			CustomId: fmt.Sprintf("%s:brk:%d:%d:n", prefix, r.pageBreakId(next), showId), Disabled: cur == pages-1},
	}}

	// Total stats + up to 24 breaks around the current one.
	window := discordMaxSelectOptions - 1
	from := 0
	if len(r.Breaks) > window {
		from = min(max(r.Selected-window/2, 0), len(r.Breaks)-window)
	}
	to := min(from+window, len(r.Breaks))
	options := []discord.SelectOption{{Label: "🔥 Total stats", Value: "0", Default: cur == 0}}
	for i := from; i < to; i++ {
		b := r.Breaks[i]
		options = append(options, discord.SelectOption{
			Label:       truncateRunes(b.Name, 100),
			Value:       strconv.FormatInt(b.Id, 10),
			Description: fmt.Sprintf("Break %d of %d", i+1, len(r.Breaks)),
			Default:     i == r.Selected,
		})
	}
	picker := discord.ActionRow{Type: discord.ComponentTypeActionRow, Components: []discord.Component{{
		Type:        discord.ComponentTypeStringSelect,
		CustomId:    fmt.Sprintf("%s:pick:%d", prefix, showId),
		Placeholder: "Choose Total stats or a break",
		Options:     options,
	}}}
	return []discord.ActionRow{arrows, picker}
}

// parseBreakNavId parses an arrow click "<prefix>:brk:<break id>:<show id>:<p|n>" or a select menu pick
// "<prefix>:pick:<show id>" with the chosen break id in values (0 = Total stats); ok=false for anything else.
func parseBreakNavId(customId string, values []string, prefix string) (breakId, showId int64, ok bool) {
	parts := strings.Split(customId, ":")
	if len(parts) < 3 || parts[0] != prefix {
		return 0, 0, false
	}
	var b, sh int64
	var err1, err2 error
	switch {
	case parts[1] == "brk" && len(parts) == 5:
		b, err1 = strconv.ParseInt(parts[2], 10, 64)
		sh, err2 = strconv.ParseInt(parts[3], 10, 64)
	case parts[1] == "pick" && len(parts) == 3 && len(values) == 1:
		b, err1 = strconv.ParseInt(values[0], 10, 64)
		sh, err2 = strconv.ParseInt(parts[2], 10, 64)
	default:
		return 0, 0, false
	}
	if err1 != nil || err2 != nil || b < 0 {
		return 0, 0, false
	}
	return b, sh, true
}

// parseViewButton reads a "<prefix>:view:<view>:<show id>" click, or an arrow/select pick (see parseBreakNavId), into
// the view to store/render ("stats:<break id>" for a Stats page, "stats:0" = Total stats) and the whatnot_show id.
func parseViewButton(customId string, values []string, prefix string) (view string, showId int64, ok bool) {
	if breakId, sh, isNav := parseBreakNavId(customId, values, prefix); isNav {
		return fmt.Sprintf("%s%d", discordStatsViewPrefix, breakId), sh, true
	}
	parts := strings.Split(customId, ":")
	if len(parts) != 4 || parts[0] != prefix || parts[1] != "view" {
		return "", 0, false
	}
	switch parts[2] {
	case entity.DiscordViewOverview, entity.DiscordViewStats, entity.DiscordViewReconciliation:
	default:
		return "", 0, false
	}
	id, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return "", 0, false
	}
	return parts[2], id, true
}
