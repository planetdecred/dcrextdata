package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/raedahgroup/dcrextdata/models"

	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
)

type pow struct {
	client *http.Client
}

type powdata struct {
	Date          null.Float64        `json:"date"`
	Hashper       null.String         `json:"hashper" `
	Blocksper     null.Float64        `json:"blocksper"`
	Luck          null.Float64        `json:"luck"`
	Miners        null.String         `json:"miners"`
	Pphash        null.String         `json:"pphash"`
	Ppshare       null.Float64        `json:"ppshare"`
	TotalKickback null.Float64        `json:"total_kickback"`
	Price         null.String         `json:"price"`
	Hashrate      null.Float64        `json:"hashrate"`
	Blocksfound   null.Float64        `json:"blocksFound"`
	Totalminers   null.Float64        `json:"totalMiners"`
	GlobalStats   []globalStatsValues `json:"globalStats"`
	DataVal       dataVal             `json:"data"`
	Decred        altpool             `json:"decred"`
	Dcr           altpoolCurrency     `json:"DCR"`
	Success       null.String         `json:"success"`
	LastUpdate    null.Float64        `json:"lastUpdate"`
	Mainnet       mainnet             `json:"mainnet"`
	BlockReward   blockReward         `json:"blockReward"`
}

type mainnet struct {
	CurrentHeight     null.Float64 `json:"currentHeight"`
	NetworkHashrate   null.String  `json:"networkHashrate"`
	NetworkDifficulty null.String  `json:"networkDifficulty"`
}

type blockReward struct {
	Total null.Float64 `json:"total"`
	Pow   null.Float64 `json:"pow"`
	Pos   null.Float64 `json:"pos"`
	Dev   null.Float64 `json:"dev"`
}

type globalStatsValues struct {
	Time              null.String  `json:"time"`
	NetworkHashrate   null.Float64 `json:"network_hashrate"`
	PoolHashrate      null.Float64 `json:"pool_hashrate"`
	Workers           null.Float64 `json:"workers"`
	NetworkDifficulty null.Float64 `json:"network_difficulty"`
	CoinPrice         null.String  `json:"coin_price"`
	BtcPrice          null.String  `json:"btc_price"`
}

type dataVal struct {
	PoolName            null.String  `json:"pool_name"`
	Hashrate            float64      `json:"hashrate"`
	Efficiency          null.Float64 `json:"efficiency"`
	Progress            null.Float64 `json:"progress"`
	Workers             null.String  `json:"workers"`
	Currentnetworkblock null.Float64 `json:"currentnetworkblock"`
	Nextnetworkblock    null.Float64 `json:"nextnetworkblock"`
	Lastblock           null.Float64 `json:"lastblock"`
	Networkdiff         null.Float64 `json:"networkdiff"`
	Esttime             null.String  `json:"esttime"`
	Estshares           null.Float64 `json:"estshares"`
	Timesincelast       null.Float64 `json:"timesincelast"`
	Nethashrate         int64        `json:"nethashrate"`
}

type altpool struct {
	Name            null.String  `json:"name"`
	Port            null.Float64 `json:"port"`
	Coins           int64        `json:"coins"`
	Fees            null.Float64 `json:"fees"`
	Hashrate        int64        `json:"hashrate"`
	Workers         int64        `json:"workers"`
	EstimateCurrent null.Float64 `json:"estimate_current"`
	EstimateLast24h null.Float64 `json:"estimate_last24h"`
	ActualLast24h   float64      `json:"actual_last24h"`
	MbtcMhFactor    null.Float64 `json:"mbtc_mh_factor"`
	HashrateLast24h null.Float64 `json:"hashrate_last24h"`
	RentalCurrent   null.Float64 `json:"rental_current"`
}

type altpoolCurrency struct {
	Algo          null.String  `json:"algo"`
	Port          null.String  `json:"port"`
	Name          null.String  `json:"name"`
	Height        null.Float64 `json:"height"`
	Workers       null.String  `json:"workers"`
	Shares        null.String  `json:"shares"`
	Hashrate      null.String  `json:"hashrate"`
	Estimate      null.Float64 `json:"estimate"`
	Blocks24h     null.Float64 `json:"24h_blocks"`
	Btc24h        null.Float64 `json:"24h_btc"`
	Lastblock     null.String  `json:"lastblock"`
	Timesincelast null.String  `json:"timesincelast"`
}

