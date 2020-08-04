// Copyright (c) 2018-2019 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package pow

import (
	"context"
	"net/http"
	"time"

	"github.com/planetdecred/dcrextdata/app"
	"github.com/planetdecred/dcrextdata/app/helpers"
	"github.com/planetdecred/dcrextdata/cache"
)

var (
	availablePows = []string{
		Coinmine,
		Luxor,
		F2pool,
		Uupool,
	}
)

type PowDataStore interface {
	PowTableName() string
	AddPowData(context.Context, []PowData) error
	LastPowEntryTime(source string) (time int64)
	FetchPowDataForSync(ctx context.Context, date int64, skip, take int) ([]PowData, int64, error)
}

type Collector struct {
	pows   []Pow
	period int64
	store  PowDataStore
	charts *cache.Manager
}

func NewCollector(disabledPows []string, period int64, store PowDataStore, charts *cache.Manager) (*Collector, error) {
	pows := make([]Pow, 0, len(availablePows)-len(disabledPows))
	disabledMap := make(map[string]struct{})
	for _, pow := range disabledPows {
		disabledMap[pow] = struct{}{}
	}

	for _, pow := range availablePows {
		if _, disabled := disabledMap[pow]; disabled {
			continue
		}

		if contructor, ok := PowConstructors[pow]; ok {
			lastEntryTime := store.LastPowEntryTime(pow)
			in, err := contructor(&http.Client{Timeout: 10 * time.Second}, lastEntryTime) // Consider if sharing a single client is better
			if err != nil {
				return nil, err
			}
			pows = append(pows, in)
		}
	}

	return &Collector{
		pows:   pows,
		period: period,
		store:  store,
		charts: charts,
	}, nil
}

func (pc *Collector) Run(ctx context.Context, cacheManager *cache.Manager) {
	app.MarkBusyIfFree()
	log.Info("Triggering PoW collectors.")

	lastCollectionDateUnix := pc.store.LastPowEntryTime("")
	lastCollectionDate := helpers.UnixTime(lastCollectionDateUnix)
	secondsPassed := time.Since(lastCollectionDate)
	period := time.Duration(pc.period) * time.Second

	if lastCollectionDateUnix > 0 && secondsPassed < period {
		timeLeft := period - secondsPassed
		log.Infof("Fetching PoW data every %dm, collected %s ago, will fetch in %s.", pc.period/60,
			helpers.DurationToString(secondsPassed), helpers.DurationToString(timeLeft))

		app.ReleaseForNewModule()
		time.Sleep(timeLeft)
	}

	if lastCollectionDateUnix > 0 && secondsPassed < period {
		// continually check the state of the app until its free to run this module
		app.MarkBusyIfFree()
	}
	pc.Collect(ctx, cacheManager)
	app.ReleaseForNewModule()
	go pc.CollectAsync(ctx, cacheManager)
}

func (pc *Collector) CollectAsync(ctx context.Context, cacheManager *cache.Manager) {
	if ctx.Err() != nil {
		return
	}

	ticker := time.NewTicker(time.Duration(pc.period) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Infof("Stopping PoW collectors")
			return
		case <-ticker.C:
			// continually check the state of the app until its free to run this module
			app.MarkBusyIfFree()
			completeCollectionCycle := pc.store.LastPowEntryTime("")
			collectionCycleDate := helpers.UnixTime(completeCollectionCycle)
			timeInterval := time.Since(collectionCycleDate)
			log.Info("The next collection cycle begins in", timeInterval)

			log.Info("Starting a new PoW collection cycle")
			pc.Collect(ctx, cacheManager)
			app.ReleaseForNewModule()
		}
	}
}

func (pc *Collector) Collect(ctx context.Context, cacheManager *cache.Manager) {
	log.Info("Fetching PoW data.")
	for _, powInfo := range pc.pows {
		select {
		case <-ctx.Done():
			return
		default:
			data, err := powInfo.Collect(ctx)
			if err != nil {
				log.Error(err, powInfo.Name())
			}
			err = pc.store.AddPowData(ctx, data)
			if err != nil {
				log.Error(err)
			}
			if err = cacheManager.Update(ctx, cache.PowChart); err != nil {
				log.Error(err)
			}
		}
	}
	if err := pc.charts.TriggerUpdate(ctx, cache.PowChart); err != nil {
		log.Errorf("Charts update problem for %s: %s", cache.PowChart, err.Error())
	}
}
