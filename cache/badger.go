package cache

import (
	"bytes"
	"encoding/gob"
	"fmt"

	"github.com/dgraph-io/badger"
	"github.com/friendsofgo/errors"
)

const (
	versionKey = "CURRENT_VERSION"
	// aDay defines the number of seconds in a day.
	aDay   = 86400
	anHour = aDay / 24
)

// chart version
func (charts ChartData) SaveVersion() error {
	return charts.SaveVal(cacheVersion, versionKey)
}

func (charts ChartData) getVersion() (semver Semver, err error) {
	err = charts.ReadVal(versionKey, &semver)
	if err == badger.ErrKeyNotFound {
		semver, err = cacheVersion, nil
	}
	return
}

func (charts ChartData) SaveVal(val interface{}, key string) error {
	var b bytes.Buffer
	e := gob.NewEncoder(&b)
	if err := e.Encode(val); err != nil {
		return err
	}
	err := charts.db.Update(func(txn *badger.Txn) error {
		err := txn.Set([]byte(key), b.Bytes())
		return err
	})
	return err
}

func (charts ChartData) SaveValTx(val interface{}, key string, txn *badger.Txn) error {
	var b bytes.Buffer
	e := gob.NewEncoder(&b)
	if err := e.Encode(val); err != nil {
		return err
	}
	return txn.Set([]byte(key), b.Bytes())
}

func (charts ChartData) ClearVLog() {
again:
	verr := charts.db.RunValueLogGC(0.7)
	if verr == nil {
		goto again
	}
}

func (charts ChartData) ReadVal(key string, result interface{}) error {
	return charts.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			d := gob.NewDecoder(bytes.NewReader(val))
			if err := d.Decode(result); err != nil {
				return err
			}
			return nil
		})
	})
}

func (charts ChartData) ReadValTx(key string, result interface{}, txn *badger.Txn) error {
	item, err := txn.Get([]byte(key))
	if err != nil {
		return err
	}

	return item.Value(func(val []byte) error {
		d := gob.NewDecoder(bytes.NewReader(val))
		if err := d.Decode(result); err != nil {
			return err
		}
		return nil
	})
}

// Appenders

func (charts ChartData) AppendChartUintsAxis(key string, set ChartUints) error {
	var data ChartUints
	err := charts.ReadVal(key, &data)
	if err != nil {
		if err != badger.ErrKeyNotFound {
			return err
		}
	}
	data = append(data, set...)
	return charts.SaveVal(data, key)
}

func (charts ChartData) AppendChartNullUintsAxis(key string, set ChartNullUints) error {
	var data chartNullIntsPointer
	err := charts.ReadVal(key, &data)
	if err != nil {
		if err != badger.ErrKeyNotFound {
			return err
		}
	}
	data = data.Append(set)
	return charts.SaveVal(data, key)
}

func (charts ChartData) AppendChartFloatsAxis(key string, set ChartFloats) error {
	var data ChartFloats
	err := charts.ReadVal(key, &data)
	if err != nil {
		if err != badger.ErrKeyNotFound {
			return err
		}
	}
	data = append(data, set...)
	return charts.SaveVal(data, key)
}

func (charts ChartData) AppendChartNullFloatsAxis(key string, set ChartNullFloats) error {
	var data chartNullFloatsPointer
	err := charts.ReadVal(key, &data)
	if err != nil {
		if err != badger.ErrKeyNotFound {
			return err
		}
	}
	data = data.Append(set)
	return charts.SaveVal(data, key)
}

func (charts ChartData) NormalizeLength(tags ...string) error {
	txn := charts.db.NewTransaction(true)
	defer txn.Discard()
	
	for _, chartID := range tags {
		if cerr := charts.normalizeLength(chartID, txn); cerr != nil {
			return errors.Wrap(cerr, "Normalize failed for " + chartID)
		}
	}
	if err := txn.Commit(); err != nil {
		return err
	}
	return nil
}

// length correction
func (charts ChartData) normalizeLength(chartID string, txn *badger.Txn) error {
	// TODO: use transaction
	switch chartID {
	case Mempool:
		return charts.normalizeMempoolLength(txn)

	case Propagation:
		return charts.normalizePropagationLength(txn)

	case PowChart:
		return charts.normalizePowChartLength(txn)

	case VSP:
		return charts.normalizeVSPLength(txn)

	case Exchange:
		return charts.normalizeExchangeLength(txn)

	case Snapshot:
		return charts.normalizeSnapshotLength(txn)
	case Community:
		return nil

	}

	return nil
}

