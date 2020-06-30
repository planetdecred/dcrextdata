// Copyright (c) 2018-2019 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/raedahgroup/dcrextdata/app/helpers"
	"github.com/raedahgroup/dcrextdata/cache"
	"github.com/raedahgroup/dcrextdata/exchanges/ticks"
	"github.com/raedahgroup/dcrextdata/postgres/models"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
)

var (
	ErrNonConsecutiveTicks = errors.New("postgres/exchanges: Non consecutive exchange ticks")
	zeroTime               time.Time
)

func (pg *PgDb) ExchangeTableName() string {
	return models.TableNames.Exchange
}

func (pg *PgDb) ExchangeTickTableName() string {
	return models.TableNames.ExchangeTick
}

func (pg *PgDb) RegisterExchange(ctx context.Context, exchange ticks.ExchangeData) (time.Time, time.Time, time.Time, error) {
	xch, err := models.Exchanges(models.ExchangeWhere.Name.EQ(exchange.Name)).One(ctx, pg.db)
	if err != nil {
		if err == sql.ErrNoRows {
			newXch := models.Exchange{
				Name: exchange.Name,
				URL:  exchange.WebsiteURL,
			}
			err = newXch.Insert(ctx, pg.db, boil.Infer())
		}
		return zeroTime, zeroTime, zeroTime, err
	}
	var shortTime, longTime, historicTime time.Time
	toMin := func(t time.Duration) int {
		return int(t.Minutes())
	}
	timeDesc := qm.OrderBy("time desc")
	lastShort, err := models.ExchangeTicks(qm.Expr(models.ExchangeTickWhere.ExchangeID.EQ(xch.ID), models.ExchangeTickWhere.Interval.EQ(toMin(exchange.ShortInterval)), timeDesc)).One(ctx, pg.db)
	if err == nil {
		shortTime = lastShort.Time
	}
	lastLong, err := models.ExchangeTicks(qm.Expr(models.ExchangeTickWhere.ExchangeID.EQ(xch.ID), models.ExchangeTickWhere.Interval.EQ(toMin(exchange.LongInterval)), timeDesc)).One(ctx, pg.db)
	if err == nil {
		longTime = lastLong.Time
	}
	lastHistoric, err := models.ExchangeTicks(qm.Expr(models.ExchangeTickWhere.ExchangeID.EQ(xch.ID), models.ExchangeTickWhere.Interval.EQ(toMin(exchange.HistoricInterval)), timeDesc)).One(ctx, pg.db)
	if err == nil {
		historicTime = lastHistoric.Time
	}
	if err != nil && err == sql.ErrNoRows {
		err = nil
	}

	// log.Debugf("Exchange %s, %v, %v, %v", exchange.Name, shortTime.UTC(), longTime.UTC(), historicTime.UTC())
	return shortTime, longTime, historicTime, err
}

// StoreExchangeTicks
func (pg *PgDb) StoreExchangeTicks(ctx context.Context, name string, interval int, pair string, ticks []ticks.Tick) (time.Time, error) {
	if len(ticks) == 0 {
		return zeroTime, fmt.Errorf("No ticks received for %s", name)
	}

	exchange, err := models.Exchanges(models.ExchangeWhere.Name.EQ(name)).One(ctx, pg.db)
	if err != nil {
		return zeroTime, err
	}

	var lastTime time.Time
	lastTick, err := models.ExchangeTicks(models.ExchangeTickWhere.ExchangeID.EQ(exchange.ID),
		models.ExchangeTickWhere.Interval.EQ(interval),
		models.ExchangeTickWhere.CurrencyPair.EQ(pair),
		qm.OrderBy(models.ExchangeTickColumns.Time)).One(ctx, pg.db)

	if err == sql.ErrNoRows {
		lastTime = ticks[0].Time.Add(-time.Duration(interval))
	} else if err != nil {
		return lastTime, err
	} else {
		lastTime = lastTick.Time
	}

	firstTime := ticks[0].Time
	added := 0
	for _, tick := range ticks {
		xcTick := tickToExchangeTick(exchange.ID, pair, interval, tick)
		err = xcTick.Insert(ctx, pg.db, boil.Infer())
		if err != nil && !strings.Contains(err.Error(), "unique constraint") {
			return lastTime, err
		}
		lastTime = xcTick.Time
		added++
	}

	if added == 0 {
		log.Infof("No new ticks on %s(%dm) for", name, pair, interval)
	} else if added == 1 {
		log.Infof("%-9s %7s, received %6dm ticks, storing      1 entries %s", name, pair,
			interval, firstTime.Format(dateTemplate))

		/*log.Infof("%10s %7s, received      1  tick %14s %s", name, pair,
		fmt.Sprintf("(%dm)", interval), firstTime.Format(dateTemplate))*/
	} else {
		log.Infof("%-9s %7s, received %6dm ticks, storing %6v entries %s to %s", name, pair,
			interval, added, firstTime.Format(dateTemplate), lastTime.Format(dateTemplate))

		/*log.Infof("%10s %7s, received %6v ticks %14s %s to %s",
		name, pair, added, fmt.Sprintf("(%dm each)", interval), firstTime.Format(dateTemplate), lastTime.Format(dateTemplate))*/
	}
	return lastTime, nil
}

