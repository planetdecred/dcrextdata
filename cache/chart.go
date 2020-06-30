package cache

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/decred/dcrd/chaincfg"
	"github.com/dgraph-io/badger"
	"github.com/raedahgroup/dcrextdata/app/helpers"
	"github.com/volatiletech/null"
)

// Keys for specifying chart data type.
const (
	Mempool     = "mempool"
	Propagation = "propagation"
	Community   = "community"
	PowChart    = "pow"
	VSP         = "vsp"
	Exchange    = "exchange"
	Snapshot    = "snapshot"
)

// binLevel specifies the granularity of data.
type binLevel string

// axisType is used to manage the type of x-axis data on display on the specified
// chart.
type axisType string

// These are the recognized binLevel and axisType values.
const (
	HeightAxis axisType = "height"
	TimeAxis   axisType = "time"

	HashrateAxis axisType = "hashrate"
	WorkerAxis   axisType = "workers"

	MempoolSize    = "size"
	MempoolFees    = "fees"
	MempoolTxCount = "tx-count"

	BlockPropagation = "block-propagation"
	BlockTimestamp   = "block-timestamp"
	VotesReceiveTime = "votes-receive-time"

	ImmatureAxis         axisType = "immature"
	LiveAxis             axisType = "live"
	VotedAxis            axisType = "voted"
	MissedAxis           axisType = "missed"
	PoolFeesAxis         axisType = "pool-fees"
	ProportionLiveAxis   axisType = "proportion-live"
	ProportionMissedAxis axisType = "proportion-missed"
	UserCountAxis        axisType = "user-count"
	UsersActiveAxis      axisType = "users-active"

	SnapshotNodes          axisType = "nodes"
	SnapshotReachableNodes axisType = "reachable-nodes"
	SnapshotLocations      axisType = "locations"
	SnapshotNodeVersions   axisType = "node-versions"

	defaultBin binLevel = "default"
	hourBin	   binLevel = "hour"
	dayBin	   binLevel = "day"
)

// ParseAxis returns the matching axis type, else the default of time axis.
func ParseAxis(aType string) axisType {
	aType = strings.ToLower(aType)
	switch axisType(aType) {
	case HeightAxis:
		return HeightAxis
		//Mempool
	case MempoolSize:
		return MempoolSize
	case MempoolTxCount:
		return MempoolTxCount
	case MempoolFees:
		return MempoolFees
		//Propagation
	case BlockPropagation:
		return BlockPropagation
	case BlockTimestamp:
		return BlockTimestamp
	case VotesReceiveTime:
		return VotesReceiveTime
		// PoW axis
	case HashrateAxis:
		return HashrateAxis
	case WorkerAxis:
		return WorkerAxis
		// vsp axis
	case ImmatureAxis:
		return ImmatureAxis
	case LiveAxis:
		return LiveAxis
	case VotedAxis:
		return VotedAxis
	case MissedAxis:
		return MissedAxis
	case PoolFeesAxis:
		return PoolFeesAxis
	case ProportionLiveAxis:
		return ProportionLiveAxis
	case ProportionMissedAxis:
		return ProportionMissedAxis
	case UserCountAxis:
		return UserCountAxis
	case UsersActiveAxis:
		return UsersActiveAxis
		// exchange
	case ExchangeCloseAxis:
		return ExchangeCloseAxis
	case ExchangeHighAxis:
		return ExchangeHighAxis
	case ExchangeOpenAxis:
		return ExchangeOpenAxis
	case ExchangeLowAxis:
		return ExchangeLowAxis
		// snapshot
	case SnapshotNodes:
		return SnapshotNodes
	case SnapshotLocations:
		return SnapshotLocations
	case SnapshotNodeVersions:
		return SnapshotNodeVersions
	default:
		return TimeAxis
	}
}

func ParseBin(binString string) binLevel {
	switch(binLevel(binString)) {
	case hourBin:
		return hourBin
	case dayBin:
		return dayBin
	default:
		return defaultBin
	}
}

// cacheVersion helps detect when the cache data stored has changed its
// structure or content. A change on the cache version results to recomputing
// all the charts data a fresh thereby making the cache to hold the latest changes.
var cacheVersion = NewSemver(1, 0, 0)

// ChartError is an Error interface for use with constant errors.
type ChartError string

func (e ChartError) Error() string {
	return string(e)
}

// UnknownChartErr is returned when a chart key is provided that does not match
// any known chart type constant.
const UnknownChartErr = ChartError("unknown chart")

// InvalidBinErr is returned when a ChartMaker receives an unknown BinLevel.
// In practice, this should be impossible, since ParseBin returns a default
// if a supplied bin specifier is invalid, and window-binned ChartMakers
// ignore the bin flag.
const InvalidBinErr = ChartError("invalid bin")

// An interface for reading and setting the length of datasets.
type Lengther interface {
	Length() int
	Truncate(int) Lengther
	IsZero(index int) bool
	Remove(index int) Lengther
}

// ChartFloats is a slice of floats. It satisfies the lengther interface, and
// provides methods for taking averages or sums of segments.
type ChartFloats []float64

// Length returns the length of data. Satisfies the lengther interface.
func (data ChartFloats) Length() int {
	return len(data)
}

// Truncate makes a subset of the underlying dataset. It satisfies the lengther
// interface.
func (data ChartFloats) Truncate(l int) Lengther {
	return data[:l]
}