func (charts ChartData) normalizeMempoolLength(txn *badger.Txn) error {
	var firstLen, shortest, longest int
	key := Mempool + "-" + string(TimeAxis)
	firstLen, err := charts.chartUintsLength(key, txn)
	if err != nil {
		return err
	}
	shortest, longest = firstLen, firstLen

	key = Mempool + "-" + string(MempoolFees)
	dLen, err := charts.chartFloatsLength(key, txn)
	if err != nil {
		return err
	}
	if dLen != firstLen {
		log.Warnf("charts.normalizeMempoolLength: dataset for %s axis has mismatched length %d != %d", key, dLen, firstLen)
		if dLen < shortest {
			shortest = dLen
		} else if dLen > longest {
			longest = dLen
		}
	}

	key = Mempool + "-" + string(MempoolSize)
	dLen, err = charts.chartUintsLength(key, txn)
	if err != nil {
		return err
	}
	if dLen != firstLen {
		log.Warnf("charts.normalizeMempoolLength: dataset for %s axis has mismatched length %d != %d", key, dLen, firstLen)
		if dLen < shortest {
			shortest = dLen
		} else if dLen > longest {
			longest = dLen
		}
	}

	key = Mempool + "-" + string(MempoolTxCount)
	dLen, err = charts.chartUintsLength(key, txn)
	if err != nil {
		return err
	}
	if dLen != firstLen {
		log.Warnf("charts.normalizeMempoolLength: dataset for %s axis has mismatched length %d != %d", key, dLen, firstLen)
		if dLen < shortest {
			shortest = dLen
		} else if dLen > longest {
			longest = dLen
		}
	}

	if longest != shortest {
		return charts.snipMempool(shortest, txn)
	}
	return nil
}

func (charts ChartData) normalizePropagationLength(txn *badger.Txn) error {
	var firstLen, shortest, longest int
	key := Propagation + "-" + string(HeightAxis)
	firstLen, err := charts.chartUintsLength(key, txn)
	if err != nil {
		return err
	}
	shortest, longest = firstLen, firstLen

	key = Propagation + "-" + string(BlockTimestamp)
	dLen, err := charts.chartFloatsLength(key, txn)
	if err != nil {
		return err
	}
	if dLen != firstLen {
		log.Warnf("charts.normalizePropagationLength: dataset for %s axis has mismatched length %d != %d", key, dLen, firstLen)
		if dLen < shortest {
			shortest = dLen
		} else if dLen > longest {
			longest = dLen
		}
	}

	key = Propagation + "-" + string(VotesReceiveTime)
	dLen, err = charts.chartFloatsLength(key, txn)
	if err != nil {
		return err
	}
	if dLen != firstLen {
		log.Warnf("charts.normalizePropagationLength: dataset for %s axis has mismatched length %d != %d", key, dLen, firstLen)
		if dLen < shortest {
			shortest = dLen
		} else if dLen > longest {
			longest = dLen
		}
	}

	if longest != shortest {
		if err = charts.snipPropagationChart(shortest, BlockTimestamp, txn); err != nil {
			log.Warn(err)
		}
	}

	// sync source data can alway have a mis-match.
	// TODO: resolve peculiar issue
	for _, source := range charts.syncSource {
		key = Propagation + "-" + string(BlockPropagation) + "-" + source
		dLen, err = charts.chartFloatsLength(key, txn)
		if err != nil {
			return err
		}
		if dLen != firstLen {
			log.Warnf("charts.normalizePropagationLength: dataset for %s axis has mismatched length %d != %d", key, dLen, firstLen)
			if dLen < shortest {
				shortest = dLen
			} else if dLen > longest {
				longest = dLen
			}
		}
	}
	if longest != shortest {
		if err = charts.snipPropagationChart(shortest, BlockPropagation, txn); err != nil {
			log.Warn(err)
		}
	}

	return nil
}

func (charts ChartData) normalizePowChartLength(txn *badger.Txn) error {
	var firstLen, shortest, longest int
	key := PowChart + "-" + string(TimeAxis)
	firstLen, err := charts.chartUintsLength(key, txn)
	if err != nil {
		return err
	}
	shortest, longest = firstLen, firstLen

	for _, source := range charts.PowSources {
		key = PowChart + "-" + string(WorkerAxis) + "-" + source
		dLen, err := charts.chartNullUintsLength(key, txn)
		if err != nil {
			return err
		}
		if dLen != firstLen {
			log.Warnf("charts.normalizePowChartLength: dataset for %s axis has mismatched length %d != %d", key, dLen, firstLen)
			if dLen < shortest {
				shortest = dLen
			} else if dLen > longest {
				longest = dLen
			}
		}

		key = PowChart + "-" + string(HashrateAxis) + "-" + source
		dLen, err = charts.chartNullUintsLength(key, txn)
		if err != nil {
			return err
		}
		if dLen != firstLen {
			log.Warnf("charts.normalizePowChartLength: dataset for %s axis has mismatched length %d != %d", key, dLen, firstLen)
			if dLen < shortest {
				shortest = dLen
			} else if dLen > longest {
				longest = dLen
			}
		}
	}

	if longest != shortest {
		return charts.snipPowChart(shortest, txn)
	}
	return nil
}

