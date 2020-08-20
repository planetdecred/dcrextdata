package cache

import (
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
)

type powSet struct {
	bin binLevel

	Time     ChartUints
	Workers  map[string]chartNullIntsPointer
	Hashrate map[string]chartNullIntsPointer
}

func (m powSet) Save(cacheManage *Manager) error {
	cacheManage.powMtx.Lock()
	defer cacheManage.powMtx.Unlock()
	filename := filepath.Join(cacheManage.dir, fmt.Sprintf("%s-%s.gob", PowChart, m.bin))
	if isFileExists(filename) {
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

func (m powSet) snip(max int) powSet {
	m.Time = m.Time.snip(max)

	for s, w := range m.Workers {
		m.Workers[s] = w.snip(max)
	}

	for s, h := range m.Hashrate {
		m.Hashrate[s] = h.snip(max)
	}

	return m
}

func (charts *Manager) PowSet(bin binLevel) (data powSet, err error) {
	data.bin = bin
	data.Workers = make(map[string]chartNullIntsPointer)
	data.Hashrate = make(map[string]chartNullIntsPointer)

	filename := filepath.Join(charts.dir, fmt.Sprintf("%s-%s.gob", PowChart, bin))
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

func (charts *Manager) PowTip() uint64 {
	return charts.powTip
}

func (charts *Manager) SetPowTip(time uint64) {
	charts.powMtx.Lock()
	defer charts.powMtx.Unlock()
	charts.powTip = time
}

func (charts *Manager) AppendPowSet(bin binLevel, time ChartUints, workers map[string]ChartNullUints,
	hashrates map[string]ChartNullUints) error {

	set, err := charts.PowSet(bin)
	if err != nil && err != UnknownChartErr {
		return err
	}
	set.Time = append(set.Time, time...)
	for s, w := range workers {
		var existingWorkers chartNullIntsPointer
		if ew, f := set.Workers[s]; f {
			existingWorkers = ew
		}
		set.Workers[s] = existingWorkers.Append(w)
	}

	for s, w := range hashrates {
		var existingHashrates chartNullIntsPointer
		if ew, f := set.Hashrate[s]; f {
			existingHashrates = ew
		}
		set.Hashrate[s] = existingHashrates.Append(w)
	}

	if err = set.Save(charts); err != nil {
		return err
	}
	return nil
}

func (charts *Manager) lengthenPow() error {
	set, err := charts.PowSet(DefaultBin)
	if err != nil && err != UnknownChartErr {
		log.Errorf("Error in lengthenPow, %s", err.Error())
		return err
	}

	hourSet, err := charts.PowSet(HourBin)
	if err != nil && err != UnknownChartErr {
		return err
	}

	// Do not continue if we don't have at least, an hour of new data
	if dLen := hourSet.Time.Length(); dLen > 0 && charts.PowTip() < hourSet.Time[dLen-1]+AnHour {
		return nil
	}

	hours, _, hourIntervals := GenerateHourBin(set.Time, nil)

	hourWorkers := make(map[string]ChartNullUints)
	hourHashrates := make(map[string]ChartNullUints)

	for s, w := range set.Workers {
		var workers chartNullIntsPointer
		for _, interval := range hourIntervals {
			workers.Items = append(workers.Items, w.Avg(interval[0], interval[1]))
		}
		hourWorkers[s] = workers.toChartNullUint()
	}

	for s, w := range set.Hashrate {
		var hashrates chartNullIntsPointer
		for _, interval := range hourIntervals {
			hashrates.Items = append(hashrates.Items, w.Avg(interval[0], interval[1]))
		}
		hourHashrates[s] = hashrates.toChartNullUint()
	}

	if err = charts.AppendPowSet(HourBin, hours, hourWorkers, hourHashrates); err != nil {
		return err
	}

	// Day set
	daySet, err := charts.PowSet(DayBin)
	if err != nil && err != UnknownChartErr {
		return err
	}

	// Continue if there at least, an hour of new data
	if dLen := daySet.Time.Length(); dLen > 0 && charts.PowTip() < daySet.Time[dLen-1]+AnHour {
		return nil
	}

	days, _, dayIntervals := GenerateDayBin(set.Time, nil)

	dayWorkers := make(map[string]ChartNullUints)
	dayHashrates := make(map[string]ChartNullUints)

	for s, w := range set.Workers {
		var workers chartNullIntsPointer
		for _, interval := range dayIntervals {
			workers.Items = append(workers.Items, w.Avg(interval[0], interval[1]))
		}
		dayWorkers[s] = workers.toChartNullUint()
	}

	for s, w := range set.Hashrate {
		var workers chartNullIntsPointer
		for _, interval := range dayIntervals {
			workers.Items = append(workers.Items, w.Avg(interval[0], interval[1]))
		}
		dayHashrates[s] = workers.toChartNullUint()
	}

	if err = charts.AppendPowSet(DayBin, days, dayWorkers, dayHashrates); err != nil {
		return err
	}
	return nil
}
