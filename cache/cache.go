package cache

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/decred/dcrd/chaincfg"
	"github.com/dgraph-io/badger"
	"github.com/pkg/errors"
	"github.com/planetdecred/dcrextdata/app/helpers"
	"github.com/volatiletech/null"
)

// Keys for specifying chart data type.
const (
	Mempool       = "mempool"
	Propagation   = "propagation"
	Community     = "community"
	PowChart      = "pow"
	VSP           = "vsp"
	Exchange      = "exchange"
	Snapshot      = "snapshot"
	SnapshotTable = Snapshot + "_table"
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

	DefaultBin binLevel = "default"
	HourBin    binLevel = "hour"
	DayBin     binLevel = "day"
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
	switch binLevel(binString) {
	case HourBin:
		return HourBin
	case DayBin:
		return DayBin
	default:
		return DefaultBin
	}
}

// cacheVersion helps detect when the cache data stored has changed its
// structure or content. A change on the cache version results to recomputing
// all the charts data a fresh thereby making the cache to hold the latest changes.
var cacheVersion = NewSemver(1, 0, 1)

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
	if s >= data.Length() || e >= data.Length() {
		return 0
	}

	if e <= s {
		return 0
	}
	var sum float64
	for _, v := range data[s:e] {
		sum += v
	}
	return sum / float64(e-s)
}

