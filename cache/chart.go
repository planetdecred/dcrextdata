package cache

import (
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/decred/dcrd/chaincfg"
	"github.com/volatiletech/null"
)

// Keys for specifying chart data type.
const (
	MempoolSize 		= "mempool-size"
	MempoolFees			= "mempool-fees"
	MempoolTxCount		= "mempool-tx-count"

	BlockPropagation	= "block-propagation"
	BlockTimestamp		= "block-timestamp"
	VotesReceiveTime	= "votes-receive-time"

	PowChart = "pow"
	VSP = "vsp"
)

// binLevel specifies the granularity of data.
type binLevel string

// axisType is used to manage the type of x-axis data on display on the specified
// chart.
type axisType string

// These are the recognized binLevel and axisType values.
const (
	DayBin     binLevel = "day"
	BlockBin   binLevel = "block"
	WindowBin  binLevel = "window"
	MempoolBin binLevel = "mempool"
	PropagationBin binLevel = "propagation"
	PowBin binLevel = "pow"

	HeightAxis axisType = "height"
	TimeAxis   axisType = "time"
	HashrateAxis axisType = "hashrate"
	WorkerAxis	axisType = "workers"

	ImmatureAxis axisType = "immature"
	LiveAxis axisType = "live"
	VotedAxis axisType = "voted"
	MissedAxis axisType = "missed"
	PoolFeesAxis axisType = "pool-fees"
	ProportionLiveAxis axisType = "proportion-live"
	ProportionMissedAxis axisType = "proportion-missed"
	UserCountAxis axisType = "user-count"
	UsersActiveAxis axisType = "users-active"
)

// DefaultBinLevel will be used if a bin level is not specified to
// (*ChartData).Chart (via empty string), or if the provided BinLevel is
// invalid.
var DefaultBinLevel = DayBin

// ParseBin will return the matching bin level, else the default bin.
func ParseBin(bin string) binLevel {
	switch binLevel(bin) {
	case BlockBin:
		return BlockBin
	case WindowBin:
		return WindowBin
	}
	return DefaultBinLevel
}

// ParseAxis returns the matching axis type, else the default of time axis.
func ParseAxis(aType string) axisType {
	aType = strings.ToLower(aType)
	switch axisType(aType) {
	case HeightAxis:
		return HeightAxis
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
	default:
		return TimeAxis
	}
}

const (
	// aDay defines the number of seconds in a day.
	aDay = 86400
)

// cacheVersion helps detect when the cache data stored has changed its
// structure or content. A change on the cache version results to recomputing
// all the charts data a fresh thereby making the cache to hold the latest changes.
var cacheVersion = NewSemver(6, 0, 0)