func (charts ChartData) normalizeVSPLength(txn *badger.Txn) error {
	var firstLen, shortest, longest int
	key := VSP + "-" + string(TimeAxis)
	firstLen, err := charts.chartUintsLength(key, txn)
	if err != nil {
		return err
	}
	shortest, longest = firstLen, firstLen

	for _, source := range charts.VSPSources {
		// ImmatureAxis
		key = VSP + "-" + string(ImmatureAxis) + "-" + source
		dLen, err := charts.chartNullUintsLength(key, txn)
		if err != nil {
			return err
		}
		if dLen != firstLen {
			log.Warnf("charts.normalizeVSPLength: dataset for %s axis has mismatched length %d != %d", key, dLen, firstLen)
			if dLen < shortest {
				shortest = dLen
			} else if dLen > longest {
				longest = dLen
			}
		}

		// LiveAxis
		key = VSP + "-" + string(LiveAxis) + "-" + source
		dLen, err = charts.chartNullUintsLength(key, txn)
		if err != nil {
			return err
		}
		if dLen != firstLen {
			log.Warnf("charts.normalizeVSPLength: dataset for %s axis has mismatched length %d != %d", key, dLen, firstLen)
			if dLen < shortest {
				shortest = dLen
			} else if dLen > longest {
				longest = dLen
			}
		}

		// VotedAxis
		key = VSP + "-" + string(VotedAxis) + "-" + source
		dLen, err = charts.chartNullUintsLength(key, txn)
		if err != nil {
			return err
		}
		if dLen != firstLen {
			log.Warnf("charts.normalizeVSPLength: dataset for %s axis has mismatched length %d != %d", key, dLen, firstLen)
			if dLen < shortest {
				shortest = dLen
			} else if dLen > longest {
				longest = dLen
			}
		}

		// MissedAxis
		key = VSP + "-" + string(MissedAxis) + "-" + source
		dLen, err = charts.chartNullUintsLength(key, txn)
		if err != nil {
			return err
		}
		if dLen != firstLen {
			log.Warnf("charts.normalizeVSPLength: dataset for %s axis has mismatched length %d != %d", key, dLen, firstLen)
			if dLen < shortest {
				shortest = dLen
			} else if dLen > longest {
				longest = dLen
			}
		}

		// PoolFeesAxis
		key = VSP + "-" + string(PoolFeesAxis) + "-" + source
		dLen, err = charts.chartNullFloatsLength(key, txn)
		if err != nil {
			return err
		}
		if dLen != firstLen {
			log.Warnf("charts.normalizeVSPLength: dataset for %s axis has mismatched length %d != %d", key, dLen, firstLen)
			if dLen < shortest {
				shortest = dLen
			} else if dLen > longest {
				longest = dLen
			}
		}

		// ProportionLiveAxis
		key = VSP + "-" + string(ProportionLiveAxis) + "-" + source
		dLen, err = charts.chartNullFloatsLength(key, txn)
		if err != nil {
			return err
		}
		if dLen != firstLen {
			log.Warnf("charts.normalizeVSPLength: dataset for %s axis has mismatched length %d != %d", key, dLen, firstLen)
			if dLen < shortest {
				shortest = dLen
			} else if dLen > longest {
				longest = dLen
			}
		}

		// ProportionMissedAxis
		key = VSP + "-" + string(ProportionMissedAxis) + "-" + source
		dLen, err = charts.chartNullFloatsLength(key, txn)
		if err != nil {
			return err
		}
		if dLen != firstLen {
			log.Warnf("charts.normalizeVSPLength: dataset for %s axis has mismatched length %d != %d", key, dLen, firstLen)
			if dLen < shortest {
				shortest = dLen
			} else if dLen > longest {
				longest = dLen
			}
		}

		// UsersActiveAxis
		key = VSP + "-" + string(UsersActiveAxis) + "-" + source
		dLen, err = charts.chartNullUintsLength(key, txn)
		if err != nil {
			return err
		}
		if dLen != firstLen {
			log.Warnf("charts.normalizeVSPLength: dataset for %s axis has mismatched length %d != %d", key, dLen, firstLen)
			if dLen < shortest {
				shortest = dLen
			} else if dLen > longest {
				longest = dLen
			}
		}

		// UserCountAxis
		key = VSP + "-" + string(UserCountAxis) + "-" + source
		dLen, err = charts.chartNullUintsLength(key, txn)
		if err != nil {
			return err
		}
		if dLen != firstLen {
			log.Warnf("charts.normalizeVSPLength: dataset for %s axis has mismatched length %d != %d", key, dLen, firstLen)
			if dLen < shortest {
				shortest = dLen
			} else if dLen > longest {
				longest = dLen
			}
		}

	}

	if longest != shortest {
		return charts.snipPowChart(shortest, txn)
	}
	return nil
}

