package cache

import (
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
