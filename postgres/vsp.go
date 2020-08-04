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

	"github.com/dgraph-io/badger"
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
	data, _, err := pg.fetchVspChart(ctx, 0, 0, dataType, vspSources...)
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

func (pg *PgDb) fetchAndAppendVspChartAxis(ctx context.Context, charts *cache.Manager,
	dates cache.ChartUints, dataType string, startDate uint64, txn *badger.Txn) error {

	var keys []string
	keyExists := func(key string) bool {
		for _, k := range keys {
			if key == k {
				return true
			}
		}
		return false
	}

	processUint := func(recMap map[int64]int, source string) error {
		var chartData cache.ChartNullUints
		var hasFoundOne bool
		for _, date := range dates {
			if date < startDate {
				continue
			}
			var data *null.Uint64
			if rec, f := recMap[int64(date)]; f {
				data = &null.Uint64{
					Uint64: uint64(rec), Valid: true,
				}
				hasFoundOne = true
			} else if hasFoundOne {
				data = &null.Uint64{}
			}
			chartData = append(chartData, data)
		}
		key := fmt.Sprintf("%s-%s-%s", cache.VSP, dataType, source)
		var retryCount int
	retry:
		if err := charts.AppendChartNullUintsAxisTx(key, chartData, txn); err != nil {
			if err == badger.ErrConflict && !keyExists(key) && retryCount < 3 {
				retryCount++
				goto retry
			}
			return fmt.Errorf("%s - ", key)
		}
		keys = append(keys, key)
		return nil
	}

	processFloat := func(recMap map[int64]float64, source string) error {
		var chartData cache.ChartNullFloats
		var hasFoundOne bool
		var fCount int
		for _, date := range dates {
			if date < startDate {
				continue
			}
			var data *null.Float64
			if rec, f := recMap[int64(date)]; f {
				data = &null.Float64{
					Float64: rec, Valid: true,
				}
				fCount++
				hasFoundOne = true
			} else if hasFoundOne {
				data = &null.Float64{}
			}
			chartData = append(chartData, data)
		}
		key := fmt.Sprintf("%s-%s-%s", cache.VSP, dataType, source)
		var retryCount int
	retry:
		if err := charts.AppendChartNullFloatsAxisTx(key, chartData, txn); err != nil {
			if err == badger.ErrConflict && !keyExists(key) && retryCount < 3 {
				retryCount++
				goto retry
			}
			return err
		}
		keys = append(keys, key)
		return nil
	}

	for _, source := range charts.VSPSources {
		points, err := pg.fetchVSPChartData(ctx, source, helpers.UnixTime(int64(startDate)), 0, dataType)
		if err != nil {
			if err.Error() == sql.ErrNoRows.Error() {
				continue
			}
			return fmt.Errorf("error in fetching records for %s: %s", source, err.Error())
		}
		switch strings.ToLower(dataType) {
		case string(cache.ImmatureAxis):
			recMap := map[int64]int{}
			for _, p := range points {
				recMap[p.Time.Unix()] = p.Immature
			}
			if err = processUint(recMap, source); err != nil {
				return err
			}
		case string(cache.LiveAxis):
			recMap := map[int64]int{}
			for _, p := range points {
				recMap[p.Time.Unix()] = p.Live
			}
			if err = processUint(recMap, source); err != nil {
				return err
			}

		case string(cache.VotedAxis):
			recMap := map[int64]int{}
			for _, p := range points {
				recMap[p.Time.Unix()] = p.Voted
			}
			if err = processUint(recMap, source); err != nil {
				return err
			}

		case string(cache.MissedAxis):
			recMap := map[int64]int{}
			for _, p := range points {
				recMap[p.Time.Unix()] = p.Missed
			}
			if err = processUint(recMap, source); err != nil {
				return err
			}

		case string(cache.PoolFeesAxis):
			recMap := map[int64]float64{}
			for _, p := range points {
				recMap[p.Time.Unix()] = p.PoolFees
			}
			if err = processFloat(recMap, source); err != nil {
				return err
			}

		case string(cache.ProportionLiveAxis):
			recMap := map[int64]float64{}
			for _, p := range points {
				recMap[p.Time.Unix()] = p.ProportionLive
			}
			if err = processFloat(recMap, source); err != nil {
				return err
			}

		case string(cache.ProportionMissedAxis):
			recMap := map[int64]float64{}
			for _, p := range points {
				recMap[p.Time.Unix()] = p.ProportionMissed
			}
			if err = processFloat(recMap, source); err != nil {
				return err
			}

		case string(cache.UserCountAxis):
			recMap := map[int64]int{}
			for _, p := range points {
				recMap[p.Time.Unix()] = p.UserCount
			}
			if err = processUint(recMap, source); err != nil {
				return err
			}

		case string(cache.UsersActiveAxis):
			recMap := map[int64]int{}
			for _, p := range points {
				recMap[p.Time.Unix()] = p.UsersActive
			}
			if err = processUint(recMap, source); err != nil {
				return err
			}
		}
	}

	return nil
}

