package postgres

import (
	"context"
	"fmt"

	"github.com/planetdecred/dcrextdata/postgres/models"
)

func (pg *PgDb) TableNames() []string {
	return []string{
		models.TableNames.Vote,
		models.TableNames.Block,
		models.TableNames.Mempool,
		models.TableNames.Exchange,
		models.TableNames.ExchangeTick,
		models.TableNames.VSP,
		models.TableNames.VSPTick,
		models.TableNames.PowData,
	}
}

func (pg *PgDb) LastEntry(ctx context.Context, tableName string, receiver interface{}) error {
	var columnName string
	switch tableName {
	case models.TableNames.Exchange:
		columnName = models.ExchangeColumns.ID
	case models.TableNames.ExchangeTick:
		columnName = models.ExchangeTickColumns.ID
	case models.TableNames.Mempool:
		columnName = models.MempoolColumns.Time
	case models.TableNames.Block:
		columnName = models.BlockColumns.Height
	case models.TableNames.Vote:
		columnName = models.VoteColumns.ReceiveTime
	case models.TableNames.PowData:
		columnName = models.PowDatumColumns.Time
	case models.TableNames.VSP:
		columnName = models.VSPColumns.ID
	case models.TableNames.VSPTick:
		columnName = models.VSPTickColumns.ID
	case models.TableNames.Reddit:
		columnName = models.RedditColumns.Date
	case models.TableNames.Twitter:
		columnName = models.TwitterColumns.Date
	case models.TableNames.Github:
		columnName = models.GithubColumns.Date
	case models.TableNames.Youtube:
		columnName = models.YoutubeColumns.Date
	case models.TableNames.NetworkSnapshot:
		columnName = models.NetworkSnapshotColumns.Timestamp
	}

	rows := pg.db.QueryRow(fmt.Sprintf("SELECT %s FROM %s ORDER BY %s DESC LIMIT 1", columnName, tableName, columnName))
	err := rows.Scan(receiver)
	return err

}
