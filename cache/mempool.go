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
	cacheManage.cacheMtx.Lock()
	defer cacheManage.cacheMtx.Unlock()
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

func (charts *Manager) MempoolSet(bin binLevel) (data mempoolSet) {
	data.bin = bin
	filename := filepath.Join(charts.dir, fmt.Sprintf("%s-%s.gob", Mempool, bin))
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

func (charts *Manager) MempoolTip() uint64 {
	return charts.mempooTip
}

func (charts *Manager) SetMempoolTip(time uint64) {
	charts.mempoolMtx.Lock()
	defer charts.mempoolMtx.Unlock()
	charts.mempooTip = time
}

func isFileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return !os.IsNotExist(err)
}