type ChartNullData interface {
	Lengther
	Value(index int) interface{}
	Valid(index int) bool
	String(index int) string
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

// Avg is the average value of a segment of the dataset.
func (data chartNullIntsPointer) Avg(s, e int) (d nullUint64Pointer) {
	if s >= data.Length() || e >= data.Length() {
		return
	}
	if e <= s {
		return
	}
	var sum uint64
	for _, v := range data.Items[s:e] {
		if v.HasValue {
			d.HasValue = true
		}
		sum += v.Value
	}
	d.Value = sum / uint64(e-s)
	return
}

func (data chartNullIntsPointer) Append(set ChartNullUints) chartNullIntsPointer {
	for _, item := range set {
		var intPointer nullUint64Pointer
		if item != nil {
			intPointer.HasValue = true
			intPointer.Value = item.Uint64
		}
		data.Items = append(data.Items, intPointer)
	}
	return data
}

func (data chartNullIntsPointer) IsZero(index int) bool {
	if index >= data.Length() {
		return false
	}
	return data.Items[index].Value == 0
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
	Value    uint64
}

func (data *chartNullIntsPointer) toChartNullUint() ChartNullUints {
	var result ChartNullUints
	for _, item := range data.Items {
		if item.HasValue {
			result = append(result, &null.Uint64{
				Uint64: item.Value, Valid: item.HasValue,
			})
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
			intPointer.Value = item.Uint64
		}
		result.Items = append(result.Items, intPointer)
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

// nullFloat64Pointer is a wrapper around ChartNullFloats with Items as []nullFloat64Pointer instead of
// []*null.Float64 to bring the possibility of writing it to god
type chartNullFloatsPointer struct {
	Items []nullFloat64Pointer
}

// nullFloat64Pointer provides a wrapper around *null.Float64 to resolve the issue of inability to write nil pointer to gob
type nullFloat64Pointer struct {
	HasValue bool
	Value    float64
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

// Avg is the average value of a segment of the dataset.
func (data chartNullFloatsPointer) Avg(s, e int) (d nullFloat64Pointer) {
	if s >= data.Length() || e >= data.Length() {
		return
	}
	if e <= s {
		return
	}
	var sum float64
	for _, v := range data.Items[s:e] {
		if v.HasValue {
			d.HasValue = true
		}
		sum += v.Value
	}
	d.Value = sum / float64(e-s)
	return
}

func (data chartNullFloatsPointer) Append(set ChartNullFloats) chartNullFloatsPointer {
	for _, item := range set {
		var intPointer nullFloat64Pointer
		if item != nil {
			intPointer.HasValue = true
			intPointer.Value = item.Float64
		}
		data.Items = append(data.Items, intPointer)
	}
	return data
}

func (data chartNullFloatsPointer) IsZero(index int) bool {
	if index >= data.Length() {
		return false
	}
	return data.Items[index].Value == 0
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

func (data *chartNullFloatsPointer) toChartNullFloats() ChartNullFloats {
	var result ChartNullFloats
	for _, item := range data.Items {
		if item.HasValue {
			result = append(result, &null.Float64{
				Float64: item.Value, Valid: item.HasValue,
			})
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
			intPointer.Value = item.Float64
		}
		result.Items = append(result.Items, intPointer)
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
	if s >= data.Length() || e >= data.Length() {
		return 0
	}
	if e <= s {
		return 0
	}
	var sum uint64
	for _, v := range data[s:e] {
		sum += v
	}
	return sum / uint64(e-s)
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
	Fetcher func(ctx context.Context, charts *Manager, page int) (interface{}, func(), bool, error)
	// The Appender will be run under mutex lock.
	Appender func(charts *Manager, recordSlice interface{}) error
}

// Retriver provides a function for directly getting a specific chart data from a store
type Retriver func(ctx context.Context, charts *Manager, dataType, axisString string, bin string, extras ...string) ([]byte, error)

// Manager is the entry point chart cache
type Manager struct {
	mtx         sync.RWMutex
	ctx         context.Context
	EnableCache bool

	cacheMtx       sync.RWMutex
	mempoolMtx     sync.RWMutex
	propagationMtx sync.RWMutex

	DB        *badger.DB
	dir       string
	cache     map[string]*cachedChart
	updaters  map[string]ChartUpdater
	retrivers map[string]Retriver

	syncSource    []string
	VSPSources    []string
	PowSources    []string
	ExchangeKeys  []string
	NodeVersion   []string
	NodeLocations []string

	mempooTip      uint64
	propagationTip uint64
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
func (charts *Manager) Lengthen(tags ...string) error {
	if len(tags) == 0 {
		tags = []string{
			Mempool, Propagation, PowChart, VSP, Exchange, Snapshot, Community,
		}
	}

	if err := charts.NormalizeLength(tags...); err != nil {
		return err
	}
	lengtheners := map[string]func() error{
		Mempool:     charts.lengthenMempool,
		Propagation: charts.lengthenPropagation,
		Snapshot:    charts.lengthenSnapshot,
		VSP:         charts.lengthenVsp,
		PowChart:    charts.lengthenPow,
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

func generateDayBin(dates, heights ChartUints) (days, dayHeights ChartUints, dayIntervals [][2]int) {
	if dates.Length() != heights.Length() {
		log.Criticalf("generateHourBin: length mismatch")
		return
	}

	// Get the current first and last midnight stamps.
	var start = midnight(dates[0])
	end := midnight(dates[len(dates)-1])

	// the index that begins new data.
	offset := 0
	// If there is day or more worth of new data, append to the Days zoomSet by
	// finding the first and last+1 blocks of each new day, and taking averages
	// or sums of the blocks in the interval.  0.06096031
	if end > start+aDay {
		next := start + aDay
		startIdx := 0
		for i, t := range dates[offset:] {
			if t >= next {
				// Once passed the next midnight, prepare a day window by
				// storing the range of indices. 0, 1, 2, 3, 4, 5
				dayIntervals = append(dayIntervals, [2]int{startIdx + offset, i + offset})
				// check for records b/4 appending.
				days = append(days, start)
				dayHeights = append(dayHeights, heights[i])
				next = midnight(t)
				start = next
				next += aDay
				startIdx = i
				if t > end {
					break
				}
			}
		}
	}
	return
}

func generateHourBin(dates, heights ChartUints) (hours, hourHeights ChartUints, hourIntervals [][2]int) {
	if dates.Length() != heights.Length() {
		log.Criticalf("generateHourBin: length mismatch")
		return
	}
	// Get the current first and last hour stamps.
	start := hourStamp(dates[0])
	end := hourStamp(dates[len(dates)-1])

	// the index that begins new data.
	offset := 0
	// If there is day or more worth of new data, append to the Days zoomSet by
	// finding the first and last+1 blocks of each new day, and taking averages
	// or sums of the blocks in the interval.
	if end > start+anHour {
		next := start + anHour
		startIdx := 0
		for i, t := range dates[offset:] {
			if t >= next {
				// Once passed the next hour, prepare a day window by storing
				// the range of indices.
				hourIntervals = append(hourIntervals, [2]int{startIdx + offset, i + offset})
				hours = append(hours, start)
				hourHeights = append(hourHeights, heights[i])
				next = hourStamp(t)
				start = next
				next += anHour
				startIdx = i
				if t > end {
					break
				}
			}
		}
	}

	return
}

func (charts *Manager) lengthenChartUintsx(data ChartUints, dayIntervals [][2]int,
	hourIntervals [][2]int) (dayData, hourData ChartUints) {

	// day bin
	for _, interval := range dayIntervals {
		// For each new day, take an appropriate snapshot.
		dayData = append(dayData, data.Avg(interval[0], interval[1]))
	}

	// hour bin
	for _, interval := range hourIntervals {
		// For each new day, take an appropriate snapshot.
		hourData = append(hourData, data.Avg(interval[0], interval[1]))
	}
	return
}

func (charts *Manager) NormalizeLength(tags ...string) error {
	txn := charts.DB.NewTransaction(true)
	defer txn.Discard()

	for _, chartID := range tags {
		if cerr := charts.normalizeLength(chartID, txn); cerr != nil {
			return errors.Wrap(cerr, "Normalize failed for "+chartID)
		}
	}
	if err := txn.Commit(); err != nil {
		return err
	}
	return nil
}

// length correction
func (charts *Manager) normalizeLength(chartID string, txn *badger.Txn) error {
	// TODO: use transaction
	switch chartID {
	case Mempool:
		return charts.normalizeMempoolLength()

	case Propagation:
		return charts.normalizePropagationLength()

	case PowChart:
		return charts.normalizePowChartLength(txn)

	case VSP:
		return charts.normalizeVSPLength(txn)

	case Exchange:
		return charts.normalizeExchangeLength(txn)

	case Snapshot:
		return charts.normalizeSnapshotLength(txn)
	case Community:
		return nil

	}

	return nil
}

func isFileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return !os.IsNotExist(err)
}

// Load loads chart data from the gob file at the specified path and performs an
// update.
func (charts *Manager) Load(ctx context.Context) error {
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
func (charts *Manager) TriggerUpdate(ctx context.Context, tag string) error {
	if err := charts.Update(ctx, tag); err != nil {
		// Only log errors from ChartsData.Update. TODO: make this more severe.
		log.Errorf("(*ChartData).Update failed: %v", err)
	}
	charts.ClearVLog()
	return nil
}

// AddRetriever adds a Retriever to the Retrievers slice.
func (charts *Manager) AddRetriever(chartID string, retriever Retriver) {
	charts.retrivers[chartID] = retriever
}

// AddUpdater adds a ChartUpdater to the Updaters slice. Updaters are run
// sequentially during (*ChartData).Update.
func (charts *Manager) AddUpdater(updater ChartUpdater) {
	charts.updaters[updater.Tag] = updater
}

// Update refreshes chart data by calling the ChartUpdaters sequentially. The
// Update is abandoned with a warning if stateID changes while running a Fetcher
// (likely due to a new update starting during a query).
func (charts *Manager) Update(ctx context.Context, tags ...string) error {
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
		var completed bool
		var page = 1
		for !completed {
			stateID := charts.cacheID(updater.Tag)
			rows, cancel, done, err := updater.Fetcher(ctx, charts, page)
			if err != nil {
				err = fmt.Errorf("error encountered during charts %s update. aborting update: %v", updater.Tag, err)
			} else {
				if updater.Appender != nil {
					charts.mtx.Lock()
					if stateID != charts.cacheID(updater.Tag) {
						if updater.Tag != VSP {
							err = fmt.Errorf("state change detected during charts %s update. aborting update", updater.Tag)
						}
					} else {
						err = updater.Appender(charts, rows)
						if err != nil {
							err = fmt.Errorf("error detected during charts %s append. aborting update: %v", updater.Tag, err)
						}
					}
					charts.mtx.Unlock()
				}
				if err == nil {
					log.Infof("Updated %s cache data", updater.Tag)
				}
			}
			completed = done
			if updater.Tag != VSP {
				completed = true
			}
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
	poolSources, vsps, nodeLocations, nodeVersion []string, chainParams *chaincfg.Params, db *badger.DB, dir string) *Manager {

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
	// create cache dir if not existing

	return &Manager{
		ctx:           ctx,
		EnableCache:   enableCache,
		DB:            db,
		dir:           dir,
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
func (charts *Manager) cacheID(chartID string) uint64 {
	switch chartID {
	case Mempool:
		return charts.MempoolTip()
	case BlockPropagation:
	case BlockTimestamp:
	case VotesReceiveTime:
		return charts.PropagationTip()
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
	return charts.MempoolTip()
}

// Grab the cached data, if it exists. The cacheID is returned as a convenience.
func (charts *Manager) getCache(chartID string, axis axisType) (data *cachedChart, found bool, cacheID uint64) {
	// Ignore zero length since bestHeight would just be set to zero anyway.
	ck := cacheKey(chartID, axis)
	charts.cacheMtx.RLock()
	defer charts.cacheMtx.RUnlock()
	cacheID = charts.cacheID(chartID)
	data, found = charts.cache[ck]

	err := charts.DB.View(func(txn *badger.Txn) error {
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
func (charts *Manager) cacheChart(chartID string, version uint64, axis axisType, data []byte) {
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
	err := charts.DB.Update(func(txn *badger.Txn) error {
		err := txn.Set([]byte(ck), b.Bytes())
		return err
	})
	if err != nil {
		log.Errorf("Error caching cart, %s, %s", chartID, err.Error())
	}

}

func (charts *Manager) removeCache(chartID string, axis axisType) {
	ck := cacheKey(chartID, axis)
	err := charts.DB.Update(func(txn *badger.Txn) error {
		err := txn.Delete([]byte(ck))
		return err
	})
	if err != nil {
		log.Errorf("Error delete chart cache, %s, %s", chartID, err.Error())
	}
}

// ChartMaker is a function that accepts a chart type and BinLevel, and returns
// a JSON-encoded chartResponse.
type ChartMaker func(ctx context.Context, charts *Manager, dataType, axis axisType, bin binLevel, sources ...string) ([]byte, error)

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
func (charts *Manager) Chart(ctx context.Context, chartID, dataType, axisString, binString string, extras ...string) ([]byte, error) {

	dataTypeAxis := ParseAxis(dataType)
	bin := ParseBin(binString)
	axis := ParseAxis(axisString)
	key := fmt.Sprintf("%s-%s-%s-%s", chartID, bin, strings.Join(extras, "-"), string(dataTypeAxis))
	cache, found, _ := charts.getCache(key, dataTypeAxis)
	if found {
		if cache.Version == charts.cacheID(chartID) {
			return cache.Data, nil
		}
		charts.removeCache(key, dataTypeAxis)
	}

	if !charts.EnableCache {
		retriever, hasRetriever := charts.retrivers[chartID]
		if !hasRetriever {
			return nil, UnknownChartErr
		}
		data, err := retriever(ctx, charts, string(dataTypeAxis), string(axis), string(bin), extras...)
		if err != nil {
			return nil, err
		}
		charts.cacheChart(key, charts.cacheID(chartID), dataTypeAxis, data)
		return data, nil
	}

	maker, hasMaker := chartMakers[chartID]
	if !hasMaker {
		return nil, UnknownChartErr
	}
	// Do the locking here, rather than in encodeXY, so that the helper functions
	// (accumulate, btw) are run under lock.
	charts.mtx.RLock()
	data, err := maker(ctx, charts, dataTypeAxis, axis, bin, extras...)
	charts.mtx.RUnlock()
	if err != nil {
		return nil, err
	}
	charts.cacheChart(key, charts.cacheID(chartID), dataTypeAxis, data)
	return data, nil
}

// Keys used for the chartResponse data sets.
var responseKeys = []string{"x", "y", "z"}

// Encode the slices. The set lengths are truncated to the smallest of the
// arguments.
func (charts *Manager) Encode(keys []string, sets ...Lengther) ([]byte, error) {
	return charts.encodeArr(keys, sets)
}

// Encode the slices. The set lengths are truncated to the smallest of the
// arguments.
func (charts *Manager) encodeArr(keys []string, sets []Lengther) ([]byte, error) {
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
func (charts *Manager) trim(sets ...Lengther) []Lengther {
	if len(sets) == 2 {
		return sets
	}
	dLen := sets[0].Length()
	for i := dLen - 1; i >= 0; i-- {
		var isZero bool = true
	out:
		for j := 1; j < len(sets); j++ {
			if sets[j] != nil && !sets[j].IsZero(i) {
				isZero = false
				break out
			}
		}
		if isZero {
			for j := 0; j < len(sets); j++ {
				if sets[j] != nil {
					sets[j] = sets[j].Remove(i)
				}
			}
		}
	}

	return sets
}

func mempool(ctx context.Context, charts *Manager, dataType, axis axisType, bin binLevel, _ ...string) ([]byte, error) {

	mempoolSet, err := charts.MempoolSet(bin)
	if err != nil {
		return nil, err
	}
	var xAxis ChartUints
	if axis == TimeAxis {
		xAxis = mempoolSet.Time
	} else {
		xAxis = mempoolSet.Heights
	}

	switch dataType {
	case MempoolSize:
		return charts.Encode(nil, xAxis, mempoolSet.Size)
	case MempoolTxCount:
		return charts.Encode(nil, xAxis, mempoolSet.TxCount)
	case MempoolFees:
		return charts.Encode(nil, xAxis, mempoolSet.Fee)
	}
	return nil, UnknownChartErr

	// switch dataType {
	// case MempoolSize:
	// 	return mempoolSize(charts, axis, bin)
	// case MempoolTxCount:
	// 	return mempoolTxCount(charts, axis, bin)
	// case MempoolFees:
	// 	return mempoolFees(charts, axis, bin)
	// }
	// return nil, UnknownChartErr
}

func mempoolSize(charts *Manager, axis axisType, bin binLevel) ([]byte, error) {
	var dates, sizes ChartUints

	key := fmt.Sprintf("%s-%s", Mempool, HeightAxis)
	if axis == TimeAxis {
		key = fmt.Sprintf("%s-%s", Mempool, TimeAxis)
	}
	if bin != DefaultBin {
		key = fmt.Sprintf("%s-%s", key, bin)
	}
	if err := charts.ReadVal(key, &dates); err != nil {
		log.Info(key)
		return nil, err
	}

	key = fmt.Sprintf("%s-%s", Mempool, MempoolSize)
	if bin != DefaultBin {
		key = fmt.Sprintf("%s-%s", key, bin)
	}
	if err := charts.ReadVal(key, &sizes); err != nil {
		return nil, err
	}

	return charts.Encode(nil, dates, sizes)
}

func mempoolTxCount(charts *Manager, axis axisType, bin binLevel) ([]byte, error) {
	var dates, txCounts ChartUints

	key := fmt.Sprintf("%s-%s", Mempool, HeightAxis)
	if axis == TimeAxis {
		key = fmt.Sprintf("%s-%s", Mempool, TimeAxis)
	}
	if bin != DefaultBin {
		key = fmt.Sprintf("%s-%s", key, bin)
	}

	if err := charts.ReadVal(key, &dates); err != nil {
		log.Info(key)
		return nil, err
	}

	key = fmt.Sprintf("%s-%s", Mempool, MempoolTxCount)
	if bin != DefaultBin {
		key = fmt.Sprintf("%s-%s", key, bin)
	}
	if err := charts.ReadVal(key, &txCounts); err != nil {
		return nil, err
	}

	return charts.Encode(nil, dates, txCounts)
}

func mempoolFees(charts *Manager, axis axisType, bin binLevel) ([]byte, error) {
	var dates ChartUints
	var fees ChartFloats

	key := fmt.Sprintf("%s-%s", Mempool, HeightAxis)
	if axis == TimeAxis {
		key = fmt.Sprintf("%s-%s", Mempool, TimeAxis)
	}
	if bin != DefaultBin {
		key = fmt.Sprintf("%s-%s", key, bin)
	}

	if err := charts.ReadVal(key, &dates); err != nil {
		return nil, err
	}

	key = fmt.Sprintf("%s-%s", Mempool, MempoolFees)
	if bin != DefaultBin {
		key = fmt.Sprintf("%s-%s", key, bin)
	}
	if err := charts.ReadVal(key, &fees); err != nil {
		return nil, err
	}
	return charts.Encode(nil, dates, fees)
}

func propagation(ctx context.Context, charts *Manager, dataType, axis axisType, bin binLevel, syncSources ...string) ([]byte, error) {

	mempoolSet := charts.PropagationSet(bin)
	var xAxis ChartUints
	if axis == TimeAxis {
		xAxis = mempoolSet.Time
	} else {
		xAxis = mempoolSet.Heights
	}

	switch dataType {
	case BlockPropagation:
		var deviations = []Lengther{xAxis}
		for _, source := range syncSources {
			deviations = append(deviations, mempoolSet.BlockPropagation[source])
		}
		return charts.Encode(nil, deviations...)
	case BlockTimestamp:
		return charts.Encode(nil, xAxis, mempoolSet.BlockDelay)
	case VotesReceiveTime:
		return charts.Encode(nil, xAxis, mempoolSet.VoteReceiveTimeDeviations)
	}
	return nil, UnknownChartErr
}

func powChart(ctx context.Context, charts *Manager, dataType, axis axisType, bin binLevel, pools ...string) ([]byte, error) {
	var dates ChartUints
	key := PowChart + "-" + string(TimeAxis)
	if bin != DefaultBin {
		key = fmt.Sprintf("%s-%s", key, bin)
	}
	if err := charts.ReadVal(key, &dates); err != nil {
		return nil, err
	}

	var deviations = make([]ChartNullUints, len(pools))

	for i, s := range pools {
		key = fmt.Sprintf("%s-%s-%s", PowChart, dataType, s)
		if bin != DefaultBin {
			key = fmt.Sprintf("%s-%s", key, bin)
		}
		var data chartNullIntsPointer
		if err := charts.ReadVal(key, &data); err != nil {
			return nil, err
		}
		deviations[i] = data.toChartNullUint()

	}

	return MakePowChart(charts, dates, deviations, pools)
}

func powCharta(ctx context.Context, charts *Manager, dataType, axis axisType, bin binLevel, pools ...string) ([]byte, error) {
	retriever, hasRetriever := charts.retrivers[PowChart]
	if !hasRetriever {
		return nil, UnknownChartErr
	}
	return retriever(ctx, charts, string(dataType), string(axis), string(bin), pools...)
}

func MakePowChart(charts *Manager, dates ChartUints, deviations []ChartNullUints, pools []string) ([]byte, error) {

	var recs = []Lengther{dates}
	for _, d := range deviations {
		recs = append(recs, d)
	}
	recs = charts.trim(recs...)
	return charts.Encode(nil, recs...)
}

func makeVspChart(ctx context.Context, charts *Manager, dataType, axis axisType, bin binLevel, vsps ...string) ([]byte, error) {
	var dates ChartUints
	key := VSP + "-" + string(TimeAxis)
	if bin != DefaultBin {
		key = fmt.Sprintf("%s-%s", key, bin)
	}
	if err := charts.ReadVal(key, &dates); err != nil {
		return nil, err
	}

	var deviations = make([]ChartNullData, len(vsps))

	for i, s := range vsps {
		key = fmt.Sprintf("%s-%s-%s", VSP, dataType, s)
		if bin != DefaultBin {
			key = fmt.Sprintf("%s-%s", key, bin)
		}
		switch dataType {
		case ImmatureAxis, LiveAxis, VotedAxis, MissedAxis, UserCountAxis, UsersActiveAxis:
			var data chartNullIntsPointer
			if err := charts.ReadVal(key, &data); err != nil {
				return nil, err
			}
			deviations[i] = data.toChartNullUint()

		case ProportionLiveAxis, ProportionMissedAxis, PoolFeesAxis:
			var data chartNullFloatsPointer
			if err := charts.ReadVal(key, &data); err != nil {
				return nil, err
			}
			deviations[i] = data.toChartNullFloats()
		}

	}

	return MakeVspChart(charts, dates, deviations, vsps)
}

// ImmatureAxis, LiveAxis, VotedAxis, MissedAxis, UserCountAxis, UsersActiveAxis
// PoolFeesAxis, ProportionLiveAxis, ProportionMissedAxis,

func MakeVspChart(charts *Manager, dates ChartUints, deviations []ChartNullData, vsps []string) ([]byte, error) {
	var recs = []Lengther{dates}
	for _, d := range deviations {
		recs = append(recs, d)
	}

	recs = charts.trim(recs...)
	return charts.Encode(nil, recs...)
}

func networkSnapshorChart(ctx context.Context, charts *Manager, dataType, _ axisType, bin binLevel, extras ...string) ([]byte, error) {
	switch dataType {
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

func networkSnapshotNodesChart(charts *Manager, bin binLevel) ([]byte, error) {
	var dates, nodes, reachableNodes ChartUints

	var key = fmt.Sprintf("%s-%s", Snapshot, TimeAxis)
	if bin != DefaultBin {
		key = fmt.Sprintf("%s-%s", key, bin)
	}
	if err := charts.ReadVal(key, &dates); err != nil {
		log.Info(key)
		return nil, err
	}

	key = fmt.Sprintf("%s-%s", Snapshot, SnapshotNodes)
	if bin != DefaultBin {
		key = fmt.Sprintf("%s-%s", key, bin)
	}
	if err := charts.ReadVal(key, &nodes); err != nil {
		return nil, err
	}

	key = fmt.Sprintf("%s-%s", Snapshot, SnapshotReachableNodes)
	if bin != DefaultBin {
		key = fmt.Sprintf("%s-%s", key, bin)
	}
	if err := charts.ReadVal(key, &reachableNodes); err != nil {
		return nil, err
	}

	return charts.Encode(nil, dates, nodes, reachableNodes)
}

func networkSnapshotLocationsChart(charts *Manager, bin binLevel, countries ...string) ([]byte, error) {
	var recs = make([]Lengther, len(countries)+1)
	var dates ChartUints
	key := fmt.Sprintf("%s-%s-%s", Snapshot, SnapshotLocations, TimeAxis)
	if bin != DefaultBin {
		key = fmt.Sprintf("%s-%s", key, bin)
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
		if bin != DefaultBin {
			key = fmt.Sprintf("%s-%s", key, bin)
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

func networkSnapshotNodeVersionsChart(charts *Manager, bin binLevel, userAgents ...string) ([]byte, error) {
	var recs = make([]Lengther, len(userAgents)+1)
	var dates ChartUints
	key := fmt.Sprintf("%s-%s-%s", Snapshot, SnapshotNodeVersions, TimeAxis)
	if bin != DefaultBin {
		key = fmt.Sprintf("%s-%s", key, bin)
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
		if bin != DefaultBin {
			key = fmt.Sprintf("%s-%s", key, bin)
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
