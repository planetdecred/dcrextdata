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
)

var repositories []string

func Repositories() []string {
	return repositories
}

func (c *Collector) startGithubCollector(ctx context.Context) {
	var lastCollectionDate time.Time
	err := c.dataStore.LastEntry(ctx, models.TableNames.Github, &lastCollectionDate)
	if err != nil && err != sql.ErrNoRows {
		log.Errorf("Cannot fetch last Github entry time, %s", err.Error())
		return
	}

	secondsPassed := time.Since(lastCollectionDate)
	period := time.Duration(c.options.TwitterStatInterval) * time.Minute

	if secondsPassed < period {
		timeLeft := period - secondsPassed
		log.Infof("Fetching Github stats every %dm, collected %s ago, will fetch in %s.", period/time.Minute, helpers.DurationToString(secondsPassed),
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
	c.collectAndStoreGithubStat(ctx)
	app.ReleaseForNewModule()

	ticker := time.NewTicker(time.Duration(c.options.TwitterStatInterval) * time.Minute)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			registerStarter()
			c.collectAndStoreGithubStat(ctx)
			app.ReleaseForNewModule()
		}
	}
}

func (c *Collector) collectAndStoreGithubStat(ctx context.Context) {
	log.Info("Starting Github stats collection cycle")
	for _, repo := range c.options.GithubRepositories {
		githubStars, githubFolks, err := c.getGithubData(ctx, repo)
		for retry := 0; err != nil; retry++ {
			if retry == retryLimit {
				log.Error(err)
				return
			}
			log.Warn(err)
			githubStars, githubFolks, err = c.getGithubData(ctx, repo)
		}

		githubStat := Github{
			Date:       helpers.NowUTC(),
			Repository: repo,
			Stars:      githubStars,
			Folks:      githubFolks,
		}
		err = c.dataStore.StoreGithubStat(ctx, githubStat)
		if err != nil {
			log.Error("Unable to save Github stat, %s", err.Error())
			return
		}

		log.Infof("New Github stat collected for %s at %s, Stars %d, Folks %d", repo,
			githubStat.Date.Format(dateMiliTemplate), githubStars, githubFolks)
	}
}

func (c *Collector) getGithubData(ctx context.Context, repository string) (int, int, error) {
	if ctx.Err() != nil {
		return 0, 0, ctx.Err()
	}

	request, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/"+repository, nil)
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
		Stars int `json:"stargazers_count"`
		Folks int `json:"network_count"`
	}
	if resp.StatusCode == http.StatusOK {
		err = json.NewDecoder(resp.Body).Decode(&response)
		if err != nil {
			return 0, 0, fmt.Errorf(fmt.Sprintf("Failed to decode json: %v", err))
		}
	} else {
		return 0, 0, fmt.Errorf("unable to fetch youtube subscribers: %s", resp.Status)
	}

	return response.Stars, response.Folks, nil
}
