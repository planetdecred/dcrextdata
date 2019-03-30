package main

import (
	"net/http"
	"time"
)

const (
	Bittrex    = "bittrex"
	BittrexUrl = "https://bittrex.com/Api/v2.0/pub/market/GetTicks" //?marketName=BTC-DCR&tickInterval=""

	Poloniex    = "poloniex"
	PoloniexUrl = "https://poloniex.com/public" //?command=returnChartData&currencyPair=BTC_DCR&start=%d&end=%d&period=%d
)

const (
	poloniexVolumeLimit int64 = 20000
	apprxPoloniexStart  int64 = 1463364000
)

var ExchangeConstructors = map[string]func(*http.Client, int64, int64) (Exchange, error){
	Bittrex:  NewBittrex,
	Poloniex: NewPoloniex,
}

type Exchange interface {
	Historic(chan []DataTick) error
	Collect(chan []DataTick) error
	LastUpdateTime() int64
	Name() string
}

type CommonExchange struct {
	client     *http.Client
	lastUpdate int64
	period     int64
	baseUrl    string
}

func (ex *CommonExchange) LastUpdateTime() int64 { return ex.lastUpdate }

type BittrexExchange struct {
	CommonExchange
}

var bittrexIntervals = map[int64]string{
	300:  "fiveMin",
	1800: "thirtyMin",
}

func NewBittrex(client *http.Client, lastUpdate int64, period int64) (Exchange, error) {
	if client == nil {
		return nil, new(NilClientError)
	}
	return &BittrexExchange{
		CommonExchange: CommonExchange{
			client:     client,
			lastUpdate: lastUpdate,
			period:     period,
			baseUrl:    BittrexUrl,
		},
	}, nil
}

func (*BittrexExchange) Name() string { return Bittrex }

func (ex *BittrexExchange) Historic(data chan []DataTick) error { return ex.Collect(data) }

func (ex *BittrexExchange) Collect(data chan []DataTick) error {
	resp := new(bittrexAPIResponse)

	requestUrl, err := addParams(ex.baseUrl, map[string]interface{}{
		"marketName":   "BTC-DCR",
		"tickInterval": bittrexIntervals[ex.period],
	})
	if err != nil {
		return err
	}
	err = GetResponse(ex.client, requestUrl, resp)

	if err != nil {
		return err
	}

	result := ex.respToDataTicks(resp, ex.lastUpdate)

	ex.lastUpdate = result[len(result)-1].Time

	data <- result
	return nil
}

func (BittrexExchange) respToDataTicks(resp *bittrexAPIResponse, start int64) []DataTick {
	dataTicks := make([]DataTick, 0, len(resp.Result))
	for _, v := range resp.Result {
		t, _ := time.Parse("2006-01-02T15:04:05", v.Time)

		// Skip all entries before the required start time
		if t.Unix() < start {
			continue
		}

		dataTicks = append(dataTicks, DataTick{
			High:     v.High,
			Low:      v.Low,
			Open:     v.Open,
			Close:    v.Close,
			Time:     t.Unix(),
			Exchange: "bittrex",
		})
	}

	return dataTicks
}

type PoloniexExchange struct {
	CommonExchange
}

func NewPoloniex(client *http.Client, lastUpdate int64, period int64) (Exchange, error) {
	if client == nil {
		return nil, new(NilClientError)
	}
	if lastUpdate == 0 {
		lastUpdate = apprxPoloniexStart
	}
	return &PoloniexExchange{
		CommonExchange: CommonExchange{
			client:     client,
			lastUpdate: lastUpdate,
			period:     period,
			baseUrl:    PoloniexUrl,
		},
	}, nil
}

func (*PoloniexExchange) Name() string { return Poloniex }

func (ex *PoloniexExchange) Historic(data chan []DataTick) error {
	now := time.Now().Unix()

	if now-ex.lastUpdate < ex.period {
		return new(CollectionIntervalTooShort)
	}

	for (now-ex.lastUpdate)/ex.period >= poloniexVolumeLimit {
		end := ex.lastUpdate + poloniexVolumeLimit*ex.period

		resp, last, err := ex.fetch(ex.lastUpdate, end, ex.period)

		if err != nil {
			return err
		}

		data <- resp
		ex.lastUpdate = last
		excLog.Debugf("Last update time is now: %s", time.Unix(last, 0).String())
		now = time.Now().Unix()
	}

	resp, last, err := ex.fetch(ex.lastUpdate, now, ex.period)

	if err != nil {
		return err
	}

	data <- resp
	ex.lastUpdate = last
	excLog.Debugf("Last update time is now: %s", time.Unix(last, 0).String())

	return nil
}

func (ex *PoloniexExchange) Collect(data chan []DataTick) error {
	resp, last, err := ex.fetch(ex.lastUpdate, ex.lastUpdate+poloniexVolumeLimit*ex.period, ex.period)

	if err != nil {
		return err
	}

	data <- resp
	ex.lastUpdate = last
	return nil
}

func (ex *PoloniexExchange) fetch(start, end, period int64) ([]DataTick, int64, error) {
	resp := new(poloniexAPIResponse)
	//?command=returnChartData&currencyPair=BTC_DCR&start=%d&end=%d&period=%d
	requestURL, err := addParams(ex.baseUrl, map[string]interface{}{
		"command":      "returnChartData",
		"currencyPair": "BTC_DCR",
		"start":        start,
		"end":          end,
		"period":       period,
	})

	if err != nil {

		return nil, 0, err
	}
	err = GetResponse(ex.client, requestURL, resp)

	res := []poloniexDataTick(*resp)
	dataTicks := make([]DataTick, 0, len(res))
	for _, v := range res {
		dataTicks = append(dataTicks, DataTick{
			High:     v.High,
			Low:      v.Low,
			Open:     v.Open,
			Close:    v.Close,
			Time:     v.Time,
			Exchange: "poloniex",
		})
	}

	if err != nil {
		return nil, 0, err
	}
	return dataTicks, dataTicks[len(dataTicks)-1].Time, nil
}
