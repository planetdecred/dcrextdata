package main

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"
)

const (
	retryDelay       = 60 * time.Second
	maxRetryAttempts = 3
)

// GetResponse attempts to collect json data from the given url string and decodes it into
// the destination
func GetResponse(client *http.Client, url string, destination interface{}) error {
	resp := new(http.Response)

	for i := 1; i <= maxRetryAttempts; i++ {
		res, err := client.Get(url)
		requestsLog.Tracef("GET %s", url)
		if err != nil {
			if i == maxRetryAttempts {
				return err
			}
			requestsLog.Warn(err)
			if res != nil {
				res.Body.Close()
			}
			time.Sleep(retryDelay)
			continue
		}
		resp = res
		break
	}

	err := json.NewDecoder(resp.Body).Decode(destination)
	if err != nil {
		return err
	}

	resp.Body.Close()
	return nil
}

func addParams(base string, params map[string]interface{}) (string, error) {
	var strBuilder strings.Builder

	_, err := strBuilder.WriteString(base)

	if err != nil {
		return base, err
	}

	strBuilder.WriteString("?")

	for param, value := range params {
		strBuilder.WriteString(param)
		strBuilder.WriteString("=")

		vType := reflect.TypeOf(value)
		switch vType.Kind() {
		case reflect.String:
			strBuilder.WriteString(reflect.ValueOf(value).String())
		case reflect.Int64:
			strBuilder.WriteString(strconv.FormatInt(reflect.ValueOf(value).Int(), 10))
		}

		strBuilder.WriteString("&")
	}

	str := strBuilder.String()
	return str[:len(str)-1], nil
}

func UnixTimeToString(t int64) string {
	return time.Unix(t, 0).UTC().String()
}
