package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/raedahgroup/dcrextdata/commstats"
	"github.com/raedahgroup/dcrextdata/postgres/models"
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
			Subreddit:		record.Subreddit,
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
	}

	err := twitterModel.Insert(ctx, pg.db, boil.Infer())
	if err != nil {
		if strings.Contains(err.Error(), "unique constraint") { // Ignore duplicate entries
			return nil
		}
	}

	return err
}

func (pg *PgDb) CountTwitterStat(ctx context.Context) (int64, error) {
	return models.Twitters().Count(ctx, pg.db)
}

func (pg *PgDb) TwitterStats(ctx context.Context, offtset int, limit int) ([]commstats.Twitter, error) {
	statSlice, err := models.Twitters(
		qm.OrderBy(fmt.Sprintf("%s DESC", models.TwitterColumns.Date)),
		qm.Offset(offtset), qm.Limit(limit)).All(ctx, pg.db)
	if err != nil {
		return nil, err
	}

	var result []commstats.Twitter
	for _, record := range statSlice {
		stat := commstats.Twitter{
			Date:           record.Date,
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
	}

	err := youtubeModel.Insert(ctx, pg.db, boil.Infer())
	if err != nil {
		if strings.Contains(err.Error(), "unique constraint") { // Ignore duplicate entries
			return nil
		}
	}

	return err
}

func (pg *PgDb) CountYoutubeStat(ctx context.Context) (int64, error) {
	return models.Youtubes().Count(ctx, pg.db)
}

func (pg *PgDb) YoutubeStat(ctx context.Context, offtset int, limit int) ([]commstats.Youtube, error) {
	statSlice, err := models.Youtubes(
		qm.OrderBy(fmt.Sprintf("%s DESC", models.YoutubeColumns.Date)),
		qm.Offset(offtset), qm.Limit(limit)).All(ctx, pg.db)
	if err != nil {
		return nil, err
	}

	var result []commstats.Youtube
	for _, record := range statSlice {
		stat := commstats.Youtube{
			Date:           record.Date,
			Subscribers: record.Subscribers,
		}

		result = append(result, stat)
	}
	return result, nil
}

// github
func (pg *PgDb) StoreGithubStat(ctx context.Context, github commstats.Github) error {
	githubModel := models.Github{
		Date:  github.Date,
		Stars: github.Stars,
		Folks: github.Folks,
	}

	err := githubModel.Insert(ctx, pg.db, boil.Infer())
	if err != nil {
		if strings.Contains(err.Error(), "unique constraint") { // Ignore duplicate entries
			return nil
		}
	}

	return err
}

func (pg *PgDb) CountGithubStat(ctx context.Context) (int64, error) {
	return models.Githubs().Count(ctx, pg.db)
}

func (pg *PgDb) GithubStat(ctx context.Context, offtset int, limit int) ([]commstats.Github, error) {
	statSlice, err := models.Githubs(
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
