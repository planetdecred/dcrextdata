package web

import (
	"database/sql"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/raedahgroup/dcrextdata/cache"
	"github.com/raedahgroup/dcrextdata/commstats"
	"github.com/raedahgroup/dcrextdata/datasync"
	"github.com/raedahgroup/dcrextdata/netsnapshot"
	"github.com/raedahgroup/dcrextdata/postgres/models"
	"github.com/raedahgroup/dcrextdata/pow"
	"github.com/raedahgroup/dcrextdata/vsp"
)

const (
	chartViewOption             = "chart"
	defaultViewOption           = chartViewOption
	mempoolDefaultChartDataType = "size"
	maxPageSize                 = 250
	defaultPageSize             = 20
	defaultInterval             = 1440 // All
	noDataMessage               = "does not have data for the selected query option(s)."

	redditPlatform  = "Reddit"
	twitterPlatform = "Twitter"
	githubPlatform  = "GitHub"
	youtubePlatform = "YouTube"
)

var (
	commStatPlatforms = []string{redditPlatform, twitterPlatform, githubPlatform, youtubePlatform}

	exchangeTickIntervals = map[int]string{
		-1:   "All",
		5:    "5m",
		60:   "1h",
		120:  "2h",
		1440: "1d",
	}

	pageSizeSelector = map[int]int{
		20:  20,
		30:  30,
		50:  50,
		100: 100,
		150: 150,
	}

	propagationRecordSet = map[string]string{
		"blocks": "Blocks",
		"votes":  "Votes",
	}

	allVspDataTypes = []string{
		"Immature",
		"Live",
		"Voted",
		"Missed",
		"Pool-Fees",
		"Proportion-Live",
		"Proportion-Missed",
		"User-Count",
		"Users-Active",
	}
)

// /home
func (s *Server) homePage(res http.ResponseWriter, req *http.Request) {
	mempoolCount, err := s.db.MempoolCount(req.Context())
	if err != nil {
		s.renderError(fmt.Sprintf("Cannot get mempools count, %s", err.Error()), res)
		return
	}

	blocksCount, err := s.db.BlockCount(req.Context())
	if err != nil {
		s.renderError(fmt.Sprintf("Cannot get blocks count, %s", err.Error()), res)
		return
	}

	votesCount, err := s.db.VotesCount(req.Context())
	if err != nil {
		s.renderError(fmt.Sprintf("Cannot get votes count, %s", err.Error()), res)
		return
	}

	powCount, err := s.db.PowCount(req.Context())
	if err != nil {
		s.renderError(fmt.Sprintf("Cannot get PoW count, %s", err.Error()), res)
		return
	}

	vspCount, err := s.db.VspTickCount(req.Context())
	if err != nil {
		s.renderError(fmt.Sprintf("Cannot get VSP count, %s", err.Error()), res)
		return
	}

	exchangeCount, err := s.db.ExchangeTickCount(req.Context())
	if err != nil {
		s.renderError(fmt.Sprintf("Cannot get Exchange count, %s", err.Error()), res)
		return
	}

	data := map[string]interface{}{
		"mempoolCount": mempoolCount,
		"blocksCount":  blocksCount,
		"votesCount":   votesCount,
		"powCount":     powCount,
		"vspCount":     vspCount,
		"exchangeTick": exchangeCount,
	}

	s.render("home.html", data, res)
}

// /exchange
func (s *Server) getExchangeTicks(res http.ResponseWriter, req *http.Request) {
	exchanges, err := s.fetchExchangeData(req)
	if err != nil {
		s.renderError(err.Error(), res)
		return
	}

	s.render("exchange.html", exchanges, res)
}

func (s *Server) getFilteredExchangeTicks(res http.ResponseWriter, req *http.Request) {
	data, err := s.fetchExchangeData(req)
	defer s.renderJSON(data, res)

	if err != nil {
		fmt.Println(err)
		s.renderErrorJSON(err.Error(), res)
		return
	}
}

func (s *Server) fetchExchangeData(req *http.Request) (map[string]interface{}, error) {
	req.ParseForm()
	page := req.FormValue("page")
	selectedExchange := req.FormValue("selected-exchange")
	numberOfRows := req.FormValue("records-per-page")
	selectedCurrencyPair := req.FormValue("selected-currency-pair")
	interval := req.FormValue("selected-interval")
	selectedTick := req.FormValue("selected-tick")
	viewOption := req.FormValue("view-option")

	if viewOption == "" {
		viewOption = defaultViewOption
	}

	if selectedTick == "" {
		selectedTick = "close"
	}

	ctx := req.Context()

	var pageSize int
	numRows, err := strconv.Atoi(numberOfRows)
	if err != nil || numRows <= 0 {
		pageSize = defaultPageSize
	} else if numRows > maxPageSize {
		pageSize = maxPageSize
	} else {
		pageSize = numRows
	}

	selectedInterval, err := strconv.Atoi(interval)
	if err != nil || selectedInterval <= 0 {
		selectedInterval = defaultInterval
	}

	if _, found := exchangeTickIntervals[selectedInterval]; !found {
		selectedInterval = defaultInterval
	}

	pageToLoad, err := strconv.Atoi(page)
	if err != nil || pageToLoad <= 0 {
		pageToLoad = 1
	}

	currencyPairs, err := s.db.AllExchangeTicksCurrencyPair(ctx)
	if err != nil {
		return nil, fmt.Errorf("Cannot fetch currency pair, %s", err.Error())
	}

	if selectedCurrencyPair == "" {
		if viewOption == "table" {
			selectedCurrencyPair = "All"
		} else if len(currencyPairs) > 0 {
			selectedCurrencyPair = currencyPairs[0].CurrencyPair
		}
	}

	offset := (pageToLoad - 1) * pageSize

	data := map[string]interface{}{
		"chartView":            true,
		"selectedViewOption":   viewOption,
		"intervals":            exchangeTickIntervals,
		"pageSizeSelector":     pageSizeSelector,
		"selectedCurrencyPair": selectedCurrencyPair,
		"selectedNum":          pageSize,
		"selectedInterval":     selectedInterval,
		"selectedTick":         selectedTick,
		"currentPage":          pageToLoad,
		"previousPage":         pageToLoad - 1,
		"totalPages":           0,
	}

	allExchangeSlice, err := s.db.AllExchange(ctx)
	if err != nil {
		return nil, fmt.Errorf("Cannot fetch exchanges, %s", err.Error())
	}

	if len(allExchangeSlice) == 0 {
		return nil, fmt.Errorf("No exchange source data. Try running dcrextdata then try again.")
	}
	data["allExData"] = allExchangeSlice

	if len(currencyPairs) == 0 {
		return nil, fmt.Errorf("No currency pairs found. Try running dcrextdata then try again.")
	}
	data["currencyPairs"] = currencyPairs

	if selectedExchange == "" && viewOption == "table" {
		selectedExchange = "All"
	} else if selectedExchange == "" && viewOption == "chart" {
		if len(allExchangeSlice) > 0 {
			selectedExchange = allExchangeSlice[0].Name
		} else {
			return nil, fmt.Errorf("No exchange source data. Try running dcrextdata then try again.")
		}
	}
	data["selectedExchange"] = selectedExchange

	if viewOption == "chart" {
		return data, nil
	}

	allExchangeTicksSlice, totalCount, err := s.db.FetchExchangeTicks(ctx, selectedCurrencyPair, selectedExchange, selectedInterval, offset, pageSize)
	if err != nil {
		return nil, fmt.Errorf("Error in fetching exchange ticks, %s", err.Error())
	}

	if len(allExchangeTicksSlice) == 0 {
		data["message"] = fmt.Sprintf("%s %s", strings.Title(selectedExchange), noDataMessage)
		return data, nil
	}

	data["exData"] = allExchangeTicksSlice
	data["totalPages"] = int(math.Ceil(float64(totalCount) / float64(pageSize)))

	totalTxLoaded := offset + len(allExchangeTicksSlice)
	if int64(totalTxLoaded) < totalCount {
		data["nextPage"] = pageToLoad + 1
	}

	return data, nil
}

