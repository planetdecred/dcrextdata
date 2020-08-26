package commstats

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/planetdecred/dcrextdata/app"
	"github.com/planetdecred/dcrextdata/app/helpers"
	"github.com/planetdecred/dcrextdata/postgres/models"
)

var youtubeChannels []string

func YoutubeChannels() []string {
	return youtubeChannels
}

func (c *Collector) startYoutubeCollector(ctx context.Context) {
	if c.options.YoutubeDataApiKey == "" {
		log.Error("youtubedataapikey is required for the youtube stat collector to work")
		return
	}

	if len(c.options.YoutubeChannelName) != len(c.options.YoutubeChannelId) {
		log.Error("Both name and ID is required for all youtube channels")
		return
	}

	var lastCollectionDate time.Time
	err := c.dataStore.LastEntry(ctx, models.TableNames.Youtube, &lastCollectionDate)
	if err != nil && err != sql.ErrNoRows {
		log.Errorf("Cannot fetch last Youtube entry time, %s", err.Error())
		return
	}

	secondsPassed := time.Since(lastCollectionDate)
	period := time.Duration(c.options.TwitterStatInterval) * time.Minute

	if secondsPassed < period {
		timeLeft := period - secondsPassed
		log.Infof("Fetching Youtube stats every %dm, collected %s ago, will fetch in %s.", period/time.Minute, helpers.DurationToString(secondsPassed),
			helpers.DurationToString(timeLeft))

		time.Sleep(timeLeft)
	}

	registerStarter := func() {
		// continually check the state of the app until its free to run this module
		app.MarkBusyIfFree()
	}

	registerStarter()
	c.collectAndStoreYoutubeStat(ctx)
	app.ReleaseForNewModule()

	ticker := time.NewTicker(time.Duration(c.options.YoutubeStatInterval) * time.Minute)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			registerStarter()
			c.collectAndStoreYoutubeStat(ctx)
			app.ReleaseForNewModule()
		}
	}
}

func (c *Collector) collectAndStoreYoutubeStat(ctx context.Context) {
	log.Info("Starting Github stats collection cycle")
	// youtube
	for index, id := range c.options.YoutubeChannelId {
		youtubeSubscribers, viewCount, err := c.getYoutubeSubscriberCount(ctx, id)
		for retry := 0; err != nil; retry++ {
			if retry == retryLimit {
				return
			}
			log.Warn(err)
			youtubeSubscribers, viewCount, err = c.getYoutubeSubscriberCount(ctx, id)
		}

		var channel = c.options.YoutubeChannelName[index]

		youtubeStat := Youtube{
			Date:        helpers.NowUTC(),
			Subscribers: youtubeSubscribers,
			ViewCount:   viewCount,
			Channel:     channel,
		}
		err = c.dataStore.StoreYoutubeStat(ctx, youtubeStat)
		if err != nil {
			log.Error("Unable to save Youtube stat, %s", err.Error())
			return
		}

		log.Infof("New Youtube stat collected for %s at %s, Subscribers %d", channel,
			youtubeStat.Date.Format(dateMiliTemplate), youtubeSubscribers)
	}

}

func (c *Collector) getYoutubeSubscriberCount(ctx context.Context, youtubeChannelId string) (int, int, error) {
	if ctx.Err() != nil {
		return 0, 0, ctx.Err()
	}

	youtubeUrl := fmt.Sprintf("https://www.googleapis.com/youtube/v3/channels?part=statistics&id=%s&key=%s",
		youtubeChannelId, c.options.YoutubeDataApiKey)

	request, err := http.NewRequest(http.MethodGet, youtubeUrl, nil)
	if err != nil {
		return 0, 0, err
	}

	request.Header.Set("user-agent",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/75.0.3770.100 Safari/537.36")

	resp, err := c.client.Do(request.WithContext(ctx))
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()
	var response struct {
		Items []struct {
			Statistics struct {
				SubscriberCount string `json:"subscriberCount"`
				ViewCount       string `json:"viewCount"`
			} `json:"statistics"`
		} `json:"items"`
	}
	if resp.StatusCode == http.StatusOK {
		err = json.NewDecoder(resp.Body).Decode(&response)
		if err != nil {
			return 0, 0, fmt.Errorf(fmt.Sprintf("Failed to decode json: %v", err))
		}
	} else {
		return 0, 0, fmt.Errorf("unable to fetch youtube subscribers: %s, %s", resp.Status, youtubeUrl)
	}

	if len(response.Items) < 1 {
		return 0, 0, errors.New("unable to fetch youtube subscribers, no response")
	}

	subscribers, err := strconv.Atoi(response.Items[0].Statistics.SubscriberCount)
	if err != nil {
		return 0, 0, errors.New("unable to fetch youtube subscribers, no response")
	}

	viewCount, err := strconv.Atoi(response.Items[0].Statistics.ViewCount)
	if err != nil {
		return 0, 0, errors.New("unable to fetch youtube view count, no response")
	}

	return subscribers, viewCount, nil
}
