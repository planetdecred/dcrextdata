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
	"github.com/volatiletech/sqlboiler/types"
)

type pow struct {
	client *http.Client
}

type powdata struct {
	date          null.Float64        `json : "date"`
	hashper       null.String         `json : "hashper" `
	blocksper     types.NullDecimal   `json:"blocksper"`
	luck          types.NullDecimal   `json:"luck"`
	miners        null.String         `json:"miners"`
	pphash        null.String         `json:"pphash"`
	ppshare       types.NullDecimal   `json:"ppshare"`
	totalKickback types.NullDecimal   `json:"total_kickback"`
	price         null.String         `json:"price"`
	hashrate      types.NullDecimal   `json:"hashrate"`
	blocksfound   types.NullDecimal   `json:"blocksfound"`
	totalminers   types.NullDecimal   `json:"totalminers"`
	globalStats   []globalStatsValues `json:"globalStats"`
	dataVal       dataVal             `json:"data"`
	decred        altpool             `json:"decred"`
	dcr           altpoolCurrency     `json:"DCR"`
	success       null.String         `json:"success"`
	lastUpdate    types.NullDecimal   `json:"lastUpdate"`
	mainnet       mainnet             `json:"mainnet"`
	blockReward   blockReward         `json:"blockReward"`
}

type mainnet struct {
	currentHeight     types.NullDecimal `json:"currentHeight"`
	networkHashrate   null.String       `json:"networkHashrate"`
	networkDifficulty null.String       `json:"networkDifficulty"`
}

type blockReward struct {
	total types.NullDecimal `json:"total"`
	pow   types.NullDecimal `json:"pow"`
	pos   types.NullDecimal `json:"pos"`
	dev   types.NullDecimal `json:"dev"`
}

type globalStatsValues struct {
	time              types.NullDecimal `json:"time"`
	networkHashrate   types.NullDecimal `json:"network_hashrate"`
	poolHashrate      null.String       `json:"pool_hashrate"`
	workers           types.NullDecimal `json:"workers"`
	networkDifficulty types.NullDecimal `json:"network_difficulty"`
	coinPrice         types.NullDecimal `json:"coin_price"`
	btcPrice          types.NullDecimal `json:"btc_price"`
}

type dataVal struct {
	poolName            null.String       `json:"pool_name"`
	hashrate            float64           `json:"hashrate"`
	efficiency          types.NullDecimal `json:'efficiency"`
	progress            types.NullDecimal `json:"progress"`
	workers             null.String       `json:"workers"`
	currentnetworkblock types.NullDecimal `json:"currentnetworkblock"`
	nextnetworkblock    types.NullDecimal `json:"nextnetworkblock"`
	lastblock           types.NullDecimal `json:"lastblock"`
	networkdiff         types.NullDecimal `json:"networkdiff"`
	esttime             null.String       `json:"esttime"`
	estshares           types.NullDecimal `json:"estshares"`
	timesincelast       types.NullDecimal `json:"timesincelast"`
	nethashrate         int64             `json:"nethashrate"`
}

type altpool struct {
	name             null.String       `json:"name"`
	port             types.NullDecimal `json:"port"`
	coins            int64             `json:"coins"`
	fees             types.NullDecimal `json:"fees"`
	hashrate         int64             `json:"hashrate"`
	workers          int64             `json:"workers"`
	estimate_current types.NullDecimal `json:"estimate_current"`
	estimate_last24h types.NullDecimal `json"estimate_last24h"`
	actual_last24h   float64           `json:"actual_last24h"`
	mbtc_mh_factor   types.NullDecimal `json:"mbtc_mh_factor"`
	hashrate_last24h types.NullDecimal `json:"hashrate_last24h"`
	rental_current   types.NullDecimal `json:"rental_current"`
}

type altpoolCurrency struct {
	algo          null.String       `json:"algo"`
	port          null.String       `json:"port"`
	name          null.String       `json:"name"`
	height        types.NullDecimal `json:"height"`
	workers       null.String       `json:"workers"`
	shares        null.String       `json:"shares"`
	hashrate      null.String       `json:"hashrate"`
	estimate      types.NullDecimal `json:"estimate"`
	blocks24h     types.NullDecimal `json:"24h_blocks"`
	btc24h        types.NullDecimal `json:"24h_btc"`
	lastblock     null.String       `json:"lastblock"`
	timesincelast null.String       `json:"timesincelast"`
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

	ctx := context.Background()
	tx, err := boil.BeginTx(ctx, nil)
	if err != nil {
		panic(err)
	}

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

		err := p1.Insert(ctx, tx, boil.Infer())
		if err != nil {
			panic(err.Error())
		}

	}

}