func (s *Server) getExchangeChartData(res http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	selectedTick := req.FormValue("selected-tick")
	selectedCurrencyPair := req.FormValue("selected-currency-pair")
	selectedInterval := req.FormValue("selected-interval")
	selectedExchange := req.FormValue("selected-exchange")

	data := map[string]interface{}{}

	ctx := req.Context()
	interval, err := strconv.Atoi(selectedInterval)
	if err != nil {
		s.renderErrorJSON(fmt.Sprintf("Invalid interval, %s", err.Error()), res)
		return
	}

	chartData, err := s.db.ExchangeTicksChartData(ctx, selectedTick, selectedCurrencyPair, interval, selectedExchange)
	if err != nil {
		s.renderErrorJSON(fmt.Sprintf("Cannot fetch chart data, %s", err.Error()), res)
		return
	}
	if len(chartData) == 0 {
		s.renderErrorJSON(fmt.Sprintf("No data to generate %s chart.", selectedExchange), res)
		return
	}

	data["chartData"] = chartData

	defer s.renderJSON(data, res)
}

func (s *Server) tickIntervalsByExchangeAndPair(res http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	selectedCurrencyPair := req.FormValue("currency-pair")
	var result = []struct {
		Label string `json:"label"`
		Value int    `json:"value"`
	}{
		{Label: "All", Value: -1},
	}
	pairs, err := s.db.TickIntervalsByExchangeAndPair(req.Context(), req.FormValue("exchange"), selectedCurrencyPair)
	if err != nil {
		if err.Error() != sql.ErrNoRows.Error() {
			s.renderErrorJSON("error in loading intervals, "+err.Error(), res)
			return
		}
		s.renderJSON(result, res)
		return
	}

	for _, p := range pairs {
		result = append(result, struct {
			Label string `json:"label"`
			Value int    `json:"value"`
		}{
			Label: exchangeTickIntervals[p.Interval],
			Value: p.Interval,
		})
	}
	s.renderJSON(result, res)
}

func (s *Server) currencyPairByExchange(res http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	var result = []string{"All"}
	pairs, err := s.db.CurrencyPairByExchange(req.Context(), req.FormValue("exchange"))
	if err != nil {
		if err.Error() != sql.ErrNoRows.Error() {
			s.renderErrorJSON("error in loading intervals, "+err.Error(), res)
			return
		}
		s.renderJSON(result, res)
		return
	}
	for _, p := range pairs {
		result = append(result, p.CurrencyPair)
	}
	s.renderJSON(result, res)
}

// /vsps
func (s *Server) getVspTicks(res http.ResponseWriter, req *http.Request) {
	vsps, err := s.fetchVSPData(req)
	if err != nil {
		s.renderError(err.Error(), res)
		return
	}

	defer s.render("vsp.html", vsps, res)
}

func (s *Server) getFilteredVspTicks(res http.ResponseWriter, req *http.Request) {
	data, err := s.fetchVSPData(req)
	defer s.renderJSON(data, res)

	if err != nil {
		s.renderErrorJSON(err.Error(), res)
		return
	}
}

func (s *Server) fetchVSPData(req *http.Request) (map[string]interface{}, error) {
	req.ParseForm()
	page := req.FormValue("page")
	selectedVsp := req.FormValue("filter")
	numberOfRows := req.FormValue("records-per-page")
	viewOption := req.FormValue("view-option")
	dataType := req.FormValue("data-type")
	selectedVsps := strings.Split(req.FormValue("vsps"), "|")

	if viewOption == "" {
		viewOption = defaultViewOption
	}

	if dataType == "" {
		dataType = "Immature"
	}

	var pageSize int
	numRows, err := strconv.Atoi(numberOfRows)
	if err != nil || numRows <= 0 {
		pageSize = defaultPageSize
	} else if numRows > maxPageSize {
		pageSize = maxPageSize
	} else {
		pageSize = numRows
	}

	pageToLoad, err := strconv.Atoi(page)
	if err != nil || pageToLoad <= 0 {
		pageToLoad = 1
	}

	if selectedVsp == "" {
		selectedVsp = "All"
	}

	offset := (pageToLoad - 1) * pageSize

	ctx := req.Context()

	data := map[string]interface{}{
		"chartView":          true,
		"selectedViewOption": viewOption,
		"selectedFilter":     selectedVsp,
		"pageSizeSelector":   pageSizeSelector,
		"selectedNum":        pageSize,
		"currentPage":        pageToLoad,
		"previousPage":       pageToLoad - 1,
		"totalPages":         0,
		"allDataTypes":       allVspDataTypes,
		"dataType":           dataType,
		"selectedVsps":       selectedVsps,
	}

	allVspData, err := s.db.FetchVSPs(ctx)
	if err != nil {
		return nil, err
	}

	if len(allVspData) == 0 {
		return nil, fmt.Errorf("No VSP source data. Try running dcrextdata then try again.")
	}
	data["allVspData"] = allVspData

	if viewOption == "chart" {
		return data, nil
	}

	var allVSPSlice []vsp.VSPTickDto
	var totalCount int64
	if selectedVsp == "All" || selectedVsp == "" {
		allVSPSlice, totalCount, err = s.db.AllVSPTicks(ctx, offset, pageSize)
		if err != nil {
			return nil, err
		}
	} else {
		allVSPSlice, totalCount, err = s.db.FiltredVSPTicks(ctx, selectedVsp, offset, pageSize)
		if err != nil {
			return nil, err
		}
	}

	if len(allVSPSlice) == 0 {
		data["message"] = fmt.Sprintf("%s %s", strings.Title(selectedVsp), noDataMessage)
		return data, nil
	}

	data["vspData"] = allVSPSlice
	data["allVspData"] = allVspData
	data["totalPages"] = int(math.Ceil(float64(totalCount) / float64(pageSize)))

	totalTxLoaded := offset + len(allVSPSlice)
	if int64(totalTxLoaded) < totalCount {
		data["nextPage"] = pageToLoad + 1
	}

	return data, nil
}

