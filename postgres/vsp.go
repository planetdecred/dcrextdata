// Copyright (c) 2018-2019 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/raedahgroup/dcrextdata/postgres/models"
	"github.com/raedahgroup/dcrextdata/vsp"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"github.com/volatiletech/sqlboiler/types"
)

var (
	vspTickExistsErr = fmt.Errorf("VSPTick exists")
)

// StoreVSPs attempts to store the vsp responses by calling storeVspResponseG and returning
// a slice of errors
func (pg *PgDb) StoreVSPs(ctx context.Context, data vsp.Response) []error {
	if ctx.Err() != nil {
		return []error{ctx.Err()}
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
			return append(errs, ctx.Err())
		}
	}
	if completed == 0 {
		log.Info("Unable to store any vsp entry")
	}
	return errs
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
	tickTime := time.Unix(int64(resp.LastUpdated), 0)

	err = vspTick.Insert(ctx, pg.db, boil.Infer())
	// if err != nil && strings.Contains(err.Error(), "unique constraint") {
	// 	log.Tracef("Tick exits for %s", name)
	// 	err = txr.Rollback()
	// 	if err != nil {
	// 		return err
	// 	}
	// 	return vspTickExistsErr
	// } else if err != nil {
	// 	txr.Rollback()
	// 	return err
	// }
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

	// vspTickTimeExits, err := models.VSPTickTimes(
	// 	models.VSPTickTimeWhere.UpdateTime.EQ(tickTime),
	// 	models.VSPTickTimeWhere.VSPTickID.EQ(vspTick.ID)).Exists(ctx, pg.db)

	// if err != nil {
	// 	txr.Rollback()
	// 	return err
	// }

	// if !vspTickTimeExits {
	// 	vtickTime := &models.VSPTickTime{
	// 		VSPTickID:  vspTick.ID,
	// 		UpdateTime: tickTime,
	// 	}

	// 	err = pg.tryInsert(ctx, txr, vtickTime)
	// 	if err != nil {
	// 		log.Debugf("Tick time %v for %d", vtickTime.UpdateTime, vtickTime.VSPTickID)
	// 		return err
	// 	}
	// }

	err = txr.Commit()
	if err != nil {
		return txr.Rollback()
	}

	log.Infof("Stored data for VSP %10s %v", name, tickTime.UTC().Format(dateTemplate))
	return nil
}

func responseToVSP(name string, resp *vsp.ResposeData) *models.VSP {
	return &models.VSP{
		Name:                 null.StringFrom(name),
		APIEnabled:           null.BoolFrom(resp.APIEnabled),
		APIVersionsSupported: types.Int64Array(resp.APIVersionsSupported),
		Network:              null.StringFrom(resp.Network),
		URL:                  null.StringFrom(resp.URL),
		Launched:             null.TimeFrom(time.Unix(resp.Launched, 0)),
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
		Time:             time.Unix(resp.LastUpdated, 0),
	}
}

func (pg *PgDb) FetchVSPs(ctx context.Context) ([]vsp.VSPDto, error) {
	vspData, err := models.VSPS().All(ctx, pg.db)
	if err != nil {
		return nil, err
	}

	var result []vsp.VSPDto
	for _, item := range vspData {

		result = append(result, vsp.VSPDto{
			Name:                 item.Name.String,
			APIEnabled:           item.APIEnabled.Bool,
			APIVersionsSupported: item.APIVersionsSupported,
			Network:              item.Network.String,
			URL:                  item.URL.String,
			Launched:             item.Launched.Time,
		})
	}

	return result, nil
}

// VSPTicks
func (pg *PgDb) FiltredVSPTicks(ctx context.Context, vspName string, offset int, limit int) ([]vsp.VSPTickDto, error) {
	vspInfo, err := models.VSPS(models.VSPWhere.Name.EQ(null.StringFrom(vspName))).One(ctx, pg.db)
	if err != nil {
		return nil, err
	}

	vspIdQuery := models.VSPTickWhere.VSPID.EQ(vspInfo.ID)

	var vspTickSlice models.VSPTickSlice
	if limit == 3000 {
		vspTickSlice, err = models.VSPTicks(qm.Load("VSP"), vspIdQuery, qm.Offset(offset), qm.OrderBy(models.VSPTickColumns.Time)).All(ctx, pg.db)
	}else{
		vspTickSlice, err = models.VSPTicks(qm.Load("VSP"), vspIdQuery, qm.Limit(limit), qm.Offset(offset), qm.OrderBy(fmt.Sprintf("%s DESC", models.VSPTickColumns.Time))).All(ctx, pg.db)
	}

	if err != nil {
		return nil, err
	}

	vspTicks := []vsp.VSPTickDto{}
	for _, tick := range vspTickSlice {
		vspTicks = append(vspTicks, vsp.VSPTickDto{
			ID:               tick.ID,
			VSP:              tick.R.VSP.Name.String,
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
		})
	}

	return vspTicks, nil
}

