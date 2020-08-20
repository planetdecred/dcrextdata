package cache

import (
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
)

type snapshotSet struct {
	bin binLevel

	Time           ChartUints
	Nodes          ChartUints
	ReachableNodes ChartUints
	Locations      map[string]ChartUints
	LocationDates  ChartUints
	Versions       map[string]ChartUints
	VersionDates   ChartUints
}

func newSnapshotSet(bin binLevel) snapshotSet {
	return snapshotSet{
		bin:       bin,
		Locations: make(map[string]ChartUints),
		Versions:  make(map[string]ChartUints),
	}
}

func (m snapshotSet) Save(cacheManage *Manager) error {
	cacheManage.powMtx.Lock()
	defer cacheManage.powMtx.Unlock()
	filename := filepath.Join(cacheManage.dir, fmt.Sprintf("%s-%s.gob", Snapshot, m.bin))
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

func (m snapshotSet) snip(max int) snapshotSet {
	m.Time = m.Time.snip(max)
	m.Nodes = m.Nodes.snip(max)
	m.ReachableNodes = m.ReachableNodes.snip(max)

	m.LocationDates = m.LocationDates.snip(max)
	for s, l := range m.Locations {
		m.Locations[s] = l.snip(max)
	}

	m.VersionDates = m.VersionDates.snip(max)
	for s, v := range m.Versions {
		m.Versions[s] = v.snip(max)
	}
	return m
}

func (charts *Manager) SnapshotSet(bin binLevel) (data snapshotSet, err error) {
	data = newSnapshotSet(bin)

	filename := filepath.Join(charts.dir, fmt.Sprintf("%s-%s.gob", Snapshot, bin))
	if !isFileExists(filename) {
		err = UnknownChartErr
		return
	}

	var file *os.File
	file, err = os.Open(filename)
	if err != nil {
		log.Errorf("Error in opening exchange cache file - %s", err.Error())
		return
	}

	defer func() {
		file.Close()
	}()

	decoder := gob.NewDecoder(file)
	err = decoder.Decode(&data)
	if err != nil {
		log.Errorf("Error in opening exchange cache file - %s", err.Error())
		return
	}

	return
}

func (charts *Manager) SnapshotTip() uint64 {
	return charts.snapshotTip
}

func (charts *Manager) SetSnapshotTip(time uint64) {
	charts.snapshotMtx.Lock()
	defer charts.snapshotMtx.Unlock()
	charts.snapshotTip = time
}

func (charts *Manager) lengthenSnapshot() error {
	set, err := charts.SnapshotSet(DefaultBin)
	if err != nil && err != UnknownChartErr {
		log.Errorf("Error in lengthenSnapshot, %s", err.Error())
		return err
	}

	hourSet, err := charts.SnapshotSet(HourBin)
	if err != nil && err != UnknownChartErr {
		return err
	}

	// Do not continue if we don't have at least, an hour of new data
	if dLen := hourSet.Time.Length(); dLen > 0 && charts.PowTip() < hourSet.Time[dLen-1]+AnHour {
		return nil
	}

	hours, _, hourIntervals := GenerateHourBin(set.Time, nil)
	hourSet.Time = hours
	for _, interval := range hourIntervals {
		hourSet.Nodes = append(hourSet.Nodes, set.Nodes.Avg(interval[0], interval[1]))
		hourSet.ReachableNodes = append(hourSet.ReachableNodes, set.ReachableNodes.Avg(interval[0], interval[1]))
	}

	hourSet.LocationDates, _, hourIntervals = GenerateHourBin(set.LocationDates, nil)

	for _, interval := range hourIntervals {
		for c := range set.Locations {
			hourSet.Locations[c] = append(hourSet.Locations[c], set.Locations[c].Avg(interval[0], interval[1]))
		}
		for c := range set.Versions {
			hourSet.Versions[c] = append(hourSet.Versions[c], set.Versions[c].Avg(interval[0], interval[1]))
		}
	}

	hourSet.VersionDates, _, hourIntervals = GenerateHourBin(set.VersionDates, nil)
	for _, interval := range hourIntervals {
		for c := range set.Versions {
			hourSet.Versions[c] = append(hourSet.Versions[c], set.Versions[c].Avg(interval[0], interval[1]))
		}
	}
	if err := hourSet.Save(charts); err != nil {
		return err
	}

	// Day set
	daySet, err := charts.SnapshotSet(DayBin)
	if err != nil && err != UnknownChartErr {
		return err
	}

	// Continue if there at least, an hour of new data
	if dLen := daySet.Time.Length(); dLen > 0 && charts.PowTip() < daySet.Time[dLen-1]+AnHour {
		return nil
	}

	days, _, dayIntervals := GenerateDayBin(set.Time, nil)
	daySet.Time = days
	for _, interval := range dayIntervals {
		daySet.Nodes = append(daySet.Nodes, set.Nodes.Avg(interval[0], interval[1]))
		daySet.ReachableNodes = append(daySet.ReachableNodes, set.ReachableNodes.Avg(interval[0], interval[1]))
	}

	daySet.LocationDates, _, dayIntervals = GenerateDayBin(set.LocationDates, nil)

	for _, interval := range dayIntervals {
		for c := range set.Locations {
			daySet.Locations[c] = append(daySet.Locations[c], set.Locations[c].Avg(interval[0], interval[1]))
		}
		for c := range set.Versions {
			daySet.Versions[c] = append(daySet.Versions[c], set.Versions[c].Avg(interval[0], interval[1]))
		}
	}

	daySet.VersionDates, _, dayIntervals = GenerateDayBin(set.VersionDates, nil)
	for _, interval := range dayIntervals {
		for c := range set.Versions {
			daySet.Versions[c] = append(daySet.Versions[c], set.Versions[c].Avg(interval[0], interval[1]))
		}
	}
	if err := daySet.Save(charts); err != nil {
		return err
	}

	return nil
}