// /PoW
func (s *Server) powPage(res http.ResponseWriter, req *http.Request) {
	data := map[string]interface{}{}

	pows, err := s.fetchPoWData(req)
	if err != nil {
		s.renderError(err.Error(), res)
		return
	}

	data["pow"] = pows
	defer s.render("pow.html", data, res)
}

func (s *Server) getFilteredPowData(res http.ResponseWriter, req *http.Request) {
	data, err := s.fetchPoWData(req)
	defer s.renderJSON(data, res)

	if err != nil {
		s.renderErrorJSON(err.Error(), res)
		return
	}
}

func (s *Server) fetchPoWData(req *http.Request) (map[string]interface{}, error) {
	req.ParseForm()
	page := req.FormValue("page")
	selectedPow := req.FormValue("filter")
	selectedDataType := req.FormValue("data-type")
	numberOfRows := req.FormValue("records-per-page")
	viewOption := req.FormValue("view-option")
	pools := strings.Split(req.FormValue("pools"), "|")

	if viewOption == "" {
		viewOption = defaultViewOption
	}

	if selectedDataType == "" {
		selectedDataType = "hashrate"
	}

	var pageSize int
	numRows, err := strconv.Atoi(numberOfRows)
	switch {
	case err != nil || numRows <= 0:
		pageSize = defaultPageSize
	case numRows > maxPageSize:
		pageSize = maxPageSize
	default:
		pageSize = numRows
	}

	pageToLoad, err := strconv.Atoi(page)
	if err != nil || pageToLoad <= 0 {
		pageToLoad = 1
	}

	if selectedPow == "" {
		selectedPow = "All"
	}

	offset := (pageToLoad - 1) * pageSize

	ctx := req.Context()

	data := map[string]interface{}{
		"chartView":          true,
		"selectedViewOption": viewOption,
		"selectedFilter":     selectedPow,
		"selectedDataType":   selectedDataType,
		"selectedPools":      pools,
		"pageSizeSelector":   pageSizeSelector,
		"selectedNum":        pageSize,
		"currentPage":        pageToLoad,
		"previousPage":       pageToLoad - 1,
	}

	powSource, err := s.db.FetchPowSourceData(ctx)
	if err != nil {
		return nil, err
	}

	if len(powSource) == 0 {
		return nil, fmt.Errorf("No PoW source data. Try running dcrextdata then try again.")
	}

	data["powSource"] = powSource

	if viewOption == defaultViewOption {
		return data, nil
	}

	var totalCount int64
	var allPowDataSlice []pow.PowDataDto
	if selectedPow == "All" || selectedPow == "" {
		allPowDataSlice, totalCount, err = s.db.FetchPowData(ctx, offset, pageSize)
		if err != nil {
			return nil, err
		}
	} else {
		allPowDataSlice, totalCount, err = s.db.FetchPowDataBySource(ctx, selectedPow, offset, pageSize)
		if err != nil {
			return nil, err
		}
	}

	if len(allPowDataSlice) == 0 {
		data["message"] = fmt.Sprintf("%s %s", strings.Title(selectedPow), noDataMessage)
		return data, nil
	}

	data["powData"] = allPowDataSlice
	data["totalPages"] = int(math.Ceil(float64(totalCount) / float64(pageSize)))

	totalTxLoaded := offset + len(allPowDataSlice)
	if int64(totalTxLoaded) < totalCount {
		data["nextPage"] = pageToLoad + 1
	}

	return data, nil
}

// /mempool
func (s *Server) mempoolPage(res http.ResponseWriter, req *http.Request) {
	data := map[string]interface{}{}

	mempoolData, err := s.fetchMempoolData(req)
	if err != nil {
		s.renderError(err.Error(), res)
		return
	}

	data["mempool"] = mempoolData
	data["blockTime"] = s.activeChain.TargetTimePerBlock.Seconds()

	s.render("mempool.html", data, res)
}

// /getmempool
func (s *Server) getMempool(res http.ResponseWriter, req *http.Request) {
	data, err := s.fetchMempoolData(req)
	defer s.renderJSON(data, res)

	if err != nil {
		s.renderErrorJSON(err.Error(), res)
		return
	}
}

func (s *Server) fetchMempoolData(req *http.Request) (map[string]interface{}, error) {
	req.ParseForm()
	page := req.FormValue("page")
	numberOfRows := req.FormValue("records-per-page")
	viewOption := req.FormValue("view-option")
	chartDataType := req.FormValue("chart-data-type")

	if chartDataType == "" {
		chartDataType = mempoolDefaultChartDataType
	}

	if viewOption == "" {
		viewOption = defaultViewOption
	}

	var pageSize int
	numRows, err := strconv.Atoi(numberOfRows)
	if err != nil || numRows <= 0 {
		pageSize = defaultPageSize
	} else if numRows > maxPageSize {
		pageSize = maxPageSize
	} else {
		pageSize = numRows
	}

	pageToLoad, err := strconv.Atoi(page)
	if err != nil || pageToLoad <= 0 {
		pageToLoad = 1
	}

	offset := (pageToLoad - 1) * pageSize

	data := map[string]interface{}{
		"chartView":            true,
		"chartDataType":        chartDataType,
		"selectedViewOption":   viewOption,
		"pageSizeSelector":     pageSizeSelector,
		"selectedNumberOfRows": pageSize,
		"currentPage":          pageToLoad,
		"previousPage":         pageToLoad - 1,
		"totalPages":           0,
	}

	if viewOption == defaultViewOption {
		return data, nil
	}

	ctx := req.Context()

	mempoolSlice, err := s.db.Mempools(ctx, offset, pageSize)
	if err != nil {
		return nil, err
	}

	totalCount, err := s.db.MempoolCount(ctx)
	if err != nil {
		return nil, err
	}

	if len(mempoolSlice) == 0 {
		data["message"] = fmt.Sprintf("Mempool %s", noDataMessage)
		return data, nil
	}

	data["mempoolData"] = mempoolSlice
	data["totalPages"] = int(math.Ceil(float64(totalCount) / float64(pageSize)))

	totalTxLoaded := offset + len(mempoolSlice)
	if int64(totalTxLoaded) < totalCount {
		data["nextPage"] = pageToLoad + 1
	}

	return data, nil
}

// /propagation
func (s *Server) propagation(res http.ResponseWriter, req *http.Request) {
	data := map[string]interface{}{}

	block, err := s.fetchPropagationData(req)
	if err != nil {
		s.renderError(err.Error(), res)
		return
	}

	data["propagation"] = block
	data["blockTime"] = s.activeChain.TargetTimePerBlock.Seconds()

	s.render("propagation.html", data, res)
}

