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

	"github.com/raedahgroup/dcrextdata/postgres/models"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"

	"github.com/raedahgroup/dcrextdata/exchanges/ticks"
)

const (
	NegativeFiveMin = time.Duration(-5) * time.Minute
	NegativeOneHour = time.Duration(-1) * time.Hour
	NegativeOneDay  = time.Duration(-24) * time.Hour
)

var (
	ErrNonConsecutiveTicks = errors.New("postgres/exchanges: Non consecutive exchange ticks")
	zeroTime               time.Time
)

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
		return zeroTime, fmt.Errorf("No ticks recieved for %s", name)
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
		// if tick.Time.Unix() <= lastTime.Unix() {
		// 	continue
		// }
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

// AllExchange fetches a slice of all exchange from the db
func (pg *PgDb) AllExchange(ctx context.Context) (models.ExchangeSlice, error) {
	exchangeSlice, err := models.Exchanges().All(ctx, pg.db)
	return exchangeSlice, err
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
	exchangeTickCPSlice, err := models.ExchangeTicks(qm.Select("currency_pair"), qm.GroupBy("currency_pair"), qm.OrderBy("currency_pair")).All(ctx, pg.db)

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
