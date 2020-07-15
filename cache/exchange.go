package cache

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	ExchangeCloseAxis axisType = "close"
	ExchangeHighAxis  axisType = "high"
	ExchangeOpenAxis  axisType = "open"
	ExchangeLowAxis   axisType = "low"
)

// BuildExchangeKey returns exchange name, currency pair and interval joined by -
func BuildExchangeKey(exchangeName string, currencyPair string, interval int) string {
	return fmt.Sprintf("%s-%s-%d", exchangeName, currencyPair, interval)
}

func ExtractExchangeKey(setKey string) (exchangeName string, currencyPair string, interval int) {
	keys := strings.Split(setKey, "-")
	if len(keys) > 0 {
		exchangeName = keys[0]
	}

	if len(keys) > 1 {
		currencyPair = keys[1]
	}

	if len(keys) > 2 {
		interval, _ = strconv.Atoi(keys[2])
	}
	return
}

func (charts *ChartData) ExchangeSetTime(key string) uint64 {
	var dates ChartUints
	if err := charts.ReadVal(key+"-"+string(TimeAxis), &dates); err != nil {
		return 0
	}
	if len(dates) < 1 {
		return 0
	}
	return dates[len(dates)-1]
}

func makeExchangeChart(ctx context.Context, charts *ChartData, dataType, _ axisType, bin binLevel, key ...string) ([]byte, error) {
	if len(key) < 1 {
		return nil, errors.New("exchange set key is required for exchange chart")
	}
	var dates ChartUints
	if err := charts.ReadVal(key[0]+"-"+string(TimeAxis), &dates); err != nil {
		return nil, err
	}

	var yAxis ChartFloats
	if err := charts.ReadVal(key[0]+"-"+string(dataType), &yAxis); err != nil {
		log.Errorf("Cannot create exchange chart, %s", err.Error())
		return nil, errors.New("no record found for the selected exchange")
	}

	return charts.Encode(nil, dates, yAxis)

}
