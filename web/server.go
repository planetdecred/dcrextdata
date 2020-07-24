package web

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/decred/dcrd/chaincfg"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/planetdecred/dcrextdata/app/helpers"
	"github.com/planetdecred/dcrextdata/cache"
	"github.com/planetdecred/dcrextdata/commstats"
	"github.com/planetdecred/dcrextdata/exchanges/ticks"
	"github.com/planetdecred/dcrextdata/mempool"
	"github.com/planetdecred/dcrextdata/netsnapshot"
	"github.com/planetdecred/dcrextdata/postgres/models"
	"github.com/planetdecred/dcrextdata/pow"
	"github.com/planetdecred/dcrextdata/vsp"
)

type DataQuery interface {
	ExchangeTickCount(ctx context.Context) (int64, error)
	AllExchangeTicks(ctx context.Context, currencyPair string, defaultInterval, offset, limit int) ([]ticks.TickDto, int64, error)
	AllExchange(ctx context.Context) (models.ExchangeSlice, error)
	FetchExchangeTicks(ctx context.Context, currencyPair, name string, defaultInterval, offset, limit int) ([]ticks.TickDto, int64, error)
	AllExchangeTicksCurrencyPair(ctx context.Context) ([]ticks.TickDtoCP, error)
	CurrencyPairByExchange(ctx context.Context, exchangeName string) ([]ticks.TickDtoCP, error)
	ExchangeTicksChartData(ctx context.Context, filter string, currencyPair string, selectedInterval int, exchanges string) ([]ticks.TickChartData, error)
	AllExchangeTicksInterval(ctx context.Context) ([]ticks.TickDtoInterval, error)
	TickIntervalsByExchangeAndPair(ctx context.Context, exchange string, currencyPair string) ([]ticks.TickDtoInterval, error)

	VspTickCount(ctx context.Context) (int64, error)
	FetchVSPs(ctx context.Context) ([]vsp.VSPDto, error)
	FiltredVSPTicks(ctx context.Context, vspName string, offset, limit int) ([]vsp.VSPTickDto, int64, error)
	AllVSPTicks(ctx context.Context, offset, limit int) ([]vsp.VSPTickDto, int64, error)

	PowCount(ctx context.Context) (int64, error)
	FetchPowData(ctx context.Context, offset, limit int) ([]pow.PowDataDto, int64, error)
	FetchPowDataBySource(ctx context.Context, source string, offset, limit int) ([]pow.PowDataDto, int64, error)
	FetchPowSourceData(ctx context.Context) ([]pow.PowDataSource, error)
	FetchPowChartData(ctx context.Context, pool string, dataType string) ([]pow.PowChartData, error)
	GetPowDistinctDates(ctx context.Context, vsps []string) ([]time.Time, error)

	MempoolCount(ctx context.Context) (int64, error)
	Mempools(ctx context.Context, offtset int, limit int) ([]mempool.Dto, error)

	BlockCount(ctx context.Context) (int64, error)
	Blocks(ctx context.Context, offset int, limit int) ([]mempool.BlockDto, error)
	BlocksWithoutVotes(ctx context.Context, offset int, limit int) ([]mempool.BlockDto, error)

	Votes(ctx context.Context, offset int, limit int) ([]mempool.VoteDto, error)
	VotesByBlock(ctx context.Context, blockHash string) ([]mempool.VoteDto, error)
	VotesCount(ctx context.Context) (int64, error)

	CountRedditStat(ctx context.Context, subreddit string) (int64, error)
	RedditStats(ctx context.Context, subreddit string, offset int, limit int) ([]commstats.Reddit, error)
	CountTwitterStat(ctx context.Context, handle string) (int64, error)
	TwitterStats(ctx context.Context, handle string, offset int, limit int) ([]commstats.Twitter, error)
	CountYoutubeStat(ctx context.Context, channel string) (int64, error)
	YoutubeStat(ctx context.Context, channel string, offset int, limit int) ([]commstats.Youtube, error)
	CountGithubStat(ctx context.Context, repository string) (int64, error)
	GithubStat(ctx context.Context, repository string, offset int, limit int) ([]commstats.Github, error)
	CommunityChart(ctx context.Context, platform string, dataType string, filters map[string]string) ([]commstats.ChartData, error)

	Snapshots(ctx context.Context, offset, limit int, forChart bool) ([]netsnapshot.SnapShot, int64, error)
	SnapshotCount(ctx context.Context) (int64, error)
	LastSnapshotTime(ctx context.Context) (timestamp int64)
	FindNetworkSnapshot(ctx context.Context, timestamp int64) (*netsnapshot.SnapShot, error)
	PreviousSnapshot(ctx context.Context, timestamp int64) (*netsnapshot.SnapShot, error)
	NextSnapshot(ctx context.Context, timestamp int64) (*netsnapshot.SnapShot, error)
	TotalPeerCount(ctx context.Context, timestamp int64) (int64, error)
	SeenNodesByTimestamp(ctx context.Context) ([]netsnapshot.NodeCount, error)
	NetworkPeers(ctx context.Context, timestamp int64, q string, offset int, limit int) ([]netsnapshot.NetworkPeer, int64, error)
	NetworkPeer(ctx context.Context, address string) (*netsnapshot.NetworkPeer, error)
	AverageLatency(ctx context.Context, address string) (int, error)
	PeerCountByUserAgents(ctx context.Context, sources string, offset, limit int) (userAgents []netsnapshot.UserAgentInfo, total int64, err error)
	PeerCountByIPVersion(ctx context.Context, timestamp int64, iPVersion int) (int64, error)
	PeerCountByCountries(ctx context.Context, sources string, offset, limit int) (countries []netsnapshot.CountryInfo, total int64, err error)
	GetIPLocation(ctx context.Context, ip string) (string, int, error)
	AllNodeVersions(ctx context.Context) ([]string, error)
	AllNodeContries(ctx context.Context) ([]string, error)
}

