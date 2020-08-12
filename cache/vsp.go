package cache

import (
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
)

type vspSet struct {
	bin binLevel

	Time             ChartUints
	Immature         map[string]chartNullIntsPointer
	Live             map[string]chartNullIntsPointer
	Voted            map[string]chartNullIntsPointer
	Missed           map[string]chartNullIntsPointer
	PoolFees         map[string]chartNullFloatsPointer
	ProportionLive   map[string]chartNullFloatsPointer
	ProportionMissed map[string]chartNullFloatsPointer
	UserCount        map[string]chartNullIntsPointer
	UsersActive      map[string]chartNullIntsPointer
}

func newVspSet(bin binLevel) vspSet {
	return vspSet{
		bin:              bin,
		Immature:         make(map[string]chartNullIntsPointer),
		Live:             make(map[string]chartNullIntsPointer),
		Voted:            make(map[string]chartNullIntsPointer),
		Missed:           make(map[string]chartNullIntsPointer),
		PoolFees:         make(map[string]chartNullFloatsPointer),
		ProportionLive:   make(map[string]chartNullFloatsPointer),
		ProportionMissed: make(map[string]chartNullFloatsPointer),
		UserCount:        make(map[string]chartNullIntsPointer),
		UsersActive:      make(map[string]chartNullIntsPointer),
	}
}