func (data ChartFloats) IsZero(index int) bool {
	if index >= data.Length() {
		return true
	}
	return data[index] == 0
}

func (data ChartFloats) Remove(index int) Lengther {
	if index >= data.Length() {
		return data
	}
	return append(data[:index], data[index+1:]...)
}

// If the data is longer than max, return a subset of length max.
func (data ChartFloats) snip(max int) ChartFloats {
	if len(data) < max {
		max = len(data)
	}
	return data[:max]
}

func (charts ChartFloats) Normalize() Lengther {
	return charts
}

// Avg is the average value of a segment of the dataset.
func (data ChartFloats) Avg(s, e int) float64 {
	if e <= s {
		return 0
	}
	var sum float64
	for _, v := range data[s:e] {
		sum += v
	}
	return sum / float64(e-s)
}

// A constructor for a sized ChartFloats.
func newChartFloats() ChartFloats {
	return make([]float64, 0, 0)
}

type ChartNullData interface {
	Lengther
	Value(index int) interface{}
	Valid(index int) bool
	String(index int) string
}

func noValidEntryBeforeIndex(data ChartNullData, index int) bool {
	for i := index; i >= 0; i-- {
		if data.Valid(i) {
			return false
		}
	}
	return true
}

// chartNullIntsPointer is a wrapper around ChartNullInt with Items as []nullUint64Pointer instead of
// []*null.Uint64 to bring the possibility of writing to god
type chartNullIntsPointer struct {
	Items []nullUint64Pointer
}

// Length returns the length of data. Satisfies the lengther interface.
func (data chartNullIntsPointer) Length() int {
	return len(data.Items)
}

// Truncate makes a subset of the underlying dataset. It satisfies the lengther
// interface.
func (data chartNullIntsPointer) Truncate(l int) Lengther {
	data.Items = data.Items[:l]
	return data
}

func (data chartNullIntsPointer) Append(set ChartNullUints) chartNullIntsPointer {
	for _, item := range set {
		var intPointer nullUint64Pointer
		if item != nil {
			intPointer.HasValue = true
			intPointer.Value = *item
		}
		data.Items = append(data.Items, intPointer)
	}
	return data
}

func (data chartNullIntsPointer) IsZero(index int) bool {
	if index >= data.Length() {
		return false
	}
	return data.Items[index].HasValue
}

func (data chartNullIntsPointer) Remove(index int) Lengther {
	if index >= data.Length() {
		return data
	}
	data.Items = append(data.Items[:index], data.Items[index+1:]...)
	return data
}

// If the data is longer than max, return a subset of length max.
func (data chartNullIntsPointer) snip(max int) chartNullIntsPointer {
	if len(data.Items) < max {
		max = len(data.Items)
	}
	data.Items = data.Items[:max]
	return data
}

// nullUint64Pointer provides a wrapper around *null.Uint64 to resolve the issue of inability to write nil pointer to gob
type nullUint64Pointer struct {
	HasValue bool
	Value    null.Uint64
}

func (data chartNullIntsPointer) toChartNullUint() ChartNullUints {
	var result ChartNullUints
	for _, item := range data.Items {
		if item.HasValue {
			result = append(result, &item.Value)
		} else {
			result = append(result, nil)
		}
	}

	return result
}

func (data ChartNullUints) toChartNullUintWrapper() chartNullIntsPointer {
	var result chartNullIntsPointer
	for _, item := range data {
		var intPointer nullUint64Pointer
		if item != nil {
			intPointer.HasValue = true
			intPointer.Value = *item
		}
		result.Items = append(result.Items, intPointer)
	}

	return result
}

func uintMapToPointer(input map[string]ChartNullUints) map[string]chartNullIntsPointer {
	result := map[string]chartNullIntsPointer{}
	for key, value := range input {
		result[key] = value.toChartNullUintWrapper()
	}
	return result
}

func uintPointerMapToUint(input map[string]chartNullIntsPointer) map[string]ChartNullUints {
	result := map[string]ChartNullUints{}
	for key, value := range input {
		result[key] = value.toChartNullUint()
	}
	return result
}

// ChartNullUints is a slice of null.uints. It satisfies the lengther interface.
type ChartNullUints []*null.Uint64

func (data ChartNullUints) Normalize() Lengther {
	return data.toChartNullUintWrapper()
}

func (data ChartNullUints) Value(index int) interface{} {
	if data == nil || len(data) <= index || data[index] == nil {
		return uint64(0)
	}
	return data[index].Uint64
}

func (data ChartNullUints) Valid(index int) bool {
	if data != nil && len(data) > index && data[index] != nil {
		return data[index].Valid
	}
	return false
}

func (data ChartNullUints) IsZero(index int) bool {
	return data.Value(index).(uint64) == 0
}

func (data ChartNullUints) Remove(index int) Lengther {
	if index >= data.Length() {
		return data
	}
	data = append(data[:index], data[index+1:]...)
	return data
}

func (data ChartNullUints) String(index int) string {
	return strconv.FormatUint(data.Value(index).(uint64), 10)
}

// Length returns the length of data. Satisfies the lengther interface.
func (data ChartNullUints) Length() int {
	return len(data)
}

// Truncate makes a subset of the underlying dataset. It satisfies the lengther
// interface.
func (data ChartNullUints) Truncate(l int) Lengther {
	return data[:l]
}

