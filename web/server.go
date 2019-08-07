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

	"github.com/go-chi/chi"
	"github.com/raedahgroup/dcrextdata/exchanges/ticks"
	"github.com/raedahgroup/dcrextdata/mempool"
	"github.com/raedahgroup/dcrextdata/postgres/models"
	"github.com/raedahgroup/dcrextdata/pow"
	"github.com/raedahgroup/dcrextdata/vsp"
)

type DataQuery interface {
	AllExchangeTicks(ctx context.Context, currencyPair string, defaultInterval, offset, limit int) ([]ticks.TickDto, int64, error)
	AllExchange(ctx context.Context) (models.ExchangeSlice, error)
	FetchExchangeTicks(ctx context.Context, currencyPair, name string, defaultInterval, offset, limit int) ([]ticks.TickDto, int64, error)
	AllExchangeTicksCurrencyPair(ctx context.Context) ([]ticks.TickDtoCP, error)
	ExchangeTicksChartData(ctx context.Context, filter string, currencyPair string, selectedInterval int, exchanges string) ([]ticks.TickChartData, error)
	AllExchangeTicksInterval(ctx context.Context) ([]ticks.TickDtoInterval, error)

	FetchVSPs(ctx context.Context) ([]vsp.VSPDto, error)
	FiltredVSPTicks(ctx context.Context, vspName string, offset, limit int) ([]vsp.VSPTickDto, int64, error)
	AllVSPTicks(ctx context.Context, offset, limit int) ([]vsp.VSPTickDto, int64, error)
	FetchChartData(ctx context.Context, attribute, vspName string) (records []vsp.ChartData, err error)
	GetVspTickDistinctDates(ctx context.Context, vsps []string) ([]time.Time, error)

	FetchPowData(ctx context.Context, offset, limit int) ([]pow.PowDataDto, int64, error)
	FetchPowDataBySource(ctx context.Context, source string, offset, limit int) ([]pow.PowDataDto, int64, error)
	FetchPowSourceData(ctx context.Context) ([]pow.PowDataSource, error)
	FetchPowChartData(ctx context.Context, pool string, dataType string) ([]pow.PowChartData, error)
	GetPowDistinctDates(ctx context.Context, vsps []string) ([]time.Time, error)

	MempoolCount(ctx context.Context) (int64, error)
	Mempools(ctx context.Context, offtset int, limit int) ([]mempool.MempoolDto, error)
	MempoolsChartData(ctx context.Context, chartFilter string) (models.MempoolSlice, error)

	BlockCount(ctx context.Context) (int64, error)
	Blocks(ctx context.Context, offset int, limit int) ([]mempool.BlockDto, error)
	BlocksWithoutVotes(ctx context.Context, offset int, limit int) ([]mempool.BlockDto, error)

	Votes(ctx context.Context, offset int, limit int) ([]mempool.VoteDto, error)
	VotesCount(ctx context.Context) (int64, error)
	PropagationVoteChartData(ctx context.Context) ([]mempool.PropagationChartData, error)
	PropagationBlockChartData(ctx context.Context) ([]mempool.PropagationChartData, error)
}

type Server struct {
	templates map[string]*template.Template
	lock      sync.RWMutex
	db        DataQuery
}

func StartHttpServer(httpHost, httpPort string, db DataQuery) {
	server := &Server{
		templates: map[string]*template.Template{},
		db:        db,
	}

	router := chi.NewRouter()
	workDir, _ := os.Getwd()

	filesDir := filepath.Join(workDir, "web/public/dist")
	FileServer(router, "/static", http.Dir(filesDir))
	server.registerHandlers(router)

	// load templates
	server.loadTemplates()

	address := net.JoinHostPort(httpHost, httpPort)

	log.Infof("starting http server on %s\n", address)
	err := http.ListenAndServe(address, router)
	if err != nil {
		log.Errorf("Error starting web server")
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
	r.Get("/exchange", s.getFilteredExchangeTicks)
	r.Get("/exchangechart", s.getChartData)
	r.Get("/vsp", s.getVspTicks)
	r.Get("/vspchartdata", s.vspChartData)
	r.Get("/vsps", s.getFilteredVspTicks)
	r.Get("/pow", s.getPowData)
	r.Get("/filteredpow", s.getFilteredPowData)
	r.Get("/powchart", s.getPowChartData)
	r.Get("/mempool", s.mempoolPage)
	r.Get("/mempoolcharts", s.getMempoolChartData)
	r.Get("/getmempool", s.getMempool)
	r.Get("/propagation", s.propagation)
	r.Get("/getpropagationdata", s.getPropagationData)
	r.Get("/propagationchartdata", s.propagationChartData)
	r.Get("/getblocks", s.getBlocks)
	r.Get("/blockdata", s.getBlockData)
	r.Get("/getvotes", s.getVotes)
	r.Get("/votesdata", s.getVoteData)
}
}