func (m vspSet) Save(cacheManage *Manager) error {
	cacheManage.powMtx.Lock()
	defer cacheManage.powMtx.Unlock()
	filename := filepath.Join(cacheManage.dir, fmt.Sprintf("%s-%s.gob", VSP, m.bin))
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

func (m vspSet) snip(max int) vspSet {
	m.Time = m.Time.snip(max)

	for s, w := range m.Immature {
		m.Immature[s] = w.snip(max)
	}
	for s, w := range m.Live {
		m.Live[s] = w.snip(max)
	}
	for s, w := range m.Voted {
		m.Voted[s] = w.snip(max)
	}
	for s, w := range m.Missed {
		m.Missed[s] = w.snip(max)
	}
	for s, w := range m.PoolFees {
		m.PoolFees[s] = w.snip(max)
	}
	for s, w := range m.ProportionLive {
		m.ProportionLive[s] = w.snip(max)
	}
	for s, w := range m.ProportionMissed {
		m.ProportionMissed[s] = w.snip(max)
	}
	for s, w := range m.UserCount {
		m.UserCount[s] = w.snip(max)
	}
	for s, w := range m.UsersActive {
		m.UsersActive[s] = w.snip(max)
	}

	return m
}

func (charts *Manager) VSPSet(bin binLevel) (data vspSet, err error) {
	data = newVspSet(bin)

	filename := filepath.Join(charts.dir, fmt.Sprintf("%s-%s.gob", VSP, bin))
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

func (charts *Manager) VSPTip() uint64 {
	return charts.vspTip
}

func (charts *Manager) SetVSPTip(time uint64) {
	charts.vspMtx.Lock()
	defer charts.vspMtx.Unlock()
	charts.vspTip = time
}

func (charts *Manager) lengthenVsp() error {
	set, err := charts.VSPSet(DefaultBin)
	if err != nil && err != UnknownChartErr {
		log.Errorf("Error in lengthenVsp, %s", err.Error())
		return err
	}

	hourSet, err := charts.VSPSet(HourBin)
	if err != nil && err != UnknownChartErr {
		return err
	}

	// Do not continue if we don't have at least, an hour of new data
	if dLen := hourSet.Time.Length(); dLen > 0 && charts.PowTip() < hourSet.Time[dLen-1]+anHour {
		return nil
	}

	hours, _, hourIntervals := generateHourBin(set.Time, nil)
	hourSet.Time = append(hourSet.Time, hours...)
	for _, s := range charts.VSPSources {
		var immature = hourSet.Immature[s]
		var live = hourSet.Live[s]
		var missed = hourSet.Missed[s]
		var voted = hourSet.Voted[s]
		var poolFees = hourSet.PoolFees[s]
		var proportionLive = hourSet.ProportionLive[s]
		var proportionMissed = hourSet.ProportionMissed[s]
		var userCount = hourSet.UserCount[s]
		var userActive = hourSet.UsersActive[s]

		for _, interval := range hourIntervals {
			immature.Items = append(immature.Items, set.Immature[s].Avg(interval[0], interval[1]))
			live.Items = append(live.Items, set.Live[s].Avg(interval[0], interval[1]))
			missed.Items = append(missed.Items, set.Missed[s].Avg(interval[0], interval[1]))
			voted.Items = append(voted.Items, set.Voted[s].Avg(interval[0], interval[1]))
			poolFees.Items = append(poolFees.Items, set.PoolFees[s].Avg(interval[0], interval[1]))
			proportionLive.Items = append(proportionLive.Items, set.ProportionLive[s].Avg(interval[0], interval[1]))
			proportionMissed.Items = append(proportionMissed.Items, set.ProportionMissed[s].Avg(interval[0], interval[1]))
			userCount.Items = append(userCount.Items, set.UserCount[s].Avg(interval[0], interval[1]))
			userActive.Items = append(userActive.Items, set.UsersActive[s].Avg(interval[0], interval[1]))
		}
		hourSet.Immature[s] = immature
		hourSet.Live[s] = live
		hourSet.Missed[s] = missed
		hourSet.Voted[s] = missed
		hourSet.PoolFees[s] = poolFees
		hourSet.ProportionLive[s] = proportionLive
		hourSet.ProportionMissed[s] = proportionMissed
		hourSet.UserCount[s] = userCount
		hourSet.UsersActive[s] = userActive
	}

	if err = hourSet.Save(charts); err != nil {
		return err
	}

	// Day set
	daySet, err := charts.VSPSet(DayBin)
	if err != nil && err != UnknownChartErr {
		return err
	}

	// Continue if there at least, an hour of new data
	if dLen := daySet.Time.Length(); dLen > 0 && charts.PowTip() < daySet.Time[dLen-1]+anHour {
		return nil
	}

	days, _, dayIntervals := generateDayBin(set.Time, nil)
	daySet.Time = append(daySet.Time, days...)
	for _, s := range charts.VSPSources {
		var immature = daySet.Immature[s]
		var live = daySet.Live[s]
		var missed = daySet.Missed[s]
		var voted = daySet.Voted[s]
		var poolFees = daySet.PoolFees[s]
		var proportionLive = daySet.ProportionLive[s]
		var proportionMissed = daySet.ProportionMissed[s]
		var userCount = daySet.UserCount[s]
		var userActive = daySet.UsersActive[s]

		for _, interval := range dayIntervals {
			immature.Items = append(immature.Items, set.Immature[s].Avg(interval[0], interval[1]))
			live.Items = append(live.Items, set.Live[s].Avg(interval[0], interval[1]))
			missed.Items = append(missed.Items, set.Missed[s].Avg(interval[0], interval[1]))
			voted.Items = append(voted.Items, set.Voted[s].Avg(interval[0], interval[1]))
			poolFees.Items = append(poolFees.Items, set.PoolFees[s].Avg(interval[0], interval[1]))
			proportionLive.Items = append(proportionLive.Items, set.ProportionLive[s].Avg(interval[0], interval[1]))
			proportionMissed.Items = append(proportionMissed.Items, set.ProportionMissed[s].Avg(interval[0], interval[1]))
			userCount.Items = append(userCount.Items, set.UserCount[s].Avg(interval[0], interval[1]))
			userActive.Items = append(userActive.Items, set.UsersActive[s].Avg(interval[0], interval[1]))
		}
		daySet.Immature[s] = immature
		daySet.Live[s] = live
		daySet.Missed[s] = missed
		daySet.Voted[s] = missed
		daySet.PoolFees[s] = poolFees
		daySet.ProportionLive[s] = proportionLive
		daySet.ProportionMissed[s] = proportionMissed
		daySet.UserCount[s] = userCount
		daySet.UsersActive[s] = userActive
	}

	if err = daySet.Save(charts); err != nil {
		return err
	}

	return nil
}
