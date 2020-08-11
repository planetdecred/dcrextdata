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

func (charts *Manager) VSPTip() uint64 {
	return charts.powTip
}

func (charts *Manager) SetVSPTip(time uint64) {
	charts.powMtx.Lock()
	defer charts.powMtx.Unlock()
	charts.powTip = time
}