// VSPTicks
// todo impliment sorting for VSP ticks as it is currently been sorted by time
func (pg *PgDb) AllVSPTicks(ctx context.Context, offset int, limit int) ([]vsp.VSPTickDto, error) {
	var vspTickSlice models.VSPTickSlice
	var err error
	if limit == 3000 {
		vspTickSlice, err = models.VSPTicks(qm.Load("VSP"), qm.Offset(offset), qm.OrderBy(models.VSPTickColumns.Time)).All(ctx, pg.db)
	}else{
		vspTickSlice, err = models.VSPTicks(qm.Load("VSP"), qm.Limit(limit), qm.Offset(offset), qm.OrderBy(fmt.Sprintf("%s DESC", models.VSPTickColumns.Time))).All(ctx, pg.db)
	}

	if err != nil {
		return nil, err
	}

	vspTicks := []vsp.VSPTickDto{}
	for _, tick := range vspTickSlice {
		vspTicks = append(vspTicks, vsp.VSPTickDto{
			ID:               tick.ID,
			VSP:              tick.R.VSP.Name.String,
			Time:             tick.Time.UTC(),
			Immature:         tick.Immature,
			Live:             tick.Live,
			Missed:           tick.Missed,
			PoolFees:         tick.PoolFees,
			ProportionLive:   tick.ProportionLive,
			ProportionMissed: tick.ProportionMissed,
			UserCount:        tick.UserCount,
			UsersActive:      tick.UsersActive,
			Voted:            tick.Voted,
		})
	}

	return vspTicks, nil
}

func (pg *PgDb) AllVSPTickCount(ctx context.Context) (int64, error) {
	return models.VSPTicks().Count(ctx, pg.db)
}

// VSPTicks count by vsp names
func (pg *PgDb) FiltredVSPTicksCount(ctx context.Context, vspName string) (int64, error) {
	vspInfo, err := models.VSPS(models.VSPWhere.Name.EQ(null.StringFrom(vspName))).One(ctx, pg.db)
	if err != nil {
		return 0, err
	}

	vspIdQuery := models.VSPTickWhere.VSPID.EQ(vspInfo.ID)
	return models.VSPTicks(qm.Load("VSP"), vspIdQuery).Count(ctx, pg.db)
}

func (pg *PgDb) LastVspTickEntryTime() (time time.Time) {
	rows := pg.db.QueryRow(lastVspTickEntryTime)
	_ = rows.Scan(&time)
	return
}

func (pg *PgDb) FetchChartData(ctx context.Context, attribute string, vspName string) (records []vsp.ChartData, err error) {
	vspInfo, err := models.VSPS(models.VSPWhere.Name.EQ(null.StringFrom(vspName))).One(ctx, pg.db)
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf("SELECT time as date, %s as record FROM vsp_tick where %s = %d ORDER BY time",
		attribute, models.VSPTickColumns.VSPID, vspInfo.ID)
	rows, err := pg.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var rec vsp.ChartData
		err = rows.Scan(&rec.Date, &rec.Record)
		if err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	return
}

func (pg *PgDb) GetVspTickDistinctDates(ctx context.Context, vsps []string) ([]time.Time, error) {
	var vspIds []string
	for _, vspName := range vsps {
		id, err := pg.vspIdByName(ctx, vspName)
		if err != nil {
			return nil, err
		}
		vspIds = append(vspIds, strconv.Itoa(id))
	}

	query := fmt.Sprintf("SELECT DISTINCT time FROM vsp_tick WHERE vsp_id IN ('%s') ORDER BY time", strings.Join(vspIds, "', '"))
	rows, err := pg.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}

	var dates []time.Time

	for rows.Next() {
		var date time.Time
		err = rows.Scan(&date)
		if err != nil {
			return nil, err
		}
		dates = append(dates, date)
	}
	return dates, nil
}

func (pg *PgDb) vspIdByName(ctx  context.Context, name string) (id int, err error) {
	vspModel, err := models.VSPS(models.VSPWhere.Name.EQ(null.StringFrom(name))).One(ctx, pg.db)
	if err != nil {
		return 0, err
	}
	return vspModel.ID, nil
}
