// Copyright (c) 2018-2019 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/planetdecred/dcrextdata/app/helpers"
	"github.com/planetdecred/dcrextdata/cache"
	"github.com/planetdecred/dcrextdata/datasync"
	"github.com/planetdecred/dcrextdata/postgres/models"
	"github.com/planetdecred/dcrextdata/vsp"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"github.com/volatiletech/sqlboiler/types"
)

var (
	vspTickExistsErr = fmt.Errorf("VSPTick exists")
)

func (pg *PgDb) VspTableName() string {
	return models.TableNames.VSP
}

func (pg *PgDb) VspTickTableName() string {
	return models.TableNames.VSPTick
}

// StoreVSPs attempts to store the vsp responses by calling storeVspResponseG and returning
// a slice of errors
func (pg *PgDb) StoreVSPs(ctx context.Context, data vsp.Response) (int, []error) {
	if ctx.Err() != nil {
		return 0, []error{ctx.Err()}
	}
	errs := make([]error, 0, len(data))
	completed := 0
	for name, tick := range data {
		err := pg.storeVspResponse(ctx, name, tick)
		if err == nil {
			completed++
		} else if err != vspTickExistsErr {
			log.Trace(err)
			errs = append(errs, err)
		}
		if ctx.Err() != nil {
			return 0, append(errs, ctx.Err())
		}
	}
	if completed == 0 {
		log.Info("Unable to store any vsp entry")
	}
	return completed, errs
}

