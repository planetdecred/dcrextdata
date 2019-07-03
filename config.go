// Copyright (c) 2018-2019 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/decred/slog"
	flags "github.com/jessevdk/go-flags"
)

const (
	defaultConfigFilename = "dcrextdata.conf"
	defaultLogFilename    = "dcrextdata.log"
	defaultLogLevel       = "info"
	hint                  = `Run dcrextdata --mode=http to start http server or just dcrextdata`

	defaultDcrdServer 	   = "localhost:9109"
	defaultDcrdNetworkType = "mainnet"
)

type config struct {
	configFileOptions
	CommandLineOptions
}

type configFileOptions struct {
	// General application behaviour
	Reset      bool   `short:"R" long:"reset" description:"Drop all database tables and start over"`
	LogFile    string `short:"L" long:"logfile" description:"File name of the log file"`
	ConfigFile string `short:"C" long:"configfile" description:"Path to configuration file"`
	DebugLevel string `short:"d" long:"debuglevel" description:"Logging level {trace, debug, info, warn, error, critical}"`
	Quiet      bool   `short:"q" long:"quiet" description:"Easy way to set debuglevel to error"`

	// Postgresql configuration
	DBHost string `long:"dbhost" description:"Database host"`
	DBPort string `long:"dbport" description:"Database port"`
	DBUser string `long:"dbuser" description:"Database username"`
	DBPass string `long:"dbpass" description:"Database password"`
	DBName string `long:"dbname" description:"Database name"`

	// Http Server
	HTTPHost string `long:"httphost" description:"HTTP server host address or IP when running godcr in http mode."`
	HTTPPort string `long:"httpport" description:"HTTP server port when running godcr in http mode."`
	// Exchange collector
	DisableExchangeTicks bool     `long:"disablexcticks" decription:"Disables collection of ticker data from exchanges"`
	DisabledExchanges    []string `long:"disableexchange" description:"Disable data collection for this exchange"`

	// PoW collector
	DisablePow   bool     `long:"disablepow" description:"Disables collection of data for pows"`
	DisabledPows []string `long:"disabledpow" description:"Disable data collection for this Pow"`
	PowInterval  int64    `long:"powI" description:"Collection interval for Pow"`

	// VSP
	DisableVSP  bool  `long:"disablevsp" description:"Disables periodic voting service pool status collection"`
	VSPInterval int64 `long:"vspinterval" description:"Collection interval for pool status collection"`

	// Mempool
	DisableMempool  bool   `long:"disablemempool" description:"Disable mempool data collection"`
	DcrdRpcServer   string `long:"dcrdrpcserver" description:"Hostname/IP and port of dcrd RPC server to connect to (default localhost:9109, testnet: localhost:19109, simnet: localhost:19556)"`
	DcrdNetworkType string `long:"dcrdnetworktype" description:"The network type of the dcrd server to connect to (default: mainnet)"`
	DcrdRpcUser     string `long:"dcrdrpcuser" description:"Daemon RPC user name"`
	DcrdRpcPassword string `long:"dcrdrpcpaswword" description:"Daemon RPC password"`
}

// CommandLineOptions holds the top-level options/flags that are displayed on the command-line menu
type CommandLineOptions struct {
	HttpMode bool `long:"http" description:"Launch http server"`
}

func defaultFileOptions() configFileOptions {
	return configFileOptions{
		LogFile:         defaultLogFilename,
		ConfigFile:      defaultConfigFilename,
		DebugLevel:      defaultLogLevel,
		VSPInterval:     300,
		PowInterval:     300,
		DcrdRpcServer:	 defaultDcrdServer,
		DcrdNetworkType: defaultDcrdNetworkType,
	}
}

func defaultConfig() config {
	return config{
		configFileOptions: defaultFileOptions(),
	}
}

// validLogLevel returns whether or not logLevel is a valid debug log level.
func validLogLevel(logLevel string) bool {
	_, ok := slog.LevelFromString(logLevel)
	return ok
}

