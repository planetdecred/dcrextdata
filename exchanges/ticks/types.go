// Copyright (c) 2018-2019 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package ticks

import (
	"context"
	"strconv"
	"time"

	"github.com/planetdecred/dcrextdata/app/helpers"
)

type Collector interface {
	GetShort(context.Context) error
	GetLong(context.Context) error
	GetHistoric(context.Context) error
}

type Store interface {
	ExchangeTickTableName() string
	ExchangeTableName() string
	RegisterExchange(ctx context.Context, exchange ExchangeData) (lastShort, lastLong, lastHistoric time.Time, err error)
	FetchExchangeForSync(ctx context.Context, lastID int, skip, take int) ([]ExchangeData, int64, error)
	StoreExchangeTicks(ctx context.Context, exchange string, interval int, pair string, data []Tick) (time.Time, error)
	LastExchangeTickEntryTime() (time time.Time)
	FetchExchangeTicksForSync(ctx context.Context, date time.Time, skip, take int) ([]TickSyncDto, int64, error)
}

type urlRequester func(time.Time, time.Duration, string) (string, error)

type ExchangeData struct {
	ID               int
	Name             string
	WebsiteURL       string
	apiLimited       bool
	availableCPairs  map[string]string
	ShortInterval    time.Duration
	LongInterval     time.Duration
	HistoricInterval time.Duration
	requester        urlRequester
}

type tickable interface {
	toTicks(int64) []Tick
}

// Tick represents an exchange data tick
type Tick struct {
	High   float64
	Low    float64
	Open   float64
	Close  float64
	Volume float64
	Time   time.Time
}

// TickSyncDto represents an exchange data, structured for sharing
type TickSyncDto struct {
	ExchangeID   int       `json:"exchange_id"`
	ID           int       `json:"id"`
	ExchangeName string    `json:"exchange_name"`
	High         float64   `json:"high"`
	Low          float64   `json:"low"`
	Open         float64   `json:"open"`
	Close        float64   `json:"close"`
	Volume       float64   `json:"volume"`
	Time         time.Time `json:"time"`
	Interval     int       `json:"interval"`
	CurrencyPair string    `json:"currency_pair"`
}

// TickDto represents an exchange data, formatted for presentation
type TickDto struct {
	ExchangeID   int     `json:"exchange_id"`
	ExchangeName string  `json:"exchange_name"`
	High         float64 `json:"high"`
	Low          float64 `json:"low"`
	Open         float64 `json:"open"`
	Close        float64 `json:"close"`
	Volume       float64 `json:"volume"`
	Time         string  `json:"time"`
	Interval     int     `json:"interval"`
	CurrencyPair string  `json:"currency_pair"`
}

// TickDto represents an exchange data, formatted for presentation
type TickChartData struct {
	Filter float64   `json:"filter"`
	Time   time.Time `json:"time"`
}

type TickDtoCP struct {
	CurrencyPair string `json:"currency_pair"`
}

type TickDtoInterval struct {
	Interval int `json:"interval"`
}

type poloniexAPIResponse []poloniexDataTick

type poloniexDataTick struct {
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Open   float64 `json:"open"`
	Close  float64 `json:"close"`
	Volume float64 `json:"volume"`
	Time   int64   `json:"date"`
}

func (resp poloniexAPIResponse) toTicks(start int64) []Tick {
	res := []poloniexDataTick(resp)
	dataTicks := make([]Tick, 0, len(res))
	for _, v := range res {
		if v.Time < start {
			continue
		}
		dataTicks = append(dataTicks, Tick{
			High:   v.High,
			Low:    v.Low,
			Open:   v.Open,
			Close:  v.Close,
			Volume: v.Volume,
			Time:   helpers.UnixTime(v.Time),
		})
	}
	return dataTicks
}

type bittrexDataTick struct {
	High   float64 `json:"H"`
	Low    float64 `json:"L"`
	Open   float64 `json:"O"`
	Close  float64 `json:"C"`
	Volume float64 `json:"BV"`
	Time   string  `json:"T"`
}

type bittrexAPIResponse struct {
	Result []bittrexDataTick `json:"result"`
}

func (resp bittrexAPIResponse) toTicks(start int64) []Tick {
	bTicks := resp.Result
	dataTicks := make([]Tick, 0, len(bTicks))
	for _, v := range bTicks {
		t, err := time.Parse("2006-01-02T15:04:05", v.Time)
		if err != nil || t.Unix() < start {
			continue
		}
		dataTicks = append(dataTicks, Tick{
			High:   v.High,
			Low:    v.Low,
			Open:   v.Open,
			Close:  v.Close,
			Volume: v.Volume,
			Time:   t.UTC(),
		})
	}
	return dataTicks
}

type bleutradeDataTick struct {
	High   float64 `json:"High"`
	Low    float64 `json:"Low"`
	Open   float64 `json:"Open"`
	Close  float64 `json:"Close"`
	Volume float64 `json:"Volume"`
	Time   string  `json:"TimeStamp"`
}

type bleutradeAPIResponse struct {
	Result []bleutradeDataTick `json:"result"`
}

func (resp bleutradeAPIResponse) toTicks(start int64) []Tick {
	res := resp.Result
	dataTicks := make([]Tick, 0, len(res))
	for i := len(res) - 1; i >= 0; i-- {
		v := res[i]
		t, err := time.Parse("2006-01-02 15:04:05", v.Time)
		if err != nil || t.Unix() < start {
			continue
		}
		dataTicks = append(dataTicks, Tick{
			High:   v.High,
			Low:    v.Low,
			Open:   v.Open,
			Close:  v.Close,
			Volume: v.Volume,
			Time:   t.UTC(),
		})
	}
	return dataTicks
}

type binanceAPIResponse []binanceDataTick
type binanceDataTick []interface{}

func (resp binanceAPIResponse) toTicks(start int64) []Tick {
	res := []binanceDataTick(resp)
	dataTicks := make([]Tick, 0, len(res))
	for _, j := range res {
		// Converting unix time from milliseconds to seconds
		secs := int64(j[0].(float64) / 1000)
		t := helpers.UnixTime(secs)

		if secs < start {
			continue
		}

		high, err := strconv.ParseFloat(j[2].(string), 64)
		if err != nil {
			continue
		}
		low, err := strconv.ParseFloat(j[3].(string), 64)
		if err != nil {
			continue
		}
		open, err := strconv.ParseFloat(j[1].(string), 64)
		if err != nil {
			continue
		}
		close, err := strconv.ParseFloat(j[4].(string), 64)
		if err != nil {
			continue
		}
		volume, err := strconv.ParseFloat(j[5].(string), 64)
		if err != nil {
			continue
		}

		dataTicks = append(dataTicks, Tick{
			High:   high,
			Low:    low,
			Open:   open,
			Close:  close,
			Volume: volume,
			Time:   t.UTC(),
		})
	}
	return dataTicks
}
