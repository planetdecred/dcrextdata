package web

import (
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/raedahgroup/dcrextdata/mempool"
	"github.com/raedahgroup/dcrextdata/pow"
	"github.com/raedahgroup/dcrextdata/vsp"
)

const (
	chartViewOption             = "chart"
	defaultViewOption           = chartViewOption
	mempoolDefaultChartDataType = "size"
	maxPageSize                 = 250
	recordsPerPage              = 20
	defaultInterval             = 1440 // All
	noDataMessage = "does not have data for selected query options."
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
)

// /home
func (s *Server) homePage(res http.ResponseWriter, req *http.Request) {
	data := map[string]interface{}{}
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
		pageSize = recordsPerPage
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

	if selectedCurrencyPair == "" {
		selectedCurrencyPair = "All"
	}

	offset := (pageToLoad - 1) * pageSize

	allExchangeSlice, err := s.db.AllExchange(ctx)
	if err != nil {
		return nil, fmt.Errorf("Cannot fetch exchanges, %s", err.Error())
	}

	currencyPairs, err := s.db.AllExchangeTicksCurrencyPair(ctx)
	if err != nil {
		return nil, fmt.Errorf("Cannot fetch currency pair, %s", err.Error())
	}

	if selectedExchange == "" && viewOption == "table" {
		selectedExchange = "All"
	} 
	if viewOption == defaultViewOption && (selectedExchange == "" || selectedExchange == "All") && len(allExchangeSlice) > 0 {
		selectedExchange = allExchangeSlice[0].Name
	}

	data := map[string]interface{}{
		"chartView":            true,
		"selectedViewOption":   viewOption,
		"currencyPairs":        currencyPairs,
		"allExData":            allExchangeSlice,
		"intervals":            exchangeTickIntervals,
		"pageSizeSelector":     pageSizeSelector,
		"selectedCurrencyPair": selectedCurrencyPair,
		"selectedNum":          pageSize,
		"selectedInterval":     selectedInterval,
		"selectedTick":         selectedTick,
		"selectedExchange":     selectedExchange,
		"currentPage":          pageToLoad,
		"previousPage":         pageToLoad - 1,
		"totalPages":           0,
	}

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
		pageSize = recordsPerPage
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

	allVspData, err := s.db.FetchVSPs(ctx)
	if err != nil {
		return nil, err
	}

	data := map[string]interface{}{
		"chartView":          true,
		"selectedViewOption": viewOption,
		"allVspData":         allVspData,
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
		pageSize = recordsPerPage
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

	offset := (pageToLoad - 1) * recordsPerPage

	ctx := req.Context()

	powSource, err := s.db.FetchPowSourceData(ctx)
	if err != nil {
		return nil, err
	}

	data := map[string]interface{}{
		"chartView":          true,
		"selectedViewOption": viewOption,
		"selectedFilter":     selectedPow,
		"selectedDataType":   selectedDataType,
		"selectedPools":      pools,
		"pageSizeSelector":   pageSizeSelector,
		"selectedNum":        pageSize,
		"powSource":          powSource,
		"currentPage":        pageToLoad,
		"previousPage":       pageToLoad - 1,
		"totalPages":         pageToLoad,
	}

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
	data["totalPages"] = int(math.Ceil(float64(totalCount) / float64(recordsPerPage)))

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
		pageSize = recordsPerPage
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

	if viewOption == "" {
		viewOption = "table"
	}

	if recordSet == "" {
		if viewOption == chartViewOption {
			recordSet = "blocks"
		} else {
			recordSet = "both"
		}
	}

	var pageSize int
	numRows, err := strconv.Atoi(numberOfRows)
	if err != nil || numRows <= 0 {
		pageSize = recordsPerPage
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
		"selectedViewOption":   viewOption,
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

// /propagationchartdata
func (s *Server) propagationChartData(res http.ResponseWriter, req *http.Request) {
	requestedRecordSet := req.FormValue("record-set")

	var data []mempool.PropagationChartData
	var err error

	if requestedRecordSet == "votes" {
		data, err = s.db.PropagationVoteChartData(req.Context())
	} else {
		data, err = s.db.PropagationBlockChartData(req.Context())
	}

	if err != nil {
		s.renderErrorJSON(err.Error(), res)
		return
	}

	var avgTimeForHeight = map[int64]float64{}
	var heightArr []int64
	for _, record := range data {
		if existingTime, found := avgTimeForHeight[record.BlockHeight]; found {
			avgTimeForHeight[record.BlockHeight] = (record.TimeDifference + existingTime) / 2
		} else {
			avgTimeForHeight[record.BlockHeight] = record.TimeDifference
			heightArr = append(heightArr, record.BlockHeight)
		}
	}

	var yLabel = "Delay (s)"
	if requestedRecordSet == "votes" {
		yLabel = "Time Difference (s)"
	}

	var csv = fmt.Sprintf("Height,%s\n", yLabel)
	for _, height := range heightArr {
		timeDifference := fmt.Sprintf("%04.2f", avgTimeForHeight[height])
		csv += fmt.Sprintf("%d, %s\n", height, timeDifference)
	}

	s.renderJSON(csv, res)
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

	var pageSize int
	numRows, err := strconv.Atoi(numberOfRows)
	if err != nil || numRows <= 0 {
		pageSize = recordsPerPage
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

	if viewOption == "" || viewOption == "chart" {
		data := map[string]interface{}{
			"chartView":            true,
			"selectedViewOption":   defaultViewOption,
			"currentPage":          pageToLoad,
			"propagationRecordSet": propagationRecordSet,
			"pageSizeSelector":     pageSizeSelector,
			"selectedFilter":       "blocks",
			"blocks":               true,
			"selectedNum":          pageSize,
			"previousPage":         pageToLoad,
			"totalPages":           pageToLoad,
		}
		return data, nil
	}

	voteSlice, err := s.db.BlocksWithoutVotes(ctx, offset, pageSize)
	if err != nil {
		return nil, err
	}

	totalCount, err := s.db.BlockCount(ctx)
	if err != nil {
		return nil, err
	}

	data := map[string]interface{}{
		"records":              voteSlice,
		"currentPage":          pageToLoad,
		"propagationRecordSet": propagationRecordSet,
		"pageSizeSelector":     pageSizeSelector,
		"selectedFilter":       "blocks",
		"blocks":               true,
		"selectedNum":          pageSize,
		"url":                  "/blockdata",
		"previousPage":         pageToLoad - 1,
		"totalPages":           int(math.Ceil(float64(totalCount) / float64(pageSize))),
	}

	totalTxLoaded := offset + len(voteSlice)
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

	var pageSize int
	numRows, err := strconv.Atoi(numberOfRows)
	if err != nil || numRows <= 0 {
		pageSize = recordsPerPage
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

	if viewOption == "" || viewOption == "chart" {
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
			"previousPage":         pageToLoad,
			"totalPages":           pageToLoad,
		}
		return data, nil
	}

	voteSlice, err := s.db.Votes(ctx, offset, pageSize)
	if err != nil {
		return nil, err
	}

	totalCount, err := s.db.VotesCount(ctx)
	if err != nil {
		return nil, err
	}

	data := map[string]interface{}{
		"voteRecords":          voteSlice,
		"currentPage":          pageToLoad,
		"propagationRecordSet": propagationRecordSet,
		"pageSizeSelector":     pageSizeSelector,
		"selectedFilter":       "votes",
		"votes":                true,
		"selectedNum":          pageSize,
		"url":                  "/votesdata",
		"previousPage":         pageToLoad - 1,
		"totalPages":           int(math.Ceil(float64(totalCount) / float64(pageSize))),
	}

	totalTxLoaded := offset + len(voteSlice)
	if int64(totalTxLoaded) < totalCount {
		data["nextPage"] = pageToLoad + 1
	}

	return data, nil
}
