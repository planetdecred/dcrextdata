// Copyright (c) 2018-2019 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package vsp

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/planetdecred/dcrextdata/cache"
	"github.com/planetdecred/dcrextdata/datasync"
)

type Response map[string]*ResposeData

type VSPDto struct {
	Name                 string    `json:"vsp"`
	APIEnabled           bool      `json:"immature"`
	APIVersionsSupported []int64   `json:"live"`
	Network              string    `json:"voted"`
	URL                  string    `json:"missed"`
	Host                 string    `json:"host"`
	Launched             time.Time `json:"pool_fees"`
	ID                   int       `json:"id"`
}

type VSPTickDto struct {
	ID               int     `json:"id"`
	VSP              string  `json:"vsp"`
	Immature         int     `json:"immature"`
	Live             int     `json:"live"`
	Voted            int     `json:"voted"`
	Missed           int     `json:"missed"`
	PoolFees         float64 `json:"pool_fees"`
	ProportionLive   string  `json:"proportion_live"`
	ProportionMissed string  `json:"proportion_missed"`
	UserCount        int     `json:"user_count"`
	UsersActive      int     `json:"users_active"`
	Time             string  `json:"time"`
}

type ResposeData struct {
	APIEnabled           bool    `json:"APIEnabled"`
	APIVersionsSupported []int64 `json:"APIVersionsSupported"`
	Network              string  `json:"Network"`
	URL                  string  `json:"URL"`
	Launched             int64   `json:"Launched"`
	LastUpdated          int64   `json:"LastUpdated"`
	Immature             int     `json:"Immature"`
	Live                 int     `json:"Live"`
	Voted                int     `json:"Voted"`
	Missed               int     `json:"Missed"`
	PoolFees             float64 `json:"PoolFees"`
	ProportionLive       float64 `json:"ProportionLive"`
	ProportionMissed     float64 `json:"ProportionMissed"`
	UserCount            int     `json:"UserCount"`
	UserCountActive      int     `json:"UserCountActive"`
}

type DataStore interface {
	VspTableName() string
	VspTickTableName() string
	StoreVSPs(context.Context, Response) (int, []error)
	LastVspTickEntryTime() (time time.Time)
	UpdateVspChart(ctx context.Context) error
	FetchVspSourcesForSync(ctx context.Context, lastID int64, skip, take int) ([]VSPDto, int64, error)
	FetchVspTicksForSync(ctx context.Context, lastID int64, skip, take int) ([]datasync.VSPTickSyncDto, int64, error)
}

type Collector struct {
	client    http.Client
	period    time.Duration
	request   *http.Request
	dataStore DataStore
	charts    *cache.Manager
}

type PoolTickTimeExistsError struct {
	PoolName string
	TickTime time.Time
}

func (err PoolTickTimeExistsError) Error() string {
	return fmt.Sprintf("Tick time at %s for %s already exists with the same data",
		err.TickTime, err.PoolName)
}

type ChartPoints []interface{}

type ChartData struct {
	Date   time.Time `json:"date"`
	Record string    `json:"record"`
}
