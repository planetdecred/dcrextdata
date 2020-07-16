// Copyright (c) 2018-2019 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package commstats

import (
	"context"
	"net/http"
	"time"

	"github.com/raedahgroup/dcrextdata/app/config"
)

type CommStat struct {
	Date               time.Time         `json:"date"`
	RedditStats        map[string]Reddit `json:"reddit_stats"`
	TwitterFollowers   int               `json:"twitter_followers"`
	YoutubeSubscribers int               `json:"youtube_subscribers"`
	GithubStars        int               `json:"github_stars"`
	GithubFolks        int               `json:"github_folks"`
}

type RedditResponse struct {
	Kind string `json:"kind"`
	Data Reddit `json:"data"`
}

type Reddit struct {
	Date           time.Time `json:"date"`
	Subscribers    int       `json:"subscribers"`
	AccountsActive int       `json:"active_user_count"`
	Subreddit      string    `json:"subreddit"`
}

type Github struct {
	Date       time.Time `json:"date"`
	Stars      int       `json:"stars"`
	Folks      int       `json:"folks"`
	Repository string    `json:"repository"`
}

type Youtube struct {
	Date        time.Time `json:"date"`
	Subscribers int       `json:"subscribers"`
	Channel     string    `json:"channel"`
	ViewCount   int       `json:"view_count"`
}

type Twitter struct {
	Date      time.Time `json:"date"`
	Followers int       `json:"followers"`
	Handle    string    `json:"handle"`
}

type ChartData struct {
	Date   time.Time `json:"date"`
	Record int64     `json:"record"`
}

type DataStore interface {
	StoreRedditStat(context.Context, Reddit) error
	LastCommStatEntry() (time time.Time)
	StoreTwitterStat(ctx context.Context, twitter Twitter) error
	StoreYoutubeStat(ctx context.Context, youtube Youtube) error
	StoreGithubStat(ctx context.Context, github Github) error
	LastEntry(ctx context.Context, tableName string, receiver interface{}) error
}

type Collector struct {
	client    http.Client
	dataStore DataStore
	options   *config.CommunityStatOptions
}
