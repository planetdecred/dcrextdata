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

func (charts *Manager) PropagationSet(bin binLevel) (data propagationSet) {
	data.bin = bin
	data.BlockPropagation = make(map[string]ChartFloats)
	filename := filepath.Join(charts.dir, fmt.Sprintf("%s-%s.gob", Propagation, bin))
	if !isFileExists(filename) {
		return
	}

	file, err := os.Open(filename)
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
		return data
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
	set := charts.PropagationSet(DefaultBin)
	if dLen, err := ValidateLengths(set.Time, set.Heights, set.BlockDelay, set.VoteReceiveTimeDeviations); err != nil {
		log.Warnf("Propagation length validation failed for %s bin - %s. Check previous warnings", DefaultBin, err.Error())
		set = set.snip(dLen)
		if err = set.Save(charts); err != nil {
			log.Errorf("normalizePropagationLength - %s", err.Error())
		}
	}

	return nil
}