func (charts ChartData) normalizeExchangeLength(txn *badger.Txn) error {

	var shortest, longest int
	for _, exchangeKeys := range charts.ExchangeKeys {
		key := exchangeKeys + "-" + string(TimeAxis)
		firstLen, err := charts.chartUintsLength(key, txn)
		if err != nil {
			return err
		}
		shortest, longest = firstLen, firstLen

		// ExchangeOpenAxis
		key = exchangeKeys + "-" + string(ExchangeOpenAxis)
		dLen, err := charts.chartFloatsLength(key, txn)
		if err != nil {
			return err
		}
		if dLen != firstLen {
			log.Warnf("charts.normalizeExchangeLength: dataset for %s axis has mismatched length %d != %d", key, dLen, firstLen)
			if dLen < shortest {
				shortest = dLen
			} else if dLen > longest {
				longest = dLen
			}
		}

		// ExchangeCloseAxis
		key = exchangeKeys + "-" + string(ExchangeCloseAxis)
		dLen, err = charts.chartFloatsLength(key, txn)
		if err != nil {
			return err
		}
		if dLen != firstLen {
			log.Warnf("charts.normalizeExchangeLength: dataset for %s axis has mismatched length %d != %d", key, dLen, firstLen)
			if dLen < shortest {
				shortest = dLen
			} else if dLen > longest {
				longest = dLen
			}
		}

		// ExchangeHighAxis
		key = exchangeKeys + "-" + string(ExchangeHighAxis)
		dLen, err = charts.chartFloatsLength(key, txn)
		if err != nil {
			return err
		}
		if dLen != firstLen {
			log.Warnf("charts.normalizeExchangeLength: dataset for %s axis has mismatched length %d != %d", key, dLen, firstLen)
			if dLen < shortest {
				shortest = dLen
			} else if dLen > longest {
				longest = dLen
			}
		}

		// ExchangeLowAxis
		key = exchangeKeys + "-" + string(ExchangeLowAxis)
		dLen, err = charts.chartFloatsLength(key, txn)
		if err != nil {
			return err
		}
		if dLen != firstLen {
			log.Warnf("charts.normalizeExchangeLength: dataset for %s axis has mismatched length %d != %d", key, dLen, firstLen)
			if dLen < shortest {
				shortest = dLen
			} else if dLen > longest {
				longest = dLen
			}
		}
	}

	if longest != shortest {
		return charts.snipPowChart(shortest, txn)
	}
	return nil
}

func (charts ChartData) normalizeSnapshotLength(txn *badger.Txn) error {
	var firstLen, shortest, longest int
	key := fmt.Sprintf("%s-%s", Snapshot, TimeAxis)
	firstLen, err := charts.chartUintsLength(key, txn)
	if err != nil {
		return err
	}
	shortest, longest = firstLen, firstLen

	// SnapshotNodes
	key = fmt.Sprintf("%s-%s", Snapshot, SnapshotNodes)
	dLen, err := charts.chartUintsLength(key, txn)
	if err != nil {
		return err
	}
	if dLen != firstLen {
		log.Warnf("charts.normalizeSnapshotLength: dataset for %s axis has mismatched length %d != %d", key, dLen, firstLen)
		if dLen < shortest {
			shortest = dLen
		} else if dLen > longest {
			longest = dLen
		}
	}

	// SnapshotReachableNodes
	key = fmt.Sprintf("%s-%s", Snapshot, SnapshotReachableNodes)
	dLen, err = charts.chartUintsLength(key, txn)
	if err != nil {
		return err
	}
	if dLen != firstLen {
		log.Warnf("charts.normalizeSnapshotLength: dataset for %s axis has mismatched length %d != %d", key, dLen, firstLen)
		if dLen < shortest {
			shortest = dLen
		} else if dLen > longest {
			longest = dLen
		}
	}
	if longest != shortest {
		if err = charts.snipSnapshotChart(shortest, SnapshotNodes, txn); err != nil {
			log.Warnf("SnapshotNodeVersions fail at %s, %s", SnapshotNodes, err.Error())
		}
	}

	// SnapshotLocations
	key = fmt.Sprintf("%s-%s-%s", Snapshot, SnapshotLocations, TimeAxis)
	firstLen, err = charts.chartUintsLength(key, txn)
	if err != nil {
		return err
	}
	shortest, longest = firstLen, firstLen
	for _, source := range charts.NodeLocations {
		key = fmt.Sprintf("%s-%s-%s", Snapshot, SnapshotLocations, source)
		dLen, err := charts.chartUintsLength(key, txn)
		if err != nil {
			return err
		}
		if dLen != firstLen {
			log.Warnf("charts.normalizeSnapshotLength: dataset for %s axis has mismatched length %d != %d", key, dLen, firstLen)
			if dLen < shortest {
				shortest = dLen
			} else if dLen > longest {
				longest = dLen
			}
		}
	}
	if longest != shortest {
		if err = charts.snipSnapshotChart(shortest, SnapshotLocations, txn); err != nil {
			log.Warnf("SnapshotNodeVersions fail at %s, %s", SnapshotLocations, err.Error())
		}
	}

	key = fmt.Sprintf("%s-%s-%s", Snapshot, SnapshotNodeVersions, TimeAxis)
	firstLen, err = charts.chartUintsLength(key, txn)
	if err != nil {
		return err
	}
	shortest, longest = firstLen, firstLen
	for _, source := range charts.NodeVersion {
		// SnapshotNodeVersions
		key = fmt.Sprintf("%s-%s-%s", Snapshot, SnapshotNodeVersions, source)
		dLen, err := charts.chartUintsLength(key, txn)
		if err != nil {
			return err
		}
		if dLen != firstLen {
			log.Warnf("charts.normalizeSnapshotLength: dataset for %s axis has mismatched length %d != %d", key, dLen, firstLen)
			if dLen < shortest {
				shortest = dLen
			} else if dLen > longest {
				longest = dLen
			}
		}
	}

	if longest != shortest {
		if err = charts.snipSnapshotChart(shortest, SnapshotNodeVersions, txn); err != nil {
			log.Warnf("SnapshotNodeVersions fail at %s, %s", SnapshotNodeVersions, err.Error())
		}
	}
	return nil
}