func (pg *PgDb) fetchCacheVspChart(ctx context.Context, charts *cache.Manager, page int) (interface{}, func(), bool, error) {
	startDate := charts.VSPTimeTip()
	// Get close to the nearest value after the start date to avoid continue loop for situations where there is a gap
	var receiver time.Time
	statement := fmt.Sprintf("SELECT %s FROM %s WHERE %s > '%s' ORDER BY %s LIMIT 1", models.VSPTickColumns.Time,
		models.TableNames.VSPTick, models.VSPTickColumns.Time,
		helpers.UnixTime(int64(startDate)).Format("2006-01-02 15:04:05+0700"), models.VSPTickColumns.Time)
	rows := pg.db.QueryRow(statement)
	if err := rows.Scan(&receiver); err != nil {
		if err.Error() != sql.ErrNoRows.Error() {
			log.Errorf("Error in getting min vsp date - %s", err.Error())
		}
	}
	if int64(startDate) < receiver.Unix() {
		startDate = uint64(receiver.Unix())
		if startDate > 0 {
			startDate -= 1
		}
	}
	//

	dates, err := pg.allVspTickDates(ctx, helpers.UnixTime(int64(startDate)))
	if err != nil && err != sql.ErrNoRows {
		return nil, func() {}, true, err
	}
	var unixTimes cache.ChartUints
	for _, d := range dates {
		unixTimes = append(unixTimes, uint64(d.Unix()))
	}

	txn := charts.DB.NewTransaction(true)
	defer txn.Discard()

	key := fmt.Sprintf("%s-%s", cache.VSP, cache.TimeAxis)
	if err := charts.AppendChartUintsAxisTx(key, unixTimes, txn); err != nil {
		return nil, func() {}, true, err
	}

	axis := []string{
		string(cache.ImmatureAxis),
		string(cache.LiveAxis),
		string(cache.VotedAxis),
		string(cache.MissedAxis),
		string(cache.PoolFeesAxis),
		string(cache.ProportionLiveAxis),
		string(cache.ProportionMissedAxis),
		string(cache.UserCountAxis),
		string(cache.UsersActiveAxis),
	}
	for _, ax := range axis {
		if err := pg.fetchAndAppendVspChartAxis(ctx, charts, unixTimes, ax, startDate, txn); err != nil {
			log.Error(err, ax)
			return nil, func() {}, true, err
		}
	}

	if err := txn.Commit(); err != nil {
		return nil, func() {}, true, err
	}

	return &vspSet{}, func() {}, true, nil
}

