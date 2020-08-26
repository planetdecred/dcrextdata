// Copyright (c) 2018-2019 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package commstats

import (
	"context"
	"net/http"
	"time"

	"github.com/planetdecred/dcrextdata/app"
	"github.com/planetdecred/dcrextdata/app/config"
)

const (
	dateMiliTemplate = "2006-01-02 15:04:05.99"
	retryLimit       = 3
)

func NewCommStatCollector(store DataStore, options *config.CommunityStatOptions) (*Collector, error) {
	return &Collector{
		client:    http.Client{Timeout: 10 * time.Second},
		dataStore: store,
		options:   options,
	}, nil
}

func (c *Collector) Run(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}

	// continually check the state of the app until its free to run this module
	app.MarkBusyIfFree()

	log.Info("Fetching community stats...")

	app.ReleaseForNewModule()

	go c.startTwitterCollector(ctx)

	go c.startYoutubeCollector(ctx)

	// github
	go c.startGithubCollector(ctx)

	go c.startRedditCollector(ctx)
}

func SetAccounts(options config.CommunityStatOptions) {
	subreddits = options.Subreddit
	twitterHandles = options.TwitterHandles
	repositories = options.GithubRepositories
	youtubeChannels = options.YoutubeChannelName
}