// /getPropagationData
func (s *Server) getPropagationData(res http.ResponseWriter, req *http.Request) {
	data, err := s.fetchPropagationData(req)
	if err != nil {
		s.renderErrorJSON(err.Error(), res)
		return
	}
	s.renderJSON(data, res)
}

func (s *Server) fetchPropagationData(req *http.Request) (map[string]interface{}, error) {
	req.ParseForm()
	page := req.FormValue("page")
	numberOfRows := req.FormValue("records-per-page")
	viewOption := req.FormValue("view-option")
	recordSet := req.FormValue("record-set")
	chartType := req.FormValue("chart-type")

	if viewOption == "" {
		viewOption = "chart"
	}

	if recordSet == "" {
		recordSet = "both"
	}

	if chartType == "" {
		chartType = "block-propagation"
	}

	var pageSize int
	numRows, err := strconv.Atoi(numberOfRows)
	if err != nil || numRows <= 0 {
		pageSize = defaultPageSize
	} else if numRows > maxPageSize {
		pageSize = maxPageSize
	} else {
		pageSize = numRows
	}

	pageToLoad, err := strconv.Atoi(page)
	if err != nil || pageToLoad <= 0 {
		pageToLoad = 1
	}

	offset := (pageToLoad - 1) * pageSize

	ctx := req.Context()

	syncSources, _ := datasync.RegisteredSources()

	data := map[string]interface{}{
		"chartView":            viewOption == "chart",
		"selectedViewOption":   viewOption,
		"chartType":            chartType,
		"currentPage":          pageToLoad,
		"propagationRecordSet": propagationRecordSet,
		"pageSizeSelector":     pageSizeSelector,
		"selectedRecordSet":    recordSet,
		"both":                 true,
		"selectedNum":          pageSize,
		"url":                  "/propagation",
		"previousPage":         pageToLoad - 1,
		"totalPages":           0,
		"syncSources":          strings.Join(syncSources, "|"),
	}

	if viewOption == defaultViewOption {
		return data, nil
	}

	blockSlice, err := s.db.BlocksWithoutVotes(ctx, offset, pageSize)
	if err != nil {
		return nil, err
	}

	for i := 0; i <= 1 && i <= len(blockSlice)-1; i++ {
		votes, err := s.db.VotesByBlock(ctx, blockSlice[i].BlockHash)
		if err != nil {
			return nil, err
		}
		blockSlice[i].Votes = votes
	}

	totalCount, err := s.db.BlockCount(ctx)
	if err != nil {
		return nil, err
	}

	if len(blockSlice) == 0 {
		data["message"] = fmt.Sprintf("%s %s", recordSet, noDataMessage)
		return data, nil
	}

	data["records"] = blockSlice
	data["totalPages"] = int(math.Ceil(float64(totalCount) / float64(pageSize)))

	totalTxLoaded := offset + len(blockSlice)
	if int64(totalTxLoaded) < totalCount {
		data["nextPage"] = pageToLoad + 1
	}

	return data, nil
}

// /getblocks
func (s *Server) getBlocks(res http.ResponseWriter, req *http.Request) {
	data, err := s.fetchBlockData(req)
	defer s.renderJSON(data, res)

	if err != nil {
		s.renderErrorJSON(err.Error(), res)
		return
	}
}

func (s *Server) getBlockData(res http.ResponseWriter, req *http.Request) {
	data := map[string]interface{}{}

	block, err := s.fetchBlockData(req)
	if err != nil {
		s.renderError(err.Error(), res)
		return
	}

	data["propagation"] = block
	defer s.render("propagation.html", data, res)
}

func (s *Server) fetchBlockData(req *http.Request) (map[string]interface{}, error) {
	req.ParseForm()
	page := req.FormValue("page")
	numberOfRows := req.FormValue("records-per-page")
	viewOption := req.FormValue("view-option")

	if viewOption == "" {
		viewOption = defaultViewOption
	}

	var pageSize int
	numRows, err := strconv.Atoi(numberOfRows)
	if err != nil || numRows <= 0 {
		pageSize = defaultPageSize
	} else if numRows > maxPageSize {
		pageSize = maxPageSize
	} else {
		pageSize = numRows
	}

	pageToLoad, err := strconv.Atoi(page)
	if err != nil || pageToLoad <= 0 {
		pageToLoad = 1
	}

	offset := (pageToLoad - 1) * pageSize

	ctx := req.Context()

	data := map[string]interface{}{
		"chartView":            true,
		"selectedViewOption":   defaultViewOption,
		"currentPage":          pageToLoad,
		"propagationRecordSet": propagationRecordSet,
		"pageSizeSelector":     pageSizeSelector,
		"selectedFilter":       "blocks",
		"blocks":               true,
		"url":                  "/blockdata",
		"selectedNum":          pageSize,
		"previousPage":         pageToLoad - 1,
		"totalPages":           pageToLoad,
	}

	if viewOption == defaultViewOption {
		return data, nil
	}

	blocksSlice, err := s.db.BlocksWithoutVotes(ctx, offset, pageSize)
	if err != nil {
		return nil, err
	}

	if len(blocksSlice) == 0 {
		data["message"] = fmt.Sprintf("Blocks %s", noDataMessage)
		return data, nil
	}

	totalCount, err := s.db.BlockCount(ctx)
	if err != nil {
		return nil, err
	}

	data["records"] = blocksSlice
	data["totalPages"] = int(math.Ceil(float64(totalCount) / float64(pageSize)))

	totalTxLoaded := offset + len(blocksSlice)
	if int64(totalTxLoaded) < totalCount {
		data["nextPage"] = pageToLoad + 1
	}

	return data, nil
}

// /getvotes
func (s *Server) getVotes(res http.ResponseWriter, req *http.Request) {
	data, err := s.fetchVoteData(req)

	if err != nil {
		s.renderErrorJSON(err.Error(), res)
		return
	}
	defer s.renderJSON(data, res)
}

func (s *Server) getVoteData(res http.ResponseWriter, req *http.Request) {
	data := map[string]interface{}{}

	vote, err := s.fetchVoteData(req)
	if err != nil {
		s.renderError(err.Error(), res)
		return
	}

	data["propagation"] = vote
	defer s.render("propagation.html", data, res)
}

