// Copyright (c) 2018-2019 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package vsp

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type Response map[string]*ResposeData

type VSPDto struct {
	Name                 string    `json:"vsp"`
	APIEnabled           bool      `json:"immature"`
	APIVersionsSupported []int64   `json:"live"`
	Network              string    `json:"voted"`
	URL                  string    `json:"missed"`
	Launched             time.Time `json:"pool_fees"`
}

type VSPTickDto struct {
	ID               int       `json:"id"`
	VSP              string    `json:"vsp"`
	Immature         int       `json:"immature"`
	Live             int       `json:"live"`
	Voted            int       `json:"voted"`
	Missed           int       `json:"missed"`
	PoolFees         float64   `json:"pool_fees"`
	ProportionLive   float64   `json:"proportion_live"`
	ProportionMissed float64   `json:"proportion_missed"`
	UserCount        int       `json:"user_count"`
	UsersActive      int       `json:"users_active"`
	Time             time.Time `json:"time"`
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
	StoreVSPs(context.Context, Response) []error
	LastVspTickEntryTime() (time time.Time)
}

type Collector struct {
	client    http.Client
	period    time.Duration
	request   *http.Request
	dataStore DataStore
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
	Date time.Time
	Record  string
}
