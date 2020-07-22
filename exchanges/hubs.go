// Copyright (c) 2018-2019 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package exchanges

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/raedahgroup/dcrextdata/app"
	"github.com/raedahgroup/dcrextdata/app/helpers"
	"github.com/raedahgroup/dcrextdata/cache"
	"github.com/raedahgroup/dcrextdata/datasync"
	"github.com/raedahgroup/dcrextdata/exchanges/ticks"
)

const (
	clientTimeout = time.Minute
)

type TickHub struct {
	collectors []ticks.Collector
	client     *http.Client
	store      ticks.Store
	charts     *cache.Manager
}

var (
	availableExchanges = []string{
		ticks.Bittrex,
		ticks.Bittrexusd,
		ticks.Binance,
		// ticks.Bleutrade,
		ticks.Poloniex,
	}
)

func NewTickHub(ctx context.Context, disabledexchanges []string, store ticks.Store, charts *cache.Manager) (*TickHub, error) {
	collectors := make([]ticks.Collector, 0, len(availableExchanges)-len(disabledexchanges))
	disabledMap := make(map[string]struct{})
	for _, e := range disabledexchanges {
		disabledMap[e] = struct{}{}
	}
	enabledExchanges := make([]string, 0, cap(collectors))
	for _, exchange := range availableExchanges {
		if _, ok := disabledMap[exchange]; !ok {
			collector, err := ticks.CollectorConstructors[exchange](ctx, store)
			if err != nil {
				log.Error(err)
				continue
			}
			collectors = append(collectors, collector)
			enabledExchanges = append(enabledExchanges, exchange)
		}
	}

	if len(collectors) == 0 {
		return nil, fmt.Errorf("No tick collectors")
	}

	log.Infof("Enabled exchange tick collection for %v", enabledExchanges)

	return &TickHub{
		collectors: collectors,
		client:     &http.Client{Timeout: clientTimeout},
		store:      store,
		charts:     charts,
	}, nil
}

func (hub *TickHub) CollectShort(ctx context.Context) {
	wg := new(sync.WaitGroup)
	for _, collector := range hub.collectors {
		if ctx.Err() != nil {
			log.Error(ctx.Err())
			break
		}
		wg.Add(1)
		func(ctx context.Context, wg *sync.WaitGroup, collector ticks.Collector) {
			err := collector.GetShort(ctx)
			if err != nil {
				log.Error(err)
			}
			wg.Done()
		}(ctx, wg, collector)
	}
	wg.Wait()
	log.Info("Completed short collection")

	if err := hub.charts.TriggerUpdate(ctx, cache.Exchange); err != nil {
		log.Errorf("Charts update problem for %s: %s", cache.Exchange, err.Error())
	}
}

func (hub *TickHub) CollectLong(ctx context.Context) {
	wg := new(sync.WaitGroup)
	for _, collector := range hub.collectors {
		if ctx.Err() != nil {
			log.Error(ctx.Err())
			break
		}
		wg.Add(1)
		func(ctx context.Context, wg *sync.WaitGroup, collector ticks.Collector) {
			err := collector.GetLong(ctx)
			if err != nil {
				log.Error(err)
			}
			wg.Done()
		}(ctx, wg, collector)
	}
	wg.Wait()
	log.Info("Completed long collection")

	if err := hub.charts.TriggerUpdate(ctx, cache.Exchange); err != nil {
		log.Errorf("Charts update problem for %s: %s", cache.Exchange, err.Error())
	}
}

func (hub *TickHub) CollectHistoric(ctx context.Context) {
	wg := new(sync.WaitGroup)
	for _, collector := range hub.collectors {
		if ctx.Err() != nil {
			log.Error(ctx.Err())
			break
		}
		wg.Add(1)
		func(ctx context.Context, wg *sync.WaitGroup, collector ticks.Collector) {
			err := collector.GetHistoric(ctx)
			if err != nil {
				log.Error(err)
			}
			wg.Done()
		}(ctx, wg, collector)
	}
	wg.Wait()
	log.Info("Completed historic collection")

	if err := hub.charts.TriggerUpdate(ctx, cache.Exchange); err != nil {
		log.Errorf("Charts update problem for %s: %s", cache.Exchange, err.Error())
	}
}

func (hub *TickHub) CollectAll(ctx context.Context) {
	for _, collector := range hub.collectors {
		if ctx.Err() != nil {
			log.Error(ctx.Err())
			break
		}

		err := collector.GetShort(ctx)
		if err != nil {
			log.Error(err)
		}

		err = collector.GetLong(ctx)
		if err != nil {
			log.Error(err)
		}

		err = collector.GetHistoric(ctx)
		if err != nil {
			log.Error(err)
		}
	}

	if err := hub.charts.TriggerUpdate(ctx, cache.Exchange); err != nil {
		log.Errorf("Charts update problem for %s: %s", cache.Exchange, err.Error())
	}

	/*hub.CollectShort(ctx)
	if ctx.Err() != nil {
		return
	}
	hub.CollectLong(ctx)
	if ctx.Err() != nil {
		return
	}
	hub.CollectHistoric(ctx)*/
}

