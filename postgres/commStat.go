package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/dgraph-io/badger"
	"github.com/planetdecred/dcrextdata/cache"
	"github.com/planetdecred/dcrextdata/commstats"
	"github.com/planetdecred/dcrextdata/postgres/models"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
)

func (pg *PgDb) StoreRedditStat(ctx context.Context, stat commstats.Reddit) error {
	reddit := models.Reddit{
		Date:           stat.Date,
		Subscribers:    stat.Subscribers,
		ActiveAccounts: stat.AccountsActive,
		Subreddit:      stat.Subreddit,
	}

	err := reddit.Insert(ctx, pg.db, boil.Infer())
	if err != nil {
		if strings.Contains(err.Error(), "unique constraint") { // Ignore duplicate entries
			return nil
		}
	}

	return err
}

func (pg *PgDb) LastCommStatEntry() (entryTime time.Time) {
	rows := pg.db.QueryRow(lastCommStatEntryTime)
	_ = rows.Scan(&entryTime)
	return
}

func (pg *PgDb) CountRedditStat(ctx context.Context, subreddit string) (int64, error) {
	return models.Reddits(models.RedditWhere.Subreddit.EQ(subreddit)).Count(ctx, pg.db)
}

func (pg *PgDb) RedditStats(ctx context.Context, subreddit string, offtset int, limit int) ([]commstats.Reddit, error) {
	redditSlices, err := models.Reddits(models.RedditWhere.Subreddit.EQ(subreddit),
		qm.OrderBy(fmt.Sprintf("%s DESC", models.RedditColumns.Date)),
		qm.Offset(offtset), qm.Limit(limit)).All(ctx, pg.db)
	if err != nil {
		return nil, err
	}

	var result []commstats.Reddit
	for _, record := range redditSlices {
		stat := commstats.Reddit{
			Date:           record.Date,
			Subreddit:      record.Subreddit,
			Subscribers:    record.Subscribers,
			AccountsActive: record.ActiveAccounts,
		}

		result = append(result, stat)
	}
	return result, nil
}

// twitter
func (pg *PgDb) StoreTwitterStat(ctx context.Context, twitter commstats.Twitter) error {
	twitterModel := models.Twitter{
		Date:      twitter.Date,
		Followers: twitter.Followers,
		Handle:    twitter.Handle,
	}

	err := twitterModel.Insert(ctx, pg.db, boil.Infer())
	if err != nil {
		if strings.Contains(err.Error(), "unique constraint") { // Ignore duplicate entries
			return nil
		}
	}

	return err
}

func (pg *PgDb) CountTwitterStat(ctx context.Context, handle string) (int64, error) {
	return models.Twitters(models.TwitterWhere.Handle.EQ(handle)).Count(ctx, pg.db)
}

func (pg *PgDb) TwitterStats(ctx context.Context, handle string, offtset int, limit int) ([]commstats.Twitter, error) {
	statSlice, err := models.Twitters(
		models.TwitterWhere.Handle.EQ(handle),
		qm.OrderBy(fmt.Sprintf("%s DESC", models.TwitterColumns.Date)),
		qm.Offset(offtset), qm.Limit(limit)).All(ctx, pg.db)
	if err != nil {
		return nil, err
	}

	var result []commstats.Twitter
	for _, record := range statSlice {
		stat := commstats.Twitter{
			Date:      record.Date,
			Followers: record.Followers,
		}

		result = append(result, stat)
	}
	return result, nil
}

// youtube
func (pg *PgDb) StoreYoutubeStat(ctx context.Context, youtube commstats.Youtube) error {
	youtubeModel := models.Youtube{
		Date:        youtube.Date,
		Subscribers: youtube.Subscribers,
		ViewCount:   youtube.ViewCount,
		Channel:     youtube.Channel,
	}

	err := youtubeModel.Insert(ctx, pg.db, boil.Infer())
	if err != nil {
		if strings.Contains(err.Error(), "unique constraint") { // Ignore duplicate entries
			return nil
		}
	}

	return err
}

func (pg *PgDb) CountYoutubeStat(ctx context.Context, channel string) (int64, error) {
	return models.Youtubes(models.YoutubeWhere.Channel.EQ(channel)).Count(ctx, pg.db)
}