func (charts ChartData) chartUintsLength(key string, txn *badger.Txn) (int, error) {
	var data ChartUints
	err := charts.ReadValTx(key, &data, txn)
	if err != nil {
		if err != badger.ErrKeyNotFound {
			return 0, err
		}
	}
	return data.Length(), nil
}

func (charts ChartData) chartFloatsLength(key string, txn *badger.Txn) (int, error) {
	var data ChartFloats
	err := charts.ReadValTx(key, &data, txn)
	if err != nil {
		if err != badger.ErrKeyNotFound {
			return 0, err
		}
	}
	return data.Length(), nil
}

func (charts ChartData) chartNullUintsLength(key string, txn *badger.Txn) (int, error) {
	var data chartNullIntsPointer
	err := charts.ReadValTx(key, &data, txn)
	if err != nil {
		if err != badger.ErrKeyNotFound {
			return 0, err
		}
	}
	return data.Length(), nil
}

func (charts ChartData) chartNullFloatsLength(key string, txn *badger.Txn) (int, error) {
	var data chartNullFloatsPointer
	err := charts.ReadValTx(key, &data, txn)
	if err != nil {
		if err != badger.ErrKeyNotFound {
			return 0, err
		}
	}
	return data.Length(), nil
}

func (charts ChartData) snipMempool(length int, txn *badger.Txn) error {
	axis := []axisType{
		TimeAxis, MempoolSize, MempoolTxCount,
	}
	for _, a := range axis {
		key := Mempool + "-" + string(a)
		if err := charts.snipChartUintsAxis(key, length, txn); err != nil {
			if err != badger.ErrKeyNotFound {
				return err
			}
		}
	}
	key := Mempool + "-" + string(MempoolFees)
	if err := charts.snipChartFloatsAxis(key, length, txn); err != nil {
		if err != badger.ErrKeyNotFound {
			return err
		}
	}
	return nil
}

func (charts ChartData) snipPropagationChart(length int, axis axisType, txn *badger.Txn) error {
	key := Propagation + "-" + string(HeightAxis)
	if err := charts.snipChartUintsAxis(key, length, txn); err != nil {
		if err != badger.ErrKeyNotFound {
			return err
		}
	}
	var keys []string
	switch axis {
	case BlockPropagation:
		for _, source := range charts.syncSource {
			keys = append(keys, Propagation+"-"+string(BlockPropagation)+"-"+source)
		}
		break
	case BlockTimestamp:
		keys = []string{
			Propagation + "-" + string(BlockTimestamp),
			Propagation + "-" + string(VotesReceiveTime),
		}
		break
	}

	for _, key := range keys {
		if err := charts.snipChartFloatsAxis(key, length, txn); err != nil {
			if err != badger.ErrKeyNotFound {
				return err
			}
		}
	}
	return nil
}

func (charts ChartData) snipPowChart(length int, txn *badger.Txn) error {
	key := PowChart + "-" + string(TimeAxis)
	if err := charts.snipChartUintsAxis(key, length, txn); err != nil {
		if err != badger.ErrKeyNotFound {
			return err
		}
	}
	var keys []string
	for _, pool := range charts.PowSources {
		keys = append(keys, PowChart+"-"+string(WorkerAxis)+"-"+pool)
		keys = append(keys, PowChart+"-"+string(HashrateAxis)+"-"+pool)
	}

	for _, key := range keys {
		if err := charts.snipChartNullUintsAxis(key, length, txn); err != nil {
			if err != badger.ErrKeyNotFound {
				return err
			}
		}
	}
	return nil
}

