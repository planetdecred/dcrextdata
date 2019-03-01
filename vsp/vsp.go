// Copyright (c) 2018-2019 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package vsp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

const requestURL = "https://api.decred.org/?c=gsd"

type Response map[string]*ResposeData

type ResposeData struct {
	APIEnabled           bool    `json:"APIEnabled"`
	APIVersionsSupported []int   `json:"APIVersionsSupported"`
	Network              string  `json:"Network"`
	URL                  string  `json:"URL"`
	Launched             int     `json:"Launched"`
	LastUpdated          int     `json:"LastUpdated"`
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

type Store interface {
	StoreVSP(time.Time, Response) error
}
type Collector struct {
	client  *http.Client
	period  time.Duration
	request *http.Request
}

func NewVspCollector(period int64) (*Collector, error) {
	request, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}

	return &Collector{
		client:  &http.Client{Timeout: 300 * time.Second},
		period:  time.Duration(period),
		request: request,
	}, nil
}

func (vsp *Collector) fetch(response interface{}) error {
	log.Tracef("GET %v", requestURL)
	resp, err := vsp.client.Do(vsp.request)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(response)
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("Failed to decode json: %v", err))
	}

	return nil
}

func (vsp *Collector) Run(dataStore Store, quit chan struct{}, wg *sync.WaitGroup) {
	resp := new(Response)
	ticker := time.NewTicker(vsp.period * time.Second)

	defer func(wg *sync.WaitGroup) {
		ticker.Stop()
		wg.Done()
	}(wg)

	for {
		select {
		case t := <-ticker.C:
			err := vsp.fetch(resp)
			if err != nil {
				log.Error(err)
				continue
			}
			if resp != nil {
				dataStore.StoreVSP(t, *resp)
			}
		case <-quit:
			return
		}
	}
}
