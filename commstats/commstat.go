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
	"github.com/planetdecred/dcrextdata/cache"
)

const (
	dateTemplate     = "2006-01-02 15:04"
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

func (c *Collector) Run(ctx context.Context, cacheManager *cache.Manager) {
	if ctx.Err() != nil {
		return
	}

	// continually check the state of the app until its free to run this module
	for {
		if app.MarkBusyIfFree() {
			break
		}
	}

	log.Info("Fetching community stats...")

	app.ReleaseForNewModule()

	go c.startTwitterCollector(ctx, cacheManager)

	go c.startYoutubeCollector(ctx, cacheManager)

	// github
	go c.startGithubCollector(ctx, cacheManager)

	go c.startRedditCollector(ctx, cacheManager)
}

func SetAccounts(options config.CommunityStatOptions) {
	subreddits = options.Subreddit
	twitterHandles = options.TwitterHandles
	repositories = options.GithubRepositories
	youtubeChannels = options.YoutubeChannelName
}