func (p *pow) getPow(id int, url string, apiKey string) {

	req, err := http.NewRequest("GET", url, nil)

	if len(apiKey) != 0 {
		q := req.URL.Query()
		q.Add("api_key", apiKey)
		req.URL.RawQuery = q.Encode()
	}

	request, err := http.NewRequest("GET", req.URL.String(), nil)
	res, _ := p.client.Do(request)

	// To check the status code of response
	fmt.Printf("POW: %+v - %+v\n", req.URL.String(), res.StatusCode)
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		panic(err.Error())
	}

	var data powdata
	err = json.Unmarshal(body, &data)
	if err != nil {
		fmt.Printf("POW ERROR: %+v\n", err)
	}
	// fmt.Printf("Results: %v\n", data)
	fmt.Printf("len: %v\n", len(data.GlobalStats))

	//Loop over the entire list to insert data into the table
	for i := 0; i < 15; i++ {

		var p1 models.PowDatum

		p1.Hashrate = data.Hashrate
		p1.Efficiency = data.DataVal.Efficiency
		p1.Progress = data.DataVal.Progress
		if data.GlobalStats != nil {
			// fmt.Printf("{{%+v}}\n", data.GlobalStats[0])
			p1.Workers = data.GlobalStats[0].Workers
			p1.Nethashrate = data.GlobalStats[0].NetworkHashrate
			p1.Networkdifficulty = data.GlobalStats[0].NetworkDifficulty
			p1.Coinprice = data.GlobalStats[0].CoinPrice
			p1.Btcprice = data.GlobalStats[0].BtcPrice
			// p1.Esttime = data.GlobalStats[0].Time
			p1.Esttime = null.NewTime(
				convertStringTime(data.GlobalStats[0].Time.String), true)
		}
		p1.Currentnetworkblock = data.DataVal.Currentnetworkblock
		p1.Nextnetworkblock = data.DataVal.Nextnetworkblock
		p1.Lastblock = data.DataVal.Lastblock
		p1.Networkdiff = data.DataVal.Networkdiff
		p1.Estshare = data.DataVal.Estshares
		p1.Timesincelast = data.DataVal.Timesincelast
		p1.Blocksfound = data.Blocksfound
		p1.Totalminers = data.Totalminers
		// p1.Time = data.GlobalStats[0].time
		p1.Est = data.Dcr.Estimate
		// p1.Date = data.date
		p1.Blocksper = data.Blocksper
		p1.Luck = data.Luck
		p1.Ppshare = data.Ppshare
		p1.Totalkickback = data.TotalKickback
		p1.Success = data.Success
		p1.Lastupdate = data.LastUpdate
		p1.Name = data.Decred.Name
		p1.Port = data.Decred.Port
		p1.Fees = data.Decred.Fees
		p1.Estimatecurrent = data.Decred.EstimateCurrent
		p1.Estimatelast24h = data.Decred.EstimateLast24h
		// p1.Actual24H = data.Decred.actual_last24h
		p1.Mbtcmhfactor = data.Decred.MbtcMhFactor
		p1.Hashratelast24h = data.Decred.HashrateLast24h
		p1.Rentalcurrent = data.Decred.RentalCurrent
		p1.Height = data.Dcr.Height
		p1.Blocks24h = data.Dcr.Blocks24h
		p1.BTC24H = data.Dcr.Btc24h
		p1.Currentheight = data.Mainnet.CurrentHeight
		p1.Total = data.BlockReward.Total
		p1.Pos = data.BlockReward.Pos
		p1.Pow = data.BlockReward.Pow
		p1.Dev = data.BlockReward.Dev

		err := p1.Insert(context.Background(), db, boil.Infer())
		if err != nil {
			panic(err.Error())
		}

	}

}