func (s *Server) fetchVoteData(req *http.Request) (map[string]interface{}, error) {
	req.ParseForm()
	page := req.FormValue("page")
	numberOfRows := req.FormValue("records-per-page")
	viewOption := req.FormValue("view-option")

	if viewOption == "" {
		viewOption = defaultViewOption
	}

	var pageSize int
	numRows, err := strconv.Atoi(numberOfRows)
	if err != nil || numRows <= 0 {
		pageSize = defaultPageSize
	} else if numRows > maxPageSize {
		pageSize = maxPageSize
	} else {
		pageSize = numRows
	}

	pageToLoad, err := strconv.Atoi(page)
	if err != nil || pageToLoad <= 0 {
		pageToLoad = 1
	}

	offset := (pageToLoad - 1) * pageSize

	ctx := req.Context()

	data := map[string]interface{}{
		"chartView":            true,
		"selectedViewOption":   defaultViewOption,
		"currentPage":          pageToLoad,
		"propagationRecordSet": propagationRecordSet,
		"pageSizeSelector":     pageSizeSelector,
		"selectedFilter":       "votes",
		"votes":                true,
		"selectedNum":          pageSize,
		"url":                  "/votesdata",
		"previousPage":         pageToLoad - 1,
		"totalPages":           pageToLoad,
	}

	if viewOption == defaultViewOption {
		return data, nil
	}

	voteSlice, err := s.db.Votes(ctx, offset, pageSize)
	if err != nil {
		return nil, err
	}

	if len(voteSlice) == 0 {
		data["message"] = fmt.Sprintf("Votes %s", noDataMessage)
		return data, nil
	}

	totalCount, err := s.db.VotesCount(ctx)
	if err != nil {
		return nil, err
	}

	data["voteRecords"] = voteSlice
	data["totalPages"] = int(math.Ceil(float64(totalCount) / float64(pageSize)))

	totalTxLoaded := offset + len(voteSlice)
	if int64(totalTxLoaded) < totalCount {
		data["nextPage"] = pageToLoad + 1
	}

	return data, nil
}

func (s *Server) getVoteByBlock(res http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	hash := req.FormValue("block_hash")
	votes, err := s.db.VotesByBlock(req.Context(), hash)
	if err != nil {
		s.renderErrorJSON(err.Error(), res)
		return
	}
	defer s.renderJSON(votes, res)
}

// /community
func (s *Server) community(res http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	pageStr := req.FormValue("page")
	viewOption := req.FormValue("view-option")
	selectedNumStr := req.FormValue("records-per-page")
	platform := req.FormValue("platform")
	subreddit := req.FormValue("subreddit")
	dataType := req.FormValue("data-type")
	twitterHandle := req.FormValue("twitter-handle")
	repository := req.FormValue("repository")
	channel := req.FormValue("channel")

	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}

	if viewOption == "" {
		viewOption = "chart"
	}

	if platform == "" {
		platform = commStatPlatforms[0]
	}

	if subreddit == "" && len(commstats.Subreddits()) > 0 {
		subreddit = commstats.Subreddits()[0]
	}

	if twitterHandle == "" && len(commstats.TwitterHandles()) > 0 {
		twitterHandle = commstats.TwitterHandles()[0]
	}

	if repository == "" && len(commstats.Repositories()) > 0 {
		repository = commstats.Repositories()[0]
	}

	if channel == "" && len(commstats.YoutubeChannels()) > 0 {
		channel = commstats.YoutubeChannels()[0]
	}

	selectedNum, _ := strconv.Atoi(selectedNumStr)
	if selectedNum == 0 {
		selectedNum = 20
	}

	var previousPage, nextPage int
	if page > 1 {
		previousPage = page - 1
	} else {
		previousPage = 1
	}

	nextPage = page + 1

	data := map[string]interface{}{
		"page":             page,
		"viewOption":       viewOption,
		"platforms":        commStatPlatforms,
		"platform":         platform,
		"subreddits":       commstats.Subreddits(),
		"subreddit":        subreddit,
		"twitterHandles":   commstats.TwitterHandles(),
		"twitterHandle":    twitterHandle,
		"repositories":     commstats.Repositories(),
		"repository":       repository,
		"channels":         commstats.YoutubeChannels(),
		"channel":          channel,
		"dataType":         dataType,
		"currentPage":      page,
		"pageSizeSelector": pageSizeSelector,
		"selectedNum":      selectedNum,
		"previousPage":     previousPage,
		"nextPage":         nextPage,
	}

	s.render("community.html", data, res)
}

// getCommunityStat
func (s *Server) getCommunityStat(resp http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	plarform := req.FormValue("platform")
	pageStr := req.FormValue("page")
	pageSizeStr := req.FormValue("records-per-page")
	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}

	pageSize, _ := strconv.Atoi(pageSizeStr)
	if pageSize < 1 {
		pageSize = 20
	}

	var stats interface{}
	var columnHeaders []string
	var totalCount int64
	var err error

	offset := (page - 1) * pageSize

	switch plarform {
	case redditPlatform:
		subreddit := req.FormValue("subreddit")
		stats, err = s.db.RedditStats(req.Context(), subreddit, offset, pageSize)
		if err != nil {
			s.renderErrorJSON(fmt.Sprintf("cannot fetch Reddit stat, %s", err.Error()), resp)
			return
		}

		totalCount, err = s.db.CountRedditStat(req.Context(), subreddit)
		if err != nil {
			s.renderErrorJSON(fmt.Sprintf("cannot fetch Reddit stat, %s", err.Error()), resp)
			return
		}

		columnHeaders = append(columnHeaders, "Date", "Subscribers", "Accounts Active")
	case twitterPlatform:
		handle := req.FormValue("twitter-handle")
		stats, err = s.db.TwitterStats(req.Context(), handle, offset, pageSize)
		if err != nil {
			s.renderErrorJSON(fmt.Sprintf("cannot fetch Twitter stat, %s", err.Error()), resp)
			return
		}

		totalCount, err = s.db.CountTwitterStat(req.Context(), handle)
		if err != nil {
			s.renderErrorJSON(fmt.Sprintf("cannot fetch Twitter stat, %s", err.Error()), resp)
			return
		}

		columnHeaders = append(columnHeaders, "Date", "Followers")
	case githubPlatform:
		repository := req.FormValue("repository")
		stats, err = s.db.GithubStat(req.Context(), repository, offset, pageSize)
		if err != nil {
			s.renderErrorJSON(fmt.Sprintf("cannot fetch Github stat, %s", err.Error()), resp)
			return
		}

		totalCount, err = s.db.CountGithubStat(req.Context(), repository)
		if err != nil {
			s.renderErrorJSON(fmt.Sprintf("cannot fetch Github stat, %s", err.Error()), resp)
			return
		}

		columnHeaders = append(columnHeaders, "Date", "Stars", "Forks")
	case youtubePlatform:
		channel := req.FormValue("channel")
		stats, err = s.db.YoutubeStat(req.Context(), channel, offset, pageSize)
		if err != nil {
			s.renderErrorJSON(fmt.Sprintf("cannot fetch Youtbue stat, %s", err.Error()), resp)
			return
		}

		totalCount, err = s.db.CountYoutubeStat(req.Context(), channel)
		if err != nil {
			s.renderErrorJSON(fmt.Sprintf("cannot fetch Youtbue stat, %s", err.Error()), resp)
			return
		}

		columnHeaders = append(columnHeaders, "Date", "Subscribers", "View Count")
	}

	totalPages := totalCount / int64(pageSize)
	if totalCount > totalPages*int64(pageSize) {
		totalPages += 1
	}

	s.renderJSON(map[string]interface{}{
		"stats":       stats,
		"columns":     columnHeaders,
		"total":       totalCount,
		"totalPages":  totalPages,
		"currentPage": page,
	}, resp)
}