// Truncate makes a subset of the underlying dataset. It satisfies the lengther
// interface.
func (data ChartNullUints) ToChartString() ChartStrings {
	var result ChartStrings
	for _, record := range data {
		if record == nil {
			result = append(result, "")
		} else if !record.Valid {
			result = append(result, "NaN")
		} else {
			result = append(result, fmt.Sprintf("%d", record.Uint64))
		}
	}

	return result
}

// If the data is longer than max, return a subset of length max.
func (data ChartNullUints) snip(max int) ChartNullUints {
	if len(data) < max {
		max = len(data)
	}
	return data[:max]
}

// A constructor for a sized ChartUints.
func newChartNullUints() ChartNullUints {
	return make(ChartNullUints, 0)
}

// nullFloat64Pointer is a wrapper around ChartNullFloats with Items as []nullFloat64Pointer instead of
// []*null.Float64 to bring the possibility of writing it to god
type chartNullFloatsPointer struct {
	Items []nullFloat64Pointer
}

// nullFloat64Pointer provides a wrapper around *null.Float64 to resolve the issue of inability to write nil pointer to gob
type nullFloat64Pointer struct {
	HasValue bool
	Value    null.Float64
}

func (data chartNullFloatsPointer) Length() int {
	return len(data.Items)
}

// Truncate makes a subset of the underlying dataset. It satisfies the lengther
// interface.
func (data chartNullFloatsPointer) Truncate(l int) Lengther {
	data.Items = data.Items[:l]
	return data
}

func (data chartNullFloatsPointer) Append(set ChartNullFloats) chartNullFloatsPointer {
	for _, item := range set {
		var intPointer nullFloat64Pointer
		if item != nil {
			intPointer.HasValue = true
			intPointer.Value = *item
		}
		data.Items = append(data.Items, intPointer)
	}
	return data
}

func (data chartNullFloatsPointer) IsZero(index int) bool {
	if index >= data.Length() {
		return false
	}
	return !data.Items[index].HasValue
}

func (data chartNullFloatsPointer) Remove(index int) Lengther {
	if index >= data.Length() {
		return data
	}
	data.Items = append(data.Items[:index], data.Items[index+1:]...)
	return data
}

// If the data is longer than max, return a subset of length max.
func (data chartNullFloatsPointer) snip(max int) chartNullFloatsPointer {
	if len(data.Items) < max {
		max = len(data.Items)
	}
	data.Items = data.Items[:max]
	return data
}

func (data chartNullFloatsPointer) toChartNullFloats() ChartNullFloats {
	var result ChartNullFloats
	for _, item := range data.Items {
		if item.HasValue {
			result = append(result, &item.Value)
		} else {
			result = append(result, nil)
		}
	}

	return result
}

func (data ChartNullFloats) toChartNullFloatsWrapper() chartNullFloatsPointer {
	var result chartNullFloatsPointer
	for _, item := range data {
		var intPointer nullFloat64Pointer
		if item != nil {
			intPointer.HasValue = true
			intPointer.Value = *item
		}
		result.Items = append(result.Items, intPointer)
	}

	return result
}

func floatMapToPointer(input map[string]ChartNullFloats) map[string]chartNullFloatsPointer {
	result := map[string]chartNullFloatsPointer{}
	for key, value := range input {
		result[key] = value.toChartNullFloatsWrapper()
	}
	return result
}

func floatPointerToChartFloatMap(input map[string]chartNullFloatsPointer) map[string]ChartNullFloats {
	result := map[string]ChartNullFloats{}
	for key, value := range input {
		result[key] = value.toChartNullFloats()
	}
	return result
}

// ChartNullFloats is a slice of null.float64. It satisfies the lengther interface.
type ChartNullFloats []*null.Float64

func (data ChartNullFloats) Normalize() Lengther {
	return data
}

func (data ChartNullFloats) Value(index int) interface{} {
	if data == nil || len(data) <= index || data[index] == nil {
		return float64(0)
	}
	return data[index].Float64
}

func (data ChartNullFloats) Valid(index int) bool {
	if data != nil && len(data) > index && data[index] != nil {
		return data[index].Valid
	}
	return false
}

func (data ChartNullFloats) IsZero(index int) bool {
	return data.Value(index).(float64) == 0
}

func (data ChartNullFloats) Remove(index int) Lengther {
	if index >= data.Length() {
		return data
	}
	return append(data[:index], data[index+1:]...)
	return data
}

func (data ChartNullFloats) String(index int) string {
	return fmt.Sprintf("%f", data.Value(index).(float64))
}

// Length returns the length of data. Satisfies the lengther interface.
func (data ChartNullFloats) Length() int {
	return len(data)
}

// Truncate makes a subset of the underlying dataset. It satisfies the lengther
// interface.
func (data ChartNullFloats) Truncate(l int) Lengther {
	return data[:l]
}

// If the data is longer than max, return a subset of length max.
func (data ChartNullFloats) snip(max int) ChartNullFloats {
	if len(data) < max {
		max = len(data)
	}
	return data[:max]
}

// A constructor for a sized ChartUints.
func newChartNullFloats() ChartNullFloats {
	return make(ChartNullFloats, 0)
}

// ChartStrings is a slice of strings. It satisfies the lengther interface, and
// provides methods for taking averages or sums of segments.
type ChartStrings []string

// Length returns the length of data. Satisfies the lengther interface.
func (data ChartStrings) Length() int {
	return len(data)
}