func (pg *PgDb) SaveExchangeFromSync(ctx context.Context, exchangeData interface{}) error {
	exchange := exchangeData.(ticks.ExchangeData)
	_, err := models.Exchanges(models.ExchangeWhere.Name.EQ(exchange.Name)).One(ctx, pg.db)
	if err != nil {
		if err == sql.ErrNoRows {
			newXch := models.Exchange{
				ID:   exchange.ID,
				Name: exchange.Name,
				URL:  exchange.WebsiteURL,
			}
			err = newXch.Insert(ctx, pg.db, boil.Infer())
			return err
		}
	}
	return err
}

// AllExchange fetches a slice of all exchange from the db
func (pg *PgDb) AllExchange(ctx context.Context) (models.ExchangeSlice, error) {
	exchangeSlice, err := models.Exchanges(models.ExchangeWhere.Name.NEQ("bluetrade")).All(ctx, pg.db)
	return exchangeSlice, err
}

func (pg *PgDb) FetchExchangeForSync(ctx context.Context, lastID int, skip, take int) ([]ticks.ExchangeData, int64, error) {
	exchangeSlice, err := models.Exchanges(
		models.ExchangeWhere.ID.GT(lastID),
		qm.Offset(skip), qm.Limit(take),
	).All(ctx, pg.db)
	if err != nil {
		return nil, 0, err
	}
	var exchanges []ticks.ExchangeData
	for _, exchange := range exchangeSlice {
		exchanges = append(exchanges, ticks.ExchangeData{
			ID:         exchange.ID,
			Name:       exchange.Name,
			WebsiteURL: exchange.URL,
		})
	}

	totalCount, err := models.Exchanges(models.ExchangeWhere.ID.GT(lastID)).Count(ctx, pg.db)

	return exchanges, totalCount, nil
}

func (pg *PgDb) ExchangeTickCount(ctx context.Context) (int64, error) {
	return models.ExchangeTicks().Count(ctx, pg.db)
}

// FetchExchangeTicks fetches a slice exchange ticks of the supplied exchange name
func (pg *PgDb) FetchExchangeTicks(ctx context.Context, currencyPair, name string, interval, offset, limit int) ([]ticks.TickDto, int64, error) {
	query := []qm.QueryMod{
		qm.Load("Exchange"),
	}
	if name != "All" && name != "" {
		exchange, err := models.Exchanges(models.ExchangeWhere.Name.EQ(name)).One(ctx, pg.db)
		if err != nil {
			return nil, 0, err
		}
		query = append(query, models.ExchangeTickWhere.ExchangeID.EQ(exchange.ID))
	}

	if currencyPair != "" && currencyPair != "All" {
		query = append(query, models.ExchangeTickWhere.CurrencyPair.EQ(currencyPair))
	}

	if interval > 0 {
		query = append(query, models.ExchangeTickWhere.Interval.EQ(interval))
	}

	exchangeTickSliceCount, err := models.ExchangeTicks(query...).Count(ctx, pg.db)

	if err != nil {
		return nil, 0, err
	}

	query = append(query,
		qm.Limit(limit),
		qm.Offset(offset),
		qm.OrderBy(fmt.Sprintf("%s DESC", models.ExchangeTickColumns.Time)),
	)

	exchangeTickSlice, err := models.ExchangeTicks(query...).All(ctx, pg.db)
	if err != nil {
		return nil, 0, err
	}

	tickDtos := []ticks.TickDto{}
	for _, tick := range exchangeTickSlice {
		tickDtos = append(tickDtos, ticks.TickDto{
			ExchangeID:   tick.ExchangeID,
			Interval:     tick.Interval,
			CurrencyPair: tick.CurrencyPair,
			Time:         tick.Time.Format(dateTemplate),
			Close:        tick.Close,
			ExchangeName: tick.R.Exchange.Name,
			High:         tick.High,
			Low:          tick.Low,
			Open:         tick.Open,
			Volume:       tick.Volume,
		})
	}

	return tickDtos, exchangeTickSliceCount, err
}

