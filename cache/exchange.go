package cache

import (
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	ExchangeCloseAxis axisType = "close"
	ExchangeHighAxis  axisType = "high"
	ExchangeOpenAxis  axisType = "open"
	ExchangeLowAxis   axisType = "low"
)

type ExchangeTickSet struct {
	bin         binLevel
	exchangeKey string

	Time  ChartUints
	Open  ChartFloats
	Close ChartFloats
	High  ChartFloats
	Low   ChartFloats
}

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

func newExchangeTickSet(exchangeKey string, bin binLevel) ExchangeTickSet {
	return ExchangeTickSet{
		bin:         bin,
		exchangeKey: exchangeKey,
	}
}

func (m ExchangeTickSet) Save(cacheManage *Manager) error {
	cacheManage.powMtx.Lock()
	defer cacheManage.powMtx.Unlock()
	filename := filepath.Join(cacheManage.dir, fmt.Sprintf("%s-%s-%s.gob", Exchange,
		strings.ReplaceAll(m.exchangeKey, "/", "--"), m.bin))
	if isFileExists(filename) {
		os.RemoveAll(filename)
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}

	defer file.Close()

	encoder := gob.NewEncoder(file)
	return encoder.Encode(m)
}

func (m ExchangeTickSet) snip(max int) ExchangeTickSet {

	m.Time = m.Time.snip(max)
	m.Open = m.Open.snip(max)
	m.Close = m.Close.snip(max)
	m.High = m.High.snip(max)
	m.Low = m.Low.snip(max)
	return m
}

func (charts *Manager) ExchangeTickSet(key string, bin binLevel) (data ExchangeTickSet, err error) {
	data = newExchangeTickSet(key, bin)

	key = strings.ReplaceAll(key, "/", "--")
	filename := filepath.Join(charts.dir, fmt.Sprintf("%s-%s-%s.gob", Exchange, key, bin))
	if !isFileExists(filename) {
		err = UnknownChartErr
		return
	}

	var file *os.File
	file, err = os.Open(filename)
	if err != nil {
		log.Errorf("Error in opening exchange cache file - %s", err.Error())
		return
	}

	defer func() {
		file.Close()
	}()

	decoder := gob.NewDecoder(file)
	err = decoder.Decode(&data)
	if err != nil {
		log.Errorf("Error in opening exchange cache file - %s", err.Error())
		return
	}

	return
}

func (charts *Manager) ExchangeSetTime(key string) uint64 {
	return charts.exchangeTickSetTip[key]
}

func (charts *Manager) SetExchangeSetTime(key string, time uint64) {
	charts.exchangeMtx.Lock()
	defer charts.exchangeMtx.Unlock()
	charts.exchangeTickSetTip[key] = time
}

func makeExchangeChart(ctx context.Context, charts *Manager, dataType, _ axisType, bin binLevel, key ...string) ([]byte, error) {
	if len(key) == 0 {
		return nil, errors.New("exchange set key is required for exchange chart")
	}
	set, err := charts.ExchangeTickSet(key[0], bin)
	if err != nil {
		return nil, err
	}

	var yAxis ChartFloats
	switch dataType {
	case ExchangeCloseAxis:
		yAxis = set.Close
	case ExchangeOpenAxis:
		yAxis = set.Open
	case ExchangeHighAxis:
		yAxis = set.High
	case ExchangeLowAxis:
		yAxis = set.Low
	}

	return charts.Encode(nil, set.Time, yAxis)
}