func (charts ChartData) snipVspChart(length int, txn *badger.Txn) error {
	key := VSP + "-" + string(TimeAxis)
	if err := charts.snipChartUintsAxis(key, length, txn); err != nil {
		if err != badger.ErrKeyNotFound {
			return err
		}
	}
	var keys []string
	for _, vspSource := range charts.VSPSources {
		keys = append(keys,
			VSP+"-"+string(ImmatureAxis)+"-"+vspSource,
			VSP+"-"+string(LiveAxis)+"-"+vspSource,
			VSP+"-"+string(VotedAxis)+"-"+vspSource,
			VSP+"-"+string(MissedAxis)+"-"+vspSource,
			VSP+"-"+string(UsersActiveAxis)+"-"+vspSource,
			VSP+"-"+string(UserCountAxis)+"-"+vspSource,
		)
	}

	for _, key := range keys {
		if err := charts.snipChartNullUintsAxis(key, length, txn); err != nil {
			if err != badger.ErrKeyNotFound {
				return err
			}
		}
	}

	keys = []string{}
	for _, vspSource := range charts.VSPSources {
		keys = append(keys,
			VSP+"-"+string(PoolFeesAxis)+"-"+vspSource,
			VSP+"-"+string(ProportionLiveAxis)+"-"+vspSource,
			VSP+"-"+string(ProportionMissedAxis)+"-"+vspSource,
		)
	}

	for _, key := range keys {
		if err := charts.snipChartNullFloatsAxis(key, length, txn); err != nil {
			if err != badger.ErrKeyNotFound {
				return err
			}
		}
	}

	return nil
}

func (charts ChartData) snipExchangeChart(length int, txn *badger.Txn) error {
	for _, exchangeKey := range charts.ExchangeKeys {
		key := exchangeKey + "-" + string(TimeAxis)
		if err := charts.snipChartUintsAxis(key, length, txn); err != nil {
			if err != badger.ErrKeyNotFound {
				return err
			}
		}
		var keys = []string{
			key + "-" + string(ExchangeOpenAxis),
			key + "-" + string(ExchangeCloseAxis),
			key + "-" + string(ExchangeHighAxis),
			key + "-" + string(ExchangeLowAxis),
		}
		for _, key := range keys {
			if err := charts.snipChartFloatsAxis(key, length, txn); err != nil {
				if err != badger.ErrKeyNotFound {
					return err
				}
			}
		}
	}
	return nil
}

func (charts ChartData) snipSnapshotChart(length int, axis axisType, txn *badger.Txn) error {
	var keys []string
	switch axis {
	case SnapshotNodes:
		keys = []string{
			Snapshot + "-" + string(TimeAxis),
			Snapshot + "-" + string(SnapshotNodes),
			Snapshot + "-" + string(SnapshotReachableNodes),
		}
		break
	case SnapshotLocations:
		keys = append(keys, Snapshot+"-"+string(SnapshotLocations)+"-"+string(TimeAxis))
		for _, country := range charts.NodeLocations {
			keys = append(keys, Snapshot+"-"+string(SnapshotLocations)+"-"+country)
		}
		break
	case SnapshotNodeVersions:
		keys = append(keys, Snapshot+"-"+string(SnapshotNodeVersions)+"-"+string(TimeAxis))
		for _, userAgent := range charts.NodeVersion {
			keys = append(keys, Snapshot+"-"+string(SnapshotNodeVersions)+"-"+userAgent)
		}
		break
	}

	for _, key := range keys {
		if err := charts.snipChartUintsAxis(key, length, txn); err != nil {
			if err != badger.ErrKeyNotFound {
				return err
			}
		}
	}
	return nil
}

func (charts ChartData) snipCommunityChart(length int, txn *badger.Txn) error {
	return nil
}

func (charts ChartData) snipChartUintsAxis(key string, length int, txn *badger.Txn) error {
	var data ChartUints
	err := charts.ReadValTx(key, &data, txn)
	if err != nil {
		if err != badger.ErrKeyNotFound {
			return err
		}
	}
	data = data.snip(length)
	return charts.SaveValTx(data, key, txn)
}

func (charts ChartData) snipChartNullUintsAxis(key string, length int, txn *badger.Txn) error {
	var data chartNullIntsPointer
	err := charts.ReadValTx(key, &data, txn)
	if err != nil {
		if err != badger.ErrKeyNotFound {
			return err
		}
	}
	data = data.snip(length)
	return charts.SaveValTx(data, key, txn)
}

func (charts ChartData) snipChartNullFloatsAxis(key string, length int, txn *badger.Txn) error {
	var data chartNullFloatsPointer
	err := charts.ReadValTx(key, &data, txn)
	if err != nil {
		if err != badger.ErrKeyNotFound {
			return err
		}
	}
	data = data.snip(length)
	return charts.SaveValTx(data, key, txn)
}

func (charts ChartData) snipChartFloatsAxis(key string, length int, txn *badger.Txn) error {
	var data ChartFloats
	err := charts.ReadValTx(key, &data, txn)
	if err != nil {
		if err != badger.ErrKeyNotFound {
			return err
		}
	}
	data = data.snip(length)
	return charts.SaveValTx(data, key, txn)
}