func (hub *TickHub) Run(ctx context.Context) {
	shortTicker := time.NewTicker(5 * time.Minute)
	longTicker := time.NewTicker(time.Hour)
	dayTicker := time.NewTicker(24 * time.Hour)
	defer shortTicker.Stop()
	defer longTicker.Stop()
	defer dayTicker.Stop()

	if ctx.Err() != nil {
		log.Error(ctx.Err())
		return
	}

	lastCollectionDate := hub.store.LastExchangeTickEntryTime()
	secondsPassed := time.Since(lastCollectionDate)
	period := 5 * time.Minute

	if secondsPassed < period {
		timeLeft := period - secondsPassed
		log.Infof("Fetching exchange ticks every %dm, collected %s ago, will fetch in %s.", period/time.Minute, helpers.DurationToString(secondsPassed),
			helpers.DurationToString(timeLeft))

		time.Sleep(timeLeft)
	}

	registerStarter := func() {
		// continually check the state of the app until its free to run this module
		for {
			if app.MarkBusyIfFree() {
				break
			}
		}

		log.Info("Starting exchange tick collection cycle")
	}

	registerStarter()
	hub.CollectAll(ctx)
	app.ReleaseForNewModule()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-shortTicker.C:
				registerStarter()
				hub.CollectShort(ctx)
				app.ReleaseForNewModule()
			case <-longTicker.C:
				registerStarter()
				hub.CollectLong(ctx)
				app.ReleaseForNewModule()
			case <-dayTicker.C:
				registerStarter()
				hub.CollectHistoric(ctx)
				app.ReleaseForNewModule()
			}
		}
	}()
}

func (hub *TickHub) RegisterSyncer(syncCoordinator *datasync.SyncCoordinator) {
	hub.registerExchangeSyncer(syncCoordinator)
	hub.registerExchangeTickSyncer(syncCoordinator)
}

func (hub *TickHub) registerExchangeSyncer(syncCoordinator *datasync.SyncCoordinator) {
	syncCoordinator.AddSyncer(hub.store.ExchangeTableName(), datasync.Syncer{
		LastEntry: func(ctx context.Context, db datasync.Store) (string, error) {
			return strconv.FormatInt(db.LastExchangeEntryID(), 10), nil
		},
		Collect: func(ctx context.Context, url string) (result *datasync.Result, err error) {
			result = new(datasync.Result)
			result.Records = []ticks.ExchangeData{}
			err = helpers.GetResponse(ctx, &http.Client{Timeout: 10 * time.Second}, url, result)
			return
		},
		Retrieve: func(ctx context.Context, last string, skip, take int) (result *datasync.Result, err error) {
			result = new(datasync.Result)
			lastID, err := strconv.ParseInt(last, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid date, %s", err.Error())
			}
			exchanges, totalCount, err := hub.store.FetchExchangeForSync(ctx, int(lastID), skip, take)
			if err != nil {
				result.Message = err.Error()
				return
			}
			result.Records = exchanges
			result.TotalCount = totalCount
			result.Success = true
			return
		},
		Append: func(ctx context.Context, store datasync.Store, data interface{}) {
			mappedData := data.([]interface{})
			var exchangeData []ticks.ExchangeData
			for _, item := range mappedData {
				var exchange ticks.ExchangeData
				err := datasync.DecodeSyncObj(item, &exchange)
				if err != nil {
					log.Errorf("Error in decoding the received exchange data, %s", err.Error())
					return
				}
				exchangeData = append(exchangeData, exchange)
			}

			for _, exchange := range exchangeData {
				err := store.SaveExchangeFromSync(ctx, exchange)
				if err != nil {
					log.Errorf("Error while appending exchange synced data, %s", err.Error())
				}
			}
		},
	})
}

func (hub *TickHub) registerExchangeTickSyncer(syncCoordinator *datasync.SyncCoordinator) {
	syncCoordinator.AddSyncer(hub.store.ExchangeTickTableName(), datasync.Syncer{
		LastEntry: func(ctx context.Context, db datasync.Store) (string, error) {
			entry := strconv.FormatInt(db.LastExchangeTickEntryTime().Unix(), 10)
			return entry, nil
		},
		Collect: func(ctx context.Context, url string) (result *datasync.Result, err error) {
			result = new(datasync.Result)
			result.Records = []ticks.TickDto{}
			err = helpers.GetResponse(ctx, &http.Client{Timeout: 10 * time.Second}, url, result)
			return
		},
		Retrieve: func(ctx context.Context, last string, skip, take int) (result *datasync.Result, err error) {
			result = new(datasync.Result)
			unitDate, err := strconv.ParseInt(last, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid date, %s", err.Error())
			}
			exchangeTicks, totalCount, err := hub.store.FetchExchangeTicksForSync(ctx, helpers.UnixTime(unitDate), skip, take)
			if err != nil {
				result.Message = err.Error()
				return
			}
			result.Records = exchangeTicks
			result.TotalCount = totalCount
			result.Success = true
			return
		},
		Append: func(ctx context.Context, store datasync.Store, data interface{}) {
			mappedData := data.([]interface{})
			var tickDtos []ticks.TickSyncDto
			for _, item := range mappedData {
				var tickDto ticks.TickSyncDto
				err := datasync.DecodeSyncObj(item, &tickDto)
				if err != nil {
					log.Errorf("Error in decoding the received exchange tick, %s", err.Error())
					return
				}
				tickDtos = append(tickDtos, tickDto)
			}

			for _, tickDto := range tickDtos {
				err := store.SaveExchangeTickFromSync(ctx, tickDto)
				if err != nil {
					log.Errorf("Error while appending exchange tick synced data, %s", err.Error())
				}
			}
		},
	})
}
