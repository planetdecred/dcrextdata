package main

// go:generate sqlboiler postgres

import (
	"database/sql"
	"fmt"
	"net/http"

	_ "github.com/lib/pq"
	"github.com/spf13/viper"
	"github.com/vattle/sqlboiler/boil"
	log15 "gopkg.in/inconshreveable/log15.v2"
)

// Open handle to database like normal
var log = log15.New()
var psqlInfo string
var db *sql.DB
var err error

func init() {
	//Set and read the config file

	viper.SetConfigFile("./config.json")
	if err := viper.ReadInConfig(); err != nil {
		panic(err)
	}

	psqlInfo = fmt.Sprintf("host=%s port=%s user=%s "+"password=%s dbname=%s sslmode=disable",
		viper.Get("Database.pghost"),
		viper.Get("Database.pgport"),
		viper.Get("Database.pguser"),
		viper.Get("Database.pgpass"),
		viper.Get("Database.pgdbname"))
	db, err = sql.Open("postgres", psqlInfo)
}

func main() {

	// Set default value for pow and exchange
	viper.SetDefault("pow", "http://api.f2pool.com/decred/address")
	viper.SetDefault("ExchangeData", "https://bittrex.com/api/v1.1/public/getmarkethistory")

	boil.SetDB(db)

	// functions to insert data

	getHistoricData("bittrex", "BTC-DCR", "1514764800", "1514851200")        //parameters : exchange name,currency pair, start time, end time
	getChartData("poloniex", "BTC_DCR", "1514764800", "1517443199", "86400") //parameters: exchange name,Currency Pair, start time , end time
	getPowData(2, "")                                                        //parameters: pool id
	getPosData()

	// functions to fetch data

	// fetchHistoricData("date")
}

// func fetchHistoricData(date string) {

// 	Result, err := models.HistoricDatum(qm.Where("created_on=?", date)).One(ctx, db)

// 	fmt.Print(Result)

// }

// Function to get Proof of Stake Data

func getPosData() {

	user := pos{
		client: &http.Client{},
	}

	user.getPos()
}

// Function to get Proof of Work Data
// @parameters - PoolID integer 0 to 7

func getPowData(PoolID int, apiKey string) {
	user := pow{
		client: &http.Client{},
	}

	powString := fmt.Sprintf("pow.%+v", PoolID)
	fmt.Print(viper.GetString(powString))
	user.getPow(PoolID, viper.GetString(powString), apiKey)

}

// Function to insert historic data into db from exchanges

func getHistoricData(exchangeName string, currencyPair string, startTime string, endTime string) {

	if exchangeName == "poloniex" {
		user := Poloniex{

			client: &http.Client{},
		}
		user.getPoloniexData(currencyPair, startTime, endTime)

	}

	if exchangeName == "bittrex" {

		user := Bittrex{
			client: &http.Client{},
		}
		user.getBittrexData(currencyPair)
	}

}

//Get chart data from exchanges

func getChartData(exchangeName string, currencyPair string, startTime string, endTime string, period string) {

	if exchangeName == "poloniex" {
		user := Poloniex{

			client: &http.Client{},
		}
		user.getChartData(currencyPair, startTime, endTime, period)

	}
	if exchangeName == "bittrex" {
		user := Bittrex{
			client: &http.Client{},
		}
		user.getChartData(currencyPair)

	}

}