func (charts ChartData) MempoolTimeTip() uint64 {
	var dates ChartUints
	err := charts.ReadVal(Mempool+"-"+string(TimeAxis), &dates)
	if err != nil {
		return 0
	}
	if len(dates) == 0 {
		return 0
	}
	return dates[dates.Length()-1]
}

func (charts ChartData) lengthenMempool() error {
	txn := charts.db.NewTransaction(true)
	defer txn.Discard()

	dayIntervals, hourIntervals, err := charts.lengthenTime(fmt.Sprintf("%s-%s", Mempool, TimeAxis), txn)
	if err != nil {
		return err
	}

	keys := []string{
		fmt.Sprintf("%s-%s", Mempool, MempoolSize),
		fmt.Sprintf("%s-%s", Mempool, MempoolTxCount),
	}

	for _, key := range keys {
		if err := charts.lengthenChartUints(key, dayIntervals, hourIntervals, txn); err != nil {
			return err
		}
	}

	key := fmt.Sprintf("%s-%s", Mempool, MempoolFees)
	if err := charts.lengthenChartFloats(key, dayIntervals, hourIntervals, txn); err != nil {
		return err
	}

	if err := txn.Commit(); err != nil {
		return err
	}
	return nil
}

func (charts ChartData) lengthenPropagation() error {
	txn := charts.db.NewTransaction(true)
	defer txn.Discard()

	key := fmt.Sprintf("%s-%s", Propagation, TimeAxis)
	dayIntervals, hourIntervals, err := charts.lengthenTime(key, txn)
	if err != nil {
		return err
	}

	keys := []string{
		fmt.Sprintf("%s-%s", Propagation, BlockTimestamp),
		fmt.Sprintf("%s-%s", Propagation, VotesReceiveTime),
	}
	for _, source := range charts.syncSource {
		keys = append(keys, fmt.Sprintf("%s-%s-%s", Propagation, BlockPropagation, source))
	}
	
	for _, key := range keys {
		if err := charts.lengthenChartFloats(key, dayIntervals, hourIntervals, txn); err != nil {
			return err
		}
	}

	if err := txn.Commit(); err != nil {
		return err
	}
	return nil
}

func (charts ChartData) lengthenSnapshot() error {
	txn := charts.db.NewTransaction(true)
	defer txn.Discard()

	dayIntervals, hourIntervals, err := charts.lengthenTime(fmt.Sprintf("%s-%s", Snapshot, TimeAxis), txn)
	if err != nil {
		return err
	}

	keys := []string{
		fmt.Sprintf("%s-%s", Snapshot, SnapshotNodes),
		fmt.Sprintf("%s-%s", Snapshot, SnapshotReachableNodes),
	}
	for _, key := range keys{
		if err := charts.lengthenChartUints(key, dayIntervals, hourIntervals, txn); err != nil {
			return err
		}
	}

	// version
	key := fmt.Sprintf("%s-%s-%s", Snapshot, SnapshotNodeVersions, TimeAxis)
	dayIntervals, hourIntervals, err = charts.lengthenTime(key, txn)
	if err != nil {
		return err
	}

	keys = []string{}
	for _, userAgent := range charts.NodeVersion {
		keys = append(keys, fmt.Sprintf("%s-%s-%s", Snapshot, SnapshotNodeVersions, userAgent))
	}
	for _, key := range keys{
		if err := charts.lengthenChartUints(key, dayIntervals, hourIntervals, txn); err != nil {
			return err
		}
	}

	// location
	key = fmt.Sprintf("%s-%s-%s", Snapshot, SnapshotLocations, TimeAxis)
	dayIntervals, hourIntervals, err = charts.lengthenTime(key, txn)
	if err != nil {
		return err
	}

	keys = []string{}
	for _, country := range charts.NodeLocations {
		keys = append(keys, fmt.Sprintf("%s-%s-%s", Snapshot, SnapshotLocations, country))
	}
	for _, key := range keys{
		if err := charts.lengthenChartUints(key, dayIntervals, hourIntervals, txn); err != nil {
			return err
		}
	}

	if err := txn.Commit(); err != nil {
		return err
	}

	return nil
}

