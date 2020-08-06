package cache

import (
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
)

type mempoolSet struct {
	bin binLevel

	Time    ChartUints
	Heights ChartUints
	Size    ChartUints
	TxCount ChartUints
	Fee     ChartFloats
}

func (m mempoolSet) Save(cacheManage *Manager) error {
	cacheManage.cacheMtx.Lock()
	defer cacheManage.cacheMtx.Unlock()
	filename := filepath.Join(cacheManage.dir, fmt.Sprintf("%s-%s.gob", Mempool, m.bin))
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
	return encoder.Encode(&m)
}

func (m mempoolSet) snip(max int) mempoolSet {
	m.Time.snip(max)
	m.Heights.snip(max)
	m.Size.snip(max)
	m.Fee.snip(max)
	m.TxCount.snip(max)

	return m
}

func (charts *Manager) MempoolSet(bin binLevel) (data mempoolSet, err error) {
	data.bin = bin
	filename := filepath.Join(charts.dir, fmt.Sprintf("%s-%s.gob", Mempool, bin))
	if !isFileExists(filename) {
		err = UnknownChartErr
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

func (charts *Manager) MempoolTip() uint64 {
	return charts.mempooTip
}

func (charts *Manager) SetMempoolTip(time uint64) {
	charts.mempoolMtx.Lock()
	defer charts.mempoolMtx.Unlock()
	charts.mempooTip = time
}

func (charts *Manager) normalizeMempoolLength() error {
	set, err := charts.MempoolSet(DefaultBin)
	if err != nil && err != UnknownChartErr {
		return err
	}
	if dLen, err := ValidateLengths(set.Time, set.Fee, set.Size, set.TxCount); err != nil {
		log.Warnf("Mempool length validation failed for %s bin - %s. Check previous warnings", DefaultBin, err.Error())
		set = set.snip(dLen)
		if err = set.Save(charts); err != nil {
			log.Errorf("normalizeMempoolLength - %s", err.Error())
		}
	}

	set, err = charts.MempoolSet(HourBin)
	if err != nil && err != UnknownChartErr {
		return err
	}
	if dLen, err := ValidateLengths(set.Time, set.Fee, set.Size, set.TxCount); err != nil {
		log.Warnf("Mempool length validation failed for %s bin - %s. Check previous warnings", HourBin, err.Error())
		set = set.snip(dLen)
		if err = set.Save(charts); err != nil {
			log.Errorf("normalizeMempoolLength - %s", err.Error())
		}
	}

	set, err = charts.MempoolSet(DayBin)
	if err != nil && err != UnknownChartErr {
		return err
	}
	if dLen, err := ValidateLengths(set.Time, set.Fee, set.Size, set.TxCount); err != nil {
		log.Warnf("Mempool length validation failed for %s bin - %s. Check previous warnings", DayBin, err.Error())
		set = set.snip(dLen)
		if err = set.Save(charts); err != nil {
			log.Errorf("normalizeMempoolLength - %s", err.Error())
		}
	}

	return nil
}

func (charts *Manager) lengthenMempool() error {

	if err := charts.updateMempoolHeights(); err != nil {
		log.Errorf("Unable to update mempool heights, %s", err.Error())
		return err
	}

	mempoolDefaultSet, err := charts.MempoolSet(DefaultBin)
	if err != nil && err != UnknownChartErr {
		return err
	}

	// TODO: Check if there is a day worth of new data
	days, dayHeights, dayIntervals := generateDayBin(mempoolDefaultSet.Time, mempoolDefaultSet.Heights)

	mempoolDaySet, err := charts.MempoolSet(DayBin)
	if err != nil && err != UnknownChartErr {
		return err
	}
	mempoolDaySet.Time = days
	mempoolDaySet.Heights = dayHeights
	for _, interval := range dayIntervals {
		// For each new day, take an appropriate snapshot.
		mempoolDaySet.Size = append(mempoolDaySet.Size, mempoolDefaultSet.Size.Avg(interval[0], interval[1]))
	}
	for _, interval := range dayIntervals {
		// For each new day, take an appropriate snapshot.
		mempoolDaySet.TxCount = append(mempoolDaySet.TxCount, mempoolDefaultSet.TxCount.Avg(interval[0], interval[1]))
	}
	for _, interval := range dayIntervals {
		// For each new day, take an appropriate snapshot.
		mempoolDaySet.Fee = append(mempoolDaySet.Fee, mempoolDefaultSet.Fee.Avg(interval[0], interval[1]))
	}
	if err := mempoolDaySet.Save(charts); err != nil {
		return err
	}

	// TODO: check if there is an hour worth of new data
	hours, hourHeights, hourIntervals := generateHourBin(mempoolDefaultSet.Time, mempoolDefaultSet.Heights)
	mempoolHourSet, err := charts.MempoolSet(HourBin)
	if err != nil && err != UnknownChartErr {
		return err
	}
	mempoolHourSet.Time = hours
	mempoolHourSet.Heights = hourHeights
	for _, interval := range hourIntervals {
		// For each new day, take an appropriate snapshot.
		mempoolHourSet.Size = append(mempoolHourSet.Size, mempoolDefaultSet.Size.Avg(interval[0], interval[1]))
	}
	for _, interval := range hourIntervals {
		// For each new day, take an appropriate snapshot.
		mempoolHourSet.TxCount = append(mempoolHourSet.TxCount, mempoolDefaultSet.TxCount.Avg(interval[0], interval[1]))
	}
	for _, interval := range hourIntervals {
		// For each new day, take an appropriate snapshot.
		mempoolHourSet.Fee = append(mempoolHourSet.Fee, mempoolDefaultSet.Fee.Avg(interval[0], interval[1]))
	}
	if err := mempoolHourSet.Save(charts); err != nil {
		return err
	}

	return nil
}

func (charts *Manager) updateMempoolHeights() error {

	mempoolSet, err := charts.MempoolSet(DefaultBin)
	if err != nil && err != UnknownChartErr {
		return err
	}
	if mempoolSet.Time.Length() == 0 {
		log.Warn("Mempool height not updated, mempool dates has no value")
		return nil
	}

	propagationSet := charts.PropagationSet(DefaultBin)
	if propagationSet.Time.Length() == 0 {
		log.Warn("Mempool height not updated, propagation dates has no value")
		return nil
	}

	pIndex := 0
	for _, date := range mempoolSet.Time {
		if pIndex+1 < propagationSet.Time.Length() && date >= propagationSet.Time[pIndex+1] {
			pIndex += 1
		}
		mempoolSet.Heights = append(mempoolSet.Heights, propagationSet.Heights[pIndex])
	}

	return mempoolSet.Save(charts)
}