func (pg *PgDb) fetchVspChart(ctx context.Context, startDate uint64, endDate uint64, axisString string, vspSources ...string) (*vspSet, bool, error) {
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
			return nil, false, err
		}
		for _, vspSource := range allVspData {
			vsps = append(vsps, vspSource.Name)
		}
	}

	dates, err := pg.allVspTickDates(ctx, helpers.UnixTime(int64(startDate)), vspSources...)
	if err != nil && err != sql.ErrNoRows {
		return nil, false, err
	}

	for _, date := range dates {
		vspDataSet.time = append(vspDataSet.time, uint64(date.Unix()))
	}

	var done = true
	for _, vspSource := range vsps {
		points, err := pg.fetchVSPChartData(ctx, vspSource, helpers.UnixTime(int64(startDate)), endDate, axisString)
		if err != nil {
			if err.Error() == sql.ErrNoRows.Error() {
				continue
			}
			return nil, false, fmt.Errorf("error in fetching records for %s: %s", vspSource, err.Error())
		}

		if len(points) > 0 {
			done = false
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

	return &vspDataSet, done, nil
}

func appendVspChart(charts *cache.Manager, data interface{}) error {
	vspDataSet := data.(*vspSet)

	if len(vspDataSet.time) == 0 {
		return nil
	}

	key := fmt.Sprintf("%s-%s", cache.VSP, cache.TimeAxis)
	if err := charts.AppendChartUintsAxis(key,
		vspDataSet.time); err != nil {
		return err
	}

	keyExists := func(arr []string, key string) bool {
		for _, s := range arr {
			if s == key {
				return true
			}
		}
		return false
	}

	for vspSource, record := range vspDataSet.immature {
		if !keyExists(charts.VSPSources, vspSource) {
			charts.VSPSources = append(charts.VSPSources, vspSource)
		}
		key := fmt.Sprintf("%s-%s-%s", cache.VSP, cache.ImmatureAxis, vspSource)
		if err := charts.AppendChartNullUintsAxis(key,
			record); err != nil {
			return err
		}
	}

	for vspSource, record := range vspDataSet.live {
		key := fmt.Sprintf("%s-%s-%s", cache.VSP, cache.LiveAxis, vspSource)
		if err := charts.AppendChartNullUintsAxis(key,
			record); err != nil {
			return err
		}
	}

	for vspSource, record := range vspDataSet.voted {
		key := fmt.Sprintf("%s-%s-%s", cache.VSP, cache.VotedAxis, vspSource)
		if err := charts.AppendChartNullUintsAxis(key,
			record); err != nil {
			return err
		}
	}

	for vspSource, record := range vspDataSet.missed {
		key := fmt.Sprintf("%s-%s-%s", cache.VSP, cache.MissedAxis, vspSource)
		if err := charts.AppendChartNullUintsAxis(key,
			record); err != nil {
			return err
		}
	}

	for vspSource, record := range vspDataSet.poolFees {
		key := fmt.Sprintf("%s-%s-%s", cache.VSP, cache.PoolFeesAxis, vspSource)
		if err := charts.AppendChartNullFloatsAxis(key,
			record); err != nil {
			return err
		}
	}

	for vspSource, record := range vspDataSet.proportionLive {
		key := fmt.Sprintf("%s-%s-%s", cache.VSP, cache.ProportionLiveAxis, vspSource)
		if err := charts.AppendChartNullFloatsAxis(key,
			record); err != nil {
			return err
		}
	}

	for vspSource, record := range vspDataSet.proportionMissed {
		key := fmt.Sprintf("%s-%s-%s", cache.VSP, cache.ProportionMissedAxis, vspSource)
		if err := charts.AppendChartNullFloatsAxis(key,
			record); err != nil {
			return err
		}
	}

	for vspSource, record := range vspDataSet.usersActive {
		key := fmt.Sprintf("%s-%s-%s", cache.VSP, cache.UsersActiveAxis, vspSource)
		if err := charts.AppendChartNullUintsAxis(key,
			record); err != nil {
			return err
		}
	}

	for vspSource, record := range vspDataSet.userCount {
		key := fmt.Sprintf("%s-%s-%s", cache.VSP, cache.UserCountAxis, vspSource)
		if err := charts.AppendChartNullUintsAxis(key,
			record); err != nil {
			return err
		}
	}

	return nil
}