func (charts ChartData) lengthenTime(key string, txn *badger.Txn) (dayIntervals [][2]int, hourIntervals [][2]int, err error) {
	var dates ChartUints
	if err = charts.ReadValTx(key, &dates, txn); err != nil {
		if err == badger.ErrKeyNotFound {
			err = nil
			return
		}
		return
	}

	if dates.Length() == 0 {
		return
	}

	// day bin
	var days ChartUints
	// Get the current first and last midnight stamps.
	var start = midnight(dates[0])
	end := midnight(dates[len(dates)-1])

	// the index that begins new data.
	offset := 0
	// If there is day or more worth of new data, append to the Days zoomSet by
	// finding the first and last+1 blocks of each new day, and taking averages
	// or sums of the blocks in the interval.
	if end > start+aDay {
		next := start + aDay
		startIdx := 0
		for i, t := range dates[offset:] {
			if t >= next {
				// Once passed the next midnight, prepare a day window by storing the
				// range of indices.
				dayIntervals = append(dayIntervals, [2]int{startIdx + offset, i + offset})
				days = append(days, start)
				start = next
				next += aDay
				startIdx = i
				if t > end {
					break
				}
			}
		}
	}

	if err = charts.SaveValTx(days, fmt.Sprintf("%s-%s", key, dayBin), txn); err != nil {
		return
	}

	// hour bin
	var hours ChartUints
	// Get the current first and last hour stamps.
	start = hourStamp(dates[0])
	end = hourStamp(dates[len(dates)-1])

	// the index that begins new data.
	offset = 0
	// If there is day or more worth of new data, append to the Days zoomSet by
	// finding the first and last+1 blocks of each new day, and taking averages
	// or sums of the blocks in the interval.
	if end > start+anHour {
		next := start + anHour
		startIdx := 0
		for i, t := range dates[offset:] {
			if t >= next {
				// Once passed the next hour, prepare a day window by storing the
				// range of indices.
				hourIntervals = append(hourIntervals, [2]int{startIdx + offset, i + offset})
				hours = append(hours, start)
				start = next
				next += anHour
				startIdx = i
				if t > end {
					break
				}
			}
		}
	}

	if err = charts.SaveValTx(hours, fmt.Sprintf("%s-%s", key, hourBin), txn); err != nil {
		return
	}

	return
}

func (charts ChartData) lengthenChartUints(key string, dayIntervals [][2]int, hourIntervals [][2]int, txn *badger.Txn) error {

	var data, dayData, hourData ChartUints
	if err := charts.ReadValTx(key, &data, txn); err != nil {
		if err == badger.ErrKeyNotFound {
			return nil
		}
		return err
	}

	// day bin
	for _, interval := range dayIntervals {
		// For each new day, take an appropriate snapshot.
		dayData = append(dayData, data.Avg(interval[0], interval[1]))
	}

	if err := charts.SaveValTx(dayData, fmt.Sprintf("%s-%s", key, dayBin), txn); err != nil {
		return err
	}

	// hour bin
	for _, interval := range hourIntervals {
		// For each new day, take an appropriate snapshot.
		hourData = append(hourData, data.Avg(interval[0], interval[1]))
	}

	if err := charts.SaveValTx(hourData, fmt.Sprintf("%s-%s", key, hourBin), txn); err != nil {
		return err
	}

	return nil
}

func (charts ChartData) lengthenChartFloats(key string, dayIntervals [][2]int, hourIntervals [][2]int, txn *badger.Txn) error {

	var data, dayData, hourData ChartFloats
	if err := charts.ReadValTx(key, &data, txn); err != nil {
		if err == badger.ErrKeyNotFound {
			return nil
		}
		return err
	}

	// day bin
	for _, interval := range dayIntervals {
		// For each new day, take an appropriate snapshot.
		dayData = append(dayData, data.Avg(interval[0], interval[1]))
	}

	if err := charts.SaveValTx(dayData, fmt.Sprintf("%s-%s", key, dayBin), txn); err != nil {
		return err
	}

	// hour bin
	for _, interval := range hourIntervals {
		// For each new day, take an appropriate snapshot.
		hourData = append(hourData, data.Avg(interval[0], interval[1]))
	}

	if err := charts.SaveValTx(hourData, fmt.Sprintf("%s-%s", key, hourBin), txn); err != nil {
		return err
	}

	return nil
}

// Reduce the timestamp to the previous midnight.
func midnight(t uint64) (mid uint64) {
	if t > 0 {
		mid = t - t%aDay
	}
	return
}

// Reduce the timestamp to the previous hour
func hourStamp(t uint64) (hour uint64) {
	if t > 0 {
		hour = t - t%anHour
	}
	return
}

func (charts ChartData) PropagationHeightTip() uint64 {
	var heights ChartUints
	err := charts.ReadVal(Propagation+"-"+string(HeightAxis), &heights)
	if err != nil {
		return 0
	}
	if len(heights) == 0 {
		return 0
	}
	return heights[heights.Length()-1]
}

func (charts ChartData) PowTimeTip() uint64 {
	var dates ChartUints
	err := charts.ReadVal(PowChart+"-"+string(TimeAxis), &dates)
	if err != nil {
		return 0
	}
	if len(dates) == 0 {
		return 0
	}
	return dates[dates.Length()-1]
}

func (charts ChartData) VSPTimeTip() uint64 {
	var dates ChartUints
	err := charts.ReadVal(VSP+"-"+string(TimeAxis), &dates)
	if err != nil {
		return 0
	}
	if len(dates) == 0 {
		return 0
	}
	return dates[dates.Length()-1]
}

func (charts ChartData) SnapshotTip() uint64 {
	var dates ChartUints
	err := charts.ReadVal(Snapshot+"-"+string(TimeAxis), &dates)
	if err != nil {
		return 0
	}
	if len(dates) == 0 {
		return 0
	}
	return dates[dates.Length()-1]
}
