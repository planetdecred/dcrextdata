// Copyright (c) 2018-2019 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"io/ioutil"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"

	"github.com/decred/dcrd/chaincfg"
	"github.com/decred/dcrd/dcrutil"
	"github.com/decred/dcrd/rpcclient"
	"github.com/jessevdk/go-flags"
	"github.com/planetdecred/dcrextdata/app"
	"github.com/planetdecred/dcrextdata/app/config"
	"github.com/planetdecred/dcrextdata/app/help"
	"github.com/planetdecred/dcrextdata/app/helpers"
	"github.com/planetdecred/dcrextdata/cache"
	"github.com/planetdecred/dcrextdata/commstats"
	"github.com/planetdecred/dcrextdata/datasync"
	"github.com/planetdecred/dcrextdata/exchanges"
	"github.com/planetdecred/dcrextdata/mempool"
	"github.com/planetdecred/dcrextdata/netsnapshot"
	"github.com/planetdecred/dcrextdata/postgres"
	"github.com/planetdecred/dcrextdata/pow"
	"github.com/planetdecred/dcrextdata/vsp"
	"github.com/planetdecred/dcrextdata/web"
)

// const dcrlaunchtime int64 = 1454889600
// var opError error
// var beginShutdown = make(chan bool)

func main() {
	// Create a context that is cancelled when a shutdown request is received
	// via requestShutdown.
	ctx := withShutdownCancel(context.Background())
	// Listen for both interrupt signals and shutdown requests.
	go shutdownListener()

	if err := _main(ctx); err != nil {
		if logRotator != nil {
			log.Error(err)
		} else {
			fmt.Println(err)
		}
		os.Exit(1)
	}
	os.Exit(0)
}

