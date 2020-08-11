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
	return encoder.Encode(m)
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

	mempoolDefaultSet, err := charts.updateMempoolHeights()
	if err != nil {
		log.Errorf("Unable to update mempool heights, %s", err.Error())
		return err
	}

	mempoolHourSet, err := charts.MempoolSet(HourBin)
	if err != nil && err != UnknownChartErr {
		return err
	}

	// Continue if there at least, an hour of new data
	if dLen := mempoolHourSet.Time.Length(); dLen > 0 && charts.MempoolTip() < mempoolHourSet.Time[dLen-1]+anHour {
		return nil
	}

	hours, hourHeights, hourIntervals := generateHourBin(mempoolDefaultSet.Time, mempoolDefaultSet.Heights)

	mempoolHourSet = mempoolSet{
		bin:     HourBin,
		Time:    hours,
		Heights: hourHeights,
	} // TODO:append the new record alone
	for _, interval := range hourIntervals {
		mempoolHourSet.Size = append(mempoolHourSet.Size, mempoolDefaultSet.Size.Avg(interval[0], interval[1]))
	}
	for _, interval := range hourIntervals {
		mempoolHourSet.TxCount = append(mempoolHourSet.TxCount, mempoolDefaultSet.TxCount.Avg(interval[0], interval[1]))
	}
	for _, interval := range hourIntervals {
		mempoolHourSet.Fee = append(mempoolHourSet.Fee, mempoolDefaultSet.Fee.Avg(interval[0], interval[1]))
	}
	if err := mempoolHourSet.Save(charts); err != nil {
		return err
	}

	// day bin
	mempoolDaySet, err := charts.MempoolSet(DayBin)
	if err != nil && err != UnknownChartErr {
		return err
	}

	// Continue if there at least, a day of new data
	if dLen := mempoolDaySet.Time.Length(); dLen > 0 && charts.MempoolTip() < mempoolDaySet.Time[dLen-1]+aDay {
		return nil
	}

	days, dayHeights, dayIntervals := generateDayBin(mempoolDefaultSet.Time, mempoolDefaultSet.Heights)

	mempoolDaySet = mempoolSet{
		bin:     DayBin,
		Time:    days,
		Heights: dayHeights,
	} // TODO:append the new record alone
	mempoolDaySet.Time = days
	mempoolDaySet.Heights = dayHeights
	for _, interval := range dayIntervals {
		mempoolDaySet.Size = append(mempoolDaySet.Size, mempoolDefaultSet.Size.Avg(interval[0], interval[1]))
	}
	for _, interval := range dayIntervals {
		mempoolDaySet.TxCount = append(mempoolDaySet.TxCount, mempoolDefaultSet.TxCount.Avg(interval[0], interval[1]))
	}
	for _, interval := range dayIntervals {
		mempoolDaySet.Fee = append(mempoolDaySet.Fee, mempoolDefaultSet.Fee.Avg(interval[0], interval[1]))
	}
	if err := mempoolDaySet.Save(charts); err != nil {
		return err
	}

	return nil
}

func (charts *Manager) updateMempoolHeights() (mempoolSet, error) {

	mempoolSet, err := charts.MempoolSet(DefaultBin)
	if err != nil && err != UnknownChartErr {
		return mempoolSet, err
	}
	if mempoolSet.Time.Length() == 0 {
		log.Warn("Mempool height not updated, mempool dates has no value")
		return mempoolSet, nil
	}

	propagationSet, err := charts.PropagationSet(DefaultBin)
	if err != nil {
		log.Warn("Mempool height not updated, propagation has no value")
		return mempoolSet, nil
	}
	if propagationSet.Time.Length() == 0 {
		log.Warn("Mempool height not updated, propagation dates has no value")
		return mempoolSet, nil
	}

	pIndex := 0
	for _, date := range mempoolSet.Time {
		if pIndex+1 < propagationSet.Time.Length() && date >= propagationSet.Time[pIndex+1] {
			pIndex += 1
		}
		mempoolSet.Heights = append(mempoolSet.Heights, propagationSet.Heights[pIndex])
	}

	return mempoolSet, nil
}
