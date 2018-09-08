package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/raedahgroup/dcrextdata/models"

	"github.com/spf13/viper"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
)

// Structure containing Poloniex client data

type Poloniex struct {
	client *http.Client
}

//Structure containing Poloniex Historic Data

type poloniexData struct {
	Result []struct {
		GlobalTradeID null.Float64 `json:"globalTradeID"`
		TradeID       null.Float64 `json:"tradeID"`
		Date          time.Time    `json:"date"`
		Types         null.String  `json:"type"`
		Rate          null.Float64 `json:"rate"`
		Amount        null.Float64 `json:"amount"`
		Total         null.Float64 `json:"total"`
	}
}

// Structure containing Poloniex Chart Data

type chartData struct {
	Result []struct {
		Date            null.Int64   `json:"date"`
		High            null.Float64 `json:"high"`
		Low             null.Float64 `json:"low"`
		Open            null.Float64 `json:"open"`
		Close           null.Float64 `json:"close"`
		Volume          null.Float64 `json:"volume"`
		QuoteVolume     null.Float64 `json:"quoteVolume"`
		WeightedAverage null.Float64 `json:"weightedAverage"`
	}
}

// Get Poloniex Historic Data
// parameters : Currency pair, Start time , End time
func (p *Poloniex) getPoloniexData(currencyPair string, start string, end string) string {

	dbInfo := fmt.Sprintf("host=%s port=%s user=%s "+"password=%s dbname=%s sslmode=disable", viper.Get("Database.pghost"), viper.Get("Database.pgport"), viper.Get("Database.pguser"), viper.Get("Database.pgpass"), viper.Get("Database.pgdbname"))
	db, err := sql.Open("postgres", dbInfo)
	if err != nil {
		panic(err.Error())
	}

	boil.SetDB(db)

	//Get Url of Poloniex API
	url := viper.Get("ExchangeData.0").(string)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic(err.Error())
	}

	//Append user provided parameters in the URL
	q := req.URL.Query()
	q.Add("command", "returnTradeHistory")
	q.Add("currencyPair", currencyPair)
	q.Add("start", start)
	q.Add("end", end)
	req.URL.RawQuery = q.Encode()

	//Get Historic Data from the API
	request, err := http.NewRequest("GET", req.URL.String(), nil)
	res, _ := p.client.Do(request)

	// To check the status code of response
	fmt.Printf("POLINEX: %+v - %+v\n", req.URL.String(), res.StatusCode)

	//Get response of the request as []byte
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		panic(err.Error())
	}

	//Map the data to poloniexData struct and unmarshal the contents

	var data poloniexData
	json.Unmarshal(body, &data)
	// fmt.Printf("Results: %v\n", data)
	fmt.Printf("len: %v\n", len(data.Result))

	for i := range data.Result {

		var p1 models.HistoricDatum

		// p1.ExchangeName = "Poloniex"
		p1.Globaltradeid = data.Result[i].GlobalTradeID
		p1.Tradeid = data.Result[i].TradeID
		p1.CreatedOn = null.NewTime(data.Result[i].Date, true)
		p1.Quantity = data.Result[i].Amount
		p1.Price = data.Result[i].Rate
		p1.Total = data.Result[i].Total
		p1.OrderType = data.Result[i].Types

		err := p1.Insert(context.Background(), db, boil.Infer())
		if err != nil {
			panic(err.Error())
		}

	}
	return "Saved poloneix historic data!"

}

//Returns Poloniex Chart Data
//Parameters : Currency pair, Start time , End time
func (p *Poloniex) getChartData(currencyPair string, start string, end string, period string) {

	url := viper.Get("ChartData.poloniex").(string)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic(err.Error())
	}

	//Append the user defined parameters to the url
	q := req.URL.Query()
	q.Add("currencyPair", currencyPair)
	q.Add("start", start)
	q.Add("end", end)
	q.Add("period", period)

	req.URL.RawQuery = q.Encode()

	//Get the data from API and convert the data to byte array
	request, err := http.NewRequest("GET", req.URL.String(), nil)
	res, _ := p.client.Do(request)

	// To check the status code of response
	fmt.Printf("CHART DATA: %+v - %+v\n", req.URL.String(), res.StatusCode)

	//Store the response in body variable as a byte array
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		panic(err.Error())
	}

	//Store the data to charData struct
	var data chartData
	err = json.Unmarshal(body, &data.Result)
	if err != nil {
		fmt.Printf("CHART DATA ERROR: %+v\n", err)
	}
	// fmt.Printf("Results: %v\n", data)
	fmt.Printf("len: %+v\n", len(data.Result))

	//Loop over the entire data and store it in the table
	for i := range data.Result {

		var p2 models.ChartDatum

		p2.CreatedOn = null.NewTime(
			time.Unix(data.Result[i].Date.Int64, 0),
			true)

		p2.High = data.Result[i].High
		p2.Low = data.Result[i].Low
		p2.Opening = data.Result[i].Open
		p2.Closing = data.Result[i].Close
		p2.Volume = data.Result[i].Volume
		p2.Quotevolume = data.Result[i].QuoteVolume
		p2.Weightedaverage = data.Result[i].WeightedAverage

		err := p2.Insert(context.Background(), db, boil.Infer())
		if err != nil {
			panic(err.Error())
		}

	}

}
