package main

import (
	"fmt"
	"github.com/SaloEater/WhatNot-Webhook-Holder/api"
	"github.com/SaloEater/WhatNot-Webhook-Holder/api/webhook"
	go_cache "github.com/SaloEater/WhatNot-Webhook-Holder/cache/go-cache"
	"github.com/SaloEater/WhatNot-Webhook-Holder/clickup"
	"github.com/SaloEater/WhatNot-Webhook-Holder/digital_ocean"
	"github.com/SaloEater/WhatNot-Webhook-Holder/entity"
	"github.com/SaloEater/WhatNot-Webhook-Holder/repository/repository_sqlx"
	"github.com/SaloEater/WhatNot-Webhook-Holder/service"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

// CORS middleware handler
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow requests from any origin
		w.Header().Set("Access-Control-Allow-Origin", "*")
		// Allow GET, POST, OPTIONS methods
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST")
		// Allow Content-Type header
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			// Preflight request, respond with success
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	err := godotenv.Load(".env.local")
	if err != nil {
		fmt.Println(err)
	}

	routeBuilder := api.RouteBuilder{
		Username: os.Getenv("Username"),
		Password: os.Getenv("Password"),
	}

	dbDSN := os.Getenv("db_dsn")

	db, err := sqlx.Connect("postgres", dbDSN)
	if err != nil {
		log.Fatalln(err)
	}
	// Unsafe: ignore result columns with no struct field, so migrations can be applied before the binary that maps them is deployed.
	db = db.Unsafe()

	service.InitFile()
	driver, err := postgres.WithInstance(db.DB, &postgres.Config{})
	m, err := migrate.NewWithDatabaseInstance(
		"file://db/migrations",
		"defaultdb",
		driver,
	)
	if err != nil {
		fmt.Println(err)
		return
	}
	err = m.Up()
	if err != migrate.ErrNoChange && err != nil {
		fmt.Println(err)
		return
	}

	breakCache := go_cache.CreateCache[*entity.Break](10 * time.Hour)
	streamCache := go_cache.CreateCache[*entity.Stream](10 * time.Hour)
	channelCache := go_cache.CreateCache[*entity.Channel](10 * time.Hour)
	seriesPricesCache := go_cache.CreateCache[[]*entity.SeriesTeamTotal](10 * time.Hour)
	seriesWithCountCache := go_cache.CreateCache[*entity.SeriesWithCount](10 * time.Hour)
	cardsBoardSettingsCache := go_cache.CreateCache[*entity.WidgetCardsBoardSettings](10 * time.Hour)
	widgetSeriesStashorpassCache := go_cache.CreateCache[*entity.WidgetSeriesStashorpass](10 * time.Hour)
	widgetSeriesPick2Cache := go_cache.CreateCache[*entity.WidgetSeriesPick2](10 * time.Hour)

	bot, err := tgbotapi.NewBotAPI(os.Getenv("mob_telegram_token"))
	if err != nil {
		log.Panic(err)
	}

	svc := &service.Service{
		BreakRepositorier:                      &repository_sqlx.BreakRepository{DB: db},
		StreamRepositorier:                     &repository_sqlx.StreamRepository{DB: db},
		EventRepositorier:                      &repository_sqlx.EventRepository{DB: db},
		ChannelRepositorier:                    &repository_sqlx.ChannelRepository{DB: db},
		TGChatRepositorier:                     &repository_sqlx.TGChatRepository{DB: db},
		BoxRepositorier:                        &repository_sqlx.BoxRepository{DB: db},
		BoxTypeRepositorier:                    &repository_sqlx.BoxTypeRepository{DB: db},
		BundleBoxesRepositorier:                &repository_sqlx.BundleBoxesRepository{DB: db},
		BundleLabelRepositorier:                &repository_sqlx.BundleLabelRepository{DB: db},
		BundleRepositorier:                     &repository_sqlx.BundleRepository{DB: db},
		SeriesRepositorier:                     &repository_sqlx.SeriesRepository{DB: db},
		PhotoRepositorier:                      &repository_sqlx.PhotoRepository{DB: db},
		LocationRepositorier:                   &repository_sqlx.LocationRepository{DB: db},
		TrackingRepositorier:                   &repository_sqlx.TrackingRepository{DB: db},
		WidgetSeriesStashorpassRepositorier:    &repository_sqlx.WidgetSeriesStashorpassRepository{DB: db},
		WidgetSeriesPick2Repositorier:          &repository_sqlx.WidgetSeriesPick2Repository{DB: db},
		WidgetSeriesBoxesPerBreakRepositorier:  &repository_sqlx.WidgetSeriesBoxesPerBreakRepository{DB: db},
		WidgetChannelCountSettingsRepositorier: &repository_sqlx.WidgetChannelCountSettingsRepository{DB: db},
		WidgetBoardPriceRangeRepositorier:      &repository_sqlx.WidgetBoardPriceRangeRepository{DB: db},
		SeriesPriceRangeRepositorier:           &repository_sqlx.SeriesPriceRangeRepository{DB: db},
		WidgetCardsBoardSettingsRepositorier:   &repository_sqlx.WidgetCardsBoardSettingsRepository{DB: db},
		WidgetPresetRepositorier:               &repository_sqlx.WidgetPresetRepository{DB: db},
		LayoutConfigRepositorier:               &repository_sqlx.LayoutConfigRepository{DB: db},
		LayoutPresetRepositorier:               &repository_sqlx.LayoutPresetRepository{DB: db},
		LayoutImageRepositorier:                &repository_sqlx.LayoutImageRepository{DB: db},
		OverlayStateRepositorier:               &repository_sqlx.OverlayStateRepository{DB: db},
		BreakCache:                             &breakCache,
		StreamCache:                            &streamCache,
		ChannelCache:                           &channelCache,
		SeriesPricesCache:                      &seriesPricesCache,
		SeriesWithCountCache:                   &seriesWithCountCache,
		CardsBoardSettingsCache:                &cardsBoardSettingsCache,
		WidgetSeriesStashorpassCache:           &widgetSeriesStashorpassCache,
		WidgetSeriesPick2Cache:                 &widgetSeriesPick2Cache,
		TelegramBot:                            bot,
		StreamShipmenter:                       clickup.Init(os.Getenv("clickup_api_key"), db),
		DigitalOceaner:                         digital_ocean.InitDigitalOcean(os.Getenv("spaces_key"), os.Getenv("spaces_secret"), os.Getenv("spaces_endpoint"), os.Getenv("spaces_region"), os.Getenv("spaces_url")),
	}
	go func() {
		fmt.Println("Starting telegram bot updates")
		//svc.RunTelegramBotUpdates(bot)
	}()

	apiO := api.API{Service: svc}

	handler := corsMiddleware(http.DefaultServeMux)

	http.HandleFunc("/webhook/product_sold", routeBuilder.WrapRoute(webhook.ProductSold, api.HttpPost, true))

	http.HandleFunc("/api/channel", routeBuilder.WrapRoute(apiO.GetChannel, api.HttpPost, true))
	http.HandleFunc("/api/channels", routeBuilder.WrapRoute(apiO.GetChannels, api.HttpGet, true))
	http.HandleFunc("/api/channel/add", routeBuilder.WrapRoute(apiO.AddChannel, api.HttpPost, true))
	http.HandleFunc("/api/channel/delete", routeBuilder.WrapRoute(apiO.DeleteChannel, api.HttpPost, true))
	http.HandleFunc("/api/channel/update", routeBuilder.WrapRoute(apiO.UpdateChannel, api.HttpPost, true))
	http.HandleFunc("/api/channel/by_stream", routeBuilder.WrapRoute(apiO.GetChannelByStream, api.HttpPost, true))

	http.HandleFunc("/api/channel/streams", routeBuilder.WrapRoute(apiO.GetChannelStreams, api.HttpPost, true))
	http.HandleFunc("/api/stream", routeBuilder.WrapRoute(apiO.GetDay, api.HttpPost, true))
	http.HandleFunc("/api/stream/add", routeBuilder.WrapRoute(apiO.AddStream, api.HttpPost, true))
	http.HandleFunc("/api/stream/usernames", routeBuilder.WrapRoute(apiO.GetUsernames, api.HttpPost, true))
	http.HandleFunc("/api/stream/delete", routeBuilder.WrapRoute(apiO.DeleteStream, api.HttpPost, true))

	http.HandleFunc("/api/channel/set_active_stream", routeBuilder.WrapRoute(apiO.SetActiveStream, api.HttpPost, true))
	http.HandleFunc("/api/stream/set_active_break", routeBuilder.WrapRoute(apiO.SetActiveBreak, api.HttpPost, true))

	http.HandleFunc("/api/stream/breaks", routeBuilder.WrapRoute(apiO.GetStreamBreaks, api.HttpPost, true))
	http.HandleFunc("/api/break", routeBuilder.WrapRoute(apiO.GetBreak, api.HttpPost, true))
	http.HandleFunc("/api/break/add", routeBuilder.WrapRoute(apiO.AddBreak, api.HttpPost, true))
	http.HandleFunc("/api/break/delete", routeBuilder.WrapRoute(apiO.DeleteBreak, api.HttpPost, true))
	http.HandleFunc("/api/break/update", routeBuilder.WrapRoute(apiO.UpdateBreak, api.HttpPost, true))
	http.HandleFunc("/api/break/events", routeBuilder.WrapRoute(apiO.GetBreakEvents, api.HttpPost, true))

	http.HandleFunc("/api/event/add", routeBuilder.WrapRoute(apiO.AddEvent, api.HttpPost, true))
	http.HandleFunc("/api/event/update", routeBuilder.WrapRoute(apiO.UpdateEvent, api.HttpPost, true))
	http.HandleFunc("/api/event/update_all", routeBuilder.WrapRoute(apiO.UpdateAllEvents, api.HttpPost, true))
	http.HandleFunc("/api/event/move", routeBuilder.WrapRoute(apiO.MoveEvent, api.HttpPost, true))
	http.HandleFunc("/api/event/delete", routeBuilder.WrapRoute(apiO.DeleteEvent, api.HttpPost, true))
	http.HandleFunc("/api/event/activate_team", routeBuilder.WrapRoute(apiO.ActivateTeamEvent, api.HttpPost, true))

	http.HandleFunc("/api/cache/clear", routeBuilder.WrapRoute(apiO.CacheClear, api.HttpPost, true))
	http.HandleFunc("/api/notification/stream_ended", routeBuilder.WrapRoute(apiO.EventStreamEnded, api.HttpPost, true))
	http.HandleFunc("/api/notification/stream_packaging_finished", routeBuilder.WrapRoute(apiO.EventStreamPackagingFinished, api.HttpPost, true))

	http.HandleFunc("/api/box_type", routeBuilder.WrapRoute(apiO.BoxTypeGet, api.HttpPost, true))
	http.HandleFunc("/api/box_type/get_list", routeBuilder.WrapRoute(apiO.BoxTypeGetList, api.HttpPost, true))
	http.HandleFunc("/api/box_type/update", routeBuilder.WrapRoute(apiO.BoxTypeUpdate, api.HttpPost, true))
	http.HandleFunc("/api/box_type/create", routeBuilder.WrapRoute(apiO.BoxTypeCreate, api.HttpPost, true))

	http.HandleFunc("/api/box/update", routeBuilder.WrapRoute(apiO.BoxUpdate, api.HttpPost, true))

	http.HandleFunc("/api/boxes/create", routeBuilder.WrapRoute(apiO.BoxesCreate, api.HttpPost, true))
	http.HandleFunc("/api/boxes/delete", routeBuilder.WrapRoute(apiO.BoxesDelete, api.HttpPost, true))
	http.HandleFunc("/api/boxes/get_by_bundle", routeBuilder.WrapRoute(apiO.BoxesGetByBundle, api.HttpPost, true))
	http.HandleFunc("/api/boxes/update", routeBuilder.WrapRoute(apiO.BoxesUpdate, api.HttpPost, true))

	http.HandleFunc("/api/bundle/create", routeBuilder.WrapRoute(apiO.BundleCreate, api.HttpPost, true))
	http.HandleFunc("/api/bundle/delete", routeBuilder.WrapRoute(apiO.BundleDelete, api.HttpPost, true))
	http.HandleFunc("/api/bundle", routeBuilder.WrapRoute(apiO.BundleGet, api.HttpPost, true))
	http.HandleFunc("/api/bundle/get_list", routeBuilder.WrapRoute(apiO.BundleGetList, api.HttpPost, true))
	http.HandleFunc("/api/bundle/to_next_status", routeBuilder.WrapRoute(apiO.BundleToNextStatus, api.HttpPost, true))
	http.HandleFunc("/api/bundle/to_previous_status", routeBuilder.WrapRoute(apiO.BundleToPreviousStatus, api.HttpPost, true))
	http.HandleFunc("/api/bundle/update", routeBuilder.WrapRoute(apiO.BundleUpdate, api.HttpPost, true))

	http.HandleFunc("/api/location/get_list", routeBuilder.WrapRoute(apiO.LocationGetList, api.HttpPost, true))

	http.HandleFunc("/api/series/create", routeBuilder.WrapRoute(apiO.SeriesCreate, api.HttpPost, true))
	http.HandleFunc("/api/series/get", routeBuilder.WrapRoute(apiO.SeriesGet, api.HttpPost, true))
	http.HandleFunc("/api/series/list", routeBuilder.WrapRoute(apiO.SeriesGetList, api.HttpGet, true))
	http.HandleFunc("/api/series/update", routeBuilder.WrapRoute(apiO.SeriesUpdate, api.HttpPost, true))
	http.HandleFunc("/api/series/close", routeBuilder.WrapRoute(apiO.SeriesClose, api.HttpPost, true))
	http.HandleFunc("/api/series/delete", routeBuilder.WrapRoute(apiO.SeriesDelete, api.HttpPost, true))
	http.HandleFunc("/api/series/get_with_count", routeBuilder.WrapRoute(apiO.SeriesGetWithCount, api.HttpPost, true))
	http.HandleFunc("/api/series/list_paginated", routeBuilder.WrapRoute(apiO.SeriesGetListPaginated, api.HttpPost, true))
	http.HandleFunc("/api/series/price_ranges", routeBuilder.WrapRoute(apiO.SeriesPriceRangeList, api.HttpPost, true))
	http.HandleFunc("/api/series/price_ranges/create", routeBuilder.WrapRoute(apiO.SeriesPriceRangeCreate, api.HttpPost, true))
	http.HandleFunc("/api/series/price_ranges/update", routeBuilder.WrapRoute(apiO.SeriesPriceRangeUpdate, api.HttpPost, true))
	http.HandleFunc("/api/series/price_ranges/delete", routeBuilder.WrapRoute(apiO.SeriesPriceRangeDelete, api.HttpPost, true))

	http.HandleFunc("/api/photo/upload", routeBuilder.WrapRoute(apiO.PhotoUpload, api.HttpPost, true))
	http.HandleFunc("/api/photo/list", routeBuilder.WrapRoute(apiO.PhotoGetBySeries, api.HttpPost, true))
	http.HandleFunc("/api/photo/delete", routeBuilder.WrapRoute(apiO.PhotoDelete, api.HttpPost, true))
	http.HandleFunc("/api/photo/update", routeBuilder.WrapRoute(apiO.PhotoUpdate, api.HttpPost, true))
	http.HandleFunc("/api/photo/mark_sold", routeBuilder.WrapRoute(apiO.PhotoMarkSold, api.HttpPost, true))
	http.HandleFunc("/api/photo/restore", routeBuilder.WrapRoute(apiO.PhotoRestore, api.HttpPost, true))
	http.HandleFunc("/api/photo/board", routeBuilder.WrapRoute(apiO.PhotoGetForBoard, api.HttpPost, true))
	http.HandleFunc("/api/photo/rotate", routeBuilder.WrapRoute(apiO.PhotoRotate, api.HttpPost, true))
	http.HandleFunc("/api/photo/thumbnail", routeBuilder.WrapRoute(apiO.PhotoThumbnail, api.HttpPost, true))
	http.HandleFunc("/api/photo/update-image", routeBuilder.WrapRoute(apiO.PhotoUpdateImage, api.HttpPost, true))

	http.HandleFunc("/api/break/set_series", routeBuilder.WrapRoute(apiO.BreakSetSeries, api.HttpPost, true))

	http.HandleFunc("/api/series/{series_id}/prices", routeBuilder.WrapRoute(apiO.SeriesGetPrices, api.HttpGet, true))

	http.HandleFunc("/api/widget/series/stashorpass", routeBuilder.WrapRoute(apiO.GetWidgetSeriesStashorpass, api.HttpPost, true))
	http.HandleFunc("/api/widget/series/stashorpass/update", routeBuilder.WrapRoute(apiO.UpdateWidgetSeriesStashorpass, api.HttpPost, true))
	http.HandleFunc("/api/widget/series/pick2", routeBuilder.WrapRoute(apiO.GetWidgetSeriesPick2, api.HttpPost, true))
	http.HandleFunc("/api/widget/series/pick2/update", routeBuilder.WrapRoute(apiO.UpdateWidgetSeriesPick2, api.HttpPost, true))
	http.HandleFunc("/api/widget/series/boxes_per_break", routeBuilder.WrapRoute(apiO.GetWidgetSeriesBoxesPerBreak, api.HttpPost, true))
	http.HandleFunc("/api/widget/series/boxes_per_break/update", routeBuilder.WrapRoute(apiO.UpdateWidgetSeriesBoxesPerBreak, api.HttpPost, true))
	http.HandleFunc("/api/widget/channel/count_settings", routeBuilder.WrapRoute(apiO.GetWidgetChannelCountSettings, api.HttpPost, true))
	http.HandleFunc("/api/widget/channel/count_settings/update", routeBuilder.WrapRoute(apiO.UpdateWidgetChannelCountSettings, api.HttpPost, true))
	http.HandleFunc("/api/widget/board/price_ranges", routeBuilder.WrapRoute(apiO.ListWidgetBoardPriceRanges, api.HttpPost, true))
	http.HandleFunc("/api/widget/board/price_ranges/create", routeBuilder.WrapRoute(apiO.CreateWidgetBoardPriceRange, api.HttpPost, true))
	http.HandleFunc("/api/widget/board/price_ranges/update", routeBuilder.WrapRoute(apiO.UpdateWidgetBoardPriceRange, api.HttpPost, true))
	http.HandleFunc("/api/widget/board/price_ranges/delete", routeBuilder.WrapRoute(apiO.DeleteWidgetBoardPriceRange, api.HttpPost, true))
	http.HandleFunc("/api/widget/presets", routeBuilder.WrapRoute(apiO.ListWidgetPresets, api.HttpPost, true))
	http.HandleFunc("/api/widget/presets/upsert", routeBuilder.WrapRoute(apiO.UpsertWidgetPreset, api.HttpPost, true))
	http.HandleFunc("/api/widget/presets/delete", routeBuilder.WrapRoute(apiO.DeleteWidgetPreset, api.HttpPost, true))
	http.HandleFunc("/api/widget/cards_board", routeBuilder.WrapRoute(apiO.GetWidgetCardsBoardSettings, api.HttpPost, true))
	http.HandleFunc("/api/widget/cards_board/update", routeBuilder.WrapRoute(apiO.UpdateWidgetCardsBoardSettings, api.HttpPost, true))

	http.HandleFunc("/api/layout/config", routeBuilder.WrapRoute(apiO.GetLayoutConfig, api.HttpPost, true))
	http.HandleFunc("/api/layout/config/update", routeBuilder.WrapRoute(apiO.UpdateLayoutConfig, api.HttpPost, true))
	http.HandleFunc("/api/layout/preset/list", routeBuilder.WrapRoute(apiO.ListLayoutPresets, api.HttpPost, true))
	http.HandleFunc("/api/layout/preset/create", routeBuilder.WrapRoute(apiO.CreateLayoutPreset, api.HttpPost, true))
	http.HandleFunc("/api/layout/preset/update", routeBuilder.WrapRoute(apiO.UpdateLayoutPreset, api.HttpPost, true))
	http.HandleFunc("/api/layout/preset/delete", routeBuilder.WrapRoute(apiO.DeleteLayoutPreset, api.HttpPost, true))
	http.HandleFunc("/api/layout/state", routeBuilder.WrapRoute(apiO.GetOverlayState, api.HttpPost, true))
	http.HandleFunc("/api/layout/state/update", routeBuilder.WrapRoute(apiO.UpdateOverlayState, api.HttpPost, true))
	http.HandleFunc("/api/layout/image/upload", routeBuilder.WrapRoute(apiO.LayoutImageUpload, api.HttpPost, true))
	http.HandleFunc("/api/layout/image/list", routeBuilder.WrapRoute(apiO.LayoutImageList, api.HttpPost, true))
	http.HandleFunc("/api/layout/image/delete", routeBuilder.WrapRoute(apiO.LayoutImageDelete, api.HttpPost, true))

	port := os.Getenv("port")
	portInt, err := strconv.Atoi(port)
	if err != nil {
		panic("Invalid port")
	}

	fmt.Println(fmt.Sprintf("Serving on port %d", portInt))
	err = http.ListenAndServe(fmt.Sprintf(":%d", portInt), handler)
	if err != nil {
		fmt.Println("An error occurred during listening: " + err.Error())
	}
}