// FetchExchangeTicks fetches a slice exchange ticks of the supplied exchange name
// todo impliment sorting for Exchange ticks as it is currently been sorted by time
func (pg *PgDb) AllExchangeTicks(ctx context.Context, currencyPair string, interval, offset, limit int) ([]ticks.TickDto, int64, error) {
	var exchangeTickSlice models.ExchangeTickSlice
	var exchangeTickSliceCount int64
	var err error

	var queries []qm.QueryMod
	if currencyPair != "" {
		queries = append(queries, models.ExchangeTickWhere.CurrencyPair.EQ(currencyPair))
	}
	if interval != -1 {
		queries = append(queries, models.ExchangeTickWhere.Interval.EQ(interval))
	}

	exchangeTickSliceCount, err = models.ExchangeTicks(queries...).Count(ctx, pg.db)
	if err != nil {
		return nil, 0, err
	}

	queries = append(queries, qm.Load("Exchange"), qm.Limit(limit),
		qm.Offset(offset), qm.OrderBy(fmt.Sprintf("%s DESC", models.ExchangeTickColumns.Time)))

	exchangeTickSlice, err = models.ExchangeTicks(queries...).All(ctx, pg.db)
	if err != nil {
		return nil, 0, err
	}

	tickDtos := []ticks.TickDto{}
	for _, tick := range exchangeTickSlice {
		tickDtos = append(tickDtos, ticks.TickDto{
			ExchangeID:   tick.ExchangeID,
			Interval:     tick.Interval,
			CurrencyPair: tick.CurrencyPair,
			Time:         tick.Time.Format(dateTemplate),
			Close:        tick.Close,
			ExchangeName: tick.R.Exchange.Name,
			High:         tick.High,
			Low:          tick.Low,
			Open:         tick.Open,
			Volume:       tick.Volume,
		})
	}

	return tickDtos, exchangeTickSliceCount, err
}

func (pg *PgDb) AllExchangeTicksCurrencyPair(ctx context.Context) ([]ticks.TickDtoCP, error) {
	exchangeTickCPSlice, err := models.ExchangeTicks(qm.Select("currency_pair"), qm.GroupBy("currency_pair"), 
	qm.OrderBy("currency_pair")).All(ctx, pg.db)

	if err != nil {
		return nil, err
	}

	var currencyPairs []ticks.TickDtoCP
	for _, cp := range exchangeTickCPSlice {
		currencyPairs = append(currencyPairs, ticks.TickDtoCP{cp.CurrencyPair})
	}

	return currencyPairs, err
}

func (pg *PgDb) CurrencyPairByExchange(ctx context.Context, exchangeName string) ([]ticks.TickDtoCP, error) {
	var query []qm.QueryMod
	if exchangeName != "All" && exchangeName != "" {
		exchange, err := models.Exchanges(models.ExchangeWhere.Name.EQ(exchangeName)).One(ctx, pg.db)
		if err != nil {
			return nil, err
		}
		query = append(query, models.ExchangeTickWhere.ExchangeID.EQ(exchange.ID))
	}

	query = append(query, qm.Select("currency_pair"), qm.GroupBy("currency_pair"), qm.OrderBy("currency_pair"))
	exchangeTickCPSlice, err := models.ExchangeTicks(query...).All(ctx, pg.db)
	if err != nil {
		return nil, err
	}

	TickDtoCP := []ticks.TickDtoCP{}
	for _, cp := range exchangeTickCPSlice {
		TickDtoCP = append(TickDtoCP, ticks.TickDtoCP{
			CurrencyPair: cp.CurrencyPair,
		})
	}

	return TickDtoCP, err
}

