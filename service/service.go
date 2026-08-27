package service

import (
	cacheInterface "github.com/SaloEater/WhatNot-Webhook-Holder/cache"
	"github.com/SaloEater/WhatNot-Webhook-Holder/entity"
	"github.com/SaloEater/WhatNot-Webhook-Holder/repository"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Service struct {
	repository.BreakRepositorier
	repository.StreamRepositorier
	repository.EventRepositorier
	repository.ChannelRepositorier
	repository.TGChatRepositorier
	repository.BundleRepositorier
	repository.BoxTypeRepositorier
	repository.LocationRepositorier
	repository.BundleBoxesRepositorier
	repository.BoxRepositorier
	repository.TrackingRepositorier
	repository.BundleLabelRepositorier
	repository.SeriesRepositorier
	repository.PhotoRepositorier
	repository.WidgetSeriesStashorpassRepositorier
	repository.WidgetSeriesPick2Repositorier
	repository.WidgetSeriesBoxesPerBreakRepositorier
	repository.WidgetChannelCountSettingsRepositorier
	repository.WidgetBoardPriceRangeRepositorier
	repository.WidgetCardsBoardSettingsRepositorier
	repository.WidgetPresetRepositorier
	repository.LayoutConfigRepositorier
	repository.OverlayStateRepositorier
	BreakCache                   cacheInterface.Cache[*entity.Break]
	StreamCache                  cacheInterface.Cache[*entity.Stream]
	ChannelCache                 cacheInterface.Cache[*entity.Channel]
	SeriesPricesCache            cacheInterface.Cache[[]*entity.SeriesTeamTotal]
	SeriesWithCountCache         cacheInterface.Cache[*entity.SeriesWithCount]
	CardsBoardSettingsCache      cacheInterface.Cache[*entity.WidgetCardsBoardSettings]
	WidgetSeriesStashorpassCache cacheInterface.Cache[*entity.WidgetSeriesStashorpass]
	WidgetSeriesPick2Cache       cacheInterface.Cache[*entity.WidgetSeriesPick2]
	TelegramBot                  *tgbotapi.BotAPI
	StreamShipmenter
	DigitalOceaner
}