// Truncate makes a subset of the underlying dataset. It satisfies the lengther
// interface.
func (data ChartStrings) Truncate(l int) Lengther {
	return data[:l]
}

func (data ChartStrings) IsZero(index int) bool {
	if index >= data.Length() {
		return false
	}
	return data[index] == ""
}

func (data ChartStrings) Remove(index int) Lengther {
	if index >= data.Length() {
		return data
	}
	return append(data[:index], data[index+1:]...)
	return data
}

// ChartUints is a slice of uints. It satisfies the lengther interface, and
// provides methods for taking averages or sums of segments.
type ChartUints []uint64

// Length returns the length of data. Satisfies the lengther interface.
func (data ChartUints) Length() int {
	return len(data)
}

// Truncate makes a subset of the underlying dataset. It satisfies the lengther
// interface.
func (data ChartUints) Truncate(l int) Lengther {
	return data[:l]
}

func (data ChartUints) IsZero(index int) bool {
	if index >= data.Length() {
		return false
	}
	return data[index] == 0
}

func (data ChartUints) Remove(index int) Lengther {
	if index >= data.Length() {
		return data
	}
	return append(data[:index], data[index+1:]...)
}

// If the data is longer than max, return a subset of length max.
func (data ChartUints) snip(max int) ChartUints {
	if len(data) < max {
		max = len(data)
	}
	return data[:max]
}

func (data ChartUints) Normalize() Lengther {
	return data
}

// Avg is the average value of a segment of the dataset.
func (data ChartUints) Avg(s, e int) uint64 {
	if e <= s {
		return 0
	}
	var sum uint64
	for _, v := range data[s:e] {
		sum += v
	}
	return sum / uint64(e-s)
}

// A constructor for a sized ChartUints.
func newChartUints() ChartUints {
	return make(ChartUints, 0)
}

// The chart data is cached with the current cacheID of the zoomSet or windowSet.
type cachedChart struct {
	CacheID uint64
	Data    []byte
	Version uint64
}

// A generic structure for JSON encoding keyed data sets.
type chartResponse map[string]interface{}

// ChartUpdater is a pair of functions for fetching and appending chart data.
// The two steps are divided so that ChartData can check whether another thread
// has updated the data during the query, and abandon an update with appropriate
// messaging.
type ChartUpdater struct {
	Tag string
	// In addition to the sql.Rows and an error, the fetcher should return a
	// context.CancelFunc if appropriate, else a dummy. A done value should be
	// returned to tell if the result is the last page
	Fetcher func(ctx context.Context, charts *ChartData, page int) (interface{}, func(), bool, error)
	// The Appender will be run under mutex lock.
	Appender func(charts *ChartData, recordSlice interface{}) error
}

// Retriver provides a function for directly getting a specific chart data from a store
type Retriver func(ctx context.Context, charts *ChartData, axisString string, bin string, extras ...string) ([]byte, error)

// ChartData is the entry point chart cache
type ChartData struct {
	mtx         sync.RWMutex
	ctx         context.Context
	EnableCache bool

	cacheMtx  sync.RWMutex
	db        *badger.DB
	cache     map[string]*cachedChart
	updaters  map[string]ChartUpdater
	retrivers map[string]Retriver

	syncSource    []string
	VSPSources    []string
	PowSources    []string
	ExchangeKeys  []string
	NodeVersion   []string
	NodeLocations []string
}

// Check that the length of all arguments is equal.
func ValidateLengths(lens ...Lengther) (int, error) {
	lenLen := len(lens)
	if lenLen == 0 {
		return 0, nil
	}
	firstLen := lens[0].Length()
	shortest, longest := firstLen, firstLen
	for i, l := range lens[1:lenLen] {
		dLen := l.Length()
		if dLen != firstLen {
			log.Warnf("charts.ValidateLengths: dataset at index %d has mismatched length %d != %d", i+1, dLen, firstLen)
			if dLen < shortest {
				shortest = dLen
			} else if dLen > longest {
				longest = dLen
			}
		}
	}
	if shortest != longest {
		return shortest, fmt.Errorf("data length mismatch")
	}
	return firstLen, nil
}

// Lengthen performs data validation, the cacheID will be incremented.
func (charts *ChartData) Lengthen(tags ...string) error {
	if len(tags) == 0 {
		tags = []string{
			Mempool, Propagation, PowChart, VSP, Exchange, Snapshot, Community,
		}
	}

	if err := charts.NormalizeLength(tags...); err != nil {
		return err
	}
	lengtheners := map[string]func () error {
		Mempool: charts.lengthenMempool,
		Snapshot: charts.lengthenSnapshot,
	}
	for _, t := range tags {
		if lengthener, f := lengtheners[t]; f {
			if err := lengthener(); err != nil {
				return err
			}
		}
	}
	
	return nil
}

// Load loads chart data from the gob file at the specified path and performs an
// update.
func (charts *ChartData) Load(ctx context.Context) error {
	t := helpers.NowUTC()
	defer func() {
		log.Debugf("Completed the initial chart load and update in %f s",
			time.Since(t).Seconds())
	}()

	currentVersion, err := charts.getVersion()
	if err != nil {
		return fmt.Errorf("Error in getting cache version, %v", err)
	}

	if !Compatible(cacheVersion, currentVersion) {
		return fmt.Errorf("Invalid cache version detected. Expected %s, got %s", 
		cacheVersion.String(), currentVersion.String())
	}
	// Bring the charts up to date.
	log.Infof("Updating charts data...")
	return charts.Update(ctx)
}

