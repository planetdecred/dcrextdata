package commstats

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/planetdecred/dcrextdata/app"
	"github.com/planetdecred/dcrextdata/app/helpers"
	"github.com/planetdecred/dcrextdata/postgres/models"
	"github.com/planetdecred/dcrextdata/cache"
)

const (
	redditRequestURL = "https://www.reddit.com/r/%s/about.json"
)

var subreddits []string

func Subreddits() []string {
	return subreddits
}

func (c *Collector) startRedditCollector(ctx context.Context, cacheManager *cache.Manager) {
	var lastCollectionDate time.Time
	err := c.dataStore.LastEntry(ctx, models.TableNames.Reddit, &lastCollectionDate)
	if err != nil && err != sql.ErrNoRows {
		log.Errorf("Cannot fetch last Reddit entry time, %s", err.Error())
		return
	}

	secondsPassed := time.Since(lastCollectionDate)
	period := time.Duration(c.options.TwitterStatInterval) * time.Minute

	if secondsPassed < period {
		timeLeft := period - secondsPassed
		log.Infof("Fetching Reddit stats every %dm, collected %s ago, will fetch in %s.", period/time.Minute, helpers.DurationToString(secondsPassed),
			helpers.DurationToString(timeLeft))

		time.Sleep(timeLeft)
	}

	registerStarter := func() {
		// continually check the state of the app until its free to run this module
		for {
			if app.MarkBusyIfFree() {
				break
			}
		}
	}

	registerStarter()
	c.collectAndStoreRedditStat(ctx)
	app.ReleaseForNewModule()

	ticker := time.NewTicker(time.Duration(c.options.RedditStatInterval) * time.Minute)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			registerStarter()
			c.collectAndStoreRedditStat(ctx)
			if err = cacheManager.Update(ctx, cache.Community); err != nil {
				log.Error(err)
			}
			app.ReleaseForNewModule()
		}
	}
}

func (c *Collector) collectAndStoreRedditStat(ctx context.Context) {
	log.Info("Starting Reddit stats collection cycle")

	for _, subreddit := range c.options.Subreddit {
		// reddit
		resp, err := c.fetchRedditStat(ctx, subreddit)
		for retry := 0; err != nil; retry++ {
			if retry == retryLimit {
				log.Error(err)
				return
			}
			log.Warn(err)
			resp, err = c.fetchRedditStat(ctx, subreddit)
		}

		err = c.dataStore.StoreRedditStat(ctx, Reddit{
			Date:           helpers.NowUTC(),
			Subscribers:    resp.Data.Subscribers,
			AccountsActive: resp.Data.AccountsActive,
			Subreddit:      subreddit,
		})
		if err != nil {
			log.Error("Unable to save reddit stat, %s", err.Error())
			return
		}
		log.Infof("New Reddit stat collected for %s at %s, Subscribers  %d, Active Users %d", subreddit,
			helpers.NowUTC().Format(dateMiliTemplate), resp.Data.Subscribers, resp.Data.AccountsActive)
	}
}

func (c *Collector) fetchRedditStat(ctx context.Context, subreddit string) (response *RedditResponse, err error) {
	if ctx.Err() != nil {
		err = ctx.Err()
		return
	}

	request, err := http.NewRequest(http.MethodGet, fmt.Sprintf(redditRequestURL, subreddit), nil)
	if err != nil {
		return
	}

	// reddit returns too many redditRequest http status if user agent is not set
	request.Header.Set("user-agent",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/75.0.3770.100 Safari/537.36")

	// log.Tracef("GET %v", redditRequestURL)
	resp, err := c.client.Do(request.WithContext(ctx))
	if err != nil {
		return
	}
	defer resp.Body.Close()
	response = new(RedditResponse)
	if resp.StatusCode == http.StatusOK {
		err = json.NewDecoder(resp.Body).Decode(response)
		if err != nil {
			return nil, fmt.Errorf(fmt.Sprintf("Failed to decode json: %v", err))
		}
	} else {
		log.Infof("Unable to fetchRedditStat data from reddit: %s", resp.Status)
	}

	return
}
