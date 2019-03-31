// Copyright (c) 2018-2019 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/raedahgroup/dcrextdata/vsp"
)

const (
	// Helpers
	getPQTimestamp = `SELECT to_timestamp($1)`

	// Exchange Table
	LastExchangeEntryTime = `SELECT time FROM exchange_data WHERE exchange=$1 ORDER BY time DESC LIMIT 1`

	InsertExchangeDataTick = `INSERT INTO exchange_data (
		high, low, open, close, time, exchange)
	VALUES ($1, $2, $3, $4, $5, $6)`

	CreateExchangeDataTable = `CREATE TABLE IF NOT EXISTS exchange_data (
		high FLOAT8,
		low FLOAT8,
		open FLOAT8,
		close FLOAT8,
		time INT,
		exchange TEXT, 
		CONSTRAINT tick PRIMARY KEY (time, exchange));`

	createVSPInfoTable = `CREATE TABLE IF NOT EXISTS vsp (
		id SERIAL PRIMARY KEY,
		name TEXT,
		api_enabled BOOLEAN,
		api_versions_supported INT8[],
		network TEXT,
		url TEXT,
		launched TIMESTAMPTZ
	);`

	createVSPDataTable = `CREATE TABLE IF NOT EXISTS vsp_data (
		id SERIAL PRIMARY KEY,
		vsp_id INT REFERENCES vsp(id),
		last_updated TIMESTAMPTZ,
		immature INT8,
		live INT8,
		voted INT8,
		missed INT8,
		pool_fees FLOAT8,
		proportion_live FLOAT8,
		proportion_missed FLOAT8,
		user_count INT8,
		users_active INT8,
		time TIMESTAMPTZ
	);`

	insertVSPInfo = `INSERT INTO vsp (
		name, api_enabled, api_versions_supported,
		network, url, launched)
	VALUES ($1,$2,$3,$4,$5,$6)
	RETURNING id;`

	insertVSPData = `INSERT INTO vsp_data(
		vsp_id, last_updated, immature, live, voted, missed, pool_fees,
		proportion_live, proportion_missed, user_count, users_active, time)
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12);`

	selectIDFromVSP = `SELECT id FROM vsp WHERE name=$1 LIMIT 1;`
)

type PgDb struct {
	db *sql.DB
}

// Core methods

//
func NewPgDb(host, port, user, pass, dbname string) (*PgDb, error) {
	var psqlInfo string
	if pass == "" {
		psqlInfo = fmt.Sprintf("host=%s user=%s "+
			"dbname=%s sslmode=disable",
			host, user, dbname)
	} else {
		psqlInfo = fmt.Sprintf("host=%s user=%s "+
			"password=%s dbname=%s sslmode=disable",
			host, user, pass, dbname)
	}
	// Only add port arg fot TCP connection since UNIX domain sockets (specified
	// by a "/" prefix) do not have a port.
	if !strings.HasPrefix(host, "/") {
		psqlInfo += fmt.Sprintf(" port=%s", port)
	}

	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		return nil, err
	}

	err = db.Ping()

	return &PgDb{db}, err
}

func (pg *PgDb) Close() error {
	pqLog.Trace("Closing postgresql connection")
	return pg.db.Close()
}

func (pg *PgDb) dropTable(name string) error {
	pqLog.Tracef("Dropping table %s", name)
	_, err := pg.db.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS %s;`, name))
	return err
}

func (pg *PgDb) DropAllTables() error {
	if err := pg.dropTable("vsp_data"); err != nil {
		return err
	}
	if err := pg.dropTable("vsp"); err != nil {
		return err
	}
	return pg.dropTable("exchange_data")
}

// Exchange methods

//
func (pg *PgDb) AddExchangeData(data []DataTick) error {
	added := 0
	for _, v := range data {
		_, err := pg.db.Exec(InsertExchangeDataTick, v.High, v.Low, v.Open, v.Close, v.Time, v.Exchange)
		if err != nil {
			if !strings.Contains(err.Error(), "unique constraint") { // Ignore duplicate entries
				return err
			}
		}
		added++
	}
	if len(data) == 1 {
		pqLog.Infof("Added %d entry from %s (%s)", added, data[0].Exchange,
			UnixTimeToString(data[0].Time))
	} else {
		last := data[len(data)-1]
		pqLog.Infof("Added %d entries from %s (%s to %s)", added, last.Exchange,
			UnixTimeToString(data[0].Time), UnixTimeToString(last.Time))
	}

	return nil
}

func (pg *PgDb) LastExchangeEntryTime(exchange string) (time int64) {
	rows := pg.db.QueryRow(LastExchangeEntryTime, exchange)
	_ = rows.Scan(&time)
	return
}

func (pg *PgDb) CreateExchangeDataTable() error {
	pqLog.Trace("Creating exchange data table")
	_, err := pg.db.Exec(CreateExchangeDataTable)
	return err
}

func (pg *PgDb) tableExists(name string) (bool, error) {
	rows, err := pg.db.Query(`SELECT relname FROM pg_class WHERE relname = $1`, name)
	if err == nil {
		defer func() {
			if e := rows.Close(); e != nil {
				log.Error("Close of Query failed: ", e)
			}
		}()
		return rows.Next(), nil
	}
	return false, err
}

func (pg *PgDb) ExchangeDataTableExits() bool {
	exists, _ := pg.tableExists("exchange_data")
	return exists
}

// VSP

//
func (pg *PgDb) CreateVSPTables() error {
	_, err := pg.db.Exec(createVSPInfoTable)
	if err != nil {
		return err
	}

	_, err = pg.db.Exec(createVSPDataTable)
	if err != nil {
		return err
	}

	return nil
}
func (pg *PgDb) StoreVSP(t time.Time, data vsp.Response) error {
	names := []string{}
	for name, stat := range data {
		id, err := pg.selectVSPID(name)
		if err != nil {
			if err != sql.ErrNoRows {
				return err
			}

			launched, err := pg.getPQTime(stat.Launched)
			if err != nil {
				return err
			}
			err = pg.db.QueryRow(insertVSPInfo, name, stat.APIEnabled, pq.Array(stat.APIVersionsSupported), stat.Network,
				stat.URL, launched).Scan(&id)
			if err != nil {
				return err
			}
		}

		lastUpdated, err := pg.getPQTime(stat.LastUpdated)
		if err != nil {
			return err
		}

		timeRetrieved, err := pg.getPQTime(int(t.Unix()))
		if err != nil {
			return err
		}

		_, err = pg.db.Exec(insertVSPData, id, lastUpdated, stat.Immature, stat.Live, stat.Voted,
			stat.Missed, stat.PoolFees, stat.ProportionLive, stat.ProportionMissed, stat.UserCount,
			stat.UserCountActive, timeRetrieved)

		if err != nil {
			return err
		}
		names = append(names, name)
	}
	log.Tracef("Add pool stats for %s", names)
	return nil
}

func (pg *PgDb) getPQTime(t int) (string, error) {
	tsRes := pg.db.QueryRow(getPQTimestamp, t)
	ts := ""
	err := tsRes.Scan(&ts)
	if err != nil {
		return "", fmt.Errorf("Could not convert UNIX time %d to postgresql TIMESTAMPTZ", t)
	}
	return ts, nil
}

func (pg *PgDb) selectVSPID(name string) (id int, err error) {
	err = pg.db.QueryRow(selectIDFromVSP, name).Scan(&id)
	return
}

// func (pg *PgDb) addVSPINFO(data vsp.ResponseData) error {

// }