func (pg *PgDb) storeVspResponse(ctx context.Context, name string, resp *vsp.ResposeData) error {
	txr, err := pg.db.Begin()
	if err != nil {
		return err
	}

	pool, err := models.VSPS(models.VSPWhere.Name.EQ(null.StringFrom(name))).One(ctx, pg.db)
	if err == sql.ErrNoRows {
		pool = responseToVSP(name, resp)
		err := pg.tryInsert(ctx, txr, pool)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	vspTick := responseToVSPTick(pool.ID, resp)

	err = vspTick.Insert(ctx, pg.db, boil.Infer())
	if err != nil {
		errR := txr.Rollback()
		if errR != nil {
			return err
		}
		if strings.Contains(err.Error(), "unique constraint") {
			return vspTickExistsErr
		}
		return err
	}

	err = txr.Commit()
	if err != nil {
		return txr.Rollback()
	}
	return nil
}

func responseToVSP(name string, resp *vsp.ResposeData) *models.VSP {
	return &models.VSP{
		Name:                 null.StringFrom(name),
		APIEnabled:           null.BoolFrom(resp.APIEnabled),
		APIVersionsSupported: types.Int64Array(resp.APIVersionsSupported),
		Network:              null.StringFrom(resp.Network),
		URL:                  null.StringFrom(resp.URL),
		Launched:             null.TimeFrom(helpers.UnixTime(resp.Launched)),
	}
}

func responseToVSPTick(poolID int, resp *vsp.ResposeData) *models.VSPTick {
	return &models.VSPTick{
		VSPID:            poolID,
		Immature:         resp.Immature,
		Live:             resp.Live,
		Voted:            resp.Voted,
		Missed:           resp.Missed,
		PoolFees:         resp.PoolFees,
		ProportionLive:   resp.ProportionLive,
		ProportionMissed: resp.ProportionMissed,
		UserCount:        resp.UserCount,
		UsersActive:      resp.UserCountActive,
		Time:             helpers.UnixTime(resp.LastUpdated),
	}
}

func (pg *PgDb) FetchVSPs(ctx context.Context) ([]vsp.VSPDto, error) {
	vspData, err := models.VSPS(qm.OrderBy(models.VSPColumns.URL), qm.OrderBy(models.VSPColumns.Name)).All(ctx, pg.db)
	if err != nil {
		return nil, err
	}

	var result []vsp.VSPDto
	for _, item := range vspData {
		parsedURL, err := url.Parse(item.URL.String)
		if err != nil {
			return nil, err
		}
		result = append(result, vsp.VSPDto{
			ID:                   item.ID,
			Name:                 item.Name.String,
			APIEnabled:           item.APIEnabled.Bool,
			APIVersionsSupported: item.APIVersionsSupported,
			Network:              item.Network.String,
			URL:                  item.URL.String,
			Host:                 parsedURL.Host,
			Launched:             item.Launched.Time,
		})
	}

	return result, nil
}

func (pg *PgDb) AddVspSourceFromSync(ctx context.Context, vspData interface{}) error {
	vspDto := vspData.(vsp.VSPDto)
	count, _ := models.VSPS(models.VSPWhere.Name.EQ(null.StringFrom(vspDto.Name))).Count(ctx, pg.db)
	if count > 0 {
		return nil
	}
	vspModel := models.VSP{
		ID:                   vspDto.ID,
		Name:                 null.StringFrom(vspDto.Name),
		APIEnabled:           null.BoolFrom(vspDto.APIEnabled),
		APIVersionsSupported: vspDto.APIVersionsSupported,
		Network:              null.StringFrom(vspDto.Network),
		URL:                  null.StringFrom(vspDto.URL),
		Launched:             null.TimeFrom(vspDto.Launched),
	}
	err := vspModel.Insert(ctx, pg.db, boil.Infer())
	return err
}

func (pg *PgDb) FetchVspSourcesForSync(ctx context.Context, lastID int64, skip, take int) ([]vsp.VSPDto, int64, error) {
	vspData, err := models.VSPS(
		models.VSPWhere.ID.GT(int(lastID)),
		qm.Offset(skip), qm.Limit(take)).All(ctx, pg.db)
	if err != nil {
		return nil, 0, err
	}

	var result []vsp.VSPDto
	for _, item := range vspData {
		parsedURL, err := url.Parse(item.URL.String)
		if err != nil {
			return nil, 0, err
		}
		result = append(result, vsp.VSPDto{
			ID:                   item.ID,
			Name:                 item.Name.String,
			APIEnabled:           item.APIEnabled.Bool,
			APIVersionsSupported: item.APIVersionsSupported,
			Network:              item.Network.String,
			URL:                  item.URL.String,
			Host:                 parsedURL.Host,
			Launched:             item.Launched.Time,
		})
	}

	totalCount, err := models.VSPS(models.VSPWhere.ID.GT(int(lastID))).Count(ctx, pg.db)

	return result, totalCount, err
}

// VSPTicks
func (pg *PgDb) FetchVspTicksForSync(ctx context.Context, lastID int64, skip, take int) ([]datasync.VSPTickSyncDto, int64, error) {
	vspIdQuery := models.VSPTickWhere.ID.GT(int(lastID))

	vspTickSlice, err := models.VSPTicks(
		vspIdQuery,
		qm.OrderBy(models.VSPTickColumns.ID),
		qm.Limit(take), qm.Offset(skip)).All(ctx, pg.db)
	if err != nil {
		return nil, 0, err
	}

	vspTickCount, err := models.VSPTicks(vspIdQuery).Count(ctx, pg.db)
	if err != nil {
		return nil, 0, err
	}

	var vspTicks = make([]datasync.VSPTickSyncDto, len(vspTickSlice))
	for i, tick := range vspTickSlice {
		vspTicks[i] = pg.vspTickModelToSyncDto(tick)
	}

	return vspTicks, vspTickCount, nil
}

func (pg *PgDb) AddVspTicksFromSync(ctx context.Context, tick datasync.VSPTickSyncDto) error {
	if _, err := models.VSPTicks(models.VSPTickWhere.VSPID.EQ(tick.VSPID),
		models.VSPTickWhere.Time.EQ(tick.Time)).One(ctx, pg.db); err == nil {
		return nil // record exists
	}
	tickModel := models.VSPTick{
		ID:               tick.ID,
		VSPID:            tick.VSPID,
		Immature:         tick.Immature,
		Live:             tick.Live,
		Voted:            tick.Voted,
		Missed:           tick.Missed,
		PoolFees:         tick.PoolFees,
		ProportionLive:   tick.ProportionLive,
		ProportionMissed: tick.ProportionMissed,
		UserCount:        tick.UserCount,
		UsersActive:      tick.UsersActive,
		Time:             tick.Time,
	}

	return tickModel.Insert(ctx, pg.db, boil.Infer())
}

func (pg *PgDb) FilteredVSPTicks(ctx context.Context, vspName string, offset, limit int) ([]vsp.VSPTickDto, int64, error) {

	vspInfo, err := models.VSPS(models.VSPWhere.Name.EQ(null.StringFrom(vspName))).One(ctx, pg.db)
	if err != nil {
		log.Errorf("Error in FilteredVSPTicks - %s", err.Error())
		return nil, 0, err
	}

	vspIdQuery := models.VSPTickWhere.VSPID.EQ(vspInfo.ID)
	vspTickCount, err := models.VSPTicks(vspIdQuery).Count(ctx, pg.db)
	if err != nil {
		return nil, 0, err
	}

	statement := `SELECT 
			t.id, 
			s.name as vsp,
			t.immature,
			t.live,
			t.voted,
			t.missed,
			t.pool_fees,
			t.proportion_live,
			t.proportion_missed,
			t.user_count,
			t.users_active,
			t.time
		FROM vsp_tick t
		INNER JOIN vsp s ON t.vsp_id = s.id
		WHERE t.vsp_id = $1 
		ORDER BY t.time DESC 
		LIMIT $2 OFFSET $3`

	var vspTicks []vsp.VSPTickDto
	if err = models.NewQuery(qm.SQL(statement, vspInfo.ID, limit, offset)).Bind(ctx, pg.db, &vspTicks); err != nil {
		log.Errorf("Error in FilteredVSPTicks - %s", err.Error())
		return nil, 0, err
	}
	return vspTicks, vspTickCount, nil
}

// VSPTicks
func (pg *PgDb) AllVSPTicks(ctx context.Context, offset, limit int) ([]vsp.VSPTickDto, int64, error) {

	vspTickCount, err := models.VSPTicks().Count(ctx, pg.db)
	if err != nil {
		log.Errorf("Error in AllVSPTicks - %s", err.Error())
		return nil, 0, err
	}

	statement := `SELECT 
		t.id, 
		s.name as vsp,
		t.immature,
		t.live,
		t.voted,
		t.missed,
		t.pool_fees,
		t.proportion_live,
		t.proportion_missed,
		t.user_count,
		t.users_active,
		t.time
		FROM vsp_tick t
		INNER JOIN vsp s ON t.vsp_id = s.id
		ORDER BY time DESC
		LIMIT $1 OFFSET $2`

	var vspTicks []vsp.VSPTickDto
	if err = models.NewQuery(qm.SQL(statement, limit, offset)).Bind(ctx, pg.db, &vspTicks); err != nil {
		log.Errorf("Error in AllVSPTicks - %s", err.Error())
		return nil, 0, err
	}
	return vspTicks, vspTickCount, nil
}

func (pg *PgDb) vspTickModelToDto(tick *models.VSPTick) vsp.VSPTickDto {
	return vsp.VSPTickDto{
		ID:               tick.ID,
		VSP:              tick.R.VSP.Name.String,
		Time:             tick.Time.Format(dateTemplate),
		Immature:         tick.Immature,
		Live:             tick.Live,
		Missed:           tick.Missed,
		PoolFees:         tick.PoolFees,
		ProportionLive:   RoundValue(tick.ProportionLive),
		ProportionMissed: RoundValue(tick.ProportionMissed),
		UserCount:        tick.UserCount,
		UsersActive:      tick.UsersActive,
		Voted:            tick.Voted,
	}
}

func (pg *PgDb) vspTickModelToSyncDto(tick *models.VSPTick) datasync.VSPTickSyncDto {
	return datasync.VSPTickSyncDto{
		ID:               tick.ID,
		VSPID:            tick.VSPID,
		Time:             tick.Time,
		Immature:         tick.Immature,
		Live:             tick.Live,
		Missed:           tick.Missed,
		PoolFees:         tick.PoolFees,
		ProportionLive:   tick.ProportionLive,
		ProportionMissed: tick.ProportionMissed,
		UserCount:        tick.UserCount,
		UsersActive:      tick.UsersActive,
		Voted:            tick.Voted,
	}
}

func (pg *PgDb) LastVspTickEntryTime() (time time.Time) {
	rows := pg.db.QueryRow(lastVspTickEntryTime)
	_ = rows.Scan(&time)
	return
}

func (pg *PgDb) VspTickCount(ctx context.Context) (int64, error) {
	return models.VSPTicks().Count(ctx, pg.db)
}

func (pg *PgDb) fetchVSPChartData(ctx context.Context, vspName string, start time.Time, endDate uint64, axisString string) (records models.VSPTickSlice, err error) {
	vspInfo, err := models.VSPS(models.VSPWhere.Name.EQ(null.StringFrom(vspName))).One(ctx, pg.db)
	if err != nil {
		return nil, err
	}
	var queries []qm.QueryMod
	if axisString != "" {
		var col string
		switch strings.ToLower(axisString) {
		case string(cache.ImmatureAxis):
			col = models.VSPTickColumns.Immature

		case string(cache.LiveAxis):
			col = models.VSPTickColumns.Live

		case string(cache.VotedAxis):
			col = models.VSPTickColumns.Voted

		case string(cache.MissedAxis):
			col = models.VSPTickColumns.Missed

		case string(cache.PoolFeesAxis):
			col = models.VSPTickColumns.PoolFees

		case string(cache.ProportionLiveAxis):
			col = models.VSPTickColumns.ProportionLive

		case string(cache.ProportionMissedAxis):
			col = models.VSPTickColumns.ProportionMissed

		case string(cache.UserCountAxis):
			col = models.VSPTickColumns.UserCount

		case string(cache.UsersActiveAxis):
			col = models.VSPTickColumns.UsersActive
		}
		queries = append(queries, qm.Select(models.VSPTickColumns.Time, col))
	}

	queries = append(queries, models.VSPTickWhere.VSPID.EQ(vspInfo.ID), models.VSPTickWhere.Time.GT(start))
	if endDate > 0 {
		queries = append(queries, models.VSPTickWhere.Time.LTE(helpers.UnixTime(int64(endDate))))
	}
	data, err := models.VSPTicks(queries...).All(ctx, pg.db)
	return data, err
}

func (pg *PgDb) allVspTickDates(ctx context.Context, start time.Time, vspSources ...string) ([]time.Time, error) {

	var query = []qm.QueryMod{
		qm.Select(fmt.Sprintf("distinct(%s)", models.VSPTickColumns.Time)),
		models.VSPTickWhere.Time.GT(start),
		qm.OrderBy(models.VSPTickColumns.Time),
	}
	var wheres []string
	if len(vspSources) > 0 {
		var args = make([]interface{}, len(vspSources))
		for i, s := range vspSources {
			args[i] = s
			wheres = append(wheres, fmt.Sprintf("%s = $%d", models.VSPColumns.Name, i+1))
		}
		vsps, err := models.VSPS(
			qm.Where(strings.Join(wheres, " OR "), args...),
		).All(ctx, pg.db)
		if err != nil {
			return nil, err
		}

		args = make([]interface{}, len(vsps))
		wheres = make([]string, len(vsps))
		for i, v := range vsps {
			args[i] = v.ID
			wheres[i] = fmt.Sprintf("%s = %d", models.VSPTickColumns.VSPID, v.ID)
		}
		query = append(query, qm.Where(strings.Join(wheres, " OR ")))
	}

	vspDates, err := models.VSPTicks(
		query...,
	).All(ctx, pg.db)

	if err != nil {
		return nil, err
	}

	var dates []time.Time
	var unique = map[time.Time]bool{}

	for _, data := range vspDates {
		if _, found := unique[data.Time]; !found {
			dates = append(dates, data.Time)
			unique[data.Time] = true
		}
	}
	return dates, nil
}

func (pg *PgDb) vspIdByName(ctx context.Context, name string) (id int, err error) {
	vspModel, err := models.VSPS(models.VSPWhere.Name.EQ(null.StringFrom(name))).One(ctx, pg.db)
	if err != nil {
		return 0, err
	}
	return vspModel.ID, nil
}

type vspSet struct {
	time             cache.ChartUints
	immature         map[string]cache.ChartNullUints
	live             map[string]cache.ChartNullUints
	voted            map[string]cache.ChartNullUints
	missed           map[string]cache.ChartNullUints
	poolFees         map[string]cache.ChartNullFloats
	proportionLive   map[string]cache.ChartNullFloats
	proportionMissed map[string]cache.ChartNullFloats
	userCount        map[string]cache.ChartNullUints
	usersActive      map[string]cache.ChartNullUints
}

func (pg *PgDb) fetchEncodeVspChart(ctx context.Context, charts *cache.Manager, dataType, _ string, binString string, vspSources ...string) ([]byte, error) {
	if binString != string(cache.DefaultBin) {
		return pg.fetchEncodeBinVspChart(ctx, charts, binString, dataType, vspSources...)
	}
	data, err := pg.fetchVspChart(ctx, 0, dataType, vspSources...)
	if err != nil {
		return nil, err
	}
	switch strings.ToLower(dataType) {
	case string(cache.ImmatureAxis):
		var deviations []cache.ChartNullData
		for _, p := range vspSources {
			deviations = append(deviations, data.immature[p])
		}
		return cache.MakeVspChart(charts, data.time, deviations, vspSources)

	case string(cache.LiveAxis):
		var deviations []cache.ChartNullData
		for _, p := range vspSources {
			deviations = append(deviations, data.live[p])
		}
		return cache.MakeVspChart(charts, data.time, deviations, vspSources)

	case string(cache.VotedAxis):
		var deviations []cache.ChartNullData
		for _, p := range vspSources {
			deviations = append(deviations, data.voted[p])
		}
		return cache.MakeVspChart(charts, data.time, deviations, vspSources)

	case string(cache.MissedAxis):
		var deviations []cache.ChartNullData
		for _, p := range vspSources {
			deviations = append(deviations, data.missed[p])
		}
		return cache.MakeVspChart(charts, data.time, deviations, vspSources)

	case string(cache.PoolFeesAxis):
		var deviations []cache.ChartNullData
		for _, p := range vspSources {
			deviations = append(deviations, data.poolFees[p])
		}
		return cache.MakeVspChart(charts, data.time, deviations, vspSources)

	case string(cache.ProportionLiveAxis):
		var deviations []cache.ChartNullData
		for _, p := range vspSources {
			deviations = append(deviations, data.proportionLive[p])
		}
		return cache.MakeVspChart(charts, data.time, deviations, vspSources)

	case string(cache.ProportionMissedAxis):
		var deviations []cache.ChartNullData
		for _, p := range vspSources {
			deviations = append(deviations, data.proportionMissed[p])
		}
		return cache.MakeVspChart(charts, data.time, deviations, vspSources)

	case string(cache.UserCountAxis):
		var deviations []cache.ChartNullData
		for _, p := range vspSources {
			deviations = append(deviations, data.userCount[p])
		}
		return cache.MakeVspChart(charts, data.time, deviations, vspSources)

	case string(cache.UsersActiveAxis):
		var deviations []cache.ChartNullData
		for _, p := range vspSources {
			deviations = append(deviations, data.usersActive[p])
		}
		return cache.MakeVspChart(charts, data.time, deviations, vspSources)
	}
	return nil, cache.UnknownChartErr
}

func (pg *PgDb) fetchEncodeBinVspChart(ctx context.Context, charts *cache.Manager, binString, dataType string, vspSources ...string) ([]byte, error) {
	var dates cache.ChartUints
	var dateMap = make(map[int64]bool)
	var deviations []cache.ChartNullData
	for _, s := range vspSources {
		vsp, err := models.VSPS(
			models.VSPWhere.Name.EQ(null.StringFrom(s)),
		).One(ctx, pg.db)
		if err != nil {
			return nil, err
		}

		data, err := models.VSPTickBins(
			models.VSPTickBinWhere.Bin.EQ(binString),
			models.VSPTickBinWhere.VSPID.EQ(vsp.ID),
			qm.OrderBy(models.VSPTickBinColumns.Time),
		).All(ctx, pg.db)
		if err != nil {
			return nil, err
		}
		switch strings.ToLower(dataType) {
		case string(cache.ImmatureAxis):
			var deviation cache.ChartNullUints
			for _, rec := range data {
				if _, f := dateMap[rec.Time]; !f {
					dates = append(dates, uint64(rec.Time))
					dateMap[rec.Time] = true
				}
				deviation = append(deviation, &null.Uint64{Uint64: uint64(rec.Immature.Int), Valid: rec.Immature.Valid})
			}
			deviations = append(deviations, deviation)

		case string(cache.LiveAxis):
			var deviation cache.ChartNullUints
			for _, rec := range data {
				if _, f := dateMap[rec.Time]; !f {
					dates = append(dates, uint64(rec.Time))
					dateMap[rec.Time] = true
				}
				deviation = append(deviation, &null.Uint64{Uint64: uint64(rec.Live.Int), Valid: rec.Live.Valid})
			}
			deviations = append(deviations, deviation)

		case string(cache.VotedAxis):
			var deviation cache.ChartNullUints
			for _, rec := range data {
				if _, f := dateMap[rec.Time]; !f {
					dates = append(dates, uint64(rec.Time))
					dateMap[rec.Time] = true
				}
				deviation = append(deviation, &null.Uint64{Uint64: uint64(rec.Voted.Int), Valid: rec.Voted.Valid})
			}
			deviations = append(deviations, deviation)

		case string(cache.MissedAxis):
			var deviation cache.ChartNullUints
			for _, rec := range data {
				if _, f := dateMap[rec.Time]; !f {
					dates = append(dates, uint64(rec.Time))
					dateMap[rec.Time] = true
				}
				deviation = append(deviation, &null.Uint64{Uint64: uint64(rec.Missed.Int), Valid: rec.Missed.Valid})
			}
			deviations = append(deviations, deviation)

		case string(cache.PoolFeesAxis):
			var deviation cache.ChartNullFloats
			for _, rec := range data {
				if _, f := dateMap[rec.Time]; !f {
					dates = append(dates, uint64(rec.Time))
					dateMap[rec.Time] = true
				}
				deviation = append(deviation, &null.Float64{Float64: rec.PoolFees.Float64, Valid: rec.PoolFees.Valid})
			}
			deviations = append(deviations, deviation)

		case string(cache.ProportionLiveAxis):
			var deviation cache.ChartNullFloats
			for _, rec := range data {
				if _, f := dateMap[rec.Time]; !f {
					dates = append(dates, uint64(rec.Time))
					dateMap[rec.Time] = true
				}
				deviation = append(deviation, &null.Float64{Float64: rec.ProportionLive.Float64, Valid: rec.ProportionLive.Valid})
			}
			deviations = append(deviations, deviation)

		case string(cache.ProportionMissedAxis):
			var deviation cache.ChartNullFloats
			for _, rec := range data {
				if _, f := dateMap[rec.Time]; !f {
					dates = append(dates, uint64(rec.Time))
					dateMap[rec.Time] = true
				}
				deviation = append(deviation, &null.Float64{Float64: rec.ProportionMissed.Float64, Valid: rec.ProportionMissed.Valid})
			}
			deviations = append(deviations, deviation)

		case string(cache.UserCountAxis):
			var deviation cache.ChartNullUints
			for _, rec := range data {
				if _, f := dateMap[rec.Time]; !f {
					dates = append(dates, uint64(rec.Time))
					dateMap[rec.Time] = true
				}
				deviation = append(deviation, &null.Uint64{Uint64: uint64(rec.UserCount.Int), Valid: rec.UserCount.Valid})
			}
			deviations = append(deviations, deviation)

		case string(cache.UsersActiveAxis):
			var deviation cache.ChartNullUints
			for _, rec := range data {
				if _, f := dateMap[rec.Time]; !f {
					dates = append(dates, uint64(rec.Time))
					dateMap[rec.Time] = true
				}
				deviation = append(deviation, &null.Uint64{Uint64: uint64(rec.UsersActive.Int), Valid: rec.UsersActive.Valid})
			}
			deviations = append(deviations, deviation)

		default:
			return nil, cache.UnknownChartErr
		}
	}
	return cache.MakeVspChart(charts, dates, deviations, vspSources)
}

func (pg *PgDb) fetchVspChart(ctx context.Context, startDate uint64, axisString string, vspSources ...string) (*vspSet, error) {
	var vspDataSet = vspSet{
		time:             []uint64{},
		immature:         make(map[string]cache.ChartNullUints),
		live:             make(map[string]cache.ChartNullUints),
		voted:            make(map[string]cache.ChartNullUints),
		missed:           make(map[string]cache.ChartNullUints),
		poolFees:         make(map[string]cache.ChartNullFloats),
		proportionLive:   make(map[string]cache.ChartNullFloats),
		proportionMissed: make(map[string]cache.ChartNullFloats),
		userCount:        make(map[string]cache.ChartNullUints),
		usersActive:      make(map[string]cache.ChartNullUints),
	}

	var vsps []string = vspSources
	if len(vsps) == 0 {
		allVspData, err := pg.FetchVSPs(ctx)
		if err != nil {
			return nil, err
		}
		for _, vspSource := range allVspData {
			vsps = append(vsps, vspSource.Name)
		}
	}

	dates, err := pg.allVspTickDates(ctx, helpers.UnixTime(int64(startDate)), vspSources...)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	for _, date := range dates {
		vspDataSet.time = append(vspDataSet.time, uint64(date.Unix()))
	}

	for _, vspSource := range vsps {
		points, err := pg.fetchVSPChartData(ctx, vspSource, helpers.UnixTime(int64(startDate)), 0, axisString)
		if err != nil {
			if err.Error() == sql.ErrNoRows.Error() {
				continue
			}
			return nil, fmt.Errorf("error in fetching records for %s: %s", vspSource, err.Error())
		}

		var pointsMap = map[time.Time]*models.VSPTick{}
		for _, record := range points {
			pointsMap[record.Time] = record
		}

		var hasFoundOne bool
		for _, date := range dates {
			if record, found := pointsMap[date]; found {
				vspDataSet.immature[vspSource] = append(vspDataSet.immature[vspSource], &null.Uint64{Valid: true, Uint64: uint64(record.Immature)})
				vspDataSet.live[vspSource] = append(vspDataSet.live[vspSource], &null.Uint64{Valid: true, Uint64: uint64(record.Live)})
				vspDataSet.voted[vspSource] = append(vspDataSet.voted[vspSource], &null.Uint64{Valid: true, Uint64: uint64(record.Voted)})
				vspDataSet.missed[vspSource] = append(vspDataSet.missed[vspSource], &null.Uint64{Valid: true, Uint64: uint64(record.Missed)})
				vspDataSet.poolFees[vspSource] = append(vspDataSet.poolFees[vspSource], &null.Float64{Valid: true, Float64: record.PoolFees})
				vspDataSet.proportionLive[vspSource] = append(vspDataSet.proportionLive[vspSource], &null.Float64{Valid: true, Float64: record.ProportionLive})
				vspDataSet.proportionMissed[vspSource] = append(vspDataSet.proportionMissed[vspSource], &null.Float64{Valid: true, Float64: record.ProportionMissed})
				vspDataSet.userCount[vspSource] = append(vspDataSet.userCount[vspSource], &null.Uint64{Valid: true, Uint64: uint64(record.UserCount)})
				vspDataSet.usersActive[vspSource] = append(vspDataSet.usersActive[vspSource], &null.Uint64{Valid: true, Uint64: uint64(record.UsersActive)})
				hasFoundOne = true
			} else {
				if hasFoundOne {
					vspDataSet.immature[vspSource] = append(vspDataSet.immature[vspSource], &null.Uint64{Valid: false})
					vspDataSet.live[vspSource] = append(vspDataSet.live[vspSource], &null.Uint64{Valid: false})
					vspDataSet.voted[vspSource] = append(vspDataSet.voted[vspSource], &null.Uint64{Valid: false})
					vspDataSet.missed[vspSource] = append(vspDataSet.missed[vspSource], &null.Uint64{Valid: false})
					vspDataSet.poolFees[vspSource] = append(vspDataSet.poolFees[vspSource], &null.Float64{Valid: false})
					vspDataSet.proportionLive[vspSource] = append(vspDataSet.proportionLive[vspSource], &null.Float64{Valid: false})
					vspDataSet.proportionMissed[vspSource] = append(vspDataSet.proportionMissed[vspSource], &null.Float64{Valid: false})
					vspDataSet.userCount[vspSource] = append(vspDataSet.userCount[vspSource], &null.Uint64{Valid: false})
					vspDataSet.usersActive[vspSource] = append(vspDataSet.usersActive[vspSource], &null.Uint64{Valid: false})
				} else {
					vspDataSet.immature[vspSource] = append(vspDataSet.immature[vspSource], nil)
					vspDataSet.live[vspSource] = append(vspDataSet.live[vspSource], nil)
					vspDataSet.voted[vspSource] = append(vspDataSet.voted[vspSource], nil)
					vspDataSet.missed[vspSource] = append(vspDataSet.missed[vspSource], nil)
					vspDataSet.poolFees[vspSource] = append(vspDataSet.poolFees[vspSource], nil)
					vspDataSet.proportionLive[vspSource] = append(vspDataSet.proportionLive[vspSource], nil)
					vspDataSet.proportionMissed[vspSource] = append(vspDataSet.proportionMissed[vspSource], nil)
					vspDataSet.userCount[vspSource] = append(vspDataSet.userCount[vspSource], nil)
					vspDataSet.usersActive[vspSource] = append(vspDataSet.usersActive[vspSource], nil)
				}
			}
		}
	}

	return &vspDataSet, nil
}

func (pg *PgDb) fetchVspChartForSource(ctx context.Context, startDate uint64, axisString string, vspSource string) (*vspSet, error) {
	var vspDataSet = vspSet{
		time:             []uint64{},
		immature:         make(map[string]cache.ChartNullUints),
		live:             make(map[string]cache.ChartNullUints),
		voted:            make(map[string]cache.ChartNullUints),
		missed:           make(map[string]cache.ChartNullUints),
		poolFees:         make(map[string]cache.ChartNullFloats),
		proportionLive:   make(map[string]cache.ChartNullFloats),
		proportionMissed: make(map[string]cache.ChartNullFloats),
		userCount:        make(map[string]cache.ChartNullUints),
		usersActive:      make(map[string]cache.ChartNullUints),
	}

	dates, err := pg.allVspTickDates(ctx, helpers.UnixTime(int64(startDate)))
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	for _, date := range dates {
		vspDataSet.time = append(vspDataSet.time, uint64(date.Unix()))
	}

	points, err := pg.fetchVSPChartData(ctx, vspSource, helpers.UnixTime(int64(startDate)), 0, axisString)
	if err != nil {
		if err.Error() == sql.ErrNoRows.Error() {
			return &vspSet{}, nil
		}
		return nil, fmt.Errorf("error in fetching records for %s: %s", vspSource, err.Error())
	}

	var pointsMap = map[time.Time]*models.VSPTick{}
	for _, record := range points {
		pointsMap[record.Time] = record
	}

	var hasFoundOne bool
	for _, date := range dates {
		if record, found := pointsMap[date]; found {
			vspDataSet.immature[vspSource] = append(vspDataSet.immature[vspSource], &null.Uint64{Valid: true, Uint64: uint64(record.Immature)})
			vspDataSet.live[vspSource] = append(vspDataSet.live[vspSource], &null.Uint64{Valid: true, Uint64: uint64(record.Live)})
			vspDataSet.voted[vspSource] = append(vspDataSet.voted[vspSource], &null.Uint64{Valid: true, Uint64: uint64(record.Voted)})
			vspDataSet.missed[vspSource] = append(vspDataSet.missed[vspSource], &null.Uint64{Valid: true, Uint64: uint64(record.Missed)})
			vspDataSet.poolFees[vspSource] = append(vspDataSet.poolFees[vspSource], &null.Float64{Valid: true, Float64: record.PoolFees})
			vspDataSet.proportionLive[vspSource] = append(vspDataSet.proportionLive[vspSource], &null.Float64{Valid: true, Float64: record.ProportionLive})
			vspDataSet.proportionMissed[vspSource] = append(vspDataSet.proportionMissed[vspSource], &null.Float64{Valid: true, Float64: record.ProportionMissed})
			vspDataSet.userCount[vspSource] = append(vspDataSet.userCount[vspSource], &null.Uint64{Valid: true, Uint64: uint64(record.UserCount)})
			vspDataSet.usersActive[vspSource] = append(vspDataSet.usersActive[vspSource], &null.Uint64{Valid: true, Uint64: uint64(record.UsersActive)})
			hasFoundOne = true
		} else {
			if hasFoundOne {
				vspDataSet.immature[vspSource] = append(vspDataSet.immature[vspSource], &null.Uint64{Valid: false})
				vspDataSet.live[vspSource] = append(vspDataSet.live[vspSource], &null.Uint64{Valid: false})
				vspDataSet.voted[vspSource] = append(vspDataSet.voted[vspSource], &null.Uint64{Valid: false})
				vspDataSet.missed[vspSource] = append(vspDataSet.missed[vspSource], &null.Uint64{Valid: false})
				vspDataSet.poolFees[vspSource] = append(vspDataSet.poolFees[vspSource], &null.Float64{Valid: false})
				vspDataSet.proportionLive[vspSource] = append(vspDataSet.proportionLive[vspSource], &null.Float64{Valid: false})
				vspDataSet.proportionMissed[vspSource] = append(vspDataSet.proportionMissed[vspSource], &null.Float64{Valid: false})
				vspDataSet.userCount[vspSource] = append(vspDataSet.userCount[vspSource], &null.Uint64{Valid: false})
				vspDataSet.usersActive[vspSource] = append(vspDataSet.usersActive[vspSource], &null.Uint64{Valid: false})
			} else {
				vspDataSet.immature[vspSource] = append(vspDataSet.immature[vspSource], nil)
				vspDataSet.live[vspSource] = append(vspDataSet.live[vspSource], nil)
				vspDataSet.voted[vspSource] = append(vspDataSet.voted[vspSource], nil)
				vspDataSet.missed[vspSource] = append(vspDataSet.missed[vspSource], nil)
				vspDataSet.poolFees[vspSource] = append(vspDataSet.poolFees[vspSource], nil)
				vspDataSet.proportionLive[vspSource] = append(vspDataSet.proportionLive[vspSource], nil)
				vspDataSet.proportionMissed[vspSource] = append(vspDataSet.proportionMissed[vspSource], nil)
				vspDataSet.userCount[vspSource] = append(vspDataSet.userCount[vspSource], nil)
				vspDataSet.usersActive[vspSource] = append(vspDataSet.usersActive[vspSource], nil)
			}
		}
	}

	return &vspDataSet, nil
}

func (pg *PgDb) UpdateVspChart(ctx context.Context) error {
	log.Info("Updating VSP bin data")
	if err := pg.UpdateVspHourlyChart(ctx); err != nil {
		return err
	}

	if err := pg.UpdateVspDailyChart(ctx); err != nil {
		return err
	}
	return nil
}

func (pg *PgDb) UpdateVspHourlyChart(ctx context.Context) error {
	log.Info("Updating VSP hourly average")
	lastHourEntry, err := models.VSPTickBins(
		models.VSPTickBinWhere.Bin.EQ(string(cache.HourBin)),
		qm.OrderBy(fmt.Sprintf("%s desc", models.VSPTickBinColumns.Time)),
	).One(ctx, pg.db)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	var nextHour = time.Time{}
	var lastHour int64
	if lastHourEntry != nil {
		nextHour = time.Unix(lastHourEntry.Time, 0).Add(cache.AnHour * time.Second).UTC()
		lastHour = lastHourEntry.Time
	}
	if time.Now().Before(nextHour) {
		return nil
	}

	allVspData, err := pg.FetchVSPs(ctx)
	if err != nil {
		return err
	}
	var vsps = make([]string, len(allVspData))
	for i, vspSource := range allVspData {
		vsps[i] = vspSource.Name
	}

	tx, err := pg.db.Begin()
	if err != nil {
		return err
	}
	for _, source := range allVspData {
		log.Infof("Updating VSP hourly average for %s", source.Name)
		vspSet, err := pg.fetchVspChartForSource(ctx, uint64(lastHour), "", source.Name)
		if err != nil {
			return err
		}
		hours, _, hourIntervals := cache.GenerateHourBin(vspSet.time, nil)
		for i, interval := range hourIntervals {
			if int64(hours[i]) < nextHour.Unix() {
				continue
			}
			immature := vspSet.immature[source.Name].Avg(interval[0], interval[1])
			live := vspSet.live[source.Name].Avg(interval[0], interval[1])
			voted := vspSet.voted[source.Name].Avg(interval[0], interval[1])
			missed := vspSet.missed[source.Name].Avg(interval[0], interval[1])
			poolFees := vspSet.poolFees[source.Name].Avg(interval[0], interval[1])
			proportionLive := vspSet.proportionLive[source.Name].Avg(interval[0], interval[1])
			proportionMissed := vspSet.proportionMissed[source.Name].Avg(interval[0], interval[1])
			userCount := vspSet.userCount[source.Name].Avg(interval[0], interval[1])
			usersActive := vspSet.usersActive[source.Name].Avg(interval[0], interval[1])
			vspBin := models.VSPTickBin{
				Time:  int64(hours[i]),
				Bin:   string(cache.HourBin),
				VSPID: source.ID,
			}
			if immature != nil {
				vspBin.Immature = null.IntFrom(int(immature.Uint64))
			}
			if live != nil {
				vspBin.Live = null.IntFrom(int(live.Uint64))
			}
			if voted != nil {
				vspBin.Voted = null.IntFrom(int(voted.Uint64))
			}
			if missed != nil {
				vspBin.Missed = null.IntFrom(int(missed.Uint64))
			}
			if poolFees != nil {
				vspBin.PoolFees = null.Float64From(poolFees.Float64)
			}
			if proportionLive != nil {
				vspBin.ProportionLive = null.Float64From(proportionLive.Float64)
			}
			if proportionMissed != nil {
				vspBin.ProportionMissed = null.Float64From(proportionMissed.Float64)
			}
			if userCount != nil {
				vspBin.UserCount = null.IntFrom(int(userCount.Uint64))
			}
			if usersActive != nil {
				vspBin.UsersActive = null.IntFrom(int(usersActive.Uint64))
			}
			if err = vspBin.Insert(ctx, tx, boil.Infer()); err != nil {
				_ = tx.Rollback()
				spew.Dump(vspBin)
				return err
			}
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (pg *PgDb) UpdateVspDailyChart(ctx context.Context) error {
	log.Info("Updating VSP daily average")
	lastDayEntry, err := models.VSPTickBins(
		models.VSPTickBinWhere.Bin.EQ(string(cache.DayBin)),
		qm.OrderBy(fmt.Sprintf("%s desc", models.VSPTickBinColumns.Time)),
	).One(ctx, pg.db)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	var nextDay = time.Time{}
	if lastDayEntry != nil {
		nextDay = time.Unix(lastDayEntry.Time, 0).Add(cache.ADay * time.Second).UTC()
	}
	if time.Now().Before(nextDay) {
		return nil
	}

	allVspData, err := pg.FetchVSPs(ctx)
	if err != nil {
		return err
	}
	var vsps = make([]string, len(allVspData))
	for i, vspSource := range allVspData {
		vsps[i] = vspSource.Name
	}

	tx, err := pg.db.Begin()
	if err != nil {
		return err
	}

	for _, source := range allVspData {
		log.Infof("Updating VSP daily average for %s", source.Name)
		records, err := models.VSPTickBins(
			models.VSPTickBinWhere.VSPID.EQ(source.ID),
			models.VSPTickBinWhere.Bin.EQ(string(cache.HourBin)),
			models.VSPTickBinWhere.Time.GTE(nextDay.Unix()),
		).All(ctx, tx)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
		var timeSet cache.ChartUints
		var immatureSet, liveSet, votedSet, missedSet, userCountSet, usersActiveSet cache.ChartNullUints
		var poolFeesSet, proportionLiveSet, proportionMissedSet cache.ChartNullFloats

		for _, rec := range records {
			timeSet = append(timeSet, uint64(rec.Time))
			immatureSet = append(immatureSet, &null.Uint64{Uint64: uint64(rec.Immature.Int), Valid: rec.Immature.Valid})
			liveSet = append(liveSet, &null.Uint64{Uint64: uint64(rec.Live.Int), Valid: rec.Live.Valid})
			missedSet = append(missedSet, &null.Uint64{Uint64: uint64(rec.Missed.Int), Valid: rec.Missed.Valid})
			votedSet = append(votedSet, &null.Uint64{Uint64: uint64(rec.Voted.Int), Valid: rec.Voted.Valid})
			userCountSet = append(userCountSet, &null.Uint64{Uint64: uint64(rec.UserCount.Int), Valid: rec.UserCount.Valid})
			usersActiveSet = append(usersActiveSet, &null.Uint64{Uint64: uint64(rec.UsersActive.Int), Valid: rec.UsersActive.Valid})
			poolFeesSet = append(poolFeesSet, &null.Float64{Float64: rec.PoolFees.Float64, Valid: rec.PoolFees.Valid})
			proportionLiveSet = append(proportionLiveSet, &null.Float64{
				Float64: rec.ProportionLive.Float64, Valid: rec.ProportionLive.Valid})
			proportionMissedSet = append(proportionMissedSet, &null.Float64{
				Float64: rec.ProportionMissed.Float64, Valid: rec.ProportionMissed.Valid})
		}
		days, _, dayIntervals := cache.GenerateDayBin(timeSet, nil)
		for i, interval := range dayIntervals {
			if int64(days[i]) < nextDay.Unix() {
				continue
			}
			immature := immatureSet.Avg(interval[0], interval[1])
			live := liveSet.Avg(interval[0], interval[1])
			voted := votedSet.Avg(interval[0], interval[1])
			missed := missedSet.Avg(interval[0], interval[1])
			poolFees := poolFeesSet.Avg(interval[0], interval[1])
			proportionLive := proportionLiveSet.Avg(interval[0], interval[1])
			proportionMissed := proportionMissedSet.Avg(interval[0], interval[1])
			userCount := userCountSet.Avg(interval[0], interval[1])
			usersActive := usersActiveSet.Avg(interval[0], interval[1])
			vspBin := models.VSPTickBin{
				Time:  int64(days[i]),
				Bin:   string(cache.DayBin),
				VSPID: source.ID,
			}
			if immature != nil {
				vspBin.Immature = null.IntFrom(int(immature.Uint64))
			}
			if live != nil {
				vspBin.Live = null.IntFrom(int(live.Uint64))
			}
			if voted != nil {
				vspBin.Voted = null.IntFrom(int(voted.Uint64))
			}
			if missed != nil {
				vspBin.Missed = null.IntFrom(int(missed.Uint64))
			}
			if poolFees != nil {
				vspBin.PoolFees = null.Float64From(poolFees.Float64)
			}
			if proportionLive != nil {
				vspBin.ProportionLive = null.Float64From(proportionLive.Float64)
			}
			if proportionMissed != nil {
				vspBin.ProportionMissed = null.Float64From(proportionMissed.Float64)
			}
			if userCount != nil {
				vspBin.UserCount = null.IntFrom(int(userCount.Uint64))
			}
			if usersActive != nil {
				vspBin.UsersActive = null.IntFrom(int(usersActive.Uint64))
			}
			if err = vspBin.Insert(ctx, tx, boil.Infer()); err != nil {
				_ = tx.Rollback()
				spew.Dump(vspBin)
				return err
			}
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}