func (pg *PgDb) AllExchangeTicksInterval(ctx context.Context) ([]ticks.TickDtoInterval, error) {
	exchangeTickIntervalSlice, err := models.ExchangeTicks(qm.Select("interval"), qm.GroupBy("interval"), qm.OrderBy("interval")).All(ctx, pg.db)
	if err != nil {
		return nil, err
	}

	TickDtoInterval := []ticks.TickDtoInterval{}
	for _, item := range exchangeTickIntervalSlice {
		TickDtoInterval = append(TickDtoInterval, ticks.TickDtoInterval{
			Interval: item.Interval,
		})
	}

	return TickDtoInterval, err
}

func (pg *PgDb) TickIntervalsByExchangeAndPair(ctx context.Context, exchangeName string, currencyPair string) ([]ticks.TickDtoInterval, error) {
	var query []qm.QueryMod
	if exchangeName != "All" && exchangeName != "" {
		exchange, err := models.Exchanges(models.ExchangeWhere.Name.EQ(exchangeName)).One(ctx, pg.db)
		if err != nil {
			return nil, err
		}
		query = append(query, models.ExchangeTickWhere.ExchangeID.EQ(exchange.ID))
	}
	
	query = append(query, models.ExchangeTickWhere.CurrencyPair.EQ(currencyPair),
		qm.Select("interval"), qm.GroupBy("interval"), qm.OrderBy("interval"))

	exchangeTickIntervalSlice, err := models.ExchangeTicks(query...).All(ctx, pg.db)

	if err != nil {
		return nil, err
	}

	TickDtoInterval := []ticks.TickDtoInterval{}
	for _, item := range exchangeTickIntervalSlice {
		TickDtoInterval = append(TickDtoInterval, ticks.TickDtoInterval{
			Interval: item.Interval,
		})
	}

	return TickDtoInterval, err
}

func (pg *PgDb) ExchangeTicksChartData(ctx context.Context, selectedTick string, currencyPair string, selectedInterval int, source string) ([]ticks.TickChartData, error) {
	exchange, err := models.Exchanges(models.ExchangeWhere.Name.EQ(source)).One(ctx, pg.db)
	if err != nil {
		return nil, fmt.Errorf("The selected exchange, %s does not exist, %s", source, err.Error())
	}

	queryMods := []qm.QueryMod{
		qm.Select(selectedTick, models.ExchangeTickColumns.Time),
		models.ExchangeTickWhere.CurrencyPair.EQ(currencyPair),
		models.ExchangeTickWhere.ExchangeID.EQ(exchange.ID),
		qm.OrderBy(models.ExchangeTickColumns.Time),
	}
	if selectedInterval != -1 {
		queryMods = append(queryMods, models.ExchangeTickWhere.Interval.EQ(selectedInterval))
	}

	exchangeFilterResult, err := models.ExchangeTicks(queryMods...).All(ctx, pg.db)
	if err != nil {
		fmt.Println(err)
		return nil, fmt.Errorf("Error in fetching exchange tick, %s", err.Error())
	}

	var Filter float64
	tickChart := []ticks.TickChartData{}
	for _, tick := range exchangeFilterResult {
		if selectedTick == "high" {
			Filter = tick.High
		} else if selectedTick == "low" {
			Filter = tick.Low
		} else if selectedTick == "open" {
			Filter = tick.Open
		} else if selectedTick == "Volume" {
			Filter = tick.Volume
		} else if selectedTick == "close" {
			Filter = tick.Close
		} else {
			Filter = tick.Close
		}

		tickChart = append(tickChart, ticks.TickChartData{
			Time:   tick.Time.UTC(),
			Filter: Filter,
		})
	}

	return tickChart, err
}

func (pg *PgDb) exchangeTicksChartData(ctx context.Context, currencyPair string, selectedInterval int,
	 exchangeName string, exchangeTickTime uint64, cols ...string) (models.ExchangeTickSlice, error) {

	exchange, err := models.Exchanges(models.ExchangeWhere.Name.EQ(exchangeName)).One(ctx, pg.db)
	if err != nil {
		return nil, fmt.Errorf("the selected exchange, %s does not exist, %s", exchangeName, err.Error())
	}

	queryMods := []qm.QueryMod{
		models.ExchangeTickWhere.Time.GT(helpers.UnixTime(int64(exchangeTickTime))),
		models.ExchangeTickWhere.CurrencyPair.EQ(currencyPair),
		models.ExchangeTickWhere.ExchangeID.EQ(exchange.ID),
		models.ExchangeTickWhere.Interval.EQ(selectedInterval),
		qm.OrderBy(models.ExchangeTickColumns.Time),
	}

	if len(cols) > 0 {
		queryMods = append(queryMods, qm.Select(cols...))
	}

	exchangeFilterResult, err := models.ExchangeTicks(queryMods...).All(ctx, pg.db)
	if err != nil {
		fmt.Println(err)
		return nil, fmt.Errorf("error in fetching exchange tick, %s", err.Error())
	}

	return exchangeFilterResult, nil
}

