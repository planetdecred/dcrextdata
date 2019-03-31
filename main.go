// Copyright (c) 2018-2019 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"sync"

	"github.com/raedahgroup/dcrextdata/version"
	"github.com/raedahgroup/dcrextdata/vsp"
)

// const dcrlaunchtime int64 = 1454889600

func main() {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Printf("Unable to load config: %v\n", err)
		return
	}

	defer func() {
		if logRotator != nil {
			logRotator.Close()
		}
	}()

	// Display app version.
	log.Infof("%s version %v (Go version %s)", version.AppName,
		version.Version(), runtime.Version())

	db, err := NewPgDb(cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPass, cfg.DBName)
	defer db.Close()

	if err != nil {
		log.Error(err)
		return
	}

	if cfg.Reset {
		log.Info("Dropping tables")
		err = db.DropAllTables()
		if err != nil {
			db.Close()
			log.Error("Could not drop tables: ", err)
			return
		}
		log.Info("Tables dropped")
	}

	resultChan := make(chan []DataTick)

	quit := make(chan struct{})
	wg := new(sync.WaitGroup)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	go func() {
		<-c
		signal.Stop(c)
		log.Info("CTRL+C hit. Closing goroutines.")
		close(quit)
	}()

	if cfg.VSPEnabled {
		log.Info("Starting VSP data collection")
		vspCollector, err := vsp.NewVspCollector(cfg.VSPInterval, db)
		if err == nil {
			wg.Add(1)
			go vspCollector.Run(quit, wg)
		} else {
			log.Error(err)
		}
	}

	if cfg.ExchangesEnabled {
		if exists := db.ExchangeDataTableExits(); !exists {
			if err := db.CreateExchangeDataTable(); err != nil {
				log.Error("Error creating exchange data table: ", err)
				return
			}
		}
		wg.Add(1)
		log.Info("Starting exchange storage goroutine")
		go storeExchangeData(db, resultChan, quit, wg)
		exchanges := make(map[string]int64)
		for _, ex := range cfg.Exchanges {
			exchanges[ex] = db.LastExchangeEntryTime(ex)
		}

		collector, err := NewExchangeCollector(exchanges, cfg.CollectionInterval)

		if err != nil {
			log.Error(err)
			close(quit)
			return
		}

		excLog.Info("Starting historic sync")

		errs := collector.HistoricSync(resultChan)

		if len(errs) > 0 {
			for _, err = range errs {
				excLog.Error(err)
			}
			excLog.Error("Historic sync failed")
			close(quit)
			return
		}

		wg.Add(1)

		excLog.Info("Starting periodic collection")
		go collector.Collect(resultChan, wg, quit)
	}

	wg.Wait()
	log.Info("Goodbye")
}

func storeExchangeData(db *PgDb, resultChan chan []DataTick, quit chan struct{}, wg *sync.WaitGroup) {
	for {
		select {
		case dataTick := <-resultChan:
			err := db.AddExchangeData(dataTick)
			if err != nil {
				log.Errorf("Could not store exchange entry: %v", err)
			}
		case <-quit:
			log.Debug("Retrieved quit signal")
			wg.Done()
			return
		}
	}
}
