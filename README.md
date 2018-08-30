
### Set the configration settings for database in config.json file


### Create database *data_feed_processor*


```
psql -U postgres data_feed_processor < data_feed_processor.sql
```
```
go generate
```

```
sqlboiler postgres
```


### Run the project

```
go run main.go bittrex.go poloniex.go pos.go pow.go 
```
