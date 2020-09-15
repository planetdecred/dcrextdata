// Copyright (c) 2018-2019 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package config

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"

	"github.com/decred/dcrd/dcrutil"
	flags "github.com/jessevdk/go-flags"
)

const (
	defaultConfigFileName      = "dcrextdata.conf"
	sampleConfigFileName       = "./sample-dcrextdata.conf"
	defaultLogFileName         = "dcrextdata.log"
	defaultChartsCacheDump     = "charts-cache.glob"
	Hint                       = `Run dcrextdata < --http > to start http server or dcrextdata < --help > for help.`
	defaultDbHost              = "localhost"
	defaultDbPort              = "5432"
	defaultDbUser              = "postgres"
	defaultDbPass              = "dbpass"
	defaultDbName              = "dcrextdata"
	defaultLogLevel            = InfoLogLevel
	defaultHttpHost            = "127.0.0.1"
	defaultHttpPort            = "7770"
	defaultDcrdServer          = "127.0.0.1:9109"
	defaultDcrdUser            = "rpcuser"
	defaultDcrdPassword        = "rpcpass"
	defaultDcrdNetworkType     = "mainnet"
	defaultMempoolInterval     = 60
	defaultVSPInterval         = 300
	defaultPowInterval         = 300
	defaultSyncInterval        = 60
	defaultSnapshotInterval    = 720
	defaultRedditInterval      = 60
	defaultTwitterStatInterval = 60 * 24
	defaultGithubStatInterval  = 60 * 24
	defaultYoutubeInterval     = 60 * 24

	//dcrseeder
	defaultSeeder            = "127.0.0.1"
	defaultSeederPort        = 9108
	maxPeerConnectionFailure = 3

	// log levels
	TraceLogLevel   = "trace"
	DebugLogLevel   = "debug"
	InfoLogLevel    = "info"
	WarningLogLevel = "warning"
	ErrorLogLevel   = "error"
)

var (
	defaultHomeDir        = dcrutil.AppDataDir("dcrextdata", false)
	defaultCacheDir       = filepath.Join(defaultHomeDir, "data")
	defaultConfigFilename = filepath.Join(defaultHomeDir, defaultConfigFileName)
	defaultLogFilename    = filepath.Join(defaultHomeDir, "log", defaultLogFileName)

	defaultSubreddits          = []string{"decred"}
	defaultTwitterHandles      = []string{"decredproject"}
	defaultGithubRepositories  = []string{"decred/dcrd", "decred/dcrdata", "decred/dcrwallet", "decred/politeia", "decred/decrediton"}
	defaultYoutubeChannelNames = []string{"Decred"}
	defaultYoutubeChannelId    = []string{"UCJ2bYDaPYHpSmJPh_M5dNSg"}
)

func defaultFileOptions() ConfigFileOptions {
	cfg := ConfigFileOptions{
		LogFile:          defaultLogFilename,
		CacheDir:         defaultCacheDir,
		DBHost:           defaultDbHost,
		DBPort:           defaultDbPort,
		DBUser:           defaultDbUser,
		DBPass:           defaultDbPass,
		DBName:           defaultDbName,
		LogLevel:         defaultLogLevel,
		VSPInterval:      defaultVSPInterval,
		PowInterval:      defaultPowInterval,
		MempoolInterval:  defaultMempoolInterval,
		DcrdNetworkType:  defaultDcrdNetworkType,
		DcrdRpcServer:    defaultDcrdServer,
		DcrdRpcUser:      defaultDcrdUser,
		DcrdRpcPassword:  defaultDcrdPassword,
		HTTPHost:         defaultHttpHost,
		HTTPPort:         defaultHttpPort,
		SyncInterval:     defaultSyncInterval,
		ChartsCacheDump:  defaultChartsCacheDump,
		EnableChartCache: true,
	}

	cfg.RedditStatInterval = defaultRedditInterval
	cfg.Subreddit = defaultSubreddits
	cfg.TwitterStatInterval = defaultTwitterStatInterval
	cfg.TwitterHandles = defaultTwitterHandles
	cfg.GithubStatInterval = defaultGithubStatInterval
	cfg.GithubRepositories = defaultGithubRepositories
	cfg.YoutubeStatInterval = defaultYoutubeInterval
	cfg.YoutubeChannelName = defaultYoutubeChannelNames
	cfg.YoutubeChannelId = defaultYoutubeChannelId
	cfg.SnapshotInterval = defaultSnapshotInterval
	cfg.Seeder = defaultSeeder
	cfg.SeederPort = defaultSeederPort
	cfg.MaxPeerConnectionFailure = maxPeerConnectionFailure

	return cfg
}

