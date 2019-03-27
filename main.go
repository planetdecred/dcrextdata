package main

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
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

	db, err := NewPgDb(cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPass, cfg.DBName)
	defer db.Close()

	if cfg.Reset {
		log.Info("Dropping tables")
		err = db.DropAllTables()
		if err != nil {
			db.Close()
			log.Error("Could not drop tables: ", err)
			return
		} else {
			log.Info("Tables dropped")
			// return err
		}
	}

	if exists := db.ExchangeDataTableExits(); !exists {
		log.Info("Creating new exchange data table")
		if err := db.CreateExchangeDataTable(); err != nil {
			log.Error("Error creating exchange data table: ", err)
			return
		}
	}

	//retrievers := make([]exchanges.Retriever, 0, 2)

	//exchangeCollector := exchanges.Collector{Retrievers: []exchanges.Retriever{poloniex, bittrex}}

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

	wg.Add(1)
	log.Info("Starting exchange storage goroutine")
	go storeExchangeData(db, resultChan, quit, wg)

	// Exchange collection enabled
	if cfg.ExchangesEnabled {
		exchanges := make(map[string]int64)
		for _, ex := range cfg.Exchanges {
			exchanges[ex] = db.LastExchangeEntryTime(ex)
		}

		//log.Debugf("exchangeMap: %v", exchanges)
		collector, err := NewExchangeCollector(exchanges, cfg.CollectionInterval)

		if err != nil {
			log.Error(err)
			return
		}

		excLog.Info("Starting historic sync")

		collector.HistoricSync(resultChan)

		wg.Add(1)

		excLog.Info("Starting periodic collection")
		go collector.Collect(resultChan, wg, quit)
	} else {
		close(quit)
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