// TriggerUpdate triggers (*ChartData).Update.
func (charts *ChartData) TriggerUpdate(ctx context.Context, tag string) error {
	if err := charts.Update(ctx, tag); err != nil {
		// Only log errors from ChartsData.Update. TODO: make this more severe.
		log.Errorf("(*ChartData).Update failed: %v", err)
	}
	charts.ClearVLog()
	return nil
}

// AddRetriever adds a Retriever to the Retrievers slice.
func (charts *ChartData) AddRetriever(chartID string, retriever Retriver) {
	charts.retrivers[chartID] = retriever
}

// AddUpdater adds a ChartUpdater to the Updaters slice. Updaters are run
// sequentially during (*ChartData).Update.
func (charts *ChartData) AddUpdater(updater ChartUpdater) {
	charts.updaters[updater.Tag] = updater
}

// Update refreshes chart data by calling the ChartUpdaters sequentially. The
// Update is abandoned with a warning if stateID changes while running a Fetcher
// (likely due to a new update starting during a query).
func (charts *ChartData) Update(ctx context.Context, tags ...string) error {
	// only run updater if caching is enabled
	if !charts.EnableCache {
		return nil
	}

	var updaters []ChartUpdater
	if len(tags) > 0 {
		for _, t := range tags {
			if updater, found := charts.updaters[t]; found {
				updaters = append(updaters, updater)
			}
		}
	} else {
		for _, updater := range charts.updaters { 
			updaters = append(updaters, updater)
		}
	}

	for _, updater := range updaters {
		stateID := charts.cacheID(updater.Tag)
		var completed bool
		var page = 1
		for !completed {
			rows, cancel, done, err := updater.Fetcher(ctx, charts, page)
			if err != nil {
				err = fmt.Errorf("error encountered during charts %s update. aborting update: %v", updater.Tag, err)
			} else {
				charts.mtx.Lock()
				if stateID != charts.cacheID(updater.Tag) {
					err = fmt.Errorf("state change detected during charts %s update. aborting update", updater.Tag)
				} else {
					err = updater.Appender(charts, rows)
					if err != nil {
						err = fmt.Errorf("error detected during charts %s append. aborting update: %v", updater.Tag, err)
					}
				}
				charts.mtx.Unlock()
			}
			completed = done
			cancel()
			if err != nil {
				return err
			}
			page++
		}
	}

	// Since the charts db data query is complete. Update derived dataset.
	if err := charts.Lengthen(tags...); err != nil {
		return fmt.Errorf("(*ChartData).Lengthen failed: %v", err)
	}
	return nil
}

// NewChartData constructs a new ChartData.
func NewChartData(ctx context.Context, enableCache bool, syncSources,
	poolSources, vsps, nodeLocations, nodeVersion []string, chainParams *chaincfg.Params, db *badger.DB) *ChartData {

	var locations, versions = make([]string, len(nodeLocations)), make([]string, len(nodeVersion))
	for i, c := range nodeLocations {
		if c == "" {
			c = "Unknown"
		}
		locations[i] = c
	}
	for i, v := range nodeVersion {
		if v == "" {
			v = "Unknown"
		}
		versions[i] = v
	}
	return &ChartData{
		ctx:           ctx,
		EnableCache:   enableCache,
		db:            db,
		cache:         make(map[string]*cachedChart),
		updaters:      make(map[string]ChartUpdater),
		retrivers:     make(map[string]Retriver),
		syncSource:    syncSources,
		PowSources:    poolSources,
		VSPSources:    vsps,
		NodeLocations: locations,
		NodeVersion:   versions,
	}
}

// A cacheKey is used to specify cached data of a given type and BinLevel.
func cacheKey(chartID string, axis axisType) string {
	return chartID + "-" + string(axis)
}

// Grabs the cacheID associated with the provided chartID. Should
// be called under at least a (ChartData).cacheMtx.RLock.
func (charts *ChartData) cacheID(chartID string) uint64 {
	switch chartID {
	case Mempool:
		return charts.MempoolTimeTip()
	case BlockPropagation:
	case BlockTimestamp:
	case VotesReceiveTime:
		return uint64(charts.PropagationHeightTip())
	case PowChart:
		return charts.PowTimeTip()
	case VSP:
		return charts.VSPTimeTip()
	case Exchange:
		var version uint64
		for _, key := range charts.ExchangeKeys {
			if charts.ExchangeSetTime(key) > version {
				version = charts.ExchangeSetTime(key)
			}
		}
		return version
	}
	return charts.MempoolTimeTip()
}

// Grab the cached data, if it exists. The cacheID is returned as a convenience.
func (charts *ChartData) getCache(chartID string, axis axisType) (data *cachedChart, found bool, cacheID uint64) {
	// Ignore zero length since bestHeight would just be set to zero anyway.
	ck := cacheKey(chartID, axis)
	charts.cacheMtx.RLock()
	defer charts.cacheMtx.RUnlock()
	cacheID = charts.cacheID(chartID)
	data, found = charts.cache[ck]

	err := charts.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(ck))
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			d := gob.NewDecoder(bytes.NewReader(val))
			if err := d.Decode(data); err != nil {
				return err
			}
			found = true
			return nil
		})
	})
	if err != nil {
		found = false
	}
	return
}