type Config struct {
	ConfigFileOptions
	CommandLineOptions
}

type ConfigFileOptions struct {
	// General application behaviour
	LogFile  string `short:"L" long:"logfile" description:"File name of the log file"`
	LogLevel string `long:"loglevel" description:"Logging level {trace, debug, info, warn, error, critical}"`
	Quiet    bool   `short:"q" long:"quiet" description:"Easy way to set debuglevel to error"`
	CacheDir string `long:"cachedir" description:"The directory for store cache data"`

	// Postgresql Configuration
	DBHost string `long:"dbhost" description:"Database host"`
	DBPort string `long:"dbport" description:"Database port"`
	DBUser string `long:"dbuser" description:"Database username"`
	DBPass string `long:"dbpass" description:"Database password"`
	DBName string `long:"dbname" description:"Database name"`

	// Http Server
	HTTPHost string `long:"httphost" description:"HTTP server host address or IP when running godcr in http mode."`
	HTTPPort string `long:"httpport" description:"HTTP server port when running godcr in http mode."`

	// pprof
	Cpuprofile string `long:"cpuprofile" description:"write cpu profile to file"`
	Memprofile string `long:"memprofile" description:"write memory profile to file"`

	// Exchange collector
	DisableExchangeTicks bool     `long:"disablexcticks" description:"Disables collection of ticker data from exchanges"`
	DisabledExchanges    []string `long:"disableexchange" description:"Disable data collection for this exchange"`

	// PoW collector
	DisablePow   bool     `long:"disablepow" description:"Disables collection of data for pows"`
	DisabledPows []string `long:"disabledpow" description:"Disable data collection for this Pow"`
	PowInterval  int64    `long:"powI" description:"Collection interval for Pow"`

	// VSP
	DisableVSP  bool  `long:"disablevsp" description:"Disables periodic voting service pool status collection"`
	VSPInterval int64 `long:"vspinterval" description:"Collection interval for pool status collection"`

	// Mempool
	DisableMempool  bool    `long:"disablemempool" description:"Disable mempool data collection"`
	MempoolInterval float64 `long:"mempoolinterval" description:"The duration of time between mempool collection"`
	DcrdRpcServer   string  `long:"dcrdrpcserver" description:"Dcrd rpc server host"`
	DcrdNetworkType string  `long:"dcrdnetworktype" description:"Dcrd rpc network type"`
	DcrdRpcUser     string  `long:"dcrdrpcuser" description:"Your Dcrd rpc username"`
	DcrdRpcPassword string  `long:"dcrdrpcpassword" description:"Your Dcrd rpc password"`
	DisableTLS      bool    `long:"dcrdisabletls" description:"DisableTLS specifies whether transport layer security should be disabled"`

	// sync
	DisableSync   bool     `long:"disablesync" description:"Disables data sharing operation"`
	SyncInterval  int      `long:"syncinterval" description:"The number of minuets between sync operations"`
	SyncSources   []string `long:"syncsource" description:"Address of remote instance to sync data from"`
	SyncDatabases []string `long:"syncdatabase" description:"Database to sync remote data to"`

	// charts
	EnableChartCache bool `long:"enablechartcache" description:"Enable chart data caching"`
	ChartsCacheDump  string

	CommunityStatOptions
	NetworkSnapshotOptions
}

// CommandLineOptions holds the top-level options/flags that are displayed on the command-line menu
type CommandLineOptions struct {
	Reset      bool   `short:"R" long:"reset" description:"Drop all database tables and start over"`
	ConfigFile string `short:"C" long:"configfile" description:"Path to Configuration file"`
	HttpMode   string `long:"http" description:"Launch http server"`
}

type CommunityStatOptions struct {
	// Community stat
	DisableCommunityStat bool     `long:"disablecommstat" description:"Disables periodic community stat collection"`
	RedditStatInterval   int64    `long:"redditstatinterval" description:"Collection interval for Reddit community stat"`
	Subreddit            []string `long:"subreddit" description:"List of subreddit for community stat collection"`
	TwitterHandles       []string `long:"twitterhandle" description:"List of twitter handles community stat collection"`
	TwitterStatInterval  int      `long:"twitterstatinterval" description:"Number of minutes between Twitter stat collection"`
	GithubRepositories   []string `long:"githubrepository" description:"List of Github repositories to track"`
	GithubStatInterval   int      `long:"githubstatinterval" description:"Number of minutes between Github stat collection"`
	YoutubeChannelName   []string `long:"youtubechannelname" description:"List of Youtube channel names to be tracked"`
	YoutubeChannelId     []string `long:"youtubechannelid" description:"List of Youtube channel ID to be tracked"`
	YoutubeStatInterval  int      `long:"youtubestatinterval" description:"Number of minutes between Youtube stat collection"`
	YoutubeDataApiKey    string   `long:"youtubedataapikey" description:"Youtube data API key gotten from google developer console"`
}