func (pg *PgDb) YoutubeStat(ctx context.Context, channel string, offtset int, limit int) ([]commstats.Youtube, error) {
	statSlice, err := models.Youtubes(
		models.YoutubeWhere.Channel.EQ(channel),
		qm.OrderBy(fmt.Sprintf("%s DESC", models.YoutubeColumns.Date)),
		qm.Offset(offtset), qm.Limit(limit)).All(ctx, pg.db)
	if err != nil {
		return nil, err
	}

	var result []commstats.Youtube
	for _, record := range statSlice {
		stat := commstats.Youtube{
			Date:        record.Date,
			Subscribers: record.Subscribers,
			ViewCount:   record.ViewCount,
			Channel:     record.Channel,
		}

		result = append(result, stat)
	}
	return result, nil
}

// github
func (pg *PgDb) StoreGithubStat(ctx context.Context, github commstats.Github) error {
	githubModel := models.Github{
		Date:       github.Date,
		Repository: github.Repository,
		Stars:      github.Stars,
		Folks:      github.Folks,
	}

	err := githubModel.Insert(ctx, pg.db, boil.Infer())
	if err != nil {
		if strings.Contains(err.Error(), "unique constraint") { // Ignore duplicate entries
			return nil
		}
	}

	return err
}

func (pg *PgDb) CountGithubStat(ctx context.Context, repository string) (int64, error) {
	return models.Githubs(models.GithubWhere.Repository.EQ(repository)).Count(ctx, pg.db)
}

func (pg *PgDb) GithubStat(ctx context.Context, repository string, offtset int, limit int) ([]commstats.Github, error) {
	statSlice, err := models.Githubs(
		models.GithubWhere.Repository.EQ(repository),
		qm.OrderBy(fmt.Sprintf("%s DESC", models.GithubColumns.Date)),
		qm.Offset(offtset), qm.Limit(limit)).All(ctx, pg.db)
	if err != nil {
		return nil, err
	}

	var result []commstats.Github
	for _, record := range statSlice {
		stat := commstats.Github{
			Date:  record.Date,
			Folks: record.Folks,
			Stars: record.Stars,
		}

		result = append(result, stat)
	}
	return result, nil
}

func (pg *PgDb) CommunityChart(ctx context.Context, platform string, dataType string, filters map[string]string) (stats []commstats.ChartData, err error) {
	dataType = strings.ToLower(dataType)

	var templateArgs = []interface{}{dataType, platform}
	sqlTemplate := "SELECT date, %s as record FROM %s"
	var wheres []string
	for attribute, value := range filters {
		wheres = append(wheres, fmt.Sprintf("%s = %s", attribute, value))
	}
	if len(wheres) > 0 {
		sqlTemplate += fmt.Sprintf(" where %s", strings.Join(wheres, " and "))
	}
	sqlTemplate += " ORDER BY date"
	query := fmt.Sprintf(sqlTemplate, templateArgs...)

	rows, err := pg.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var rec commstats.ChartData
		err = rows.Scan(&rec.Date, &rec.Record)
		if err != nil {
			return nil, err
		}
		stats = append(stats, rec)
	}
	return
}

var (
	redditPlatform  = "Reddit"
	twitterPlatform = "Twitter"
	githubPlatform  = "GitHub"
	youtubePlatform = "YouTube"
)

func (pg *PgDb) fetchAppendCommunityChart(ctx context.Context,
	cacheManager *cache.Manager, page int) (interface{}, func(), bool, error) {

	txn := cacheManager.DB.NewTransaction(true)
	defer txn.Discard()

	if err := pg.fetchAppendGithubChart(ctx, cacheManager, txn); err != nil {
		return nil, func() {}, true, err
	}

	if err := pg.fetchAppendRedditChart(ctx, cacheManager, txn); err != nil {
		return nil, func() {}, true, err
	}

	if err := pg.fetchAppendTwitterChart(ctx, cacheManager, txn); err != nil {
		return nil, func() {}, true, err
	}

	if err := pg.fetchAppendYouTubeChart(ctx, cacheManager, txn); err != nil {
		return nil, func() {}, true, err
	}

	if err := txn.Commit(); err != nil {
		return nil, func() {}, true, err
	}

	return nil, func() {}, true, nil
}

func (pg *PgDb) fetchAppendYouTubeChart(ctx context.Context, cacheManager *cache.Manager, txn *badger.Txn) error {
	var channels = commstats.YoutubeChannels()
	columns := []string{models.YoutubeColumns.Subscribers, models.YoutubeColumns.ViewCount}
	for _, channel := range channels {
		filter := map[string]string{models.YoutubeColumns.Channel: fmt.Sprintf("'%s'", channel)}
		for _, dataType := range columns {
			data, err := pg.CommunityChart(ctx, youtubePlatform, dataType, filter)
			if err != nil && err.Error() != sql.ErrNoRows.Error() {
				return err
			}
			var dates, records cache.ChartUints
			for _, record := range data {
				dates = append(dates, uint64(record.Date.Unix()))
				records = append(records, uint64(record.Record))
			}
			dateKey := fmt.Sprintf("%s-%s-%s-%s", cache.Community, youtubePlatform, channel, cache.TimeAxis)
			if err = cacheManager.SaveValTx(dateKey, dates, txn); err != nil {
				return err
			}
			dataKey := fmt.Sprintf("%s-%s-%s-%s", cache.Community, youtubePlatform, channel, dataType)
			if err = cacheManager.SaveValTx(dataKey, records, txn); err != nil {
				return err
			}
		}
	}

	return nil
}

