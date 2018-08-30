package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/raedahgroup/dcrextdata/models"

	null "gopkg.in/nullbio/null.v6"
)

type pow struct {
	client *http.Client
}

type powdata struct {
	date          null.Float64        `json : "date"`
	hashper       null.String         `json : "hashper" `
	blocksper     null.Float64        `json:"blocksper"`
	luck          null.Float64        `json:"luck"`
	miners        null.String         `json:"miners"`
	pphash        null.String         `json:"pphash"`
	ppshare       null.Float64        `json:"ppshare"`
	totalKickback null.Float64        `json:"total_kickback"`
	price         null.String         `json:"price"`
	hashrate      null.Float64        `json:"hashrate"`
	blocksfound   null.Float64        `json:"blocksfound"`
	totalminers   null.Float64        `json:"totalminers"`
	globalStats   []globalStatsValues `json:"globalStats"`
	dataVal       dataVal             `json:"data"`
	decred        altpool             `json:"decred"`
	dcr           altpoolCurrency     `json:"DCR"`
	success       null.String         `json:"success"`
	lastUpdate    null.Float64        `json:"lastUpdate"`
	mainnet       mainnet             `json:"mainnet"`
	blockReward   blockReward         `json:"blockReward"`
}

type mainnet struct {
	currentHeight     null.Float64 `json:"currentHeight"`
	networkHashrate   null.String  `json:"networkHashrate"`
	networkDifficulty null.String  `json:"networkDifficulty"`
}

type blockReward struct {
	total null.Float64 `json:"total"`
	pow   null.Float64 `json:"pow"`
	pos   null.Float64 `json:"pos"`
	dev   null.Float64 `json:"dev"`
}

type globalStatsValues struct {
	time              null.Float64 `json:"time"`
	networkHashrate   null.Float64 `json:"network_hashrate"`
	poolHashrate      null.String  `json:"pool_hashrate"`
	workers           null.Float64 `json:"workers"`
	networkDifficulty null.Float64 `json:"network_difficulty"`
	coinPrice         null.Float64 `json:"coin_price"`
	btcPrice          null.Float64 `json:"btc_price"`
}

type dataVal struct {
	poolName            null.String  `json:"pool_name"`
	hashrate            float64      `json:"hashrate"`
	efficiency          null.Float64 `json:'efficiency"`
	progress            null.Float64 `json:"progress"`
	workers             null.String  `json:"workers"`
	currentnetworkblock null.Float64 `json:"currentnetworkblock"`
	nextnetworkblock    null.Float64 `json:"nextnetworkblock"`
	lastblock           null.Float64 `json:"lastblock"`
	networkdiff         null.Float64 `json:"networkdiff"`
	esttime             null.String  `json:"esttime"`
	estshares           null.Float64 `json:"estshares"`
	timesincelast       null.Float64 `json:"timesincelast"`
	nethashrate         int64        `json:"nethashrate"`
}

type altpool struct {
	name             null.String  `json:"name"`
	port             null.Float64 `json:"port"`
	coins            int64        `json:"coins"`
	fees             null.Float64 `json:"fees"`
	hashrate         int64        `json:"hashrate"`
	workers          int64        `json:"workers"`
	estimate_current null.Float64 `json:"estimate_current"`
	estimate_last24h null.Float64 `json"estimate_last24h"`
	actual_last24h   float64      `json:"actual_last24h"`
	mbtc_mh_factor   null.Float64 `json:"mbtc_mh_factor"`
	hashrate_last24h null.Float64 `json:"hashrate_last24h"`
	rental_current   null.Float64 `json:"rental_current"`
}

type altpoolCurrency struct {
	algo          null.String  `json:"algo"`
	port          null.String  `json:"port"`
	name          null.String  `json:"name"`
	height        null.Float64 `json:"height"`
	workers       null.String  `json:"workers"`
	shares        null.String  `json:"shares"`
	hashrate      null.String  `json:"hashrate"`
	estimate      null.Float64 `json:"estimate"`
	blocks24h     null.Float64 `json:"24h_blocks"`
	btc24h        null.Float64 `json:"24h_btc"`
	lastblock     null.String  `json:"lastblock"`
	timesincelast null.String  `json:"timesincelast"`
}

func (p *pow) getPow(id int, url string, api_key string) {

	req, err := http.NewRequest("GET", url, nil)

	if len(api_key) != 0 {
		q := req.URL.Query()
		q.Add("api_key", api_key)
		req.URL.RawQuery = q.Encode()
	}

	request, err := http.NewRequest("GET", req.URL.String(), nil)

	res, _ := p.client.Do(request)

	fmt.Println(res.StatusCode)
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		panic(err.Error())
	}

	var data powdata
	json.Unmarshal(body, &data)

	fmt.Println(string(body))

	fmt.Printf("Results: %v\n", data)

	//Loop over the entire list to insert data into the table
	for i := 0; i < 15; i++ {

		var p1 models.PowDatum
		p1.Hashrate = data.hashrate
		p1.Efficiency = data.dataVal.efficiency
		p1.Progress = data.dataVal.progress
		if data.globalStats != nil {
			fmt.Printf("{{%+v}}\n", data.globalStats)
			p1.Workers = data.globalStats[0].workers
			p1.Esttime = data.globalStats[0].time
			p1.Nethashrate = data.globalStats[0].networkHashrate
			p1.Networkdifficulty = data.globalStats[0].networkDifficulty
			p1.Coinprice = data.globalStats[0].coinPrice
			p1.Btcprice = data.globalStats[0].btcPrice
		}
		p1.Currentnetworkblock = data.dataVal.currentnetworkblock
		p1.Nextnetworkblock = data.dataVal.nextnetworkblock
		p1.Lastblock = data.dataVal.lastblock
		p1.Networkdiff = data.dataVal.networkdiff
		p1.Estshare = data.dataVal.estshares
		p1.Timesincelast = data.dataVal.timesincelast
		p1.Blocksfound = data.blocksfound
		p1.Totalminers = data.totalminers
		// p1.Time = data.globalStats[0].time
		p1.Est = data.dcr.estimate
		// p1.Date = data.date
		p1.Blocksper = data.blocksper
		p1.Luck = data.luck
		p1.Ppshare = data.ppshare
		p1.Totalkickback = data.totalKickback
		p1.Success = data.success
		p1.Lastupdate = data.lastUpdate
		p1.Name = data.decred.name
		p1.Port = data.decred.port
		p1.Fees = data.decred.fees
		p1.Estimatecurrent = data.decred.estimate_current
		p1.Estimatelast24h = data.decred.estimate_last24h
		// p1.Actual24H = data.decred.actual_last24h
		p1.Mbtcmhfactor = data.decred.mbtc_mh_factor
		p1.Hashratelast24h = data.decred.hashrate_last24h
		p1.Rentalcurrent = data.decred.rental_current
		p1.Height = data.dcr.height
		p1.Blocks24h = data.dcr.blocks24h
		p1.BTC24H = data.dcr.btc24h
		p1.Currentheight = data.mainnet.currentHeight
		p1.Total = data.blockReward.total
		p1.Pos = data.blockReward.pos
		p1.Pow = data.blockReward.pow
		p1.Dev = data.blockReward.dev

		err := p1.Insert(db)
		if err != nil {
			panic(err.Error())
		}

	}

}
