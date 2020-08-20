package cache

import (
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
)

type propagationSet struct {
	bin binLevel

	Time                      ChartUints
	Heights                   ChartUints
	BlockDelay                ChartFloats
	VoteReceiveTimeDeviations ChartFloats
	BlockPropagation          map[string]ChartFloats
}

func (m propagationSet) Save(cacheManage *Manager) error {

	cacheManage.propagationMtx.Lock()
	defer cacheManage.propagationMtx.Unlock()

	filename := filepath.Join(cacheManage.dir, fmt.Sprintf("%s-%s.gob", Propagation, m.bin))
	if isFileExists(filename) {
		// delete the old dump files before creating new ones.
		os.RemoveAll(filename)
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}

	defer file.Close()

	encoder := gob.NewEncoder(file)
	return encoder.Encode(m)
}

func (m propagationSet) snip(max int) propagationSet {
	m.Time.snip(max)
	m.Heights.snip(max)
	m.BlockDelay.snip(max)
	m.VoteReceiveTimeDeviations.snip(max)

	return m
}

func (charts *Manager) PropagationSet(bin binLevel) (data propagationSet, err error) {
	data.bin = bin
	data.BlockPropagation = make(map[string]ChartFloats)
	filename := filepath.Join(charts.dir, fmt.Sprintf("%s-%s.gob", Propagation, bin))
	if !isFileExists(filename) {
		return
	}

	var file *os.File
	file, err = os.Open(filename)
	if err != nil {
		log.Errorf("Error in opening mempool cache file - %s", err.Error())
		return
	}

	defer func() {
		file.Close()
	}()

	decoder := gob.NewDecoder(file)
	err = decoder.Decode(&data)
	if err != nil {
		log.Errorf("Error in opening mempool cache file - %s", err.Error())
		return
	}

	return
}

func (charts *Manager) PropagationTip() uint64 {
	return charts.propagationTip
}

func (charts *Manager) SetPropagationTip(time uint64) {
	charts.mempoolMtx.Lock()
	defer charts.mempoolMtx.Unlock()
	charts.propagationTip = time
}

func (charts *Manager) normalizePropagationLength() error {
	set, err := charts.PropagationSet(DefaultBin)
	if err != nil && err != UnknownChartErr {
		return err
	}
	if dLen, err := ValidateLengths(set.Time, set.Heights, set.BlockDelay, set.VoteReceiveTimeDeviations); err != nil {
		log.Warnf("Propagation length validation failed for %s bin - %s. Check previous warnings", DefaultBin, err.Error())
		set = set.snip(dLen)
		if err = set.Save(charts); err != nil {
			log.Errorf("normalizePropagationLength - %s", err.Error())
		}
	}

	return nil
}

func (charts *Manager) lengthenPropagation() error {
	set, err := charts.PropagationSet(DefaultBin)
	if err != nil && err != UnknownChartErr {
		log.Errorf("Error in lengthenPropagation, %s", err.Error())
		return err
	}

	hourSet, err := charts.PropagationSet(HourBin)
	if err != nil && err != UnknownChartErr {
		return err
	}

	// Continue if there at least, an hour of new data
	if dLen := hourSet.Time.Length(); dLen > 0 && charts.MempoolTip() < hourSet.Time[dLen-1]+AnHour {
		return nil
	}

	hours, hourHeights, hourIntervals := GenerateHourBin(set.Time, set.Heights)

	hourSet = propagationSet{
		bin:              HourBin,
		Time:             hours,
		Heights:          hourHeights,
		BlockPropagation: make(map[string]ChartFloats),
	}
	for _, interval := range hourIntervals {
		hourSet.BlockDelay = append(hourSet.BlockDelay, set.BlockDelay.Avg(interval[0], interval[1]))
	}
	for _, interval := range hourIntervals {
		hourSet.VoteReceiveTimeDeviations = append(hourSet.VoteReceiveTimeDeviations, set.VoteReceiveTimeDeviations.Avg(interval[0], interval[1]))
	}
	for _, source := range charts.syncSource {
		for _, interval := range hourIntervals {
			hourSet.BlockPropagation[source] = append(
				hourSet.BlockPropagation[source],
				set.BlockPropagation[source].Avg(interval[0], interval[1]),
			)
		}
	}

	if err := hourSet.Save(charts); err != nil {
		return err
	}

	daySet, err := charts.PropagationSet(DayBin)
	if err != nil && err != UnknownChartErr {
		return err
	}

	// Continue if there at least, an hour of new data
	if dLen := daySet.Time.Length(); dLen > 0 && charts.MempoolTip() < daySet.Time[dLen-1]+AnHour {
		return nil
	}

	days, dayHeights, dayIntervals := GenerateDayBin(set.Time, set.Heights)

	daySet = propagationSet{
		bin:              DayBin,
		Time:             days,
		Heights:          dayHeights,
		BlockPropagation: make(map[string]ChartFloats),
	}
	for _, interval := range dayIntervals {
		daySet.BlockDelay = append(daySet.BlockDelay, set.BlockDelay.Avg(interval[0], interval[1]))
	}
	for _, interval := range dayIntervals {
		daySet.VoteReceiveTimeDeviations = append(daySet.VoteReceiveTimeDeviations, set.VoteReceiveTimeDeviations.Avg(interval[0], interval[1]))
	}
	for _, source := range charts.syncSource {
		for _, interval := range dayIntervals {
			daySet.BlockPropagation[source] = append(
				daySet.BlockPropagation[source],
				set.BlockPropagation[source].Avg(interval[0], interval[1]),
			)
		}
	}

	if err := daySet.Save(charts); err != nil {
		return err
	}
	return nil
}