type Server struct {
	templates    map[string]*template.Template
	lock         sync.RWMutex
	db           DataQuery
	activeChain  *chaincfg.Params
	extDbFactory func(name string) (DataQuery, error)
	charts       *cache.Manager
}

func StartHttpServer(httpHost, httpPort string, charts *cache.Manager, db DataQuery,
	activeChain *chaincfg.Params, extDbFactory func(name string) (DataQuery, error)) {

	server := &Server{
		templates:    map[string]*template.Template{},
		db:           db,
		activeChain:  activeChain,
		extDbFactory: extDbFactory,
		charts:       charts,
	}

	router := chi.NewRouter()
	router.Use(middleware.DefaultCompress)
	workDir, _ := os.Getwd()

	filesDir := filepath.Join(workDir, "web/public/dist")
	FileServer(router, "/static", http.Dir(filesDir))
	server.registerHandlers(router)

	// load templates
	server.loadTemplates()

	address := net.JoinHostPort(httpHost, httpPort)

	log.Infof("Starting http server on %s", address)
	err := http.ListenAndServe(address, router)
	if err != nil {
		log.Error("Error starting web server")
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func FileServer(r chi.Router, path string, root http.FileSystem) {
	if strings.ContainsAny(path, "{}*") {
		panic("FileServer does not permit URL parameters.")
	}

	fs := http.StripPrefix(path, http.FileServer(root))

	if path != "/" && path[len(path)-1] != '/' {
		r.Get(path, http.RedirectHandler(path+"/", 301).ServeHTTP)
		path += "/"
	}
	path += "*"

	r.Get(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fs.ServeHTTP(w, r)
	}))
}

func (s *Server) registerHandlers(r *chi.Mux) {
	r.Get("/", s.homePage)
	r.Get("/exchanges", s.getExchangeTicks)
	r.Get("/exchangedata", s.getFilteredExchangeTicks)
	r.Get("/exchangechart", s.getExchangeChartData)
	r.Get("/api/exchanges/intervals", s.tickIntervalsByExchangeAndPair)
	r.Get("/api/exchanges/currency-pairs", s.currencyPairByExchange)
	r.Get("/vsp", s.getVspTicks)
	r.Get("/vsps", s.getFilteredVspTicks)
	r.Get("/pow", s.powPage)
	r.Get("/filteredpow", s.getFilteredPowData)
	r.Get("/mempool", s.mempoolPage)
	r.Get("/getmempool", s.getMempool)
	r.Get("/propagation", s.propagation)
	r.Get("/getpropagationdata", s.getPropagationData)
	r.Get("/getblocks", s.getBlocks)
	r.Get("/blockdata", s.getBlockData)
	r.Get("/getvotes", s.getVotes)
	r.Get("/getVoteByBlock", s.getVoteByBlock)
	r.Get("/votesdata", s.getVoteData)

	r.Get("/community", s.community)
	r.Get("/getCommunityStat", s.getCommunityStat)
	r.Get("/communitychat", s.communityChat)
	r.Get("/stats", s.statsPage)

	r.Get("/nodes", s.snapshot)
	r.With(addTimestampToCtx).Get("/nodes/{timestamp}", s.snapshot)
	r.With(addNodeIPToCtx).Get("/nodes/view/{address}", s.nodeInfo)
	r.Get("/api/snapshots", s.snapshots)
	r.Get("/api/snapshots/chart", s.snapshotsChart)
	r.Get("/api/snapshots/user-agents", s.nodesCountUserAgents)
	r.Get("/api/snapshots/user-agents/chart", s.nodesCountUserAgentsChart)
	r.Get("/api/snapshots/countries", s.nodesCountByCountries)
	r.Get("/api/snapshots/countries/chart", s.nodesCountByCountriesChart)
	r.With(addTimestampToCtx).Get("/api/snapshot/{timestamp}/nodes", s.nodes)
	r.Get("/api/snapshot/nodes/count-by-timestamp", s.nodeCountByTimestamp)
	r.Get("/api/snapshots/ip-info", s.ipInfo)
	r.Get("/api/snapshot/node-versions", s.nodeVersions)
	r.Get("/api/snapshot/node-countries", s.nodeCountries)

	r.With(syncDataType).Get("/api/sync/{dataType}", s.sync)
	r.With(chartTypeCtx).With(chartDataTypeCtx).Get("/api/charts/{chartType}/{chartDataType}", s.chartTypeData)
	r.With(chartTypeCtx).Get("/api/charts/{chartType}", s.chartTypeData)
}

func (s *Server) getExplorerBestBlock(ctx context.Context) (uint32, error) {
	var explorerUrl string
	switch s.activeChain.Name {
	case chaincfg.MainNetParams.Name:
		explorerUrl = "https://explorer.dcrdata.org/api/block/best"
	case chaincfg.TestNet3Params.Name:
		explorerUrl = "https://testnet.dcrdata.org/api/block/best"
	}

	var bestBlock = struct {
		Height uint32 `json:"height"`
	}{}

	err := helpers.GetResponse(ctx, &http.Client{}, explorerUrl, &bestBlock)
	if err != nil {
		return 0, err
	}

	return bestBlock.Height, nil
}