// supportedSubsystems returns a sorted slice of the supported subsystems for
// logging purposes.
func supportedSubsystems() []string {
	// Convert the subsystemLoggers map keys to a slice.
	subsystems := make([]string, 0, len(subsystemLoggers))
	for subsysID := range subsystemLoggers {
		subsystems = append(subsystems, subsysID)
	}

	// Sort the subsytems for stable display.
	sort.Strings(subsystems)
	return subsystems
}

// parseAndSetDebugLevels attempts to parse the specified debug level and set
// the levels accordingly.  An appropriate error is returned if anything is
// invalid.
func parseAndSetDebugLevels(debugLevel string) error {
	// When the specified string doesn't have any delimters, treat it as
	// the log level for all subsystems.
	if !strings.Contains(debugLevel, ",") && !strings.Contains(debugLevel, "=") {
		// Validate debug log level.
		if !validLogLevel(debugLevel) {
			str := "The specified debug level [%v] is invalid"
			return fmt.Errorf(str, debugLevel)
		}

		// Change the logging level for all subsystems.
		setLogLevels(debugLevel)

		return nil
	}

	// Split the specified string into subsystem/level pairs while detecting
	// issues and update the log levels accordingly.
	for _, logLevelPair := range strings.Split(debugLevel, ",") {
		if !strings.Contains(logLevelPair, "=") {
			str := "The specified debug level contains an invalid " +
				"subsystem/level pair [%v]"
			return fmt.Errorf(str, logLevelPair)
		}

		// Extract the specified subsystem and log level.
		fields := strings.Split(logLevelPair, "=")
		subsysID, logLevel := fields[0], fields[1]

		// Validate subsystem.
		if _, exists := subsystemLoggers[subsysID]; !exists {
			str := "The specified subsystem [%v] is invalid -- " +
				"supported subsytems %v"
			return fmt.Errorf(str, subsysID, supportedSubsystems())
		}

		// Validate log level.
		if !validLogLevel(logLevel) {
			str := "The specified debug level [%v] is invalid"
			return fmt.Errorf(str, logLevel)
		}

		setLogLevel(subsysID, logLevel)
	}

	return nil
}

func loadConfig() (*config, []string, error) {
	cfg := defaultConfig()
	parser := flags.NewParser(&cfg, flags.IgnoreUnknown)
	err := flags.NewIniParser(parser).ParseFile(cfg.configFileOptions.ConfigFile)
	if err != nil {
		if _, ok := err.(*os.PathError); ok {
			fmt.Printf("Missing config file %s in current directory\n", cfg.configFileOptions.ConfigFile)
		} else {
			return nil, nil, err
		}
	}

	unknownArg, err := parser.Parse()
	if err != nil {
		e, ok := err.(*flags.Error)
		if ok && e.Type == flags.ErrHelp {
			os.Exit(0)
		}
		return nil, nil, err
	}

	initLogRotator(cfg.configFileOptions.LogFile)

	// Special show command to list supported subsystems and exit.
	if cfg.configFileOptions.DebugLevel == "show" {
		fmt.Println("Supported subsystems", supportedSubsystems())
		os.Exit(0)
	}

	// Parse, validate, and set debug log level(s).
	if cfg.Quiet {
		cfg.configFileOptions.DebugLevel = "error"
	}

	// Parse, validate, and set debug log level(s).
	if err := parseAndSetDebugLevels(cfg.configFileOptions.DebugLevel); err != nil {
		err = fmt.Errorf("%s: %v", "loadConfig", err.Error())
		fmt.Fprintln(os.Stderr, err)
		parser.WriteHelp(os.Stderr)
		return nil, nil, err
	}

	if cfg.configFileOptions.VSPInterval < 300 {
		log.Warn("VSP collection interval cannot be less that 300, setting to 300")
		cfg.configFileOptions.VSPInterval = 300
	}

	return &cfg, unknownArg, nil
}
