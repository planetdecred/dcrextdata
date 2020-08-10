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
func (charts *Manager) SaveVersion() error {
	return charts.SaveVal(versionKey, cacheVersion)
}

func (charts *Manager) getVersion() (semver Semver, err error) {
	err = charts.ReadVal(versionKey, &semver)
	if err == badger.ErrKeyNotFound {
		semver, err = cacheVersion, nil
	}
	return
}

func (charts *Manager) SaveVal(key string, val interface{}) error {
	var b bytes.Buffer
	e := gob.NewEncoder(&b)
	if err := e.Encode(val); err != nil {
		return err
	}
	_ = charts.DB.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})
	err := charts.DB.Update(func(txn *badger.Txn) error {
		err := txn.Set([]byte(key), b.Bytes())
		return err
	})
	return err
}

func (charts *Manager) SaveValTx(key string, val interface{}, txn *badger.Txn) error {
	var b bytes.Buffer
	e := gob.NewEncoder(&b)
	if err := e.Encode(val); err != nil {
		return err
	}
	_ = txn.Delete([]byte(key))
	return txn.Set([]byte(key), b.Bytes())
}

func (charts *Manager) ClearVLog() {
again:
	verr := charts.DB.RunValueLogGC(0.7)
	if verr == nil {
		goto again
	}
}

