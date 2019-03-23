package main

type DataTick struct {
	High     float64
	Low      float64
	Open     float64
	Close    float64
	Volume   float64
	Time     int64
	Exchange string
}

type poloniexAPIResponse []poloniexDataTick

type poloniexDataTick struct {
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Open   float64 `json:"open"`
	Close  float64 `json:"close"`
	Volume float64 `json:"volume"`
	Time   int64   `json:"date"`
}

type bittrexDataTick struct {
	High   float64 `json:"H"`
	Low    float64 `json:"L"`
	Open   float64 `json:"O"`
	Close  float64 `json:"C"`
	Volume float64 `json:"BV"`
	Time   string  `json:"T"`
}

type bittrexAPIResponse struct {
	Result []bittrexDataTick `json:"result"`
}

type bleutradeDataTick struct {
	High   string `json:"high"`
	Low    string `json:"low"`
	Open   string `json:"open"`
	Close  string `json:"close"`
	Volume string `json:"volume"`
	Time   string `json:"TimeStamp"`
}

type bleutradeAPIResponse struct {
	Result []bleutradeDataTick `json:"result"`
}

type binanceAPIResponse []binanceDataTick
type binanceDataTick []interface{}
