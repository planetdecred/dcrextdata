
### Set the configration settings for database in config.json and sqlboiler.toml files



### Create database *data_feed_processor*

```
createdb -U postgres data_feed_processor
```

```
psql -U postgres data_feed_processor < data_feed_processor.sql
```

```
go get -u -t github.com/volatiletech/sqlboiler
```

```
go get github.com/volatiletech/sqlboiler/drivers/sqlboiler-psql
```

```
go generate
```

```
sqlboiler psql
```


### Install dependences

```
curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
```

```
dep ensure -v
```


### Run the project

```
go run main.go bittrex.go poloniex.go pos.go pow.go 
```
