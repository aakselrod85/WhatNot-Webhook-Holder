package service

import (
	cacheInterface "github.com/SaloEater/WhatNot-Webhook-Holder/cache"
	"github.com/SaloEater/WhatNot-Webhook-Holder/discord"
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
	repository.SeriesPriceRangeRepositorier
	repository.PhotoRepositorier
	repository.WidgetSeriesStashorpassRepositorier
	repository.WidgetSeriesPick2Repositorier
	repository.WidgetSeriesBoxesPerBreakRepositorier
	repository.WidgetChannelCountSettingsRepositorier
	repository.WidgetBoardPriceRangeRepositorier
	repository.WidgetCardsBoardSettingsRepositorier
	repository.WidgetPresetRepositorier
	repository.LayoutConfigRepositorier
	repository.LayoutPresetRepositorier
	repository.SeriesPresetRepositorier
	repository.LayoutImageRepositorier
	repository.OverlayStateRepositorier
	repository.WhatnotWebhookEventRepositorier
	repository.ShowReconciliationRepositorier
	// Discord tables live in the holder database; the show they refer to (by id) lives in mobwebapp's.
	repository.DiscordFeedChannelRepositorier
	repository.DiscordShowMessageRepositorier
	BreakCache                   cacheInterface.Cache[*entity.Break]
	StreamCache                  cacheInterface.Cache[*entity.Stream]
	ChannelCache                 cacheInterface.Cache[*entity.Channel]
	SeriesPricesCache            cacheInterface.Cache[[]*entity.SeriesTeamTotal]
	SeriesWithCountCache         cacheInterface.Cache[*entity.SeriesWithCount]
	CardsBoardSettingsCache      cacheInterface.Cache[*entity.WidgetCardsBoardSettings]
	WidgetSeriesStashorpassCache cacheInterface.Cache[*entity.WidgetSeriesStashorpass]
	WidgetSeriesPick2Cache       cacheInterface.Cache[*entity.WidgetSeriesPick2]
	// EventsCache is keyed by break id. Only the read endpoint (GetBreakEvents) serves from this cache;
	// any service that fetches events in order to mutate them must read from the repository, or it
	// would mutate the cached slice in place.
	EventsCache cacheInterface.Cache[[]*entity.Event]
	TelegramBot *tgbotapi.BotAPI
	// Discord is never nil in main; Discord.Enabled() is false without a bot token.
	Discord *discord.Client
	// DiscordAlertUserId is the numeric Discord user pinged on verification failures; empty = no ping.
	DiscordAlertUserId string
	// FrontendUrl is the admin UI base URL (no trailing slash) for Discord links; empty = no such links.
	FrontendUrl string
	StreamShipmenter
	DigitalOceaner
}
