package service

func (s *Service) CacheClear() error {
	s.BreakCache.Clear()
	s.StreamCache.Clear()
	s.ChannelCache.Clear()
	s.CardsBoardSettingsCache.Clear()
	s.WidgetSeriesStashorpassCache.Clear()
	s.WidgetSeriesPick2Cache.Clear()
	return nil
}