// Store the chart associated with the provided type and BinLevel.
func (charts *ChartData) cacheChart(chartID string, version uint64, axis axisType, data []byte) {
	ck := cacheKey(chartID, axis)

	c := &cachedChart{
		Version: version,
		Data:    data,
	}

	var b bytes.Buffer
	e := gob.NewEncoder(&b)
	if err := e.Encode(c); err != nil {
		log.Errorf("Error caching cart, %s, %s", chartID, err.Error())
	}
	err := charts.db.Update(func(txn *badger.Txn) error {
		err := txn.Set([]byte(ck), b.Bytes())
		return err
	})
	if err != nil {
		log.Errorf("Error caching cart, %s, %s", chartID, err.Error())
	}

}

func (charts *ChartData) removeCache(chartID string, axis axisType) {
	ck := cacheKey(chartID, axis)
	err := charts.db.Update(func(txn *badger.Txn) error {
		err := txn.Delete([]byte(ck))
		return err
	})
	if err != nil {
		log.Errorf("Error delete chart cache, %s, %s", chartID, err.Error())
	}
}

// ChartMaker is a function that accepts a chart type and BinLevel, and returns
// a JSON-encoded chartResponse.
type ChartMaker func(ctx context.Context, charts *ChartData, axis axisType, bin binLevel, sources ...string) ([]byte, error)

var chartMakers = map[string]ChartMaker{
	Mempool:     mempool,
	Propagation: propagation,
	PowChart:    powChart,
	VSP:         makeVspChart,
	Exchange:    makeExchangeChart,
	Snapshot:    networkSnapshorChart,
}

// Chart will return a JSON-encoded chartResponse of the provided type
// and BinLevel.
func (charts *ChartData) Chart(ctx context.Context, chartID, axisString string, binString string, extras ...string) ([]byte, error) {
	
	axis := ParseAxis(axisString)
	bin := ParseBin(binString)
	key := fmt.Sprintf("%s-%s-%s-%s", chartID, bin, strings.Join(extras, "-"), string(axis))
	cache, found, _ := charts.getCache(key, axis)
	if found {
		if cache.Version == charts.cacheID(chartID) {
			return cache.Data, nil
		}
		charts.removeCache(key, axis)
	}

	if !charts.EnableCache {
		retriever, hasRetriever := charts.retrivers[chartID]
		if !hasRetriever {
			return nil, UnknownChartErr
		}
		data, err := retriever(ctx, charts, string(axis), string(bin), extras...)
		if err != nil {
			return nil, err
		}
		charts.cacheChart(key, charts.cacheID(chartID), axis, data)
		return data, nil
	}

	maker, hasMaker := chartMakers[chartID]
	if !hasMaker {
		return nil, UnknownChartErr
	}
	// Do the locking here, rather than in encodeXY, so that the helper functions
	// (accumulate, btw) are run under lock.
	charts.mtx.RLock()
	data, err := maker(ctx, charts, axis, bin, extras...)
	charts.mtx.RUnlock()
	if err != nil {
		return nil, err
	}
	charts.cacheChart(key, charts.cacheID(chartID), axis, data)
	return data, nil
}

// Keys used for the chartResponse data sets.
var responseKeys = []string{"x", "y", "z"}

// Encode the slices. The set lengths are truncated to the smallest of the
// arguments.
func (charts *ChartData) Encode(keys []string, sets ...Lengther) ([]byte, error) {
	return charts.encodeArr(keys, sets)
}

// Encode the slices. The set lengths are truncated to the smallest of the
// arguments.
func (charts *ChartData) encodeArr(keys []string, sets []Lengther) ([]byte, error) {
	if keys == nil {
		keys = responseKeys
	}
	if len(sets) == 0 {
		return nil, fmt.Errorf("encode called without arguments")
	}
	var smaller int = sets[0].Length()
	for _, x := range sets {
		if x == nil {
			smaller = 0
			continue
		}
		l := x.Length()
		if l < smaller {
			smaller = l
		}
	}
	for i := range sets {
		if sets[i] == nil {
			continue
		}
		sets[i] = sets[i].Truncate(smaller)
	}
	response := make(chartResponse)
	for i := range sets {
		rk := keys[i%len(keys)]
		// If the length of the responseKeys array has been exceeded, add a integer
		// suffix to the response key. The key progression is x, y, z, x1, y1, z1,
		// x2, ...
		if i >= len(keys) {
			rk += strconv.Itoa(i / len(keys))
		}
		response[rk] = sets[i]
	}
	return json.Marshal(response)
}

// trim remove points that has 0s in all yAxis.
func (charts *ChartData) trim(sets ...Lengther) []Lengther {

	dLen := sets[0].Length()
	for i := dLen - 1; i >= 0; i-- {
		var isZero bool = true
	out:
		for j := 1; j < len(sets); j++ {
			if !sets[j].IsZero(i) {
				isZero = false
				break out
			}
		}
		if isZero {
			for j := 0; j < len(sets); j++ {
				sets[j] = sets[j].Remove(i)
			}
		}
	}

	return sets
}

func mempool(ctx context.Context, charts *ChartData, axis axisType, bin binLevel, _ ...string) ([]byte, error) {
	switch axis {
	case MempoolSize:
		return mempoolSize(charts, bin)
	case MempoolTxCount:
		return mempoolTxCount(charts, bin)
	case MempoolFees:
		return mempoolFees(charts, bin)
	}
	return nil, UnknownChartErr
}