// versionedCacheData defines the cache data contents to be written into a .gob file.
type versionedCacheData struct {
	Version string
	Data    *ChartGobject
}

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
type lengther interface {
	Length() int
	Truncate(int) lengther
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
func (data ChartFloats) Truncate(l int) lengther {
	return data[:l]
}

// If the data is longer than max, return a subset of length max.
func (data ChartFloats) snip(max int) ChartFloats {
	if len(data) < max {
		max = len(data)
	}
	return data[:max]
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

// Sum is the accumulation of a segment of the dataset.
func (data ChartFloats) Sum(s, e int) (sum float64) {
	if e <= s {
		return 0
	}
	for _, v := range data[s:e] {
		sum += v
	}
	return
}

// A constructor for a sized ChartFloats.
func newChartFloats(size int) ChartFloats {
	return make([]float64, 0, size)
}

type ChartNullData interface {
	lengther
	Value(index int) interface{}
	Valid(index int) bool
	IsZero(index int) bool
	String(index int) string
}

// ChartNullUints is a slice of null.uints. It satisfies the lengther interface.
type ChartNullUints []*null.Uint64

func (data ChartNullUints) Value(index int) interface{} {
	if data == nil || len(data) <= index || data[index] == nil {
		return uint64(0)
	}
	return data[index].Uint64
}

func (data ChartNullUints) Valid(index int) bool {
	if data != nil && len(data) > index && data[index] != nil{
		return data[index].Valid
	}
	return false
}

func (data ChartNullUints) IsZero(index int) bool {
	return data.Value(index).(uint64) == 0
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
func (data ChartNullUints) Truncate(l int) lengther {
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
func newChartNullUints(size int) ChartNullUints {
	return make(ChartNullUints, 0, size)
}

// ChartNullFloats is a slice of null.float64. It satisfies the lengther interface.
type ChartNullFloats []*null.Float64


func (data ChartNullFloats) Value(index int) interface{} {
	if data == nil || len(data) <= index  || data[index] == nil{
		return float64(0)
	}
	return data[index].Float64
}

func (data ChartNullFloats) Valid(index int) bool {
	if data != nil && len(data) > index  && data[index] != nil{
		return data[index].Valid
	}
	return false
}

func (data ChartNullFloats) IsZero(index int) bool {
	return data.Value(index).(float64) == 0
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
func (data ChartNullFloats) Truncate(l int) lengther {
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
func newChartNullFloats(size int) ChartNullFloats {
	return make(ChartNullFloats, 0, size)
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
func (data ChartStrings) Truncate(l int) lengther {
	return data[:l]
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
func (data ChartUints) Truncate(l int) lengther {
	return data[:l]
}

// If the data is longer than max, return a subset of length max.
func (data ChartUints) snip(max int) ChartUints {
	if len(data) < max {
		max = len(data)
	}
	return data[:max]
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

// Sum is the accumulation of a segment of the dataset.
func (data ChartUints) Sum(s, e int) (sum uint64) {
	if e <= s {
		return 0
	}
	for _, v := range data[s:e] {
		sum += v
	}
	return
}

func margeChartUints(data ...ChartUints) ChartUints {
	var allRecords ChartUints
	for _, record := range data {
		allRecords = append(allRecords, record...)
	}

	sort.Slice(allRecords, func(i, j int) bool {
		return allRecords[j] > allRecords[i]
	})

	var keys = map[uint64]bool{}
	var result ChartUints
	for _, record := range allRecords {
		if _, found := keys[record]; !found {
			keys[record] = true
			result = append(result, record)
		}
	}

	return result
}

// A constructor for a sized ChartUints.
func newChartUints(size int) ChartUints {
	return make(ChartUints, 0, size)
}

// mempoolSet holds data for mempool fees, size and tx-count chart
type mempoolSet struct {
	cacheID uint64
	Time    ChartUints
	Size    ChartUints
	TxCount ChartUints
	Fees 	ChartFloats
}

// Snip truncates the zoomSet to a provided length.
func (set *mempoolSet) Snip(length int) {
	if length < 0 {
		length = 0
	}
	set.Time = set.Time.snip(length)
	set.Size = set.Size.snip(length)
	set.Fees = set.Fees.snip(length)
	set.TxCount = set.TxCount.snip(length)
}

// Constructor for a sized zoomSet for blocks, which has has no PropagationHeight slice
// since the height is implicit for block-binned data.
func newMempoolSet(size int) *mempoolSet {
	return &mempoolSet{
		Time:      newChartUints(size),
		Size:  newChartUints(size),
		TxCount:   newChartUints(size),
		Fees:      newChartFloats(size),
	}
}

// propagationSet is a set of propagation data
type propagationSet struct {
	cacheID                    uint64
	Height                     ChartUints
	BlockPropagation           map[string]ChartFloats
	BlockDelays                ChartFloats
	VotesReceiveTimeDeviations ChartFloats
}

// Snip truncates the zoomSet to a provided length.
func (set *propagationSet) Snip(length int) {
	if length < 0 {
		length = 0
	}
	set.Height = set.Height.snip(length)
	for source, records := range set.BlockPropagation {
		set.BlockPropagation[source] = records.snip(length)
	}
	set.BlockDelays = set.BlockDelays.snip(length)
	set.VotesReceiveTimeDeviations = set.VotesReceiveTimeDeviations.snip(length)
}

// Constructor for a sized zoomSet for blocks, which has has no PropagationHeight slice
// since the height is implicit for block-binned data.
func newPropagationSet(size int, syncSources []string) *propagationSet {
	blockPropagation := make(map[string]ChartFloats)
	for _, source := range syncSources {
		blockPropagation[source] = newChartFloats(size)
	}
	return &propagationSet{
		Height:                     newChartUints(size),
		BlockDelays:                newChartFloats(size),
		VotesReceiveTimeDeviations: newChartFloats(size),
		BlockPropagation:           blockPropagation,
	}
}

// powSet is a set of powChart data
type powSet struct {
	cacheID  uint64
	Time     ChartUints
	Hashrate map[string]ChartNullUints
	Workers  map[string]ChartNullUints
}

// Snip truncates the zoomSet to a provided length.
func (set *powSet) Snip(length int) {
	if length < 0 {
		length = 0
	}

	set.Time = set.Time.snip(length)

	for pool, records := range set.Hashrate {
		set.Hashrate[pool] = records.snip(length)
	}

	for pool, records := range set.Workers {
		set.Workers[pool] = records.snip(length)
	}
}

// Constructor for a sized zoomSet for blocks, which has has no PropagationHeight slice
// since the height is implicit for block-binned data.
func newPowSet(pools []string, size int) *powSet {
	hashrate := make(map[string]ChartNullUints)
	for _, pool := range pools {
		hashrate[pool] = newChartNullUints(size)
	}

	workers := make(map[string]ChartNullUints)
	for _, pool := range pools {
		workers[pool] = newChartNullUints(size)
	}

	return &powSet{
		Time:     newChartUints(size),
		Hashrate: hashrate,
		Workers:  workers,
	}
}

// vspSet is a set of Vsp chart data
type vspSet struct {
	cacheID          uint64
	Time             ChartUints
	Immature         map[string]ChartNullUints
	Live             map[string]ChartNullUints
	Voted            map[string]ChartNullUints
	Missed           map[string]ChartNullUints
	PoolFees         map[string]ChartNullFloats
	ProportionLive   map[string]ChartNullFloats
	ProportionMissed map[string]ChartNullFloats
	UserCount        map[string]ChartNullUints
	UsersActive      map[string]ChartNullUints
}

// Snip truncates the vspSet to a provided length.
func (set *vspSet) Snip(length int) {
	if length < 0 {
		length = 0
	}

	set.Time = set.Time.snip(length)

	for vsp, records := range set.Immature {
		set.Immature[vsp] = records.snip(length)
	}
	for vsp, records := range set.Live {
		set.Live[vsp] = records.snip(length)
	}
	for vsp, records := range set.Voted {
		set.Voted[vsp] = records.snip(length)
	}
	for vsp, records := range set.Missed {
		set.Missed[vsp] = records.snip(length)
	}
	for vsp, records := range set.PoolFees {
		set.PoolFees[vsp] = records.snip(length)
	}
	for vsp, records := range set.ProportionMissed {
		set.ProportionMissed[vsp] = records.snip(length)
	}
	for vsp, records := range set.ProportionLive {
		set.ProportionLive[vsp] = records.snip(length)
	}
	for vsp, records := range set.UsersActive {
		set.UsersActive[vsp] = records.snip(length)
	}
	for vsp, records := range set.UserCount {
		set.UserCount[vsp] = records.snip(length)
	}
}

// Constructor for a sized vspSet.
func newVspSet(vsps []string, size int) *vspSet {
	immature := make(map[string]ChartNullUints)
	for _, vsp := range vsps {
		immature[vsp] = newChartNullUints(size)
	}

	live := make(map[string]ChartNullUints)
	for _, vsp := range vsps {
		live[vsp] = newChartNullUints(size)
	}

	voted := make(map[string]ChartNullUints)
	for _, vsp := range vsps {
		voted[vsp] = newChartNullUints(size)
	}

	missed := make(map[string]ChartNullUints)
	for _, vsp := range vsps {
		missed[vsp] = newChartNullUints(size)
	}

	poolFees := make(map[string]ChartNullFloats)
	for _, vsp := range vsps {
		poolFees[vsp] = newChartNullFloats(size)
	}

	proportionLive := make(map[string]ChartNullFloats)
	for _, vsp := range vsps {
		proportionLive[vsp] = newChartNullFloats(size)
	}

	proportionMissed := make(map[string]ChartNullFloats)
	for _, vsp := range vsps {
		proportionMissed[vsp] = newChartNullFloats(size)
	}

	userCount := make(map[string]ChartNullUints)
	for _, vsp := range vsps {
		userCount[vsp] = newChartNullUints(size)
	}

	usersActive := make(map[string]ChartNullUints)
	for _, vsp := range vsps {
		immature[vsp] = newChartNullUints(size)
	}

	return &vspSet{
		Time:             newChartUints(size),
		Immature:         immature,
		Live:             live,
		Voted:            voted,
		Missed:           missed,
		PoolFees:         poolFees,
		ProportionLive:   proportionLive,
		ProportionMissed: proportionMissed,
		UserCount:        userCount,
		UsersActive:      usersActive,
	}
}

// zoomSet is a set of binned data. The smallest bin is block-sized. The zoomSet
// is managed by explorer, and subsequently the database packages. ChartData
// provides methods for validating the data and handling concurrency. The
// cacheID is updated anytime new data is added and validated (see
// Lengthen), typically once per bin duration.
type zoomSet struct {
	cacheID   uint64
	Height    ChartUints
	Time      ChartUints
	PoolSize  ChartUints
	PoolValue ChartFloats
	BlockSize ChartUints
	TxCount   ChartUints
	NewAtoms  ChartUints
	Chainwork ChartUints
	Fees      ChartUints
}

// Snip truncates the zoomSet to a provided length.
func (set *zoomSet) Snip(length int) {
	if length < 0 {
		length = 0
	}
	set.Height = set.Height.snip(length)
	set.Time = set.Time.snip(length)
	set.PoolSize = set.PoolSize.snip(length)
	set.PoolValue = set.PoolValue.snip(length)
	set.BlockSize = set.BlockSize.snip(length)
	set.TxCount = set.TxCount.snip(length)
	set.NewAtoms = set.NewAtoms.snip(length)
	set.Chainwork = set.Chainwork.snip(length)
	set.Fees = set.Fees.snip(length)
}

// Constructor for a sized zoomSet for blocks, which has has no PropagationHeight slice
// since the height is implicit for block-binned data.
func newBlockSet(size int) *zoomSet {
	return &zoomSet{
		Height:    newChartUints(size),
		Time:      newChartUints(size),
		PoolSize:  newChartUints(size),
		PoolValue: newChartFloats(size),
		BlockSize: newChartUints(size),
		TxCount:   newChartUints(size),
		NewAtoms:  newChartUints(size),
		Chainwork: newChartUints(size),
		Fees:      newChartUints(size),
	}
}

// Constructor for a sized zoomSet for day-binned data.
func newDaySet(size int) *zoomSet {
	set := newBlockSet(size)
	set.Height = newChartUints(size)
	return set
}

// windowSet is for data that only changes at the difficulty change interval,
// 144 blocks on mainnet. stakeValid defines the number windows before the
// stake validation height.
type windowSet struct {
	cacheID     uint64
	Time        ChartUints
	PowDiff     ChartFloats
	TicketPrice ChartUints
	StakeCount  ChartUints
	MissedVotes ChartUints
}

// Snip truncates the windowSet to a provided length.
func (set *windowSet) Snip(length int) {
	if length < 0 {
		length = 0
	}

	set.Time = set.Time.snip(length)
	set.PowDiff = set.PowDiff.snip(length)
	set.TicketPrice = set.TicketPrice.snip(length)
	set.StakeCount = set.StakeCount.snip(length)
	set.MissedVotes = set.MissedVotes.snip(length)
}

// Constructor for a sized windowSet.
func newWindowSet(size int) *windowSet {
	return &windowSet{
		Time:        newChartUints(size),
		PowDiff:     newChartFloats(size),
		TicketPrice: newChartUints(size),
		StakeCount:  newChartUints(size),
		MissedVotes: newChartUints(size),
	}
}

// ChartGobject is the storage object for saving to a gob file. ChartData itself
// has a lot of extraneous fields, and also embeds sync.RWMutex, so is not
// suitable for gobbing.
type ChartGobject struct {
	MempoolTime       ChartUints
	MempoolSize       ChartUints
	MempoolFees       ChartFloats
	MempoolTxCount    ChartUints
	PropagationHeight ChartUints
	PropagationTime   ChartUints
	BlockPropagation  map[string]ChartFloats
	ChartDelays       ChartFloats
	VotesReceiveTime  ChartFloats

	PowTime			  ChartUints
	PowHashrate		  map[string]ChartNullUints
	PowWorkers		  map[string]ChartNullUints

	VspTime             ChartUints
	VspImmature         map[string]ChartNullUints
	VspLive             map[string]ChartNullUints
	VspVoted            map[string]ChartNullUints
	VspMissed           map[string]ChartNullUints
	VspPoolFees         map[string]ChartNullFloats
	VspProportionLive   map[string]ChartNullFloats
	VspProportionMissed map[string]ChartNullFloats
	VspUserCount        map[string]ChartNullUints
	VspUsersActive      map[string]ChartNullUints


	PoolSize         ChartUints
	PoolValue        ChartFloats
	BlockSize        ChartUints
	TxCount          ChartUints
	NewAtoms         ChartUints
	Chainwork        ChartUints
	Fees             ChartUints
	WindowTime       ChartUints
	PowDiff          ChartFloats
	TicketPrice      ChartUints
	StakeCount       ChartUints
	MissedVotes      ChartUints
}

// The chart data is cached with the current cacheID of the zoomSet or windowSet.
type cachedChart struct {
	cacheID uint64
	data    []byte
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
	// context.CancelFunc if appropriate, else a dummy.
	Fetcher func(ctx context.Context, charts *ChartData) (interface{}, func(), error)
	// The Appender will be run under mutex lock.
	Appender func(charts *ChartData, recordSlice interface{}) error
}

// ChartData is a set of data used for charts. It provides methods for
// managing data validation and update concurrency, but does not perform any
// data retrieval and must be used with care to keep the data valid. The Blocks
// and Windows fields must be updated by (presumably) a database package. The
// Days data is auto-generated from the Blocks data during Lengthen-ing.
type ChartData struct {
	mtx          sync.RWMutex
	ctx          context.Context
	DiffInterval int32
	StartPOS     int32
	Mempool      *mempoolSet
	Propagation  *propagationSet
	Pow          *powSet
	Vsp 		  *vspSet

	Blocks       *zoomSet
	Windows      *windowSet
	Days         *zoomSet
	cacheMtx     sync.RWMutex
	cache        map[string]*cachedChart
	updaters     []ChartUpdater
	syncSource   []string
}

// Check that the length of all arguments is equal.
func ValidateLengths(lens ...lengther) (int, error) {
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

// Reduce the timestamp to the previous midnight.
func midnight(t uint64) (mid uint64) {
	if t > 0 {
		mid = t - t%aDay
	}
	return
}

// Lengthen performs data validation and populates the Days zoomSet. If there is
// an update to a zoomSet or windowSet, the cacheID will be incremented.
func (charts *ChartData) Lengthen() error {
	charts.mtx.Lock()
	defer charts.mtx.Unlock()

	// Make sure the database has set equal number of mempool data set
	mempool := charts.Mempool
	shortest, err := ValidateLengths(mempool.Time, mempool.Fees, mempool.Size, mempool.TxCount)
	if err != nil {
		log.Warnf("ChartData.Lengthen: mempool data length mismatch detected. Truncating mempool to %d", shortest)
		mempool.Snip(shortest)
	}
	if shortest == 0 {
		// no mempool yet. Not an error
		return nil
	}

	// Make sure the database has set equal number of block propagation data set
	propagation := charts.Propagation
	shortest, err = ValidateLengths(propagation.Height, propagation.BlockDelays, propagation.VotesReceiveTimeDeviations)
	if err != nil {
		log.Warnf("ChartData.Lengthen: propagation data length mismatch detected. Truncating propagation to %d", shortest)
		mempool.Snip(shortest)
	}
	if shortest == 0 {
		// no propagation data yet. Not an error
		return nil
	}

	charts.cacheMtx.Lock()
	defer charts.cacheMtx.Unlock()

	// For mempool, blocks and windows, the cacheID is the last timestamp.
	charts.Mempool.cacheID = mempool.Time[len(mempool.Time)-1]
	charts.Propagation.cacheID = propagation.Height[len(propagation.Height)-1]
	return nil
}

// isfileExists checks if the provided file paths exists. It returns true if
// it does exist and false if otherwise.
func isfileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return !os.IsNotExist(err)
}

// writeCacheFile creates the charts cache in the provided file path if it
// doesn't exists. It dumps the ChartsData contents using the .gob encoding.
// Drops the old .gob dump before creating a new one. Delete the old cache here
// rather than after loading so that a dump will still be available after a crash.
func (charts *ChartData) writeCacheFile(filePath string) error {
	if isfileExists(filePath) {
		// delete the old dump files before creating new ones.
		os.RemoveAll(filePath)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}

	defer file.Close()

	encoder := gob.NewEncoder(file)
	charts.mtx.RLock()
	defer charts.mtx.RUnlock()
	return encoder.Encode(versionedCacheData{cacheVersion.String(), charts.gobject()})
}

// readCacheFile reads the contents of the charts cache dump file encoded in
// .gob format if it exists returns an error if otherwise.
func (charts *ChartData) readCacheFile(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}

	defer func() {
		file.Close()
	}()

	var data = new(versionedCacheData)
	decoder := gob.NewDecoder(file)
	err = decoder.Decode(&data)
	if err != nil {
		return err
	}

	// If the required cache version was not found in the .gob file return an error.
	if data.Version != cacheVersion.String() {
		return fmt.Errorf("expected cache version v%s but found v%s",
			cacheVersion, data.Version)
	}

	gobject := data.Data

	charts.mtx.Lock()
	charts.Mempool.Time = gobject.MempoolTime
	charts.Mempool.TxCount = gobject.MempoolTxCount
	charts.Mempool.Size = gobject.MempoolSize
	charts.Mempool.Fees = gobject.MempoolFees

	charts.Propagation.Height = gobject.PropagationHeight
	charts.Propagation.VotesReceiveTimeDeviations = gobject.VotesReceiveTime
	charts.Propagation.BlockDelays = gobject.ChartDelays
	charts.Propagation.BlockPropagation = gobject.BlockPropagation

	charts.Pow.Time = gobject.PowTime
	charts.Pow.Hashrate = gobject.PowHashrate
	charts.Pow.Workers = gobject.PowWorkers

	charts.Blocks.Height = gobject.PropagationHeight
	charts.Blocks.Time = gobject.PropagationTime
	charts.Blocks.PoolSize = gobject.PoolSize
	charts.Blocks.PoolValue = gobject.PoolValue
	charts.Blocks.BlockSize = gobject.BlockSize
	charts.Blocks.TxCount = gobject.TxCount
	charts.Blocks.NewAtoms = gobject.NewAtoms
	charts.Blocks.Chainwork = gobject.Chainwork
	charts.Blocks.Fees = gobject.Fees
	charts.Windows.Time = gobject.WindowTime
	charts.Windows.PowDiff = gobject.PowDiff
	charts.Windows.TicketPrice = gobject.TicketPrice
	charts.Windows.StakeCount = gobject.StakeCount
	charts.Windows.MissedVotes = gobject.MissedVotes
	charts.mtx.Unlock()

	err = charts.Lengthen()
	if err != nil {
		log.Warnf("problem detected during (*ChartData).Lengthen. clearing datasets: %v", err)
		charts.Mempool.Snip(0)
		charts.Blocks.Snip(0)
		charts.Windows.Snip(0)
		charts.Days.Snip(0)
	}

	return nil
}

// Load loads chart data from the gob file at the specified path and performs an
// update.
func (charts *ChartData) Load(ctx context.Context, cacheDumpPath string) error {
	t := time.Now()
	defer func() {
		log.Debugf("Completed the initial chart load and update in %f s",
			time.Since(t).Seconds())
	}()

	if err := charts.readCacheFile(cacheDumpPath); err != nil {
		log.Debugf("Cache dump data loading failed: %v", err)
		// Do not return non-nil error since a new cache file will be generated.
		// Also, return only after Update has restored the charts data.
	}

	// Bring the charts up to date.
	log.Infof("Updating charts data...")
	return charts.Update(ctx)
}

// Dump dumps a ChartGobject to a gob file at the given path.
func (charts *ChartData) Dump(dumpPath string) {
	err := charts.writeCacheFile(dumpPath)
	if err != nil {
		log.Errorf("ChartData.writeCacheFile failed: %v", err)
	} else {
		log.Debug("Dumping the charts cache data was successful")
	}
}

// TriggerUpdate triggers (*ChartData).Update.
func (charts *ChartData) TriggerUpdate(ctx context.Context) error {
	if err := charts.Update(ctx); err != nil {
		// Only log errors from ChartsData.Update. TODO: make this more severe.
		log.Errorf("(*ChartData).Update failed: %v", err)
	}
	return nil
}

func (charts *ChartData) gobject() *ChartGobject {
	return &ChartGobject{
		MempoolTime:         charts.Mempool.Time,
		MempoolSize:         charts.Mempool.Size,
		MempoolFees:         charts.Mempool.Fees,
		MempoolTxCount:      charts.Mempool.TxCount,
		PropagationHeight:   charts.Propagation.Height,
		PropagationTime:     charts.Blocks.Time,
		BlockPropagation:    charts.Propagation.BlockPropagation,
		ChartDelays:         charts.Propagation.BlockDelays,
		VotesReceiveTime:    charts.Propagation.VotesReceiveTimeDeviations,
		PowTime:             charts.Pow.Time,
		PowHashrate:         charts.Pow.Hashrate,
		PowWorkers:          charts.Pow.Workers,

		VspTime:             charts.Vsp.Time,
		VspImmature:         charts.Vsp.Immature,
		VspLive:             charts.Vsp.Live,
		VspVoted:            charts.Vsp.Voted,
		VspMissed:           charts.Vsp.Missed,
		VspPoolFees:         charts.Vsp.PoolFees,
		VspProportionLive:   charts.Vsp.ProportionLive,
		VspProportionMissed: charts.Vsp.ProportionMissed,
		VspUserCount:        charts.Vsp.UserCount,
		VspUsersActive:      charts.Vsp.UsersActive,

		PoolSize:            charts.Blocks.PoolSize,
		PoolValue:           charts.Blocks.PoolValue,
		BlockSize:           charts.Blocks.BlockSize,
		TxCount:             charts.Blocks.TxCount,
		NewAtoms:            charts.Blocks.NewAtoms,
		Chainwork:           charts.Blocks.Chainwork,
		Fees:                charts.Blocks.Fees,
		WindowTime:          charts.Windows.Time,
		PowDiff:             charts.Windows.PowDiff,
		TicketPrice:         charts.Windows.TicketPrice,
		StakeCount:          charts.Windows.StakeCount,
		MissedVotes:         charts.Windows.MissedVotes,
	}
}

// StateID returns a unique (enough) ID associated with the state of the Blocks
// data in a thread-safe way.
func (charts *ChartData) StateID() uint64 {
	charts.mtx.RLock()
	defer charts.mtx.RUnlock()
	return charts.stateID()
}

// stateID returns a unique (enough) ID associated with the state of the Blocks
// data.
func (charts *ChartData) stateID() uint64 {
	timeLen := len(charts.Blocks.Time)
	if timeLen > 0 {
		return charts.Blocks.Time[timeLen-1]
	}
	return 0
}

// ValidState checks whether the provided chartID is still valid. ValidState
// should be used under at least a (*ChartData).RLock.
func (charts *ChartData) validState(stateID uint64) bool {
	return charts.stateID() == stateID
}

// MempoolTime is the time of the latest mempool appended to the chart
func (charts *ChartData) MempoolTime() uint64 {
	charts.mtx.RLock()
	defer charts.mtx.RUnlock()
	if len(charts.Mempool.Time) == 0 {
		return 0
	}
	return charts.Mempool.Time[len(charts.Mempool.Time)-1]
}

// PropagationHeight is the height of the propagation blocks data, which is the most recent entry
func (charts *ChartData) Height() int32 {
	charts.mtx.RLock()
	defer charts.mtx.RUnlock()
	if len(charts.Propagation.Height) == 0 {
		return 0
	}
	return int32(charts.Propagation.Height[len(charts.Propagation.Height) -1])
}

// PowTime is the time of the latest PoW data appended to the chart
func (charts *ChartData) PowTime() uint64 {
	charts.mtx.RLock()
	defer charts.mtx.RUnlock()
	if len(charts.Pow.Time) == 0 {
		return 0
	}
	return charts.Pow.Time[len(charts.Pow.Time)-1]
}

// VspTime is the time of the latest Vsp data appended to the chart
func (charts *ChartData) VspTime() uint64 {
	charts.mtx.RLock()
	defer charts.mtx.RUnlock()
	if len(charts.Vsp.Time) == 0 {
		return 0
	}
	return charts.Vsp.Time[len(charts.Vsp.Time)-1]
}

// FeesTip is the height of the Fees data.
func (charts *ChartData) FeesTip() int32 {
	charts.mtx.RLock()
	defer charts.mtx.RUnlock()
	return int32(len(charts.Blocks.Fees)) - 1
}

// NewAtomsTip is the height of the NewAtoms data.
func (charts *ChartData) NewAtomsTip() int32 {
	charts.mtx.RLock()
	defer charts.mtx.RUnlock()
	return int32(len(charts.Blocks.NewAtoms)) - 1
}

// TicketPriceTip is the height of the TicketPrice data.
func (charts *ChartData) TicketPriceTip() int32 {
	charts.mtx.RLock()
	defer charts.mtx.RUnlock()
	return int32(len(charts.Windows.TicketPrice))*charts.DiffInterval - 1
}

// PoolSizeTip is the height of the PoolSize data.
func (charts *ChartData) PoolSizeTip() int32 {
	charts.mtx.RLock()
	defer charts.mtx.RUnlock()
	return int32(len(charts.Blocks.PoolSize)) - 1
}

// MissedVotesTip is the height of the MissedVotes data.
func (charts *ChartData) MissedVotesTip() int32 {
	charts.mtx.RLock()
	defer charts.mtx.RUnlock()
	return int32(len(charts.Windows.MissedVotes))*charts.DiffInterval - 1
}

// AddUpdater adds a ChartUpdater to the Updaters slice. Updaters are run
// sequentially during (*ChartData).Update.
func (charts *ChartData) AddUpdater(updater ChartUpdater) {
	charts.updaters = append(charts.updaters, updater)
}

// Update refreshes chart data by calling the ChartUpdaters sequentially. The
// Update is abandoned with a warning if stateID changes while running a Fetcher
// (likely due to a new update starting during a query).
func (charts *ChartData) Update(ctx context.Context) error {
	for _, updater := range charts.updaters {
		stateID := charts.StateID()
		rows, cancel, err := updater.Fetcher(ctx, charts)
		if err != nil {
			err = fmt.Errorf("error encountered during charts %s update. aborting update: %v", updater.Tag, err)
		} else {
			charts.mtx.Lock()
			if !charts.validState(stateID) {
				err = fmt.Errorf("state change detected during charts %s update. aborting update", updater.Tag)
			} else {
				err = updater.Appender(charts, rows)
				if err != nil {
					err = fmt.Errorf("error detected during charts %s append. aborting update: %v", updater.Tag, err)
				}
			}
			charts.mtx.Unlock()
		}
		cancel()
		if err != nil {
			return err
		}
	}

	// Since the charts db data query is complete. Update chart.Days derived dataset.
	if err := charts.Lengthen(); err != nil {
		return fmt.Errorf("(*ChartData).Lengthen failed: %v", err)
	}
	// clear cached
	charts.cache = map[string]*cachedChart{} // TODO: look for a way to only remove updated chart
	return nil
}

// NewChartData constructs a new ChartData.
func NewChartData(ctx context.Context, height uint32, syncSources []string, poolSources []string, vsps []string, chainParams *chaincfg.Params) *ChartData {
	base64Height := int64(height)
	// Allocate datasets for at least as many blocks as in a sdiff window.
	if base64Height < chainParams.StakeDiffWindowSize {
		height = uint32(chainParams.StakeDiffWindowSize)
	}
	genesis := chainParams.GenesisBlock.Header.Timestamp
	// Start datasets at 25% larger than height. This matches golang's default
	// capacity size increase for slice lengths > 1024
	// https://github.com/golang/go/blob/87e48c5afdcf5e01bb2b7f51b7643e8901f4b7f9/src/runtime/slice.go#L100-L112
	size := int(height * 5 / 4)
	days := int(time.Since(genesis)/time.Hour/24)*5/4 + 1 // at least one day
	windows := int(base64Height/chainParams.StakeDiffWindowSize+1) * 5 / 4
	return &ChartData{
		ctx:          ctx,
		DiffInterval: int32(chainParams.StakeDiffWindowSize),
		StartPOS:     int32(chainParams.StakeValidationHeight),
		Mempool:      newMempoolSet(size),
		Propagation:  newPropagationSet(size, syncSources),
		Pow:          newPowSet(poolSources, size),
		Vsp:		  newVspSet(vsps, size),
		Blocks:       newBlockSet(size),
		Windows:      newWindowSet(windows),
		Days:         newDaySet(days),
		cache:        make(map[string]*cachedChart),
		updaters:     make([]ChartUpdater, 0),
		syncSource:   syncSources,
	}
}

// A cacheKey is used to specify cached data of a given type and BinLevel.
func cacheKey(chartID string, bin binLevel, axis axisType) string {
	return chartID + "-" + string(bin) + "-" + string(axis)
}

// Grabs the cacheID associated with the provided BinLevel. Should
// be called under at least a (ChartData).cacheMtx.RLock.
func (charts *ChartData) cacheID(bin binLevel) uint64 {
	//TODO: cacheID should consider all chart types and axis
	switch bin {
	case MempoolBin:
		return charts.Mempool.cacheID
	case PropagationBin:
		return charts.Propagation.cacheID
	case PowBin:
		return charts.Pow.cacheID
	case BlockBin:
		return charts.Blocks.cacheID
	case DayBin:
		return charts.Days.cacheID
	case WindowBin:
		return charts.Windows.cacheID
	}
	return 0
}

// Grab the cached data, if it exists. The cacheID is returned as a convenience.
func (charts *ChartData) getCache(chartID string, bin binLevel, axis axisType) (data *cachedChart, found bool, cacheID uint64) {
	// Ignore zero length since bestHeight would just be set to zero anyway.
	ck := cacheKey(chartID, bin, axis)
	charts.cacheMtx.RLock()
	defer charts.cacheMtx.RUnlock()
	cacheID = charts.cacheID(bin)
	data, found = charts.cache[ck]
	return
}

// Store the chart associated with the provided type and BinLevel.
func (charts *ChartData) cacheChart(chartID string, bin binLevel, axis axisType, data []byte) {
	ck := cacheKey(chartID, bin, axis)
	charts.cacheMtx.Lock()
	defer charts.cacheMtx.Unlock()
	// Using the current best cacheID. This leaves open the small possibility that
	// the cacheID is wrong, if the cacheID has been updated between the
	// ChartMaker and here. This would just cause a one block delay.
	charts.cache[ck] = &cachedChart{
		cacheID: charts.cacheID(bin),
		data:    data,
	}
}

// ChartMaker is a function that accepts a chart type and BinLevel, and returns
// a JSON-encoded chartResponse.
type ChartMaker func(charts *ChartData, bin binLevel, axis axisType, sources ...string) ([]byte, error)

var chartMakers = map[string]ChartMaker{
	MempoolSize:    mempoolSize,
	MempoolTxCount: mempoolTxCount,
	MempoolFees:    mempoolFees,

	BlockPropagation: blockPropagation,
	BlockTimestamp:   blockTimestamp,
	VotesReceiveTime: votesReceiveTime,

	PowChart: powChart,
	
	VSP: makeVspChart,
}

// Chart will return a JSON-encoded chartResponse of the provided type
// and BinLevel.
func (charts *ChartData) Chart(chartID, binString, axisString string, sources ...string) ([]byte, error) {
	bin := ParseBin(binString)
	axis := ParseAxis(axisString)

	sort.Strings(sources)
	completeId := strings.Join(append(sources, chartID), "-")

	cache, found, cacheID := charts.getCache(completeId, bin, axis)
	if found && cache.cacheID == cacheID {
		return cache.data, nil
	}
	maker, hasMaker := chartMakers[chartID]
	if !hasMaker {
		return nil, UnknownChartErr
	}
	// Do the locking here, rather than in encodeXY, so that the helper functions
	// (accumulate, btw) are run under lock.
	charts.mtx.RLock()
	data, err := maker(charts, bin, axis, sources...)
	charts.mtx.RUnlock()
	if err != nil {
		return nil, err
	}
	charts.cacheChart(completeId, bin, axis, data)
	return data, nil
}

// Keys used for the chartResponse data sets.
var responseKeys = []string{"x", "y", "z"}

// Encode the slices. The set lengths are truncated to the smallest of the
// arguments.
func (charts *ChartData) encode(keys []string, sets ...lengther) ([]byte, error) {
	return charts.encodeArr(keys, sets)
}

// Encode the slices. The set lengths are truncated to the smallest of the
// arguments.
func (charts *ChartData) encodeArr(keys []string, sets []lengther) ([]byte, error) {
	if keys == nil {
		keys = responseKeys
	}
	if len(sets) == 0 {
		return nil, fmt.Errorf("encode called without arguments")
	}
	smaller := sets[0].Length()
	for _, x := range sets {
		l := x.Length()
		if l < smaller {
			smaller = l
		}
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
		response[rk] = sets[i].Truncate(smaller)
	}
	return json.Marshal(response)
}

// Each point is translated to the sum of all points before and itself.
func accumulate(data ChartUints) ChartUints {
	d := make(ChartUints, 0, len(data))
	var accumulator uint64
	for _, v := range data {
		accumulator += v
		d = append(d, accumulator)
	}
	return d
}

// Translate the times slice to a slice of differences. The original dataset
// minus the first element is returned for convenience.
func blockTimes(blocks ChartUints) (ChartUints, ChartUints) {
	times := make(ChartUints, 0, len(blocks))
	dataLen := len(blocks)
	if dataLen < 2 {
		// Fewer than two data points is invalid for btw. Return empty data sets so
		// that the JSON encoding will have the correct type.
		return times, times
	}
	last := blocks[0]
	for _, v := range blocks[1:] {
		dif := v - last
		if int64(dif) < 0 {
			dif = 0
		}
		times = append(times, dif)
		last = v
	}
	return blocks[1:], times
}

// Take the average block times on the intervals defined by the ticks argument.
func avgBlockTimes(ticks, blocks ChartUints) (ChartUints, ChartUints) {
	if len(ticks) < 2 {
		// Return empty arrays so that JSON-encoding will have the correct type.
		return ChartUints{}, ChartUints{}
	}
	avgDiffs := make(ChartUints, 0, len(ticks)-1)
	times := make(ChartUints, 0, len(ticks)-1)
	nextIdx := 1
	workingOn := ticks[0]
	next := ticks[nextIdx]
	lastIdx := 0
	for i, t := range blocks {
		if t > next {
			_, pts := blockTimes(blocks[lastIdx:i])
			avgDiffs = append(avgDiffs, pts.Avg(0, len(pts)))
			times = append(times, workingOn)
			nextIdx++
			if nextIdx > len(ticks)-1 {
				break
			}
			lastIdx = i
			next = ticks[nextIdx]
			workingOn = next
		}
	}
	return times, avgDiffs
}

func mempoolSize(charts *ChartData, bin binLevel, axis axisType, _ ...string) ([]byte, error) {
	return charts.encode(nil, charts.Mempool.Time, charts.Mempool.Size)
}

func mempoolTxCount(charts *ChartData, bin binLevel, axis axisType, _ ...string) ([]byte, error) {
	return charts.encode(nil, charts.Mempool.Time, charts.Mempool.TxCount)
}

func mempoolFees(charts *ChartData, bin binLevel, axis axisType, _ ...string) ([]byte, error) {
	return charts.encode(nil, charts.Mempool.Time, charts.Mempool.Fees)
}

func blockPropagation(charts *ChartData, _ binLevel, _ axisType, syncSources ...string) ([]byte, error) {
	var deviations = []lengther{charts.Propagation.Height}
	for _, source := range syncSources {
		deviations = append(deviations, charts.Propagation.BlockPropagation[source])
	}

	return charts.encodeArr(nil, deviations)
}

func blockTimestamp(charts *ChartData, _ binLevel, _ axisType, _ ...string) ([]byte, error) {
	return charts.encode(nil, charts.Propagation.Height, charts.Propagation.BlockDelays)
}

func votesReceiveTime(charts *ChartData, _ binLevel, _ axisType, _ ...string) ([]byte, error) {
	return charts.encode(nil, charts.Propagation.Height, charts.Propagation.VotesReceiveTimeDeviations)
}

func powChart(charts *ChartData, _ binLevel, axis axisType, pools ...string) ([]byte, error) {
	var deviations []ChartNullUints

	for _, pool := range pools {
		switch axis {
		case WorkerAxis:
			deviations = append(deviations, charts.Pow.Workers[pool])
			continue
		case HashrateAxis:
			deviations = append(deviations, charts.Pow.Hashrate[pool])
			continue
		}
	}


	var powChartData = struct {
		CSV     string    `json:"csv"`
		MinDate time.Time `json:"min_date"`
		MaxDate time.Time `json:"max_date"`
	}{
		CSV: fmt.Sprintf("Date,%s\n", strings.Join(pools, ",")),
	}

	if len(charts.Pow.Time) == 0 {
		return json.Marshal(powChartData)
	}

	hasAny := func(index int) bool {
		for _, data := range deviations {
			if index >= len(data){
				continue
			}

			if record := data[index]; record != nil && record.Valid && record.Uint64 > 0 {
				return true
			}
		}
		return false
	}

	for index := range charts.Pow.Time {
		if !hasAny(index) {
			continue
		}

		var lineRecords = []string{time.Unix(int64(charts.Pow.Time[index]), 0).UTC().String()}
		for _, data := range deviations {
			if record := data[index]; record != nil && record.Valid {
				lineRecords = append(lineRecords, strconv.FormatUint(record.Uint64, 10))
			} else {
				lineRecords = append(lineRecords, "NaN")
			}
		}

		powChartData.CSV += fmt.Sprintf("%s\n", strings.Join(lineRecords, ","))
	}

	powChartData.MinDate = time.Unix(int64(charts.Pow.Time[0]), 0).UTC()
	powChartData.MaxDate = time.Unix(int64(charts.Pow.Time[len(charts.Pow.Time) - 1]), 0).UTC()

	return json.Marshal(powChartData)
}

func makeVspChart(charts *ChartData, _ binLevel, axis axisType, vsps ...string) ([]byte, error) {
	var deviations []ChartNullData

	for _, vsp := range vsps {
		switch axis {
		case ImmatureAxis:
			deviations = append(deviations, charts.Vsp.Immature[vsp])
			continue
		case LiveAxis:
			deviations = append(deviations, charts.Vsp.Live[vsp])
			continue
		case VotedAxis:
			deviations = append(deviations, charts.Vsp.Voted[vsp])
			continue
		case MissedAxis:
			deviations = append(deviations, charts.Vsp.Missed[vsp])
			continue
		case PoolFeesAxis:
			deviations = append(deviations, charts.Vsp.PoolFees[vsp])
			continue
		case ProportionLiveAxis:
			deviations = append(deviations, charts.Vsp.ProportionLive[vsp])
			continue
		case ProportionMissedAxis:
			deviations = append(deviations, charts.Vsp.ProportionMissed[vsp])
			continue
		case UserCountAxis:
			deviations = append(deviations, charts.Vsp.UserCount[vsp])
			continue
		case UsersActiveAxis:
			deviations = append(deviations, charts.Vsp.UsersActive[vsp])
			continue
		}
	}

	var vspChartData = struct {
		CSV     string    `json:"csv"`
		MinDate time.Time `json:"min_date"`
		MaxDate time.Time `json:"max_date"`
	}{
		CSV: fmt.Sprintf("Date,%s\n", strings.Join(vsps, ",")),
	}

	if len(charts.Vsp.Time) == 0 {
		return json.Marshal(vspChartData)
	}

	hasAny := func(index int) bool {
		for _, data := range deviations {
			if index >= data.Length() {
				continue
			}

			if data.Valid(index) && !data.IsZero(index) {
				return true
			}
		}
		return false
	}

	var minDate, maxDate uint64
	for index := range charts.Vsp.Time {
		if !hasAny(index) {
			continue
		}

		var lineRecords = []string{time.Unix(int64(charts.Vsp.Time[index]), 0).UTC().String()}
		for _, data := range deviations {
			if data.Valid(index) {
				lineRecords = append(lineRecords, data.String(index))
			} else {
				lineRecords = append(lineRecords, "NaN")
			}
		}

		if minDate == 0 || minDate > charts.Vsp.Time[index] {
			minDate = charts.Vsp.Time[index]
		}

		if maxDate < charts.Vsp.Time[index] {
			maxDate = charts.Vsp.Time[index]
		}
		vspChartData.CSV += fmt.Sprintf("%s\n", strings.Join(lineRecords, ","))
	}

	vspChartData.MinDate = time.Unix(int64(minDate), 0).UTC()
	vspChartData.MaxDate = time.Unix(int64(maxDate), 0).UTC()

	return json.Marshal(vspChartData)
}