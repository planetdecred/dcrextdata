// Copyright (c) 2018-2019 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

type NilClientError struct{}

func (err NilClientError) Error() string {
	return "Cannot use a nil http client"
}

type NoExchangeError struct{}

func (err NoExchangeError) Error() string {
	return "Cannot create an ExchangeCollector without exchanges"
}

type CollectionIntervalTooShort struct{}

func (err CollectionIntervalTooShort) Error() string {
	return "Collection interval too short"
}