// /communitychat
func (s *Server) communityChat(resp http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	platform := req.FormValue("platform")
	dataType := req.FormValue("data-type")

	var yLabel, subAccount string
	switch platform {
	case githubPlatform:
		if dataType == models.GithubColumns.Folks {
			yLabel = "Forks"
		} else {
			yLabel = "Stars"
		}
		subAccount = req.FormValue("repository")
	case twitterPlatform:
		yLabel = "Followers"
		dataType = models.TwitterColumns.Followers
		subAccount = req.FormValue("twitter-handle")
	case redditPlatform:
		if dataType == models.RedditColumns.ActiveAccounts {
			yLabel = "Active Accounts"
		} else if dataType == models.RedditColumns.Subscribers {
			yLabel = "Subscribers"
		}
		subAccount = req.FormValue("subreddit")
	case youtubePlatform:
		if dataType == models.YoutubeColumns.ViewCount {
			yLabel = "View Count"
		} else if dataType == models.YoutubeColumns.Subscribers {
			yLabel = "Subscribers"
		}
		subAccount = req.FormValue("channel")
	}

	if dataType == "" {
		s.renderErrorJSON("Data type cannot be empty", resp)
		return
	}

	var dates, records cache.ChartUints
	dateKey := fmt.Sprintf("%s-%s-%s-%s", cache.Community, platform, subAccount, cache.TimeAxis)
	if err := s.charts.ReadVal(dateKey, &dates); err != nil {
		s.renderErrorJSON(fmt.Sprintf("Cannot fetch chart data, %s, %s", err.Error(), dateKey), resp)
		return
	}

	dataKey := fmt.Sprintf("%s-%s-%s-%s", cache.Community, platform, subAccount, dataType)
	if err := s.charts.ReadVal(dataKey, &records); err != nil {
		s.renderErrorJSON(fmt.Sprintf("Cannot fetch chart data, %s, %s", err.Error(), dataKey), resp)
		return
	}

	s.renderJSON(map[string]interface{}{
		"x":      dates,
		"y":      records,
		"ylabel": yLabel,
	}, resp)
}

// /nodes
func (s *Server) snapshot(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	page, _ := strconv.Atoi(r.FormValue("page"))
	if page < 1 {
		page = 1
	}

	pageSize, _ := strconv.Atoi(r.FormValue("page-size"))
	if pageSize < 1 {
		pageSize = defaultPageSize
	}

	viewOption := r.FormValue("view-option")
	if viewOption == "" {
		viewOption = defaultViewOption
	}

	var timestamp, previousTimestamp, nextTimestamp int64

	t, _ := strconv.Atoi(r.FormValue("timestamp"))
	timestamp = int64(t)

	if timestamp == 0 {
		timestamp = s.db.LastSnapshotTime(r.Context())
		if timestamp == 0 {
			s.renderError("No snapshot has been taken, please confirm that snapshot taker is configured.", w)
			return
		}
	}

	if snapshot, err := s.db.PreviousSnapshot(r.Context(), timestamp); err == nil {
		previousTimestamp = snapshot.Timestamp
	}

	if snapshot, err := s.db.NextSnapshot(r.Context(), timestamp); err == nil {
		nextTimestamp = snapshot.Timestamp
	}

	snapshot, err := s.db.FindNetworkSnapshot(r.Context(), timestamp)
	if err != nil {
		s.renderError(fmt.Sprintf("Cannot find a snapshot of the specified timestamp, %s", err.Error()), w)
		return
	}

	dataType := r.FormValue("data-type")
	if dataType == "" {
		dataType = "nodes"
	}

	//
	var totalCount, pageCount int64
	switch dataType {
	case "snapshot":
	default:
		totalCount, err = s.db.SnapshotCount(r.Context())
		if err != nil {
			s.renderError(err.Error(), w)
			return
		}
	}

	if totalCount%int64(pageSize) == 0 {
		pageCount = totalCount / int64(pageSize)
	} else {
		pageCount = 1 + (totalCount-totalCount%int64(pageSize))/int64(pageSize)
	}

	var previousPage int = page - 1
	var nextPage int = page + 1

	data := map[string]interface{}{
		"selectedViewOption": viewOption,
		"dataType":           dataType,
		"pageSizeSelector":   pageSizeSelector,
		"previousPage":       previousPage,
		"currentPage":        page,
		"nextPage":           nextPage,
		"pageSize":           pageSize,
		"totalPages":         pageCount,
		"timestamp":          timestamp,
		"height":             snapshot.Height,
		"previousTimestamp":  previousTimestamp,
		"nextTimestamp":      nextTimestamp,
	}

	s.render("nodes.html", data, w)
}

// /api/snapshots
func (s *Server) snapshots(w http.ResponseWriter, r *http.Request) {
	pageSize, err := strconv.Atoi(r.FormValue("page-size"))
	if err != nil {
		pageSize = defaultPageSize
	}

	page, err := strconv.Atoi(r.FormValue("page"))
	if err != nil {
		page = 1
	}

	offset := (page - 1) * pageSize

	result, total, err := s.db.Snapshots(r.Context(), offset, pageSize, false)
	if err != nil {
		s.renderErrorfJSON("Cannot fetch snapshots: %s", w, err.Error())
		return
	}
	var totalPages int64
	if total%int64(pageSize) == 0 {
		totalPages = total / int64(pageSize)
	} else {
		totalPages = 1 + (total-total%int64(pageSize))/int64(pageSize)
	}
	s.renderJSON(map[string]interface{}{"data": result, "total": total, "totalPages": totalPages}, w)
}

// /api/snapshots/chart
func (s *Server) snapshotsChart(w http.ResponseWriter, r *http.Request) {
	result, _, err := s.db.Snapshots(r.Context(), 0, -1, true)
	if err != nil {
		s.renderErrorfJSON("Cannot fetch snapshots: %s", w, err.Error())
		return
	}
	s.renderJSON(result, w)
}

// /nodes/view/{ip}
func (s *Server) nodeInfo(w http.ResponseWriter, r *http.Request) {
	address := getNodeIPFromCtx(r)
	if address == "" {
		s.renderError("Address is required", w)
		return
	}

	ctx := r.Context()

	node, err := s.db.NetworkPeer(ctx, address)
	if err != nil {
		s.renderErrorf("Cannot get not details, %s", w, err.Error())
		return
	}

	averageLatency, err := s.db.AverageLatency(ctx, address)
	if err != nil {
		s.renderErrorf("Cannot load detail, error in getting average latency, %s", w, err.Error())
		return
	}

	bestBlockHeight, err := s.getExplorerBestBlock(ctx)
	if err != nil {
		s.renderErrorf("Cannot load detail, error in getting best block height, %s", w, err.Error())
		return
	}

	s.render("node.html", map[string]interface{}{
		"node": node, "bestBlockHeight": int64(bestBlockHeight),
		"snapshotinterval": netsnapshot.Snapshotinterval(),
		"averageLatency":   averageLatency,
	}, w)
}