func mempoolSize(charts *ChartData, bin binLevel) ([]byte, error) {
	var dates, sizes ChartUints
	
	var key = fmt.Sprintf("%s-%s", Mempool, TimeAxis)
	if bin != defaultBin {
		key = fmt.Sprintf("%s-%s",  key, bin)
	}
	if err := charts.ReadVal(key, &dates); err != nil {
		log.Info(key)
		return nil, err
	}

	key = fmt.Sprintf("%s-%s", Mempool, MempoolSize)
	if bin != defaultBin {
		key = fmt.Sprintf("%s-%s",  key, bin)
	}
	if err := charts.ReadVal(key, &sizes); err != nil {
		return nil, err
	}

	return charts.Encode(nil, dates, sizes)
}

func mempoolTxCount(charts *ChartData, bin binLevel) ([]byte, error) {
	var dates, txCounts ChartUints

	var key = fmt.Sprintf("%s-%s", Mempool, TimeAxis)
	if bin != defaultBin {
		key = fmt.Sprintf("%s-%s",  key, bin)
	}

	if err := charts.ReadVal(key, &dates); err != nil {
		log.Info(key)
		return nil, err
	}

	key = fmt.Sprintf("%s-%s", Mempool, MempoolTxCount)
	if bin != defaultBin {
		key = fmt.Sprintf("%s-%s",  key, bin)
	}
	if err := charts.ReadVal(key, &txCounts); err != nil {
		return nil, err
	}

	return charts.Encode(nil, dates, txCounts)
}

func mempoolFees(charts *ChartData, bin binLevel) ([]byte, error) {
	var dates ChartUints
	var fees ChartFloats

	var key = fmt.Sprintf("%s-%s", Mempool, TimeAxis)
	if bin != defaultBin {
		key = fmt.Sprintf("%s-%s",  key, bin)
	}

	if err := charts.ReadVal(key, &dates); err != nil {
		return nil, err
	}

	key = fmt.Sprintf("%s-%s", Mempool, MempoolFees)
	if bin != defaultBin {
		key = fmt.Sprintf("%s-%s",  key, bin)
	}
	if err := charts.ReadVal(key, &fees); err != nil {
		return nil, err
	}
	return charts.Encode(nil, dates, fees)
}

func propagation(ctx context.Context, charts *ChartData, axis axisType, bin binLevel, syncSources ...string) ([]byte, error) {
	switch axis {
	case BlockPropagation:
		return blockPropagation(charts, syncSources...)
	case BlockTimestamp:
		return blockTimestamp(charts)
	case VotesReceiveTime:
		return votesReceiveTime(charts)
	}
	return nil, UnknownChartErr
}

func blockPropagation(charts *ChartData, syncSources ...string) ([]byte, error) {
	var heights ChartUints
	key := fmt.Sprintf("%s-%s", Propagation, HeightAxis)
	if err := charts.ReadVal(key, &heights); err != nil {
		log.Info(err, key)
		return nil, err
	}
	var deviations = []Lengther{heights}
	for _, source := range syncSources {
		var d ChartFloats
		key = fmt.Sprintf("%s-%s-%s", Propagation, BlockPropagation, source)
		if err := charts.ReadVal(key, &d); err != nil {
			log.Info(err, key)
			return nil, err
		}
		deviations = append(deviations, d)
	}

	return charts.encodeArr(nil, deviations)
}

func blockTimestamp(charts *ChartData) ([]byte, error) {
	var heights ChartUints
	if err := charts.ReadVal(Propagation+"-"+string(HeightAxis), &heights); err != nil {
		return nil, err
	}
	var blockDelays ChartFloats
	if err := charts.ReadVal(Propagation+"-"+string(BlockTimestamp), &blockDelays); err != nil {
		return nil, err
	}
	return charts.Encode(nil, heights, blockDelays)
}

func votesReceiveTime(charts *ChartData) ([]byte, error) {
	var heights ChartUints
	if err := charts.ReadVal(Propagation+"-"+string(HeightAxis), &heights); err != nil {
		return nil, err
	}
	var votesReceiveTime ChartFloats
	if err := charts.ReadVal(Propagation+"-"+string(VotesReceiveTime), &votesReceiveTime); err != nil {
		return nil, err
	}
	return charts.Encode(nil, heights, votesReceiveTime)
}

func powChart(ctx context.Context, charts *ChartData, axis axisType, bin binLevel, pools ...string) ([]byte, error) {
	retriever, hasRetriever := charts.retrivers[PowChart]
	if !hasRetriever {
		return nil, UnknownChartErr
	}
	return retriever(ctx, charts, string(axis), string(bin), pools...)
}

func MakePowChart(charts *ChartData, dates ChartUints, deviations []ChartNullUints, pools []string) ([]byte, error) {

	var recs = []Lengther{dates}
	for _, d := range deviations {
		recs = append(recs, d)
	}
	recs = charts.trim(recs...)
	return charts.Encode(nil, recs...)
}

