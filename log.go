// Copyright (c) 2018-2019 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/decred/slog"
	"github.com/jrick/logrotate/rotator"
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

// logWriter implements an io.Writer that outputs to both standard output and
// the write-end pipe of an initialized log rotator.
type logWriter struct{}

// Write writes the data in p to standard out and the log rotator.
func (logWriter) Write(p []byte) (n int, err error) {
	os.Stdout.Write(p)
	return logRotator.Write(p)
}

var (
	// backendLog is the logging backend used to create all subsystem loggers.
	// The backend must not be used before the log rotator has been initialized,
	// or data races and/or nil pointer dereferences will occur.
	backendLog = slog.NewBackend(logWriter{})

	// logRotator is one of the logging outputs.  It should be closed on
	// application shutdown.
	logRotator  *rotator.Rotator
	log         = backendLog.Logger("DEXD")
	excLog      = backendLog.Logger("EXCH")
	pqLog       = backendLog.Logger("PSQL")
	vspLog      = backendLog.Logger("VSPC")
	powLog      = backendLog.Logger("POWL")
	mempoolLog  = backendLog.Logger("MEMP")
	redditLog   = backendLog.Logger("REDD")
	commStatLog = backendLog.Logger("COMM")
	webLog      = backendLog.Logger("WEBL")
	syncLog     = backendLog.Logger("SYNC")
	snapshotLog = backendLog.Logger("NETS")
	cacheLog    = backendLog.Logger("CACH")
)

// subsystemLoggers maps each subsystem identifier to its associated logger.
var subsystemLoggers = map[string]slog.Logger{
	"DATD": log,
	"EXCH": excLog,
	"PSQL": pqLog,
	"VSPC": vspLog,
	"POWL": powLog,
	"MEMP": mempoolLog,
	"COMM": commStatLog,
	"WEBL": webLog,
	"SYNC": syncLog,
	"NETS": snapshotLog,
	"CACH": cacheLog,
}

func init() {
	pow.UseLogger(powLog)
	exchanges.UseLogger(excLog)
	postgres.UseLogger(pqLog)
	vsp.UseLogger(vspLog)
	mempool.UseLogger(mempoolLog)
	commstats.UseLogger(commStatLog)
	web.UseLogger(webLog)
	datasync.UseLogger(syncLog)
	netsnapshot.UseLogger(snapshotLog)
	cache.UseLogger(cacheLog)
}

// initLogRotator initializes the logging rotater to write logs to logFile and
// create roll files in the same directory.  It must be called before the
// package-global log rotater variables are used.
func initLogRotator(logFile string) {
	logDir, _ := filepath.Split(logFile)
	if logDir != "" {
		err := os.MkdirAll(logDir, 0700)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to create log directory: %v\n", err)
			os.Exit(1)
		}
	}

	r, err := rotator.New(logFile, 10*1024, false, 8)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create file rotator: %v\n", err)
		os.Exit(1)
	}

	logRotator = r
}

// setLogLevel sets the logging level for provided subsystem.  Invalid
// subsystems are ignored.  Uninitialized subsystems are dynamically created as
// needed.
func setLogLevel(subsystemID string, logLevel string) {
	// Ignore invalid subsystems.
	logger, ok := subsystemLoggers[subsystemID]
	if !ok {
		return
	}

	// Defaults to info if the log level is invalid.
	level, _ := slog.LevelFromString(logLevel)
	logger.SetLevel(level)
}

// setLogLevels sets the log level for all subsystem loggers to the passed
// level.  It also dynamically creates the subsystem loggers as needed, so it
// can be used to initialize the logging system.
func setLogLevels(logLevel string) {
	// Configure all sub-systems with the new logging level.  Dynamically
	// create loggers as needed.
	for subsystemID := range subsystemLoggers {
		setLogLevel(subsystemID, logLevel)
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

// fatalf logs a message, flushes the logger, and finally exit the process with
// a non-zero return code.
func fatalf(format string, args ...interface{}) {
	log.Errorf(format, args...)
	os.Stdout.Sync()
	logRotator.Close()
	os.Exit(1)
}
