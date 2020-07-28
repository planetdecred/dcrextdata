package commstats

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/planetdecred/dcrextdata/app"
	"github.com/planetdecred/dcrextdata/app/helpers"
	"github.com/planetdecred/dcrextdata/postgres/models"
	"github.com/planetdecred/dcrextdata/cache"
)

const (
	twitterRequestURL = "https://cdn.syndication.twimg.com/widgets/followbutton/info.json?screen_names=%s"
)

var twitterHandles []string

func TwitterHandles() []string {
	return twitterHandles
}

func (c *Collector) startTwitterCollector(ctx context.Context, cacheManager *cache.Manager) {
	var lastCollectionDate time.Time
	err := c.dataStore.LastEntry(ctx, models.TableNames.Twitter, &lastCollectionDate)
	if err != nil && err != sql.ErrNoRows {
		log.Errorf("Cannot fetch last twitter entry time, %s", err.Error())
		return
	}

	secondsPassed := time.Since(lastCollectionDate)
	period := time.Duration(c.options.TwitterStatInterval) * time.Minute

	if secondsPassed < period {
		timeLeft := period - secondsPassed
		log.Infof("Fetching Twitter stats every %dm, collected %s ago, will fetch in %s.", period/time.Minute, helpers.DurationToString(secondsPassed),
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
	c.collectAndStoreTwitterStat(ctx)
	app.ReleaseForNewModule()

	ticker := time.NewTicker(time.Duration(c.options.TwitterStatInterval) * time.Minute)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			registerStarter()
			c.collectAndStoreTwitterStat(ctx)
			if err = cacheManager.Update(ctx, cache.Community); err != nil {
				log.Error(err)
			}
			app.ReleaseForNewModule()
		}
	}
}

func (c *Collector) collectAndStoreTwitterStat(ctx context.Context) {
	log.Info("Starting Twitter stats collection cycle")
	for _, handle := range c.options.TwitterHandles {
		followers, err := c.getTwitterFollowers(ctx, handle)
		for retry := 0; err != nil; retry++ {
			if retry == retryLimit {
				log.Error(err)
				return
			}
			log.Warn(err)
			followers, err = c.getTwitterFollowers(ctx, handle)
		}

		var twitterStat = Twitter{Date: helpers.NowUTC(), Followers: followers, Handle: handle}
		err = c.dataStore.StoreTwitterStat(ctx, twitterStat)
		if err != nil {
			log.Error("Unable to save twitter stat, %s", err.Error())
			return
		}

		log.Infof("New Twitter stat collected for %s at %s, Followers %d", handle,
			twitterStat.Date.Format(dateMiliTemplate), twitterStat.Followers)
	}
}

func (c *Collector) getTwitterFollowers(ctx context.Context, handle string) (int, error) {
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}

	request, err := http.NewRequest(http.MethodGet, fmt.Sprintf(twitterRequestURL, handle), nil)
	if err != nil {
		return 0, err
	}

	// reddit returns too many redditRequest http status if user agent is not set
	request.Header.Set("user-agent",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/75.0.3770.100 Safari/537.36")

	// log.Tracef("GET %v", redditRequestURL)
	resp, err := c.client.Do(request.WithContext(ctx))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	var response []struct {
		Followers int `json:"followers_count"`
	}
	if resp.StatusCode == http.StatusOK {
		err = json.NewDecoder(resp.Body).Decode(&response)
		if err != nil {
			return 0, fmt.Errorf(fmt.Sprintf("Failed to decode json: %v", err))
		}
	} else {
		return 0, fmt.Errorf("unable to fetch twitter followers: %s", resp.Status)
	}

	if len(response) < 1 {
		return 0, errors.New("unable to fetch twitter followers, no response")
	}

	return response[0].Followers, nil
}
