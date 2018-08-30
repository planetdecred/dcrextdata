package main

import (
	"database/sql"
	"dcrextdata/models"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/spf13/viper"

	boil "github.com/vattle/sqlboiler/boil"
	null "gopkg.in/nullbio/null.v6"
)

//Bittrex ash
type Bittrex struct {
	client *http.Client
}

type bittrexData struct {
	Success string `json:"success"`
	Message string `json:"message"`

	Result []ResultArray `json:"result"`
}

type ticksData struct {
	Success string `json:"success"`
	Message string `json:"message"`

	Result []tickDataArray `json:"result"`
}

type tickDataArray struct {
	O  null.Float64 `json:"O"`
	H  null.Float64 `json:"H"`
	L  null.Float64 `json:"L"`
	C  null.Float64 `json:"C"`
	V  null.Float64 `json:"V"`
	T  null.String  `json:"T"`
	BV null.Float64 `json:"BV"`
}

//ResultArray Export the values to ResultArray struct
type ResultArray struct {
	ID        null.Float64 `json:"Id"`
	Timestamp null.String  `json:"TimeStamp"`
	Quantity  null.Float64 `json:"Quantity"`
	Price     null.Float64 `json:"Price"`
	Total     null.Float64 `json:"Total"`
	Filltype  null.String  `json:"FillType"`
	Ordertype null.String  `json:"OrderType"`
}

//Function to Return Historic Pricing Data from Bittrex Exchange
//Parameters : Currency Pair

func (b *Bittrex) getBittrexData(currencyPair string) {

	//Get the base url
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s "+"password=%s dbname=%s sslmode=disable", viper.Get("Database.pghost"), 5432, viper.Get("Database.pguser"), viper.Get("Database.pgpass"), viper.Get("Database.pgdbname"))
	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {

		panic(err.Error())

	}

	boil.SetDB(db)

	url := viper.Get("ExchangeData.bittrex")

	req, err := http.NewRequest("GET", url.(string), nil)
	if err != nil {

		panic(err.Error())
	}
	q := req.URL.Query()

	//Append the user defined parameters to complete the url

	q.Add("market", currencyPair)

	req.URL.RawQuery = q.Encode()

	//Sends the GET request to the API

	request, err := http.NewRequest("GET", req.URL.String(), nil)

	res, _ := b.client.Do(request)

	// To check the status code of response
	fmt.Println(res.StatusCode)

	//Store the response in body variable as a byte array
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {

		panic(err.Error())
	}

	//Store the data in bittrexData struct
	var data bittrexData

	json.Unmarshal(body, &data)
	// fmt.Printf("Results: %v\n", data.Result)

	//Loop over array of struct and store them in the table

	fmt.Print(data.Result[99].Filltype)
	for i := 0; i <= 99; i++ {

		var p models.HistoricDatum

		// p1.ExchangeName =
		p.Globaltradeid = data.Result[i].ID
		p.Quantity = data.Result[i].Quantity
		p.Price = data.Result[i].Price
		p.Total = data.Result[i].Total
		p.FillType = data.Result[i].Filltype
		p.OrderType = data.Result[i].Ordertype
		p.CreatedOn = data.Result[i].Timestamp

		// fmt.Print(data.Result[i].Filltype)
		err := p.Insert(db)
		if err != nil {
			panic(err.Error())
		}
	}
	return

}

//To get Ticks from Bittrex Exchange every 24 hours
//Parameters : Currency Pair

func (b *Bittrex) getChartData(currencyPair string) {

	url := viper.Get("ChartData")

	req, err := http.NewRequest("GET", url.(string), nil)
	if err != nil {
		panic(err.Error())
	}
	q := req.URL.Query()

	//Append user defined parameters to the base URL

	q.Add("marketName", currencyPair)
	q.Add("tickInterval", "day")

	req.URL.RawQuery = q.Encode()

	request, err := http.NewRequest("GET", req.URL.String(), nil)

	//Sends the GET request to the API and stores the response

	res, _ := b.client.Do(request)

	// To check the status code of response

	fmt.Println(res.StatusCode)

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		panic(err.Error())
	}

	//Stores the response in ticksData struct

	var data ticksData

	json.Unmarshal(body, &data)
	fmt.Printf("Results: %v\n", data.Result)

	//Loop over array of struct and stores the response in table

	for i := range data.Result {

		var p1 models.ChartDatum

		// p1.Exchangeid = 1
		p1.CreatedOn = data.Result[i].T
		p1.High = data.Result[i].H
		p1.Low = data.Result[i].O
		p1.Opening = data.Result[i].C
		p1.Closing = data.Result[i].V
		p1.Quotevolume = data.Result[i].BV
		err := p1.Insert(db)
		panic(err.Error())

	}
	return
}