// /api/snapshots/user-agents
func (s *Server) nodesCountUserAgents(w http.ResponseWriter, r *http.Request) {
	pageSize, err := strconv.Atoi(r.FormValue("page-size"))
	if err != nil {
		pageSize = defaultPageSize
	}

	page, _ := strconv.Atoi(r.FormValue("page"))
	var offset int
	if page < 1 {
		page = 1
	}
	offset = (page - 1) * pageSize

	var userAgents []netsnapshot.UserAgentInfo
	if err = s.charts.ReadVal(fmt.Sprintf("%s-%s-*", cache.Snapshot, cache.SnapshotNodeVersions), &userAgents); err != nil {
		s.renderErrorfJSON("Cannot fetch data: %s", w, err.Error())
		return
	}

	total := len(userAgents)
	var totalPages int
	if total%(pageSize) == 0 {
		totalPages = total / (pageSize)
	} else {
		totalPages = 1 + (total-total%(pageSize))/(pageSize)
	}

	end := offset + pageSize
	if end >= total {
		end = total - 1
	}
	s.renderJSON(map[string]interface{}{"userAgents": userAgents[offset:end], "totalPages": totalPages}, w)
}

// /api/snapshots/user-agents/chart
func (s *Server) nodesCountUserAgentsChart(w http.ResponseWriter, r *http.Request) {
	limit := -1
	offset := 0
	var err error
	userAgents := []netsnapshot.UserAgentInfo{}
	sources := r.FormValue("sources")
	if len(sources) > 0 {
		userAgents, _, err = s.db.PeerCountByUserAgents(r.Context(), sources, offset, limit)
		if err != nil {
			s.renderErrorfJSON("Cannot fetch data: %s", w, err.Error())
			return
		}
	}

	var datesMap = map[int64]struct{}{}
	var allDates []int64
	var userAgentMap = map[string]struct{}{}
	var allUserAgents []string
	var dateUserAgentCount = make(map[int64]map[string]int64)

	for _, item := range userAgents {
		if _, exists := datesMap[item.Timestamp]; !exists {
			datesMap[item.Timestamp] = struct{}{}
			allDates = append(allDates, item.Timestamp)
		}

		if _, exists := dateUserAgentCount[item.Timestamp]; !exists {
			dateUserAgentCount[item.Timestamp] = make(map[string]int64)
		}

		if _, exists := userAgentMap[item.UserAgent]; !exists {
			userAgentMap[item.UserAgent] = struct{}{}
			allUserAgents = append(allUserAgents, item.UserAgent)
		}
		dateUserAgentCount[item.Timestamp][item.UserAgent] = item.Nodes
	}

	var row = []string{"Date (UTC)"}
	row = append(row, allUserAgents...)
	csv := strings.Join(row, ",") + "\n"

	var minDate, maxDate int64
	for _, timestamp := range allDates {
		if minDate == 0 || timestamp < minDate {
			minDate = timestamp
		}

		if maxDate == 0 || timestamp > maxDate {
			maxDate = timestamp
		}

		row = []string{time.Unix(timestamp, 0).UTC().String()}
		for _, userAgent := range allUserAgents {
			row = append(row, strconv.FormatInt(dateUserAgentCount[timestamp][userAgent], 10))
		}
		csv += strings.Join(row, ",") + "\n"
	}

	s.renderJSON(map[string]interface{}{
		"csv":     csv,
		"minDate": time.Unix(minDate, 0).UTC().String(),
		"maxDate": time.Unix(maxDate, 0).UTC().String(),
	}, w)
}

// /api/snapshots/countries
func (s *Server) nodesCountByCountries(w http.ResponseWriter, r *http.Request) {
	pageSize, err := strconv.Atoi(r.FormValue("page-size"))
	if err != nil {
		pageSize = defaultPageSize
	}

	page, _ := strconv.Atoi(r.FormValue("page"))
	var offset int
	if page < 1 {
		page = 1
	}
	offset = (page - 1) * pageSize

	var countries []netsnapshot.UserAgentInfo
	if err = s.charts.ReadVal(fmt.Sprintf("%s-%s-*", cache.Snapshot, cache.SnapshotLocations), &countries); err != nil {
		s.renderErrorfJSON("Cannot fetch data: %s", w, err.Error())
		return
	}

	total := len(countries)
	var totalPages int
	if total%(pageSize) == 0 {
		totalPages = total / (pageSize)
	} else {
		totalPages = 1 + (total-total%(pageSize))/(pageSize)
	}

	end := offset + pageSize
	if end >= total {
		end = total - 1
	}

	s.renderJSON(map[string]interface{}{"countries": countries[offset:end], "totalPages": totalPages}, w)
}

// /api/snapshots/countries/chart
func (s *Server) nodesCountByCountriesChart(w http.ResponseWriter, r *http.Request) {
	limit := -1
	offset := 0
	sources := r.FormValue("sources")
	var err error
	countries := []netsnapshot.CountryInfo{}
	if len(sources) > 0 {
		countries, _, err = s.db.PeerCountByCountries(r.Context(), sources, offset, limit)
		if err != nil {
			s.renderErrorfJSON("Cannot fetch data: %s", w, err.Error())
			return
		}
	}

	var datesMap = map[int64]struct{}{}
	var allDates []int64
	var countryMap = map[string]struct{}{}
	var allCountries []string
	var dateCountryCount = make(map[int64]map[string]int64)

	for _, item := range countries {
		if _, exists := datesMap[item.Timestamp]; !exists {
			datesMap[item.Timestamp] = struct{}{}
			allDates = append(allDates, item.Timestamp)
		}

		if _, exists := dateCountryCount[item.Timestamp]; !exists {
			dateCountryCount[item.Timestamp] = make(map[string]int64)
		}

		if _, exists := countryMap[item.Country]; !exists {
			countryMap[item.Country] = struct{}{}
			allCountries = append(allCountries, item.Country)
		}
		dateCountryCount[item.Timestamp][item.Country] = item.Nodes
	}

	var row = []string{"Date (UTC)"}
	row = append(row, allCountries...)
	csv := strings.Join(row, ",") + "\n"

	var minDate, maxDate int64
	for _, timestamp := range allDates {
		if minDate == 0 || timestamp < minDate {
			minDate = timestamp
		}

		if maxDate == 0 || timestamp > maxDate {
			maxDate = timestamp
		}

		row = []string{time.Unix(timestamp, 0).UTC().String()}
		for _, country := range allCountries {
			row = append(row, strconv.FormatInt(dateCountryCount[timestamp][country], 10))
		}
		csv += strings.Join(row, ",") + "\n"
	}

	s.renderJSON(map[string]interface{}{
		"csv":     csv,
		"minDate": time.Unix(minDate, 0).UTC().String(),
		"maxDate": time.Unix(maxDate, 0).UTC().String(),
	}, w)
}