func makeVspChart(ctx context.Context, charts *ChartData, axis axisType, bin binLevel, vsps ...string) ([]byte, error) {
	// var dates ChartUints
	// if err := charts.ReadAxis(VSP+"-"+string(TimeAxis), &dates); err != nil {
	// 	return nil, err
	// }

	// var deviations = make([]ChartNullData, len(vsps))

	// for i, s := range vsps {
	// 	switch axis {
	// 	case ImmatureAxis, LiveAxis, VotedAxis, MissedAxis, UserCountAxis, UsersActiveAxis:
	// 		var data chartNullIntsPointer
	// 		if err := charts.ReadAxis(VSP + "-" + string(axis) + "-" + s, &data); err != nil {
	// 			return nil, err
	// 		}
	// 		deviations[i] = data.toChartNullUint()

	// 	case ProportionLiveAxis, ProportionMissedAxis:
	// 		var data chartNullFloatsPointer
	// 		if err := charts.ReadAxis(VSP + "-" + string(axis) + "-" + s, &data); err != nil {
	// 			return nil, err
	// 		}
	// 		deviations[i] = data.toChartNullFloats()
	// 	}

	// }

	// return MakeVspChart(charts, dates, deviations, vsps)

	// Because the dates for vsp tick vary from source to source,
	// a single date collection cannot be used for all and so
	// the record is retrieved at every request.

	retriever, hasRetriever := charts.retrivers[VSP]
	if !hasRetriever {
		return nil, UnknownChartErr
	}
	return retriever(ctx, charts, string(axis), string(bin), vsps...)
}

func MakeVspChart(charts *ChartData, dates ChartUints, deviations []ChartNullData, vsps []string) ([]byte, error) {
	var recs = []Lengther{dates}
	for _, d := range deviations {
		recs = append(recs, d)
	}

	recs = charts.trim(recs...)
	return charts.Encode(nil, recs...)
}

func networkSnapshorChart(ctx context.Context, charts *ChartData, axis axisType, bin binLevel, extras ...string) ([]byte, error) {
	switch axis {
	case SnapshotNodes:
		return networkSnapshotNodesChart(charts, bin)
	case SnapshotLocations:
		return networkSnapshotLocationsChart(charts, bin, extras...)
	case SnapshotNodeVersions:
		return networkSnapshotNodeVersionsChart(charts, bin, extras...)
	default:
		return nil, UnknownChartErr
	}
}

func networkSnapshotNodesChart(charts *ChartData, bin binLevel) ([]byte, error) {
	var dates, nodes, reachableNodes ChartUints
	
	var key = fmt.Sprintf("%s-%s", Mempool, TimeAxis)
	if bin != defaultBin {
		key = fmt.Sprintf("%s-%s",  key, bin)
	}
	if err := charts.ReadVal(key, &dates); err != nil {
		log.Info(key)
		return nil, err
	}

	key = fmt.Sprintf("%s-%s", Snapshot, SnapshotNodes)
	if bin != defaultBin {
		key = fmt.Sprintf("%s-%s",  key, bin)
	}
	if err := charts.ReadVal(key, &nodes); err != nil {
		return nil, err
	}

	key = fmt.Sprintf("%s-%s", Snapshot, SnapshotReachableNodes)
	if bin != defaultBin {
		key = fmt.Sprintf("%s-%s",  key, bin)
	}
	if err := charts.ReadVal(key, &reachableNodes); err != nil {
		return nil, err
	}

	return charts.Encode(nil, dates, nodes, reachableNodes)
}

func networkSnapshotLocationsChart(charts *ChartData, bin binLevel, countries ...string) ([]byte, error) {
	var recs = make([]Lengther, len(countries)+1)
	var dates ChartUints
	key := fmt.Sprintf("%s-%s-%s", Snapshot, SnapshotLocations, TimeAxis)
	if bin != defaultBin {
		key = fmt.Sprintf("%s-%s",  key, bin)
	}
	if err := charts.ReadVal(key, &dates); err != nil {
		return nil, err
	}
	recs[0] = dates

	for i, country := range countries {
		if country == "" {
			continue
		}
		var key = fmt.Sprintf("%s-%s-%s", Snapshot, SnapshotLocations, country)
		if bin != defaultBin {
			key = fmt.Sprintf("%s-%s",  key, bin)
		}
		var rec ChartUints
		if err := charts.ReadVal(key, &rec); err != nil {
			log.Criticalf("%s - %s", err.Error(), key)
			return nil, err
		}
		recs[i+1] = rec
	}
	return charts.Encode(nil, recs...)
}

func networkSnapshotNodeVersionsChart(charts *ChartData, bin binLevel, userAgents ...string) ([]byte, error) {
	var recs = make([]Lengther, len(userAgents)+1)
	var dates ChartUints
	key := fmt.Sprintf("%s-%s-%s", Snapshot, SnapshotNodeVersions, TimeAxis)
	if bin != defaultBin {
		key = fmt.Sprintf("%s-%s",  key, bin)
	}
	if err := charts.ReadVal(key, &dates); err != nil {
		log.Info(key)
		return nil, err
	}
	recs[0] = dates

	for i, userAgent := range userAgents {
		if userAgent == "" {
			continue
		}

		var key = fmt.Sprintf("%s-%s-%s", Snapshot, SnapshotNodeVersions, userAgent)
		if bin != defaultBin {
			key = fmt.Sprintf("%s-%s",  key, bin)
		}
		var rec ChartUints
		if err := charts.ReadVal(key, &rec); err != nil {
			log.Criticalf("%s - %s", err.Error(), key)
			return nil, err
		}
		recs[i+1] = rec
	}
	return charts.Encode(nil, recs...)
}
