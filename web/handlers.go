package web

import (
	"database/sql"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/raedahgroup/dcrextdata/commstats"
	"github.com/raedahgroup/dcrextdata/datasync"
	"github.com/raedahgroup/dcrextdata/mempool"
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
		"Pool_Fees",
		"Proportion_Live",
		"Proportion_Missed",
		"User_Count",
		"Users_Active",
	}

	commStatPlatforms = []string{redditPlatform, twitterPlatform, githubPlatform, youtubePlatform}
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
	var result = []struct{
		Label string `json:"label"`
		Value int `json:"value"`
	} {
		{Label: "All", Value: -1},
	}
	pairs, err := s.db.TickIntervalsByExchangeAndPair(req.Context(), req.FormValue("exchange"), selectedCurrencyPair)
	if err != nil {
		if err.Error() != sql.ErrNoRows.Error() {
			s.renderErrorJSON("error in loading intervals, " + err.Error(), res)
			return
		}
		s.renderJSON(result, res)
		return
	}

	

	for _, p := range pairs {
		result = append(result, struct {
			Label string `json:"label"`
			Value int `json:"value"`
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
			s.renderErrorJSON("error in loading intervals, " + err.Error(), res)
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

// vspchartdata
func (s *Server) vspChartData(res http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	selectedExchange := req.FormValue("vsps")
	selectedAttribute := req.FormValue("data-type")

	vsps := strings.Split(selectedExchange, "|")

	ctx := req.Context()
	dates, err := s.db.GetVspTickDistinctDates(ctx, vsps)
	if err != nil {
		s.renderErrorJSON(fmt.Sprintf("Error is getting dates from VSP table, %s", err.Error()), res)
		return
	}

	var vspChartData = struct {
		CSV     string    `json:"csv"`
		MinDate time.Time `json:"min_date"`
		MaxDate time.Time `json:"max_date"`
	}{
		CSV: "Date," + strings.Join(vsps, ",") + "\n",
	}

	var resultMap = map[time.Time][]string{}
	for _, date := range dates {
		if vspChartData.MinDate.IsZero() || date.Before(vspChartData.MinDate) {
			vspChartData.MinDate = date
		}
		if vspChartData.MaxDate.IsZero() || date.After(vspChartData.MaxDate) {
			vspChartData.MaxDate = date
		}
		resultMap[date] = []string{date.String()}
	}

	for _, source := range vsps {
		points, err := s.db.FetchChartData(ctx, selectedAttribute, source)
		if err != nil {
			s.renderErrorJSON(fmt.Sprintf("Error in fetching %s records for %s: %s", selectedAttribute, source, err.Error()), res)
			return
		}

		var vspPointMap = map[time.Time]string{}
		var vspDates []time.Time
		for _, point := range points {
			vspPointMap[point.Date] = point.Record
			vspDates = append(vspDates, point.Date)
		}

		sort.Slice(vspDates, func(i, j int) bool {
			return vspDates[i].Before(vspDates[j])
		})

		for date, _ := range resultMap {
			if date.Year() == 1970 || date.IsZero() {
				continue
			}
			if record, found := vspPointMap[date]; found {
				skip := false
				if record == "0" || record == "" {
					skip = true
					for _, vspDate := range vspDates {
						if vspDate.Before(date) && vspPointMap[vspDate] != "" && vspPointMap[vspDate] != "0" {
							skip = false
						}
					}
				}
				if !skip {
					resultMap[date] = append(resultMap[date], record)
				} else {
					resultMap[date] = append(resultMap[date], "Nan")
				}
			} else {
				// if they have not been any record for this vsp, give a gap (Nan) else use space
				padding := "Nan"
				for _, vspDate := range vspDates {
					if vspDate.Before(date) && vspPointMap[vspDate] != "" && vspPointMap[vspDate] != "0" {
						padding = ""
					}
				}
				resultMap[date] = append(resultMap[date], padding)
			}
		}
	}

	for _, date := range dates {
		if date.Year() == 1970 || date.IsZero() {
			continue
		}

		points := resultMap[date]
		hasAtleastOneRecord := false
		for index, point := range points {
			// the first index is the date
			if index == 0 {
				continue
			}
			if point != "" && point != "Nan" {
				hasAtleastOneRecord = true
			}
		}

		if !hasAtleastOneRecord {
			continue
		}

		vspChartData.CSV += fmt.Sprintf("%s\n", strings.Join(points, ","))
	}

	s.renderJSON(vspChartData, res)
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
		selectedDataType = "pool_hashrate"
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

	if selectedPow == "" {
		selectedPow = "All"
	}

	offset := (pageToLoad - 1) * defaultPageSize

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
		"totalPages":         pageToLoad,
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
	data["totalPages"] = int(math.Ceil(float64(totalCount) / float64(defaultPageSize)))

	totalTxLoaded := offset + len(allPowDataSlice)
	if int64(totalTxLoaded) < totalCount {
		data["nextPage"] = pageToLoad + 1
	}

	return data, nil
}

func (s *Server) getPowChartData(res http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	sources := req.FormValue("pools")
	dataType := req.FormValue("data-type")

	pools := strings.Split(sources, "|")

	ctx := req.Context()
	dates, err := s.db.GetPowDistinctDates(ctx, pools)
	if err != nil {
		s.renderErrorJSON(fmt.Sprintf("Error is getting dates from PoW table, %s", err.Error()), res)
		return
	}

	var powChartData = struct {
		CSV     string    `json:"csv"`
		MinDate time.Time `json:"min_date"`
		MaxDate time.Time `json:"max_date"`
	}{
		CSV: "Date," + strings.Join(pools, ",") + "\n",
	}

	var resultMap = map[time.Time][]string{}
	for _, date := range dates {
		if powChartData.MinDate.IsZero() || date.Before(powChartData.MinDate) {
			powChartData.MinDate = date
		}
		if powChartData.MaxDate.IsZero() || date.After(powChartData.MaxDate) {
			powChartData.MaxDate = date
		}
		resultMap[date] = []string{date.String()}
	}

	for _, source := range pools {
		points, err := s.db.FetchPowChartData(ctx, source, dataType)
		if err != nil {
			s.renderErrorJSON(fmt.Sprintf("Error in fetching %s records for %s: %s", dataType, source, err.Error()), res)
			return
		}

		var pointMaps = map[time.Time]string{}
		var powDates []time.Time
		for _, point := range points {
			pointMaps[point.Date] = point.Record
			powDates = append(powDates, point.Date)
		}

		sort.Slice(powDates, func(i, j int) bool {
			return powDates[i].Before(powDates[j])
		})

		for date, _ := range resultMap {
			if date.Year() == 1970 || date.IsZero() {
				continue
			}
			if record, found := pointMaps[date]; found {
				skip := false
				if record == "0" || record == "" {
					skip = true
					for _, powDate := range powDates {
						if powDate.Before(date) && pointMaps[powDate] != "" && pointMaps[powDate] != "0" {
							skip = false
						}
					}
				}
				if !skip {
					resultMap[date] = append(resultMap[date], record)
				} else {
					resultMap[date] = append(resultMap[date], "Nan")
				}
			} else {
				// if they have not been any record for this vsp, give a gap (Nan) else use space
				padding := "Nan"
				for _, powDate := range powDates {
					if powDate.Before(date) && pointMaps[powDate] != "" && pointMaps[powDate] != "0" {
						padding = ""
					}
				}
				resultMap[date] = append(resultMap[date], padding)
			}
		}
	}

	for _, date := range dates {
		if date.Year() == 1970 || date.IsZero() {
			continue
		}

		points := resultMap[date]
		hasAtleastOneRecord := false
		for index, point := range points {
			// the first index is the date
			if index == 0 {
				continue
			}
			if point != "" && point != "Nan" {
				hasAtleastOneRecord = true
			}
		}

		if !hasAtleastOneRecord {
			continue
		}

		powChartData.CSV += fmt.Sprintf("%s\n", strings.Join(points, ","))
	}

	s.renderJSON(powChartData, res)

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

func (s *Server) getMempoolChartData(res http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	chartFilter := req.FormValue("chart-data-type")
	ctx := req.Context()

	mempoolDataSlice, err := s.db.MempoolsChartData(ctx, chartFilter)
	if err != nil {
		s.renderErrorJSON(err.Error(), res)
		return
	}

	if len(mempoolDataSlice) == 0 {
		s.renderErrorJSON("mempool chart data is empty", res)
		return
	}

	data := map[string]interface{}{
		"mempoolchartData": mempoolDataSlice,
		"chartFilter":      chartFilter,
	}

	defer s.renderJSON(data, res)
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

	s.render("propagation.html", data, res)
}

// /getPropagationData
func (s *Server) getPropagationData(res http.ResponseWriter, req *http.Request) {
	data, err := s.fetchPropagationData(req)
	defer s.renderJSON(data, res)

	if err != nil {
		s.renderErrorJSON(err.Error(), res)
		return
	}
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
		chartType = "propagation"
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
	}

	if viewOption == defaultViewOption {
		return data, nil
	}

	blockSlice, err := s.db.Blocks(ctx, offset, pageSize)
	if err != nil {
		return nil, err
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

// /blocksChartData
func (s *Server) blocksChartData(res http.ResponseWriter, req *http.Request) {
	data, err := s.db.PropagationBlockChartData(req.Context())

	if err != nil {
		s.renderErrorJSON(err.Error(), res)
		return
	}

	var avgTimeForHeight = map[int64]float64{}
	var heightArr []int64
	for _, record := range data {
		avgTimeForHeight[record.BlockHeight] = record.TimeDifference
		heightArr = append(heightArr, record.BlockHeight)
	}

	var yLabel = "Delay (s)"

	var csv = fmt.Sprintf("Height,%s\n", yLabel)
	for _, height := range heightArr {
		timeDifference := fmt.Sprintf("%04.2f", avgTimeForHeight[height])
		csv += fmt.Sprintf("%d, %s\n", height, timeDifference)
	}

	s.renderJSON(csv, res)
}

// /votesChartDate
func (s *Server) votesChartDate(res http.ResponseWriter, req *http.Request) {
	var data []mempool.PropagationChartData
	var err error

	data, err = s.db.PropagationVoteChartData(req.Context())

	if err != nil {
		s.renderErrorJSON(err.Error(), res)
		return
	}

	var receiveTimeRecordsForHeight = map[int64][]float64{}
	heightArr, err := s.db.BlockHeights(req.Context())

	for _, record := range data {
		receiveTimeRecordsForHeight[record.BlockHeight] = append(receiveTimeRecordsForHeight[record.BlockHeight], record.TimeDifference)
	}

	var yLabel = "Time Difference (Milliseconds)"
	var csv = fmt.Sprintf("Height,%s\n", yLabel)

	avg := func(records []float64) float64 {
		if len(records) == 0 {
			return 0
		}
		var sum float64
		for _, record := range records {
			sum += record
		}

		return sum / float64(len(records))
	}

	for _, height := range heightArr {
		timeDifference := fmt.Sprintf("%04.2f", avg(receiveTimeRecordsForHeight[height])*1000)
		csv += fmt.Sprintf("%d, %s\n", height, timeDifference)
	}

	s.renderJSON(csv, res)
}

// propagationChartData
func (s *Server) propagationChartData(res http.ResponseWriter, req *http.Request) {
	req.ParseForm()

	var chartData struct {
		CSV       string `json:"csv"`
		YLabel    string `json:"yLabel"`
		MinHeight int64  `json:"min_height"`
		MaxHeight int64  `json:"max_height"`
	}
	pointsMap := map[int64][]string{}
	var localBlockHeights []int64
	var localBlockReceiveTimes = map[int64]time.Time{}

	heightIsInLocalDb := func(height int64) bool {
		for _, localHieght := range localBlockHeights {
			if localHieght == height {
				return true
			}
		}
		return false
	}

	fetchChartDataForSource := func(db DataQuery, isLocal bool) {
		data, err := db.FetchBlockReceiveTime(req.Context())

		if err != nil {
			s.renderErrorJSON(err.Error(), res)
			return
		}

		var timeVarianceForHeights = map[int64]float64{}
		for _, record := range data {
			if isLocal {
				localBlockHeights = append(localBlockHeights, record.BlockHeight)
				localBlockReceiveTimes[record.BlockHeight] = record.ReceiveTime
				continue
			}

			if !isLocal && !heightIsInLocalDb(record.BlockHeight) {
				continue
			}

			if localTime, found := localBlockReceiveTimes[record.BlockHeight]; found {
				timeVarianceForHeights[record.BlockHeight] = localTime.Sub(record.ReceiveTime).Seconds()
			}
		}

		if isLocal {
			sort.Slice(localBlockHeights, func(i, j int) bool {
				return localBlockHeights[j] >= localBlockHeights[i]
			})
			return
		}

		for _, h := range localBlockHeights {
			if timeDiff, found := timeVarianceForHeights[h]; found {
				pointsMap[h] = append(pointsMap[h], fmt.Sprintf("%.4f", timeDiff))
			}
		}
	}

	fetchChartDataForSource(s.db, true)

	syncSources, err := datasync.RegisteredSources()
	if err != nil {
		log.Error(err)
	}

	if len(syncSources) == 0 {
		s.renderErrorJSON("Please register at least one source to view chart", res)
		return
	}

	var yLabel = "Block Time Variance (seconds)"

	chartData.YLabel = yLabel

	for _, source := range syncSources {
		db, err := s.extDbFactory(source)
		if err != nil {
			s.renderErrorJSON(err.Error(), res)
			return
		}
		fetchChartDataForSource(db, false)
	}

	chartData.CSV = fmt.Sprintf("%s,%s\n", "Height", strings.Join(syncSources, ","))
	for _, height := range localBlockHeights {
		if len(pointsMap[height]) == 0 {
			continue
		}
		chartData.CSV += fmt.Sprintf("%d, %s\n", height, strings.Join(pointsMap[height], ","))
	}

	s.renderJSON(chartData, res)
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
	defer s.renderJSON(data, res)

	if err != nil {
		s.renderErrorJSON(err.Error(), res)
		return
	}
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
		break
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
		break
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
		break
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
		break
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
	plarform := req.FormValue("platform")
	dataType := req.FormValue("data-type")

	filters := map[string]string{}
	yLabel := ""
	switch plarform {
	case githubPlatform:
		if dataType == models.GithubColumns.Folks {
			yLabel = "Forks"
		} else {
			yLabel = "Stars"
		}
		plarform = models.TableNames.Github
		filters[models.GithubColumns.Repository] = fmt.Sprintf("'%s'", req.FormValue("repository"))
		break
	case twitterPlatform:
		yLabel = "Followers"
		dataType = models.TwitterColumns.Followers
		plarform = models.TableNames.Twitter
		break
	case redditPlatform:
		if dataType == models.RedditColumns.ActiveAccounts {
			yLabel = "Active Accounts"
		} else if dataType == models.RedditColumns.Subscribers {
			yLabel = "Subscribers"
		}
		plarform = models.TableNames.Reddit
		filters[models.RedditColumns.Subreddit] = fmt.Sprintf("'%s'", req.FormValue("subreddit"))
	case youtubePlatform:
		plarform = models.TableNames.Youtube
		if dataType == models.YoutubeColumns.ViewCount {
			yLabel = "View Count"
		} else if dataType == models.YoutubeColumns.Subscribers {
			yLabel = "Subscribers"
		}
		filters[models.YoutubeColumns.Channel] = fmt.Sprintf("'%s'", req.FormValue("channel"))
		break
	}

	if dataType == "" {
		s.renderErrorJSON("Data type cannot be empty", resp)
		return
	}

	data, err := s.db.CommunityChart(req.Context(), plarform, dataType, filters)
	if err != nil {
		s.renderErrorJSON(fmt.Sprintf("Cannot fetch chart data, %s", err.Error()), resp)
		return
	}

	var dates []time.Time
	var pointsMap = map[time.Time]int64{}

	csv := "" //fmt.Sprintf("Date,%s\n", yLabel)
	for _, stat := range data {
		dates = append(dates, stat.Date)
		pointsMap[stat.Date] = stat.Record
	}

	sort.Slice(dates, func(i, j int) bool {
		return dates[i].Before(dates[j])
	})

	for _, date := range dates {
		csv += fmt.Sprintf("%s,%d\n", date.Format(time.RFC3339Nano), pointsMap[date])
	}

	s.renderJSON(map[string]interface{}{
		"stats":  csv,
		"ylabel": yLabel,
	}, resp)
}

// /snapshot
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
	limit := -1
	var offset int
	if r.FormValue("chart") != "1" {
		if page < 1 {
			page = 1
		}
		offset = (page -1) * pageSize
		limit = pageSize
	}
	

	userAgents, total, err := s.db.PeerCountByUserAgents(r.Context(), r.FormValue("sources"), offset, limit)
	if err != nil {
		s.renderErrorfJSON("Cannot fetch data: %s", w, err.Error())
		return
	}

	var totalPages int64
	if total%int64(pageSize) == 0 {
		totalPages = total / int64(pageSize)
	} else {
		totalPages = 1 + (total-total%int64(pageSize))/int64(pageSize)
	}

	s.renderJSON(map[string]interface{}{"userAgents": userAgents, "totalPages": totalPages}, w)
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

	var row = []string{ "Date (UTC)" }
	for _, userAgent := range allUserAgents {
		row = append(row, userAgent)
	}
	csv := strings.Join(row, ",") + "\n"

	var minDate, maxDate int64
	for _, timestamp := range allDates {
		if minDate == 0 || timestamp < minDate {
			minDate = timestamp
		}

		if maxDate == 0 || timestamp > maxDate {
			maxDate = timestamp
		}

		row = []string{ time.Unix(timestamp, 0).UTC().String() }
		for _, userAgent := range allUserAgents {
			row = append(row, strconv.FormatInt(dateUserAgentCount[timestamp][userAgent], 10))
		}
		csv += strings.Join(row, ",") + "\n"
	}

	s.renderJSON(map[string]interface{}{ 
		"csv" : csv, 
		"minDate" : time.Unix(minDate, 0).UTC().String(), 
		"maxDate" : time.Unix(maxDate, 0).UTC().String(), 
	}, w)
}

// /api/snapshots/countries
func (s *Server) nodesCountByCountries(w http.ResponseWriter, r *http.Request) {
	pageSize, err := strconv.Atoi(r.FormValue("page-size"))
	if err != nil {
		pageSize = defaultPageSize
	}

	page, _ := strconv.Atoi(r.FormValue("page"))
	limit := -1
	var offset int
	if r.FormValue("chart") != "1" {
		if page < 1 {
			page = 1
		}
		offset = (page -1) * pageSize
		limit = pageSize
	}

	countries, total, err := s.db.PeerCountByCountries(r.Context(), r.FormValue("sources"), offset, limit)
	if err != nil {
		s.renderErrorJSON(fmt.Sprintf("Cannot retrieve peer count by countries, %s", err.Error()), w)
		return
	}

	if r.FormValue("chart") != "1" {
		sort.Slice(countries, func(i, j int) bool {
			return countries[i].Nodes > countries[j].Nodes
		})
	}

	var totalPages int64
	if total%int64(pageSize) == 0 {
		totalPages = total / int64(pageSize)
	} else {
		totalPages = 1 + (total-total%int64(pageSize))/int64(pageSize)
	}

	s.renderJSON(map[string]interface{}{"countries": countries, "totalPages": totalPages}, w)
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

	var row = []string{ "Date (UTC)" }
	for _, country := range allCountries {
		row = append(row, country)
	}
	csv := strings.Join(row, ",") + "\n"

	var minDate, maxDate int64
	for _, timestamp := range allDates {
		if minDate == 0 || timestamp < minDate {
			minDate = timestamp
		}

		if maxDate == 0 || timestamp > maxDate {
			maxDate = timestamp
		}

		row = []string{ time.Unix(timestamp, 0).UTC().String() }
		for _, country := range allCountries {
			row = append(row, strconv.FormatInt(dateCountryCount[timestamp][country], 10))
		}
		csv += strings.Join(row, ",") + "\n"
	}

	s.renderJSON(map[string]interface{}{ 
		"csv" : csv, 
		"minDate" : time.Unix(minDate, 0).UTC().String(), 
		"maxDate" : time.Unix(maxDate, 0).UTC().String(), 
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

	var timestamp int64

	timestamp = getTitmestampCtx(r)
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

	return
}