func (pg *PgDb) fetchAppendTwitterChart(ctx context.Context, cacheManager *cache.Manager, txn *badger.Txn) error {
	var twitterHandles = commstats.TwitterHandles()
	columns := []string{models.TwitterColumns.Followers}
	for _, handle := range twitterHandles {
		filter := map[string]string{models.TwitterColumns.Handle: fmt.Sprintf("'%s'", handle)}
		for _, dataType := range columns {
			data, err := pg.CommunityChart(ctx, twitterPlatform, dataType, filter)
			if err != nil && err.Error() != sql.ErrNoRows.Error() {
				return err
			}
			var dates, records cache.ChartUints
			for _, record := range data {
				dates = append(dates, uint64(record.Date.Unix()))
				records = append(records, uint64(record.Record))
			}
			dateKey := fmt.Sprintf("%s-%s-%s-%s", cache.Community, twitterPlatform, handle, cache.TimeAxis)
			if err = cacheManager.SaveValTx(dateKey, dates, txn); err != nil {
				return err
			}
			dataKey := fmt.Sprintf("%s-%s-%s-%s", cache.Community, twitterPlatform, handle, dataType)
			if err = cacheManager.SaveValTx(dataKey, records, txn); err != nil {
				return err
			}
		}
	}

	return nil
}

func (pg *PgDb) fetchAppendRedditChart(ctx context.Context, cacheManager *cache.Manager, txn *badger.Txn) error {
	var subreddits = commstats.Subreddits()
	columns := []string{models.RedditColumns.ActiveAccounts, models.RedditColumns.Subscribers}
	for _, subreddit := range subreddits {
		filter := map[string]string{models.RedditColumns.Subreddit: fmt.Sprintf("'%s'", subreddit)}
		for _, dataType := range columns {
			data, err := pg.CommunityChart(ctx, redditPlatform, dataType, filter)
			if err != nil && err.Error() != sql.ErrNoRows.Error() {
				return err
			}
			var dates, records cache.ChartUints
			for _, record := range data {
				dates = append(dates, uint64(record.Date.Unix()))
				records = append(records, uint64(record.Record))
			}
			dateKey := fmt.Sprintf("%s-%s-%s-%s", cache.Community, redditPlatform, subreddit, cache.TimeAxis)
			if err = cacheManager.SaveValTx(dateKey, dates, txn); err != nil {
				return err
			}
			dataKey := fmt.Sprintf("%s-%s-%s-%s", cache.Community, redditPlatform, subreddit, dataType)
			if err = cacheManager.SaveValTx(dataKey, records, txn); err != nil {
				return err
			}
		}
	}

	return nil
}

func (pg *PgDb) fetchAppendGithubChart(ctx context.Context, cacheManager *cache.Manager, txn *badger.Txn) error {
	var repositories = commstats.Repositories()
	columns := []string{models.GithubColumns.Stars, models.GithubColumns.Folks}
	for _, repo := range repositories {
		filter := map[string]string{models.GithubColumns.Repository: fmt.Sprintf("'%s'", repo)}
		for _, dataType := range columns {
			data, err := pg.CommunityChart(ctx, githubPlatform, dataType, filter)
			if err != nil && err.Error() != sql.ErrNoRows.Error() {
				return err
			}
			var dates, records cache.ChartUints
			for _, record := range data {
				dates = append(dates, uint64(record.Date.Unix()))
				records = append(records, uint64(record.Record))
			}
			dateKey := fmt.Sprintf("%s-%s-%s-%s", cache.Community, githubPlatform, repo, cache.TimeAxis)
			if err = cacheManager.SaveValTx(dateKey, dates, txn); err != nil {
				return err
			}
			dataKey := fmt.Sprintf("%s-%s-%s-%s", cache.Community, githubPlatform, repo, dataType)
			if err = cacheManager.SaveValTx(dataKey, records, txn); err != nil {
				return err
			}
		}
	}

	return nil
}