func (charts *Manager) ReadVal(key string, result interface{}) error {
	return charts.DB.View(func(txn *badger.Txn) error {
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

func (charts *Manager) ReadValTx(key string, result interface{}, txn *badger.Txn) error {
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

func (charts *Manager) AppendChartUintsAxis(key string, set ChartUints) error {
	var data ChartUints
	err := charts.ReadVal(key, &data)
	if err != nil {
		if err != badger.ErrKeyNotFound {
			return err
		}
	}
	data = append(data, set...)
	return charts.SaveVal(key, data)
}

func (charts *Manager) AppendChartUintsAxisTx(key string, set ChartUints, txn *badger.Txn) error {
	var data ChartUints
	err := charts.ReadValTx(key, &data, txn)
	if err != nil {
		if err != badger.ErrKeyNotFound {
			return err
		}
	}
	data = append(data, set...)
	return charts.SaveValTx(key, data, txn)
}

func (charts *Manager) AppendChartNullUintsAxis(key string, set ChartNullUints) error {
	var data chartNullIntsPointer
	err := charts.ReadVal(key, &data)
	if err != nil {
		if err != badger.ErrKeyNotFound {
			return err
		}
	}
	data = data.Append(set)
	return charts.SaveVal(key, data)
}

func (charts *Manager) AppendChartNullUintsAxisTx(key string, set ChartNullUints, txn *badger.Txn) error {
	var data chartNullIntsPointer
	err := charts.ReadValTx(key, &data, txn)
	if err != nil {
		if err != badger.ErrKeyNotFound {
			return err
		}
	}
	data = data.Append(set)
	return charts.SaveValTx(key, data, txn)
}

func (charts *Manager) AppendChartFloatsAxis(key string, set ChartFloats) error {
	var data ChartFloats
	err := charts.ReadVal(key, &data)
	if err != nil {
		if err != badger.ErrKeyNotFound {
			return err
		}
	}
	data = append(data, set...)
	return charts.SaveVal(key, data)
}

func (charts *Manager) AppendChartNullFloatsAxis(key string, set ChartNullFloats) error {
	var data chartNullFloatsPointer
	err := charts.ReadVal(key, &data)
	if err != nil {
		if err != badger.ErrKeyNotFound {
			return err
		}
	}
	data = data.Append(set)
	return charts.SaveVal(key, data)
}

func (charts *Manager) AppendChartNullFloatsAxisTx(key string, set ChartNullFloats, txn *badger.Txn) error {
	var data chartNullFloatsPointer
	err := charts.ReadValTx(key, &data, txn)
	if err != nil {
		if err != badger.ErrKeyNotFound {
			return err
		}
	}
	data = data.Append(set)
	return charts.SaveValTx(key, data, txn)
}

func (charts *Manager) NormalizeLength(tags ...string) error {
	txn := charts.DB.NewTransaction(true)
	defer txn.Discard()

	for _, chartID := range tags {
		if cerr := charts.normalizeLength(chartID, txn); cerr != nil {
			return errors.Wrap(cerr, "Normalize failed for "+chartID)
		}
	}
	if err := txn.Commit(); err != nil {
		return err
	}
	return nil
}

// length correction
func (charts *Manager) normalizeLength(chartID string, txn *badger.Txn) error {
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

func (charts *Manager) normalizeMempoolLength(txn *badger.Txn) error {
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

func (charts *Manager) normalizePropagationLength(txn *badger.Txn) error {
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

func (charts *Manager) normalizePowChartLength(txn *badger.Txn) error {
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

func (charts *Manager) normalizeVSPLength(txn *badger.Txn) error {
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

func (charts *Manager) normalizeExchangeLength(txn *badger.Txn) error {

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

func (charts *Manager) normalizeSnapshotLength(txn *badger.Txn) error {
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

func (charts *Manager) chartUintsLength(key string, txn *badger.Txn) (int, error) {
	var data ChartUints
	err := charts.ReadValTx(key, &data, txn)
	if err != nil {
		if err != badger.ErrKeyNotFound {
			return 0, err
		}
	}
	return data.Length(), nil
}

func (charts *Manager) chartFloatsLength(key string, txn *badger.Txn) (int, error) {
	var data ChartFloats
	err := charts.ReadValTx(key, &data, txn)
	if err != nil {
		if err != badger.ErrKeyNotFound {
			return 0, err
		}
	}
	return data.Length(), nil
}

func (charts *Manager) chartNullUintsLength(key string, txn *badger.Txn) (int, error) {
	var data chartNullIntsPointer
	err := charts.ReadValTx(key, &data, txn)
	if err != nil {
		if err != badger.ErrKeyNotFound {
			return 0, err
		}
	}
	return data.Length(), nil
}

func (charts *Manager) chartNullFloatsLength(key string, txn *badger.Txn) (int, error) {
	var data chartNullFloatsPointer
	err := charts.ReadValTx(key, &data, txn)
	if err != nil {
		if err != badger.ErrKeyNotFound {
			return 0, err
		}
	}
	return data.Length(), nil
}

func (charts *Manager) snipMempool(length int, txn *badger.Txn) error {
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

func (charts *Manager) snipPropagationChart(length int, axis axisType, txn *badger.Txn) error {
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
	case BlockTimestamp:
		keys = []string{
			Propagation + "-" + string(BlockTimestamp),
			Propagation + "-" + string(VotesReceiveTime),
		}
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

func (charts *Manager) snipPowChart(length int, txn *badger.Txn) error {
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

func (charts *Manager) snipVspChart(length int, txn *badger.Txn) error {
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

func (charts *Manager) snipExchangeChart(length int, txn *badger.Txn) error {
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

func (charts *Manager) snipSnapshotChart(length int, axis axisType, txn *badger.Txn) error {
	var keys []string
	switch axis {
	case SnapshotNodes:
		keys = []string{
			Snapshot + "-" + string(TimeAxis),
			Snapshot + "-" + string(SnapshotNodes),
			Snapshot + "-" + string(SnapshotReachableNodes),
		}
	case SnapshotLocations:
		keys = append(keys, Snapshot+"-"+string(SnapshotLocations)+"-"+string(TimeAxis))
		for _, country := range charts.NodeLocations {
			keys = append(keys, Snapshot+"-"+string(SnapshotLocations)+"-"+country)
		}
	case SnapshotNodeVersions:
		keys = append(keys, Snapshot+"-"+string(SnapshotNodeVersions)+"-"+string(TimeAxis))
		for _, userAgent := range charts.NodeVersion {
			keys = append(keys, Snapshot+"-"+string(SnapshotNodeVersions)+"-"+userAgent)
		}
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

func (charts *Manager) snipCommunityChart(length int, txn *badger.Txn) error {
	return nil
}

func (charts *Manager) snipChartUintsAxis(key string, length int, txn *badger.Txn) error {
	var data ChartUints
	err := charts.ReadValTx(key, &data, txn)
	if err != nil {
		if err != badger.ErrKeyNotFound {
			return err
		}
	}
	data = data.snip(length)
	return charts.SaveValTx(key, data, txn)
}

func (charts *Manager) snipChartNullUintsAxis(key string, length int, txn *badger.Txn) error {
	var data chartNullIntsPointer
	err := charts.ReadValTx(key, &data, txn)
	if err != nil {
		if err != badger.ErrKeyNotFound {
			return err
		}
	}
	data = data.snip(length)
	return charts.SaveValTx(key, data, txn)
}

func (charts *Manager) snipChartNullFloatsAxis(key string, length int, txn *badger.Txn) error {
	var data chartNullFloatsPointer
	err := charts.ReadValTx(key, &data, txn)
	if err != nil {
		if err != badger.ErrKeyNotFound {
			return err
		}
	}
	data = data.snip(length)
	return charts.SaveValTx(key, data, txn)
}

func (charts *Manager) snipChartFloatsAxis(key string, length int, txn *badger.Txn) error {
	var data ChartFloats
	err := charts.ReadValTx(key, &data, txn)
	if err != nil {
		if err != badger.ErrKeyNotFound {
			return err
		}
	}
	data = data.snip(length)
	return charts.SaveValTx(key, data, txn)
}

func (charts *Manager) MempoolTimeTip() uint64 {
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

func (charts *Manager) lengthenMempool() error {
	txn := charts.DB.NewTransaction(true)
	defer txn.Discard()

	if err := charts.updateMempoolHeights(txn); err != nil {
		log.Errorf("Unable to update mempool heights, %s", err.Error())
		return err
	}

	dayIntervals, hourIntervals, err := charts.lengthenTimeAndHeight(
		fmt.Sprintf("%s-%s", Mempool, TimeAxis),
		fmt.Sprintf("%s-%s", Mempool, HeightAxis), txn)
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

func (charts *Manager) updateMempoolHeights(txn *badger.Txn) error {
	var mempoolDates, propagationDates, mempoolHeights, propagationHeights ChartUints
	if err := charts.ReadValTx(fmt.Sprintf("%s-%s", Mempool, TimeAxis), &mempoolDates, txn); err != nil {
		if err == badger.ErrKeyNotFound {
			log.Warn("Mempool height not updated, mempool dates has no value")
			return nil
		}
		return err
	}

	if mempoolDates.Length() == 0 {
		log.Warn("Mempool height not updated, mempool dates has no value")
		return nil
	}

	if err := charts.ReadValTx(fmt.Sprintf("%s-%s", Propagation, TimeAxis), &propagationDates, txn); err != nil {
		if err == badger.ErrKeyNotFound {
			log.Warn("Mempool height not updated, propagation dates has no value")
			return nil
		}
		return err
	}

	if propagationDates.Length() == 0 {
		log.Warn("Mempool height not updated, propagation dates has no value")
		return nil
	}

	if err := charts.ReadValTx(fmt.Sprintf("%s-%s", Propagation, HeightAxis), &propagationHeights, txn); err != nil {
		if err == badger.ErrKeyNotFound {
			log.Warn("Mempool height not updated, propagation heights has no value")
			return nil
		}
		return err
	}

	if propagationHeights.Length() == 0 {
		log.Warn("Mempool height not updated, propagation heights has no value")
		return nil
	}

	pIndex := 0
	for _, date := range mempoolDates {
		if pIndex+1 < propagationDates.Length() && date >= propagationDates[pIndex+1] {
			pIndex += 1
		}
		mempoolHeights = append(mempoolHeights, propagationHeights[pIndex])
	}

	if err := charts.SaveValTx(fmt.Sprintf("%s-%s", Mempool, HeightAxis), mempoolHeights, txn); err != nil {
		return err
	}

	return nil
}

func (charts *Manager) lengthenPropagation() error {
	txn := charts.DB.NewTransaction(true)
	defer txn.Discard()

	key := fmt.Sprintf("%s-%s", Propagation, TimeAxis)
	dayIntervals, hourIntervals, err := charts.lengthenTimeAndHeight(key, fmt.Sprintf("%s-%s", Propagation, HeightAxis), txn)
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

func (charts *Manager) lengthenVsp() error {
	txn := charts.DB.NewTransaction(true)
	defer txn.Discard()

	key := fmt.Sprintf("%s-%s", VSP, TimeAxis)
	dayIntervals, hourIntervals, err := charts.lengthenTime(key, txn)
	if err != nil {
		return err
	}

	var uintAxisKeys, floatAxisKeys []string

	for _, source := range charts.VSPSources {
		uintAxisKeys = append(uintAxisKeys, fmt.Sprintf("%s-%s-%s", VSP, ImmatureAxis, source))
		uintAxisKeys = append(uintAxisKeys, fmt.Sprintf("%s-%s-%s", VSP, LiveAxis, source))
		uintAxisKeys = append(uintAxisKeys, fmt.Sprintf("%s-%s-%s", VSP, VotedAxis, source))
		uintAxisKeys = append(uintAxisKeys, fmt.Sprintf("%s-%s-%s", VSP, MissedAxis, source))
		uintAxisKeys = append(uintAxisKeys, fmt.Sprintf("%s-%s-%s", VSP, UserCountAxis, source))
		uintAxisKeys = append(uintAxisKeys, fmt.Sprintf("%s-%s-%s", VSP, UsersActiveAxis, source))

		floatAxisKeys = append(floatAxisKeys, fmt.Sprintf("%s-%s-%s", VSP, PoolFeesAxis, source))
		floatAxisKeys = append(floatAxisKeys, fmt.Sprintf("%s-%s-%s", VSP, ProportionLiveAxis, source))
		floatAxisKeys = append(floatAxisKeys, fmt.Sprintf("%s-%s-%s", VSP, ProportionMissedAxis, source))
	}

	for _, key := range uintAxisKeys {
		if err := charts.lengthenChartNullUints(key, dayIntervals, hourIntervals, txn); err != nil {
			return err
		}
	}

	for _, key := range floatAxisKeys {
		if err := charts.lengthenChartNullFloats(key, dayIntervals, hourIntervals, txn); err != nil {
			return err
		}
	}

	if err := txn.Commit(); err != nil {
		return err
	}

	return nil
}

func (charts *Manager) lengthenPow() error {
	txn := charts.DB.NewTransaction(true)
	defer txn.Discard()

	key := fmt.Sprintf("%s-%s", PowChart, TimeAxis)
	dayIntervals, hourIntervals, err := charts.lengthenTime(key, txn)
	if err != nil {
		return err
	}

	var keys []string

	for _, source := range charts.PowSources {
		keys = append(keys, fmt.Sprintf("%s-%s-%s", PowChart, WorkerAxis, source))
		keys = append(keys, fmt.Sprintf("%s-%s-%s", PowChart, HashrateAxis, source))
	}

	for _, key := range keys {
		if err := charts.lengthenChartNullUints(key, dayIntervals, hourIntervals, txn); err != nil {
			return err
		}
	}

	if err := txn.Commit(); err != nil {
		return err
	}

	return nil
}

func (charts *Manager) lengthenSnapshot() error {
	txn := charts.DB.NewTransaction(true)
	defer txn.Discard()

	dayIntervals, hourIntervals, err := charts.lengthenTime(fmt.Sprintf("%s-%s", Snapshot, TimeAxis), txn)
	if err != nil {
		return err
	}

	keys := []string{
		fmt.Sprintf("%s-%s", Snapshot, SnapshotNodes),
		fmt.Sprintf("%s-%s", Snapshot, SnapshotReachableNodes),
	}
	for _, key := range keys {
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
	for _, key := range keys {
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
	for _, key := range keys {
		if err := charts.lengthenChartUints(key, dayIntervals, hourIntervals, txn); err != nil {
			return err
		}
	}

	if err := txn.Commit(); err != nil {
		return err
	}

	return nil
}

func (charts *Manager) lengthenTime(key string, txn *badger.Txn) (dayIntervals [][2]int, hourIntervals [][2]int, err error) {
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
	// or sums of the blocks in the interval.  0.06096031
	if end > start+aDay {
		next := start + aDay
		startIdx := 0
		for i, t := range dates[offset:] {
			if t >= next {
				// Once passed the next midnight, prepare a day window by
				// storing the range of indices. 0, 1, 2, 3, 4, 5
				dayIntervals = append(dayIntervals, [2]int{startIdx + offset, i + offset})
				// check for records b/4 appending.
				days = append(days, start)
				next = midnight(t)
				start = next
				next += aDay
				startIdx = i
				if t > end {
					break
				}
			}
		}
	}

	if err = charts.SaveValTx(fmt.Sprintf("%s-%s", key, dayBin), days, txn); err != nil {
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
				// Once passed the next hour, prepare a day window by storing
				// the range of indices.
				hourIntervals = append(hourIntervals, [2]int{startIdx + offset, i + offset})
				hours = append(hours, start)
				next = hourStamp(t)
				start = next
				next += anHour
				startIdx = i
				if t > end {
					break
				}
			}
		}
	}

	if err = charts.SaveValTx(fmt.Sprintf("%s-%s", key, hourBin), hours, txn); err != nil {
		return
	}

	return
}

func (charts *Manager) lengthenTimeAndHeight(timeKey, heightKey string, txn *badger.Txn) (dayIntervals [][2]int, hourIntervals [][2]int, err error) {
	var dates, heights ChartUints
	if err = charts.ReadValTx(timeKey, &dates, txn); err != nil {
		if err == badger.ErrKeyNotFound {
			err = nil
			return
		}
		return
	}

	if err = charts.ReadValTx(heightKey, &heights, txn); err != nil {
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
	var days, dayHeights ChartUints
	// Get the current first and last midnight stamps.
	var start = midnight(dates[0])
	end := midnight(dates[len(dates)-1])

	// the index that begins new data.
	offset := 0
	// If there is day or more worth of new data, append to the Days zoomSet by
	// finding the first and last+1 blocks of each new day, and taking averages
	// or sums of the blocks in the interval.  0.06096031
	if end > start+aDay {
		next := start + aDay
		startIdx := 0
		for i, t := range dates[offset:] {
			if t >= next {
				// Once passed the next midnight, prepare a day window by
				// storing the range of indices. 0, 1, 2, 3, 4, 5
				dayIntervals = append(dayIntervals, [2]int{startIdx + offset, i + offset})
				// check for records b/4 appending.
				days = append(days, start)
				dayHeights = append(dayHeights, heights[i])
				next = midnight(t)
				start = next
				next += aDay
				startIdx = i
				if t > end {
					break
				}
			}
		}
	}

	if err = charts.SaveValTx(fmt.Sprintf("%s-%s", timeKey, dayBin), days, txn); err != nil {
		return
	}

	if err = charts.SaveValTx(fmt.Sprintf("%s-%s", heightKey, dayBin), dayHeights, txn); err != nil {
		return
	}

	// hour bin
	var hours, hourHeights ChartUints
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
				// Once passed the next hour, prepare a day window by storing
				// the range of indices.
				hourIntervals = append(hourIntervals, [2]int{startIdx + offset, i + offset})
				hours = append(hours, start)
				hourHeights = append(hourHeights, heights[i])
				next = hourStamp(t)
				start = next
				next += anHour
				startIdx = i
				if t > end {
					break
				}
			}
		}
	}

	if err = charts.SaveValTx(fmt.Sprintf("%s-%s", timeKey, hourBin), hours, txn); err != nil {
		return
	}
	if err = charts.SaveValTx(fmt.Sprintf("%s-%s", heightKey, hourBin), hourHeights, txn); err != nil {
		return
	}

	return
}

func (charts *Manager) lengthenChartUints(key string, dayIntervals [][2]int, hourIntervals [][2]int, txn *badger.Txn) error {

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

	if err := charts.SaveValTx(fmt.Sprintf("%s-%s", key, dayBin), dayData, txn); err != nil {
		return err
	}

	// hour bin
	for _, interval := range hourIntervals {
		// For each new day, take an appropriate snapshot.
		hourData = append(hourData, data.Avg(interval[0], interval[1]))
	}

	if err := charts.SaveValTx(fmt.Sprintf("%s-%s", key, hourBin), hourData, txn); err != nil {
		return err
	}

	return nil
}

func (charts *Manager) lengthenChartNullUints(key string, dayIntervals [][2]int, hourIntervals [][2]int, txn *badger.Txn) error {

	var data, dayData, hourData chartNullIntsPointer
	if err := charts.ReadValTx(key, &data, txn); err != nil {
		if err == badger.ErrKeyNotFound {
			return nil
		}
		return err
	}

	// day bin
	for _, interval := range dayIntervals {
		// For each new day, take an appropriate snapshot.
		dayData.Items = append(dayData.Items, data.Avg(interval[0], interval[1]))
	}

	if err := charts.SaveValTx(fmt.Sprintf("%s-%s", key, dayBin), dayData, txn); err != nil {
		return err
	}

	// hour bin
	for _, interval := range hourIntervals {
		// For each new day, take an appropriate snapshot.
		hourData.Items = append(hourData.Items, data.Avg(interval[0], interval[1]))
	}

	if err := charts.SaveValTx(fmt.Sprintf("%s-%s", key, hourBin), hourData, txn); err != nil {
		return err
	}

	return nil
}

func (charts *Manager) lengthenChartFloats(key string, dayIntervals [][2]int, hourIntervals [][2]int, txn *badger.Txn) error {

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

	if err := charts.SaveValTx(fmt.Sprintf("%s-%s", key, dayBin), dayData, txn); err != nil {
		return err
	}

	// hour bin
	for _, interval := range hourIntervals {
		// For each new day, take an appropriate snapshot.
		hourData = append(hourData, data.Avg(interval[0], interval[1]))
	}

	if err := charts.SaveValTx(fmt.Sprintf("%s-%s", key, hourBin), hourData, txn); err != nil {
		return err
	}

	return nil
}

func (charts *Manager) lengthenChartNullFloats(key string, dayIntervals [][2]int, hourIntervals [][2]int, txn *badger.Txn) error {

	var data, dayData, hourData chartNullFloatsPointer
	if err := charts.ReadValTx(key, &data, txn); err != nil {
		if err == badger.ErrKeyNotFound {
			return nil
		}
		return err
	}

	// day bin
	for _, interval := range dayIntervals {
		// For each new day, take an appropriate snapshot.
		dayData.Items = append(dayData.Items, data.Avg(interval[0], interval[1]))
	}

	if err := charts.SaveValTx(fmt.Sprintf("%s-%s", key, dayBin), dayData, txn); err != nil {
		return err
	}

	// hour bin
	for _, interval := range hourIntervals {
		// For each new day, take an appropriate snapshot.
		hourData.Items = append(hourData.Items, data.Avg(interval[0], interval[1]))
	}

	if err := charts.SaveValTx(fmt.Sprintf("%s-%s", key, hourBin), hourData, txn); err != nil {
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

func (charts *Manager) PropagationHeightTip() uint64 {
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

func (charts *Manager) PowTimeTip() uint64 {
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

func (charts *Manager) VSPTimeTip() uint64 {
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

func (charts *Manager) SnapshotTip() uint64 {
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
