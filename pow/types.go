package pow

import "time"

type PowDataSource struct {
	Source string
}

type PowData struct {
	Time         int64
	PoolHashrate float64
	Workers      int64
	CoinPrice    float64
	BtcPrice     float64
	Source       string
}

type PowDataDto struct {
	Time           string  `json:"time"`
	PoolHashrateTh string  `json:"pool_hashrate_th"`
	Workers        int64   `json:"workers"`
	CoinPrice      float64 `json:"coin_price"`
	BtcPrice       float64 `json:"btc_price"`
	Source         string  `json:"source"`
}

type PowChartData struct {
	Date   time.Time `json:"date"`
	Record string    `json:"record"`
}

type luxorPowData struct {
	Time         string  `json:"time"`
	PoolHashrate float64 `json:"pool_hashrate"`
	Workers      int64   `json:"workers"`
	CoinPrice    string  `json:"coin_price"`
	BtcPrice     string  `json:"btc_price"`
}

type luxorAPIResponse struct {
	GlobalStats []luxorPowData `json:"globalStats"`
}

type f2poolPowData map[string]float64

type f2poolAPIResponse struct {
	Hashrate f2poolPowData `json:"hashrate_history"`
}

type coinmineAPIResponse struct {
	PoolHashrate float64 `json:"hashrate"`
	Workers      int64   `json:"workers"`
}

type uupoolData struct {
	PoolHashrate  float64 `json:"hr1"`
	OnlineWorkers int64   `json:"onlineWorkers"`
}

type uunetworkData struct {
}

type uupoolAPIResponse struct {
	Pool    uupoolData    `json:"pool"`
	Network uunetworkData `json:"network"`
}