// /api/snapshot/nodes/count-by-timestamp
func (s *Server) nodeCountByTimestamp(w http.ResponseWriter, r *http.Request) {
	result, err := s.db.SeenNodesByTimestamp(r.Context())
	if err != nil {
		s.renderErrorfJSON("Cannot fetch node count: %s", w, err.Error())
		return
	}
	s.renderJSON(result, w)
}

// /api/snapshot/{timestamp}/nodes
func (s *Server) nodes(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	page, _ := strconv.Atoi(r.FormValue("page"))
	if page < 1 {
		page = 1
	}

	pageSize, _ := strconv.Atoi(r.FormValue("page-size"))
	if pageSize < 1 {
		pageSize = defaultPageSize
	}

	offset := (page - 1) * pageSize
	query := r.FormValue("q")

	timestamp := getTitmestampCtx(r)
	if timestamp == 0 {
		s.renderErrorJSON("timestamp is required and cannot be zero", w)
		return
	}

	nodes, peerCount, err := s.db.NetworkPeers(r.Context(), timestamp, query, offset, pageSize)
	if err != nil {
		s.renderErrorfJSON("Error in fetching network nodes, %s", w, err.Error())
		return
	}

	rem := peerCount % defaultPageSize
	pageCount := (peerCount - rem) / defaultPageSize
	if rem > 0 {
		pageCount += 1
	}

	s.renderJSON(map[string]interface{}{
		"page":      page,
		"pageCount": pageCount,
		"peerCount": peerCount,
		"nodes":     nodes,
	}, w)
}

// /api/snapshots/ip-info
func (s *Server) ipInfo(w http.ResponseWriter, r *http.Request) {
	address := r.FormValue("ip")
	if address == "" {
		s.renderErrorJSON("please specify a valid IP", w)
		return
	}
	country, version, err := s.db.GetIPLocation(r.Context(), address)
	if err != nil {
		s.renderErrorJSON(err.Error(), w)
		return
	}

	s.renderJSON(map[string]interface{}{"country": country, "ip_version": version}, w)
}

// api/snapshot/node-versions
func (s *Server) nodeVersions(w http.ResponseWriter, r *http.Request) {
	version, err := s.db.AllNodeVersions(r.Context())
	if err != nil {
		s.renderErrorfJSON("Cannot fetch node versions - %s", w, err.Error())
		return
	}
	s.renderJSON(version, w)
}

// api/snapshot/node-countries
func (s *Server) nodeCountries(w http.ResponseWriter, r *http.Request) {
	version, err := s.db.AllNodeContries(r.Context())
	if err != nil {
		s.renderErrorfJSON("Cannot fetch node contries - %s", w, err.Error())
		return
	}
	s.renderJSON(version, w)
}

// api/sync/{dataType}
func (s *Server) sync(res http.ResponseWriter, req *http.Request) {
	dataType := getSyncDataTypeCtx(req)
	result := new(datasync.Result)
	defer s.renderJSON(result, res)
	if dataType == "" {
		result.Message = "Invalid data type"
		return
	}

	dataType = strings.Replace(dataType, "-", "_", -1)

	req.ParseForm()

	last := req.FormValue("last")

	skip, err := strconv.Atoi(req.FormValue("skip"))
	if err != nil {
		result.Message = "Invalid skip value"
		return
	}

	take, err := strconv.Atoi(req.FormValue("take"))
	if err != nil {
		result.Message = "Invalid take value"
		return
	}

	response, err := datasync.Retrieve(req.Context(), dataType, last, skip, take)

	if err != nil {
		result.Message = err.Error()
		return
	}

	result.Success = response.Success
	result.Records = response.Records
	result.TotalCount = response.TotalCount
}

// api/charts/{chartType}/{dataType}
func (s *Server) chartTypeData(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	chartType := getChartTypeCtx(r)
	dataType := getChartDataTypeCtx(r)
	bin := r.URL.Query().Get("bin")
	axis := r.URL.Query().Get("axis")
	extras := r.URL.Query().Get("extras")

	// the extra data passed for exchange chart is the exchange set key
	if chartType == cache.Exchange {
		selectedCurrencyPair := r.FormValue("selected-currency-pair")
		selectedInterval := r.FormValue("selected-interval")
		selectedExchange := r.FormValue("selected-exchange")

		interval, err := strconv.Atoi(selectedInterval)
		if err != nil {
			s.renderErrorJSON(fmt.Sprintf("Invalid interval, %s", err.Error()), w)
			return
		}

		extras = cache.BuildExchangeKey(selectedExchange, selectedCurrencyPair, interval)
	}

	chartData, err := s.charts.Chart(r.Context(), chartType, dataType, axis, bin, strings.Split(extras, "|")...)
	if err != nil {
		s.renderErrorJSON(err.Error(), w)
		log.Warnf(`Error fetching %s chart: %v`, chartType, err)
		return
	}
	s.renderJSONBytes(chartData, w)
}

func (s *Server) statsPage(res http.ResponseWriter, req *http.Request) {
	mempoolCount, err := s.db.MempoolCount(req.Context())
	if err != nil {
		s.renderError(fmt.Sprintf("Cannot get mempools count, %s", err.Error()), res)
		return
	}

	blocksCount, err := s.db.BlockCount(req.Context())
	if err != nil {
		s.renderError(fmt.Sprintf("Cannot get blocks count, %s", err.Error()), res)
		return
	}

	votesCount, err := s.db.VotesCount(req.Context())
	if err != nil {
		s.renderError(fmt.Sprintf("Cannot get votes count, %s", err.Error()), res)
		return
	}

	powCount, err := s.db.PowCount(req.Context())
	if err != nil {
		s.renderError(fmt.Sprintf("Cannot get PoW count, %s", err.Error()), res)
		return
	}

	vspCount, err := s.db.VspTickCount(req.Context())
	if err != nil {
		s.renderError(fmt.Sprintf("Cannot get VSP count, %s", err.Error()), res)
		return
	}

	exchangeCount, err := s.db.ExchangeTickCount(req.Context())
	if err != nil {
		s.renderError(fmt.Sprintf("Cannot get Exchange count, %s", err.Error()), res)
		return
	}

	data := map[string]interface{}{
		"mempoolCount": mempoolCount,
		"blocksCount":  blocksCount,
		"votesCount":   votesCount,
		"powCount":     powCount,
		"vspCount":     vspCount,
		"exchangeTick": exchangeCount,
	}

	s.render("stats.html", data, res)
}