func _main(ctx context.Context) error {
	cfg, args, err := config.LoadConfig()
	if err != nil {
		return err
	}

	if cfg.Cpuprofile != "" {
		f, err := os.Create(cfg.Cpuprofile)
		if err != nil {
			log.Critical("could not create CPU profile: ", err)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Critical("could not start CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
	}

	// Initialize log rotation.  After log rotation has been initialized, the
	// logger variables may be used.
	initLogRotator(cfg.ConfigFileOptions.LogFile)
	defer func() {
		if logRotator != nil {
			logRotator.Close()
		}
	}()

	// Special show command to list supported subsystems and exit.
	if cfg.LogLevel == "show" {
		fmt.Println("Supported subsystems", supportedSubsystems())
		os.Exit(0)
	}

	// Parse, validate, and set debug log level(s).
	if cfg.Quiet {
		cfg.ConfigFileOptions.LogLevel = "error"
	}

	// Parse, validate, and set debug log level(s).
	if err := parseAndSetDebugLevels(cfg.LogLevel); err != nil {
		err := fmt.Errorf("loadConfig: %s", err.Error())
		return err
	}

	if cfg.ConfigFileOptions.VSPInterval < 300 {
		log.Warn("VSP collection interval cannot be less that 300, setting to 300")
		cfg.ConfigFileOptions.VSPInterval = 300
	}

	// if len(args) == 0, then there's nothing to execute as all command-line args were parsed as app options
	if len(args) > 0 {
		err := executeHelpCommand()
		if err != nil {
			return fmt.Errorf("%s: %s", err, config.Hint)
		}
		return nil
	}

	db, err := postgres.NewPgDb(cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPass, cfg.DBName, cfg.LogLevel == config.DebugLogLevel)

	if err != nil {
		return fmt.Errorf("error in establishing database connection: %s", err.Error())
	}

	defer func(db *postgres.PgDb) {
		err := db.Close()
		if err != nil {
			log.Error("Could not close database connection: %v", err)
		}
	}(db)

	if len(cfg.Migrate) > 0 {
		err = db.MigrateDatabase(cfg.Migrate)
		if err == nil {
			log.Info("Migrate database successfully")
		} else {
			log.Error("Migrate database fail: ", err)
		}
		return err
	}

	if cfg.ResetCache {
		resetTables, err := helpers.RequestYesNoConfirmation("Are you sure you want to reset the dcrextdata cache data?", "")
		if err != nil {
			return fmt.Errorf("error reading your response: %s", err.Error())
		}

		if resetTables {
			err = db.DropCacheTables()
			if err != nil {
				log.Error("Could not drop tables: ", err)
				return err
			}

			log.Info("Done. You can restart the server now.")
			return nil
		} else {
			log.Error("Migrate database fail: ", err)
		}
		return err
	}

	// Display app version.
	log.Infof("%s version %v (Go version %s)", app.AppName, app.Version(), runtime.Version())

	syncCoordinator := datasync.NewCoordinator(!cfg.DisableSync, cfg.SyncInterval)

	var syncDbs = map[string]*postgres.PgDb{}
	//register instances
	for i := 0; i < len(cfg.SyncSources); i++ {
		source := cfg.SyncSources[i]
		databaseName := cfg.SyncDatabases[i]
		db, err := postgres.NewPgDb(cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPass, databaseName, cfg.LogLevel == config.DebugLogLevel)
		if err != nil {
			log.Errorf("Error in open database connection for the sync instance, %s, %s", source, err.Error())
			continue
		}
		syncDbs[databaseName] = db
		syncCoordinator.AddSource(source, db, databaseName)
	}

	pools, _ := db.FetchPowSourceData(ctx)
	var poolSources = make([]string, len(pools))
	for i, pool := range pools {
		poolSources[i] = pool.Source
	}

	allVspData, _ := db.FetchVSPs(ctx)
	var vsps = make([]string, len(allVspData))
	for i, vspSource := range allVspData {
		vsps[i] = vspSource.Name
	}

	noveVersions, err := db.AllNodeVersions(ctx)
	if err != nil {
		log.Error(err)
	}

	nodeCountries, err := db.AllNodeContries(ctx)
	if err != nil {
		log.Error(err)
	}

	commstats.SetAccounts(cfg.CommunityStatOptions)
	cacheManager := cache.NewChartData(ctx, cfg.EnableChartCache, cfg.SyncDatabases, poolSources, vsps,
		nodeCountries, noveVersions, netParams(cfg.DcrdNetworkType), cfg.CacheDir)
	db.RegisterCharts(cacheManager, cfg.SyncDatabases, func(name string) (*postgres.PgDb, error) {
		db, found := syncDbs[name]
		if !found {
			return nil, fmt.Errorf("no db is registered for the source, %s", name)
		}
		return db, nil
	})
	if err = db.UpdateMempoolAggregateData(ctx); err != nil {
		return fmt.Errorf("Error in initial mempool bin update, %s", err.Error())
	}
	if err = db.UpdatePropagationData(ctx); err != nil {
		return fmt.Errorf("Error in initial propagation data update, %s", err.Error())
	}
	if err = db.UpdateBlockBinData(ctx); err != nil {
		return fmt.Errorf("Error in initial block data update, %s", err.Error())
	}
	if err = db.UpdateVoteTimeDeviationData(ctx); err != nil {
		return fmt.Errorf("Error in initial vote receive time deviation data update, %s", err.Error())
	}
	if err = db.UpdatePowChart(ctx); err != nil {
		return fmt.Errorf("Error in initial PoW bin update, %s", err.Error())
	}
	if err = db.UpdateVspChart(ctx); err != nil {
		return fmt.Errorf("Error in initial VSP bin update, %s", err.Error())
	}
	if err = db.UpdateSnapshotNodesBin(ctx); err != nil {
		return fmt.Errorf("Error in initial network snapshot bin update, %s", err.Error())
	}

	// http server method
	if strings.ToLower(cfg.HttpMode) == "true" || cfg.HttpMode == "1" {
		extDbFactory := func(name string) (query web.DataQuery, e error) {
			db, found := syncDbs[name]
			if !found {
				return nil, fmt.Errorf("no db is registered for the source, %s", name)
			}
			return db, nil
		}
		go web.StartHttpServer(cfg.HTTPHost, cfg.HTTPPort, cacheManager, db, netParams(cfg.DcrdNetworkType), extDbFactory)
	}

	var dcrClient *rpcclient.Client
	var collector *mempool.Collector

	// if mempool is not disable, check that a dcrclient can be created before showing app version
	if !cfg.DisableMempool {
		connCfg := &rpcclient.ConnConfig{
			Host:       cfg.DcrdRpcServer,
			Endpoint:   "ws",
			User:       cfg.DcrdRpcUser,
			Pass:       cfg.DcrdRpcPassword,
			DisableTLS: cfg.DisableTLS,
		}

		if !cfg.DisableTLS {
			dcrdHomeDir := dcrutil.AppDataDir("dcrd", false)
			certs, err := ioutil.ReadFile(filepath.Join(dcrdHomeDir, "rpc.cert"))
			if err != nil {
				log.Error("Error in reading dcrd cert: ", err)
				return nil
			}
			connCfg.Certificates = certs
		}

		collector = mempool.NewCollector(cfg.MempoolInterval, netParams(cfg.DcrdNetworkType), db)
		collector.RegisterSyncer(syncCoordinator)

		dcrClient, err = rpcclient.New(connCfg, collector.DcrdHandlers(ctx, cacheManager))
		if err != nil {
			dcrNotRunningErr := "No connection could be made because the target machine actively refused it"
			if strings.Contains(err.Error(), dcrNotRunningErr) {
				log.Errorf(fmt.Sprintf("Unable to connect to dcrd at %s. Is it running?", cfg.DcrdRpcServer))
				return nil
			} //running on port
			fmt.Printf("Error in opening a dcrd connection: %s\n", err.Error())
			return nil
		}

		err = collector.SetExplorerBestBlock(ctx)
		if err != nil {
			log.Errorf("Unable to retrieve explorer best block height. Dcrextdata will not be able to filter out staled blocks, %s", err.Error())
		}
	}

	if !cfg.DisableMempool {
		// register the close function to be run before shutdown
		app.ShutdownOps = append(app.ShutdownOps, func() {
			log.Info("Shutting down dcrd dcrClient")
			dcrClient.Shutdown()
		})

		if err := dcrClient.NotifyNewTransactions(true); err != nil {
			log.Error(err)
		}

		if err := dcrClient.NotifyBlocks(); err != nil {
			log.Errorf("Unable to register block notification for dcrClient: %s", err.Error())
		}

		collector.SetClient(dcrClient)

		go collector.StartMonitoring(ctx)
	}

	if !cfg.DisableVSP {
		vspCollector, err := vsp.NewVspCollector(cfg.VSPInterval, db)
		if err == nil {
			go vspCollector.Run(ctx, cacheManager)
		} else {
			log.Error(err)
		}
	}

	if !cfg.DisableExchangeTicks {
		go func() {
			ticksHub, err := exchanges.NewTickHub(ctx, cfg.DisabledExchanges, db)
			if err != nil {
				log.Error(err)
				return
			}
			ticksHub.Run(ctx)
		}()
	}

	if !cfg.DisablePow {
		go func() {
			powCollector, err := pow.NewCollector(cfg.DisabledPows, cfg.PowInterval, db)
			if err != nil {
				log.Error(err)
				return
			}
			powCollector.Run(ctx)
		}()
	}

	if !cfg.DisableCommunityStat {
		redditCollector, err := commstats.NewCommStatCollector(db, &cfg.CommunityStatOptions)
		if err == nil {
			go redditCollector.Run(ctx)
		} else {
			log.Error(err)
		}
	}

	if !cfg.DisableNetworkSnapshot {
		snapshotTaker := netsnapshot.NewTaker(db, cfg.NetworkSnapshotOptions)
		go snapshotTaker.Start(ctx)
	}

	go syncCoordinator.StartSyncing(ctx)

	// wait for shutdown signal
	<-ctx.Done()

	if cfg.Memprofile != "" {
		f, err := os.Create(cfg.Memprofile)
		if err != nil {
			log.Critical("could not create memory profile: ", err)
		}
		defer f.Close()
		runtime.GC() // get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			log.Critical("could not write memory profile: ", err)
		}
	}

	return ctx.Err()
}

func netParams(netType string) *chaincfg.Params {
	switch strings.ToLower(netType) {
	case strings.ToLower(chaincfg.MainNetParams.Name):
		return &chaincfg.MainNetParams
	case strings.ToLower(chaincfg.TestNet3Params.Name):
		return &chaincfg.TestNet3Params
	default:
		return nil
	}
}

// executeHelpCommand checks if the operation requested by the user is -h, --help flags. If it not a help flag is throw an error.
func executeHelpCommand() (err error) {
	configWithCommands := &config.Config{}
	parser := flags.NewParser(configWithCommands, flags.HelpFlag|flags.PassDoubleDash)

	// re-parse command-line args to catch help flag or execute any commands passed
	_, err = parser.Parse()
	if err != nil {
		e, ok := err.(*flags.Error)
		if ok && e.Type == flags.ErrHelp {
			help.PrintGeneralHelp(os.Stdout, help.HelpParser())
			return nil
		}
		return err
	}

	return fmt.Errorf(config.Hint)
}
