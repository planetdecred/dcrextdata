package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/raedahgroup/dcrextdata/mempool"
	"github.com/raedahgroup/dcrextdata/postgres/models"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
)

func (pg PgDb) StoreMempool(ctx context.Context, mempoolDto mempool.Mempool) error {
	mempoolModel := mempoolDtoToModel(mempoolDto)
	err := mempoolModel.Insert(ctx, pg.db, boil.Infer())
	if err != nil {
		if !strings.Contains(err.Error(), "unique constraint") { // Ignore duplicate entries
			return err
		}
	}
	//  tx count 76, total size 54205 B, fees 0.00367100
	log.Infof("Added mempool entry at %s, tx count %2d, total size: %6d B, Total Fee: %010.8f",
		mempoolDto.Time.Format(dateTemplate), mempoolDto.NumberOfTransactions, mempoolDto.Size, mempoolDto.TotalFee)
	return nil
}

func mempoolDtoToModel(mempoolDto mempool.Mempool) models.Mempool {
	return models.Mempool{
		Time:                 mempoolDto.Time,
		FirstSeenTime:        null.TimeFrom(mempoolDto.FirstSeenTime),
		Size:                 null.IntFrom(int(mempoolDto.Size)),
		NumberOfTransactions: null.IntFrom(mempoolDto.NumberOfTransactions),
		Revocations:          null.IntFrom(mempoolDto.Revocations),
		Tickets:              null.IntFrom(mempoolDto.Tickets),
		Voters:               null.IntFrom(mempoolDto.Voters),
		Total:                null.Float64From(mempoolDto.Total),
		TotalFee:             null.Float64From(mempoolDto.TotalFee),
	}
}

func (pg *PgDb) LastMempoolBlockHeight() (height int64, err error) {
	rows := pg.db.QueryRow(lastMempoolBlockHeight)
	err = rows.Scan(&height)
	return
}

func (pg *PgDb) LastMempoolTime() (entryTime time.Time, err error) {
	rows := pg.db.QueryRow(lastMempoolEntryTime)
	err = rows.Scan(&entryTime)
	if err == sql.ErrNoRows {
		err = nil
	}
	return
}

func (pg *PgDb) MempoolCount(ctx context.Context) (int64, error) {
	return models.Mempools().Count(ctx, pg.db)
}

func (pg *PgDb) Mempools(ctx context.Context, offtset int, limit int) ([]mempool.MempoolDto, error) {
	mempoolSlice, err := models.Mempools(qm.OrderBy("time DESC"), qm.Offset(offtset), qm.Limit(limit)).All(ctx, pg.db)
	if err != nil {
		return nil, err
	}
	var result []mempool.MempoolDto
	for _, m := range mempoolSlice {
		result = append(result, mempool.MempoolDto{
			TotalFee:             m.TotalFee.Float64,
			FirstSeenTime:        m.FirstSeenTime.Time.Format(dateTemplate),
			Total:                m.Total.Float64,
			Voters:               m.Voters.Int,
			Tickets:              m.Tickets.Int,
			Revocations:          m.Revocations.Int,
			Time:                 m.Time.Format(dateTemplate),
			Size:                 int32(m.Size.Int),
			NumberOfTransactions: m.NumberOfTransactions.Int,
		})
	}
	return result, nil
}

func (pg *PgDb) MempoolsChartData(ctx context.Context, chartFilter string) (models.MempoolSlice, error) {
	mempoolChartSlice, err := models.Mempools(qm.Select(fmt.Sprintf("%s, time", chartFilter)), qm.OrderBy("time")).All(ctx, pg.db)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	return mempoolChartSlice, nil
}