// FetchExchangeTicks fetches a slice exchange ticks for the sync operation
func (pg *PgDb) FetchExchangeTicksForSync(ctx context.Context, date time.Time, skip, take int) ([]ticks.TickSyncDto, int64, error) {
	query := []qm.QueryMod{
		qm.Load("Exchange"),
		models.ExchangeTickWhere.Time.GTE(date),
	}

	exchangeTickSliceCount, err := models.ExchangeTicks(query...).Count(ctx, pg.db)

	if err != nil {
		return nil, 0, err
	}

	query = append(query,
		qm.Limit(take),
		qm.Offset(skip),
		qm.OrderBy(fmt.Sprintf("%s DESC", models.ExchangeTickColumns.Time)),
	)

	exchangeTickSlice, err := models.ExchangeTicks(query...).All(ctx, pg.db)
	if err != nil {
		return nil, 0, err
	}

	var tickDtos []ticks.TickSyncDto
	for _, tick := range exchangeTickSlice {
		tickDtos = append(tickDtos, ticks.TickSyncDto{
			ID:           tick.ID,
			ExchangeID:   tick.ExchangeID,
			Interval:     tick.Interval,
			CurrencyPair: tick.CurrencyPair,
			Time:         tick.Time,
			Close:        tick.Close,
			ExchangeName: tick.R.Exchange.Name,
			High:         tick.High,
			Low:          tick.Low,
			Open:         tick.Open,
			Volume:       tick.Volume,
		})
	}

	return tickDtos, exchangeTickSliceCount, err
}

func (pg *PgDb) SaveExchangeTickFromSync(ctx context.Context, tickData interface{}) error {
	tick := tickData.(ticks.TickSyncDto)

	tickModel := models.ExchangeTick{
		ID:           tick.ID,
		ExchangeID:   tick.ExchangeID,
		Interval:     tick.Interval,
		High:         tick.High,
		Low:          tick.Low,
		Open:         tick.Open,
		Close:        tick.Close,
		Volume:       tick.Volume,
		CurrencyPair: tick.CurrencyPair,
		Time:         tick.Time,
	}

	err := tickModel.Insert(ctx, pg.db, boil.Infer())
	if isUniqueConstraint(err) {
		return nil
	}

	return err
}

func tickToExchangeTick(exchangeID int, pair string, interval int, tick ticks.Tick) *models.ExchangeTick {
	return &models.ExchangeTick{
		ExchangeID:   exchangeID,
		High:         tick.High,
		Low:          tick.Low,
		Open:         tick.Open,
		Close:        tick.Close,
		Volume:       tick.Volume,
		Time:         tick.Time.UTC(),
		CurrencyPair: pair,
		Interval:     interval,
	}
}

// LastExchangeTickEntryTime
func (pg *PgDb) LastExchangeTickEntryTime() (time time.Time) {
	rows := pg.db.QueryRow(lastExchangeTickEntryTime)
	_ = rows.Scan(&time)
	return
}

// LastExchangeTickEntryTime
func (pg *PgDb) LastExchangeEntryID() (id int64) {
	rows := pg.db.QueryRow(lastExchangeEntryID)
	_ = rows.Scan(&id)
	return
}

type exchangeTickSet struct {
	time  cache.ChartUints
	open  cache.ChartFloats
	close cache.ChartFloats
	high  cache.ChartFloats
	low   cache.ChartFloats
}

