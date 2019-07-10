// Copyright (c) 2018-2019 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package vsp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/raedahgroup/dcrextdata/app/helpers"
)

const (
	requestURL = "https://api.decred.org/?c=gsd"
	retryLimit = 3
)

func NewVspCollector(period int64, store DataStore) (*Collector, error) {
	request, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}

	return &Collector{
		client:    http.Client{Timeout: time.Minute},
		period:    time.Duration(period),
		request:   request,
		dataStore: store,
	}, nil
}

func (vsp *Collector) fetch(ctx context.Context, response interface{}) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	// log.Tracef("GET %v", requestURL)
	resp, err := vsp.client.Do(vsp.request.WithContext(ctx))
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

func (vsp *Collector) Run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	if ctx.Err() != nil {
		return
	}

	lastCollectionDate := vsp.dataStore.LastVspTickEntryTime()
	secondsPassed := time.Since(lastCollectionDate)
	period := vsp.period * time.Second

	log.Info("Starting VSP collection cycle.")

	if secondsPassed < period {
		timeLeft := period - secondsPassed
		//Fetching VSPs every 5m, collected 1m7.99s ago, will fetch in 3m52.01s
		log.Infof("Fetching VSPs every %dm, collected %s ago, will fetch in %s.", vsp.period/60, helpers.DurationToString(secondsPassed),
			helpers.DurationToString(timeLeft))

		time.Sleep(timeLeft)
	}

	log.Info("Fetching VSP from source")

	if err := vsp.collectAndStore(ctx); err != nil {
		log.Errorf("Could not start collection: %v", err)
		return
	}

	go func() {
		ticker := time.NewTicker(vsp.period * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				log.Info("Starting a VSP collection cycle")
				if err := vsp.collectAndStore(ctx); err != nil {
					return
				}
			case <-ctx.Done():
				log.Infof("Shutting down collector")
				return
			}
		}
	}()
}

func (vsp *Collector) collectAndStore(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	resp := new(Response)
	err := vsp.fetch(ctx, resp)
	for retry := 0; err != nil; retry++ {
		if retry == retryLimit {
			return err
		}
		log.Warn(err)
		err = vsp.fetch(ctx, resp)
	}

	// log.Infof("Collected data for %d vsps", len(*resp))

	errs := vsp.dataStore.StoreVSPs(ctx, *resp)
	for _, err = range errs {
		if err != nil {
			if e, ok := err.(PoolTickTimeExistsError); ok {
				log.Trace(e)
			} else {
				log.Error(err)
				return err
			}
		}
	}
	return nil
}
