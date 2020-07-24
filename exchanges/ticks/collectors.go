// Copyright (c) 2018-2019 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package ticks

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/planetdecred/dcrextdata/app/helpers"
)

const (
	Bittrex        = "bittrex"
	Bittrexusd     = Bittrex + "usd"
	bittrexAPIURL  = "https://bittrex.com/Api/v2.0/pub/market/GetTicks"
	Poloniex       = "poloniex"
	poloniexAPIURL = "https://poloniex.com/public"
	Binance        = "binance"
	binanceAPIURL  = "https://api.binance.com/api/v1/klines"

	btcdcrPair = "BTC/DCR"
	usdbtcPair = "USD/BTC"

	fiveMin = time.Minute * 5
	oneDay  = time.Hour * 24

	apprxBinanceStart  int64 = 1540353600
	binanceVolumeLimit int64 = 1000

	apprxPoloniexStart  int64 = 1463364000
	poloniexVolumeLimit int64 = 20000

	clientTimeout = time.Minute

	IntervalShort    = "short"
	IntervalLong     = "long"
	IntervalHistoric = "historic"
)

var (
	zeroTime time.Time

	CollectorConstructors = map[string]func(context.Context, Store) (Collector, error){
		Bittrex:    NewBittrexCollector,
		Bittrexusd: NewBittrexUSDCollector,
		Poloniex:   NewPoloniexCollector,
		Binance:    NewBinanceCollector,
	}

	bittrexIntervals = map[float64]string{
		300:   "fiveMin",
		1800:  "thirtyMin",
		3600:  "hour",
		86400: "day",
	}

	bleutradeIntervals = map[float64]string{
		3600:  "1h",
		14400: "4h",
		86400: "1d",
	}

	binanceIntervals = map[float64]string{
		300:   "5m",
		3600:  "1h",
		86400: "1d",
	}

	poloniexData = ExchangeData{
		Name:       Poloniex,
		WebsiteURL: "https://poloniex.com",
		availableCPairs: map[string]string{
			btcdcrPair: "BTC_DCR",
		},
		apiLimited:       true,
		ShortInterval:    fiveMin,
		LongInterval:     2 * time.Hour,
		HistoricInterval: oneDay,
		requester: func(last time.Time, interval time.Duration, cpair string) (string, error) {
			return helpers.AddParams(poloniexAPIURL, map[string]interface{}{
				"command":      "returnChartData",
				"currencyPair": cpair,
				"start":        last.Unix(),
				"end":          helpers.NowUTC().Unix(),
				"period":       int(interval.Seconds()),
			})
		},
	}

	binanceData = ExchangeData{
		Name:       Binance,
		WebsiteURL: "https://binance.com",
		availableCPairs: map[string]string{
			btcdcrPair: "DCRBTC",
		},
		apiLimited:       true,
		ShortInterval:    fiveMin,
		LongInterval:     time.Hour,
		HistoricInterval: oneDay,
		requester: func(last time.Time, interval time.Duration, cpair string) (string, error) {
			start := last.Unix() * 1000
			end := start + binanceVolumeLimit*int64(interval.Seconds())*1000
			return helpers.AddParams(binanceAPIURL, map[string]interface{}{
				"symbol":    cpair,
				"startTime": start,
				"endTime":   end,
				"interval":  binanceIntervals[interval.Seconds()],
				"limit":     binanceVolumeLimit,
			})
		},
	}

	bittrexData = ExchangeData{
		Name:       Bittrex,
		WebsiteURL: "https://bittrex.com",
		availableCPairs: map[string]string{
			btcdcrPair: "BTC-DCR",
			usdbtcPair: "USD-BTC",
		},
		apiLimited:       false,
		ShortInterval:    fiveMin,
		LongInterval:     time.Hour,
		HistoricInterval: oneDay,
		requester: func(last time.Time, interval time.Duration, cpair string) (string, error) {
			return helpers.AddParams(bittrexAPIURL, map[string]interface{}{
				"marketName":   cpair,
				"tickInterval": bittrexIntervals[interval.Seconds()],
			})
		},
	}
)

type commonExchange struct {
	*ExchangeData
	currencyPair string
	store        Store
	client       *http.Client
	lastShort    time.Time
	lastLong     time.Time
	lastHistoric time.Time
	respLock     sync.Mutex
	apiResp      tickable
}

func (xc *commonExchange) GetShort(ctx context.Context) error {
	return xc.Get(ctx, &xc.lastShort, xc.ShortInterval, IntervalShort)
}

func (xc *commonExchange) GetLong(ctx context.Context) error {
	return xc.Get(ctx, &xc.lastLong, xc.LongInterval, IntervalLong)
}

func (xc *commonExchange) GetHistoric(ctx context.Context) error {
	return xc.Get(ctx, &xc.lastHistoric, xc.HistoricInterval, IntervalHistoric)
}

func (xc *commonExchange) Get(ctx context.Context, last *time.Time, interval time.Duration, intervalStr string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	xc.respLock.Lock()
	defer xc.respLock.Unlock()
	for helpers.NowUTC().Add(-interval).Unix() > last.Unix() {
		requestURL, err := xc.requester(*last, interval, xc.availableCPairs[xc.currencyPair])
		if err != nil {
			return err
		}
		// fmt.Printf("Debug: %s\n", requestURL)
		err = helpers.GetResponse(ctx, xc.client, requestURL, xc.apiResp)
		if err != nil {
			return err
		}

		ticks := xc.apiResp.toTicks(last.Unix())

		newLast, err := xc.store.StoreExchangeTicks(ctx, xc.Name, int(interval.Minutes()), xc.currencyPair, ticks)
		if err != nil {
			return err
		}
		if newLast != zeroTime {
			*last = newLast
		}
		if !xc.apiLimited || len(ticks) == 1 {
			break
		}
	}
	return nil
}

func newCollector(ctx context.Context, store Store, exchange ExchangeData, currencyPair string, historicStart time.Time, response tickable) (Collector, error) {
	lastShort, lastLong, lastHistoric, err := store.RegisterExchange(ctx, exchange)
	if err != nil {
		return nil, err
	}

	now := helpers.NowUTC()
	if lastShort == zeroTime {
		lastShort = now.Add((-14) * oneDay)
	}
	if lastLong == zeroTime {
		lastLong = now.Add((-30) * oneDay)
	}
	if lastHistoric == zeroTime && historicStart != zeroTime {
		lastHistoric = historicStart
	}

	return &commonExchange{
		ExchangeData: &exchange,
		client:       &http.Client{Timeout: 10 * time.Second},
		store:        store,
		lastShort:    lastShort,
		lastLong:     lastLong,
		lastHistoric: lastHistoric,
		apiResp:      response,
		currencyPair: currencyPair,
	}, nil
}

func NewPoloniexCollector(ctx context.Context, store Store) (Collector, error) {
	return newCollector(ctx, store, poloniexData, btcdcrPair, helpers.UnixTime(apprxPoloniexStart), new(poloniexAPIResponse))
}

func NewBittrexCollector(ctx context.Context, store Store) (Collector, error) {
	return newCollector(ctx, store, bittrexData, btcdcrPair, zeroTime, new(bittrexAPIResponse))
}

func NewBittrexUSDCollector(ctx context.Context, store Store) (Collector, error) {
	return newCollector(ctx, store, bittrexData, usdbtcPair, zeroTime, new(bittrexAPIResponse))
}

func NewBinanceCollector(ctx context.Context, store Store) (Collector, error) {
	return newCollector(ctx, store, binanceData, btcdcrPair, helpers.UnixTime(apprxBinanceStart), new(binanceAPIResponse))
}
