package postgres

import "github.com/raedahgroup/dcrextdata/cache"

func (pg *PgDb) RegisterCharts(charts *cache.Manager, syncSources []string, syncSourceDbProvider func(source string) (*PgDb, error)) {
	pg.syncSourceDbProvider = syncSourceDbProvider
	pg.syncSources = syncSources

	charts.AddUpdater(cache.ChartUpdater{
		Tag:      cache.Mempool,
		Fetcher:  pg.retrieveChartMempool,
		Appender: appendChartMempool,
	})
	charts.AddRetriever(cache.Mempool, pg.fetchEncodeMempoolChart)

	charts.AddUpdater(cache.ChartUpdater{
		Tag:      cache.Propagation,
		Fetcher:  pg.fetchBlockPropagationChart,
		Appender: appendBlockPropagationChart,
	})
	charts.AddRetriever(cache.Propagation, pg.fetchEncodePropagationChart)

	charts.AddUpdater(cache.ChartUpdater{
		Tag:      cache.PowChart,
		Fetcher:  pg.fetchCachePowChart,
		Appender: appendPowChart,
	})
	charts.AddRetriever(cache.PowChart, pg.fetchEncodePowChart)

	charts.AddUpdater(cache.ChartUpdater{
		Tag:      cache.VSP,
		Fetcher:  pg.fetchCacheVspChart,
		Appender: appendVspChart,
	})
	charts.AddRetriever(cache.VSP, pg.fetchEncodeVspChart)

	charts.AddUpdater(cache.ChartUpdater{
		Tag:      cache.Exchange,
		Fetcher:  pg.fetchExchangeChart,
		Appender: appendExchangeChart,
	})
	charts.AddRetriever(cache.Exchange, pg.fetchEncodeExchangeChart)

	charts.AddUpdater(cache.ChartUpdater{
		Tag:      cache.Snapshot,
		Fetcher:  pg.fetchNetworkSnapshotChart,
		Appender: appendSnapshotChart,
	})
	charts.AddUpdater(cache.ChartUpdater{
		Tag:      cache.Snapshot + "-*",
		Fetcher:  pg.fetchNetworkSnapshotTable,
		Appender: appendSnapshotTable,
	})
	charts.AddRetriever(cache.Snapshot, pg.fetchEncodeSnapshotChart)
}