func (pg *PgDb) SaveBlock(ctx context.Context, block mempool.Block) error {
	blockModel := blockDtoToModel(block)
	err := blockModel.Insert(ctx, pg.db, boil.Infer())
	if err != nil {
		if !strings.Contains(err.Error(), "unique constraint") { // Ignore duplicate entries
			return err
		}
	}

	votes, err := pg.votesByBlock(ctx, int64(block.BlockHeight))
	if err == nil {
		for _, vote := range votes {
			voteModel, err := models.FindVote(ctx, pg.db, vote.Hash)
			if err == nil {
				voteModel.BlockReceiveTime = null.TimeFrom(block.BlockReceiveTime)
				voteModel.BlockHash = null.StringFrom(block.BlockHash)
				_, err = voteModel.Update(ctx, pg.db, boil.Infer())
				if err != nil {
					log.Errorf("Unable to fetch vote for block receive time update: %s", err.Error())
				}
			}
		}
	}

	log.Infof("New block received at %s, Height: %d, Hash: ...%s",
		block.BlockReceiveTime.Format(dateMiliTemplate), block.BlockHeight, block.BlockHash[len(block.BlockHash)-23:])
	return nil
}

func blockDtoToModel(block mempool.Block) models.Block {
	return models.Block{
		Height:            int(block.BlockHeight),
		Hash:              null.StringFrom(block.BlockHash),
		InternalTimestamp: null.TimeFrom(block.BlockInternalTime),
		ReceiveTime:       null.TimeFrom(block.BlockReceiveTime),
	}
}

func (pg *PgDb) BlockCount(ctx context.Context) (int64, error) {
	return models.Blocks().Count(ctx, pg.db)
}

func (pg *PgDb) Blocks(ctx context.Context, offset int, limit int) ([]mempool.BlockDto, error) {
	blockSlice, err := models.Blocks(qm.OrderBy(fmt.Sprintf("%s DESC", models.BlockColumns.ReceiveTime)), qm.Offset(offset), qm.Limit(limit)).All(ctx, pg.db)
	if err != nil {
		return nil, err
	}

	var blocks []mempool.BlockDto

	for _, block := range blockSlice {
		timeDiff := block.ReceiveTime.Time.Sub(block.InternalTimestamp.Time).Seconds()

		votes, err := pg.votesByBlock(ctx, int64(block.Height))
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}

		blocks = append(blocks, mempool.BlockDto{
			BlockHash:         block.Hash.String,
			BlockHeight:       uint32(block.Height),
			BlockInternalTime: block.InternalTimestamp.Time.Format(dateTemplate),
			BlockReceiveTime:  block.ReceiveTime.Time.Format(dateTemplate),
			Delay:             fmt.Sprintf("%04.2f", timeDiff),
			Votes:             votes,
		})
	}

	return blocks, nil
}

func (pg *PgDb) BlocksWithoutVotes(ctx context.Context, offset int, limit int) ([]mempool.BlockDto, error) {
	blockSlice, err := models.Blocks(qm.OrderBy(fmt.Sprintf("%s DESC", models.BlockColumns.ReceiveTime)), qm.Offset(offset), qm.Limit(limit)).All(ctx, pg.db)
	if err != nil {
		return nil, err
	}

	var blocks []mempool.BlockDto

	for _, block := range blockSlice {
		timeDiff := block.ReceiveTime.Time.Sub(block.InternalTimestamp.Time).Seconds()

		blocks = append(blocks, mempool.BlockDto{
			BlockHash:         block.Hash.String,
			BlockHeight:       uint32(block.Height),
			BlockInternalTime: block.InternalTimestamp.Time.Format(dateTemplate),
			BlockReceiveTime:  block.ReceiveTime.Time.Format(dateTemplate),
			Delay:             fmt.Sprintf("%04.2f", timeDiff),
		})
	}

	return blocks, nil
}

func (pg *PgDb) getBlock(ctx context.Context, height int) (*models.Block, error) {
	block, err := models.Blocks(models.BlockWhere.Height.EQ(height)).One(ctx, pg.db)
	if err != nil {
		return nil, err
	}

	return block, nil
}

