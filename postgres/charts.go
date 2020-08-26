package postgres

import "github.com/planetdecred/dcrextdata/cache"

func (pg *PgDb) RegisterCharts(charts *cache.Manager, syncSources []string, syncSourceDbProvider func(source string) (*PgDb, error)) {
	pg.syncSourceDbProvider = syncSourceDbProvider
	pg.syncSources = syncSources

	charts.AddRetriever(cache.Mempool, pg.fetchEncodeMempoolChart)

	charts.AddRetriever(cache.Propagation, pg.fetchEncodePropagationChart)

	charts.AddRetriever(cache.PowChart, pg.fetchEncodePowChart)

	charts.AddRetriever(cache.VSP, pg.fetchEncodeVspChart)

	charts.AddRetriever(cache.Exchange, pg.fetchEncodeExchangeChart)

	charts.AddRetriever(cache.Snapshot, pg.fetchEncodeSnapshotChart)
}