type NetworkSnapshotOptions struct {
	DisableNetworkSnapshot   bool   `long:"disablesnapshot" description:"Disable network snapshot"`
	SnapshotInterval         int    `long:"snapshotinterval" description:"The number of minutes between snapshot (default 5)"`
	MaxPeerConnectionFailure int    `long:"maxPeerConnectionFailure" description:"Number of failed connection before a pair is marked a dead"`
	Seeder                   string `short:"s" long:"seeder" description:"IP address of a working node"`
	SeederPort               uint16 `short:"p" long:"seederport" description:"Port of a working node"`
	IpStackAccessKey         string `long:"ipStackAccessKey" description:"IP stack access key https://ipstack.com/"`
	IpLocationProvidingPeer  string `long:"ipLocationProvidingPeer" description:"An optional peer address for getting IP info"`
	TestNet                  bool   `long:"testnet" description:"Use testnet"`
}

func defaultConfig() Config {
	return Config{
		CommandLineOptions: CommandLineOptions{
			ConfigFile: defaultConfigFilename,
			HttpMode:   "true",
		},
		ConfigFileOptions: defaultFileOptions(),
	}
}

func copyFile(sourec, destination string) error {
	from, err := os.Open(sourec)
	if err != nil {
		return err
	}
	defer from.Close()

	to, err := os.OpenFile(destination, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	defer to.Close()

	_, err = io.Copy(to, from)
	if err != nil {
		return err
	}

	return nil
}

func LoadConfig() (*Config, []string, error) {
	cfg := defaultConfig()

	// Pre-parse the command line options to see if an alternative config file
	// or the version flag was specified. Override any environment variables
	// with parsed command line flags.
	preCfg := cfg
	preParser := flags.NewParser(&preCfg, flags.HelpFlag|flags.PassDoubleDash)
	_, flagerr := preParser.Parse()

	if flagerr != nil {
		e, ok := flagerr.(*flags.Error)
		if ok && e.Type != flags.ErrHelp {
			return nil, nil, flagerr
		}
	}

	// create cache dir if not existing
	if _, err := os.Stat(defaultCacheDir); os.IsNotExist(err) {
		if err = os.MkdirAll(defaultCacheDir, 0777); err != nil {
			return nil, nil, fmt.Errorf("error in creating default cache dir - %s", err.Error())
		}
	}

	// if the config file is missing, create the default
	if _, err := os.Stat(defaultConfigFilename); os.IsNotExist(err) {
		if err = copyFile(sampleConfigFileName, defaultConfigFilename); err != nil {
			return nil, nil, fmt.Errorf("Missing default config file and cannot copy the sample - %s", err.Error())
		}
	}
	fmt.Printf("Loading config file from %s\n", preCfg.ConfigFile)
	parser := flags.NewParser(&cfg, flags.IgnoreUnknown)
	err := flags.NewIniParser(parser).ParseFile(preCfg.ConfigFile)
	if err != nil {
		if _, ok := err.(*os.PathError); !ok {
			return nil, nil, err
		}
		fmt.Printf("Missing Config file %s\n", preCfg.ConfigFile)
	}

	unknownArg, err := parser.Parse()
	if err != nil {
		e, ok := err.(*flags.Error)
		if ok && e.Type == flags.ErrHelp {
			os.Exit(0)
		}
		return nil, nil, err
	}

	// network snapshot validation
	if len(cfg.Seeder) == 0 {
		return nil, nil, fmt.Errorf("Please specify a seeder")
	}

	if net.ParseIP(cfg.Seeder) == nil {
		str := "\"%s\" is not a valid textual representation of an IP address"
		return nil, nil, fmt.Errorf(str, cfg.Seeder)
	}

	if len(cfg.SyncDatabases) != len(cfg.SyncSources) {
		return nil, nil, errors.New("You must set the same number of sync source and database.")
	}

	return &cfg, unknownArg, nil
}

// normalizeAddress returns addr with the passed default port appended if
// there is not already a port specified.
func normalizeAddress(addr, defaultPort string) string {
	_, _, err := net.SplitHostPort(addr)
	if err != nil {
		return net.JoinHostPort(addr, defaultPort)
	}
	return addr
}