func (pg *PgDb) SaveVote(ctx context.Context, vote mempool.Vote) error {
	voteModel := models.Vote{
		Hash:              vote.Hash,
		VotingOn:          null.Int64From(int64(vote.VotingOn)),
		BlockHash:         null.StringFrom(vote.BlockHash),
		ReceiveTime:       null.TimeFrom(vote.ReceiveTime),
		TargetedBlockTime: null.TimeFrom(vote.TargetedBlockTime),
		ValidatorID:       null.IntFrom(vote.ValidatorId),
		Validity:          null.StringFrom(vote.Validity),
	}

	// get the target block
	block, err := pg.getBlock(ctx, int(vote.VotingOn))
	if err == nil {
		voteModel.BlockReceiveTime = null.TimeFrom(block.ReceiveTime.Time)
	}

	err = voteModel.Insert(ctx, pg.db, boil.Infer())
	if err != nil {
		if strings.Contains(err.Error(), "unique constraint") { // Ignore duplicate entries
			return nil
		}
		return err
	}

	log.Infof("New vote received at %s for %d, Validator Id %d, Hash ...%s",
		vote.ReceiveTime.Format(dateMiliTemplate), vote.VotingOn, vote.ValidatorId, vote.Hash[len(vote.Hash)-23:])
	return nil
}

func (pg *PgDb) Votes(ctx context.Context, offset int, limit int) ([]mempool.VoteDto, error) {
	voteSlice, err := models.Votes(qm.OrderBy(fmt.Sprintf("%s DESC", models.BlockColumns.ReceiveTime)), qm.Offset(offset), qm.Limit(limit)).All(ctx, pg.db)
	if err != nil {
		return nil, err
	}

	var votes []mempool.VoteDto
	for _, vote := range voteSlice {
		votes = append(votes, pg.voteModelToDto(vote))
	}

	return votes, nil
}

func (pg *PgDb) votesByBlock(ctx context.Context, blockHeight int64) ([]mempool.VoteDto, error) {
	voteSlice, err := models.Votes(models.VoteWhere.VotingOn.EQ(null.Int64From(blockHeight)),
		qm.OrderBy(models.BlockColumns.ReceiveTime)).All(ctx, pg.db)
	if err != nil {
		return nil, err
	}

	var votes []mempool.VoteDto
	for _, vote := range voteSlice {
		votes = append(votes, pg.voteModelToDto(vote))
	}

	return votes, nil
}

func (pg *PgDb) voteModelToDto(vote *models.Vote) mempool.VoteDto {
	timeDiff := vote.ReceiveTime.Time.Sub(vote.TargetedBlockTime.Time).Seconds()
	blockReceiveTimeDiff := vote.ReceiveTime.Time.Sub(vote.BlockReceiveTime.Time).Seconds()
	var shortBlockHash string
	if len(vote.BlockHash.String) > 0 {
		shortBlockHash = vote.BlockHash.String[len(vote.BlockHash.String)-8:]
	}

	return mempool.VoteDto{
		Hash:                  vote.Hash,
		ReceiveTime:           vote.ReceiveTime.Time.Format(dateTemplate),
		TargetedBlockTimeDiff: fmt.Sprintf("%04.2f", timeDiff),
		BlockReceiveTimeDiff:  fmt.Sprintf("%04.2f", blockReceiveTimeDiff),
		VotingOn:              vote.VotingOn.Int64,
		BlockHash:             vote.BlockHash.String,
		ShortBlockHash:        shortBlockHash,
		ValidatorId:           vote.ValidatorID.Int,
		Validity:              vote.Validity.String,
	}
}

func (pg *PgDb) VotesCount(ctx context.Context) (int64, error) {
	return models.Votes().Count(ctx, pg.db)
}

func (pg *PgDb) PropagationChartData(ctx context.Context) ([]mempool.PropagationChartData, error) {
	voteSlice, err := models.Votes(qm.OrderBy(models.VoteColumns.VotingOn)).All(ctx, pg.db)
	if err != nil {
		return nil, err
	}

	var chartData []mempool.PropagationChartData
	for _, vote := range voteSlice {
		// timeDiff := vote.ReceiveTime.Time.Sub(vote.BlockReceiveTime.Time).Seconds()
		timeDiff := vote.ReceiveTime.Time.Sub(vote.TargetedBlockTime.Time).Seconds()
		chartData = append(chartData, mempool.PropagationChartData{
			BlockHeight: vote.VotingOn.Int64, TimeDifference: timeDiff,
		})
	}

	return chartData, nil
}