func (pg *PgDb) fetchEncodeExchangeChart(ctx context.Context, charts *cache.ChartData, dataType, _ string, binString string, setKey ...string) ([]byte, error) {
	if len(setKey) < 1 {
		return nil, errors.New("exchange set key is required for exchange chart")
	}
	exchangeName, currencyPair, interval := cache.ExtractExchangeKey(setKey[0])
	tickSlice, err := pg.exchangeTicksChartData(ctx, currencyPair, interval, exchangeName, 0,)
	if err != nil {
		return nil, err
	}
	var dates cache.ChartUints
	var yAxis cache.ChartFloats
	for _, t := range tickSlice {
		dates = append(dates, uint64(t.Time.Unix()))
		switch(strings.ToLower(dataType)) {
		case string(cache.ExchangeOpenAxis):
			yAxis = append(yAxis, t.Open)
			break
		case string(cache.ExchangeCloseAxis):
			yAxis = append(yAxis, t.Close)
			break
		case string(cache.ExchangeHighAxis):
			yAxis = append(yAxis, t.High)
			break
		case string(cache.ExchangeLowAxis):
			yAxis = append(yAxis, t.Low)
			break
		}
	}

	return charts.Encode(nil, dates, yAxis)
}

func (pg *PgDb) fetchExchangeChart(ctx context.Context, charts *cache.ChartData, _ int) (interface{}, func(), bool, error) {
	cancel := func() {}
	exchanges, err := pg.AllExchange(ctx)
	if err != nil {
		return nil, cancel, false, err
	}

	exchangeTickIntervals := []int{
		5,
		60,
		120,
		1440,
	}

	currencyPairs, err := pg.AllExchangeTicksCurrencyPair(ctx)
	if err != nil {
		return nil, cancel, false, fmt.Errorf("Chart:Updater:Exchange cannot fetch currency pair, %s", err.Error())
	}

	var tickSets = map[string]exchangeTickSet{}
	for _, currencyPair := range currencyPairs {
		for _, exchange := range exchanges {
			for _, interval := range exchangeTickIntervals {
				key := cache.BuildExchangeKey(exchange.Name, currencyPair.CurrencyPair, interval)
				exchangeTickTime := charts.ExchangeSetTime(key)
				tickSlice, err := pg.exchangeTicksChartData(ctx, currencyPair.CurrencyPair, interval, exchange.Name, exchangeTickTime)
				if err != nil && err != sql.ErrNoRows {
					return nil, cancel, false, err
				}

				var tickSet exchangeTickSet
				for _, exchangeTick := range tickSlice {
					tickSet.time = append(tickSet.time, uint64(exchangeTick.Time.UTC().Unix()))
					tickSet.open = append(tickSet.open, exchangeTick.Open)
					tickSet.close = append(tickSet.close, exchangeTick.Close)
					tickSet.high = append(tickSet.high, exchangeTick.High)
					tickSet.low = append(tickSet.low, exchangeTick.Low)
				}

				tickSets[key] = tickSet
			}
		}
	}

	return tickSets, cancel, true, nil
}

func appendExchangeChart(charts *cache.ChartData, data interface{}) error {
	var tickSets = data.(map[string]exchangeTickSet)
	keyExists := func (arr []string, key string) bool {
		for _, s := range arr {
			if s == key {
				return true
			}
		}
		return false
	}
	for key, tickSet := range tickSets {
		if !keyExists(charts.ExchangeKeys, key) {
			charts.ExchangeKeys = append(charts.ExchangeKeys, key)
		}
		if len(tickSet.time) == 0 {
			continue
		}
		if err := charts.AppendChartUintsAxis(key+"-"+string(cache.TimeAxis), tickSet.time); err != nil {
			log.Errorf("Error in append exchange time, %s", err.Error())
		}
		if err := charts.AppendChartFloatsAxis(key+"-"+string(cache.ExchangeOpenAxis), tickSet.open); err != nil {
			log.Errorf("Error in append exchange open axis, %s", err.Error())
		}
		if err := charts.AppendChartFloatsAxis(key+"-"+string(cache.ExchangeCloseAxis), tickSet.close); err != nil {
			log.Errorf("Error in append exchange close axis, %s", err.Error())
		}
		if err := charts.AppendChartFloatsAxis(key+"-"+string(cache.ExchangeHighAxis), tickSet.high); err != nil {
			log.Errorf("Error in append exchange high axis, %s", err.Error())
		}
		if err := charts.AppendChartFloatsAxis(key+"-"+string(cache.ExchangeLowAxis), tickSet.low); err != nil {
			log.Errorf("Error in append exchange low axis, %s", err.Error())
		}
	}

	return nil
}
