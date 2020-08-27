package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/planetdecred/dcrextdata/cache"
	"github.com/planetdecred/dcrextdata/mempool"
	"github.com/planetdecred/dcrextdata/postgres/models"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
)

func (pg PgDb) MempoolTableName() string {
	return models.TableNames.Mempool
}

func (pg PgDb) BlockTableName() string {
	return models.TableNames.Block
}

func (pg PgDb) VoteTableName() string {
	return models.TableNames.Vote
}

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
	if err = pg.UpdateMempoolAggregateData(ctx); err != nil {
		return err
	}
	return nil
}

func (pg PgDb) StoreMempoolFromSync(ctx context.Context, mempoolDto interface{}) error {
	mempoolModel := mempoolDtoToModel(mempoolDto.(mempool.Mempool))
	err := mempoolModel.Insert(ctx, pg.db, boil.Infer())
	if isUniqueConstraint(err) {
		return nil
	}
	return err
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

func (pg *PgDb) Mempools(ctx context.Context, offtset int, limit int) ([]mempool.Dto, error) {
	mempoolSlice, err := models.Mempools(qm.OrderBy("time DESC"), qm.Offset(offtset), qm.Limit(limit)).All(ctx, pg.db)
	if err != nil {
		return nil, err
	}
	var result []mempool.Dto
	for _, m := range mempoolSlice {
		result = append(result, mempool.Dto{
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

func (pg *PgDb) FetchMempoolForSync(ctx context.Context, date time.Time, offtset int, limit int) ([]mempool.Mempool, int64, error) {
	mempoolSlice, err := models.Mempools(
		models.MempoolWhere.Time.GT(date),
		qm.OrderBy(models.MempoolColumns.Time), qm.Offset(offtset), qm.Limit(limit)).All(ctx, pg.db)
	if err != nil {
		return nil, 0, err
	}
	var result []mempool.Mempool
	for _, m := range mempoolSlice {
		result = append(result, mempool.Mempool{
			TotalFee:             m.TotalFee.Float64,
			FirstSeenTime:        m.FirstSeenTime.Time,
			Total:                m.Total.Float64,
			Voters:               m.Voters.Int,
			Tickets:              m.Tickets.Int,
			Revocations:          m.Revocations.Int,
			Time:                 m.Time,
			Size:                 int32(m.Size.Int),
			NumberOfTransactions: m.NumberOfTransactions.Int,
		})
	}
	totalCount, err := models.Mempools(models.MempoolWhere.Time.GTE(date)).Count(ctx, pg.db)

	return result, totalCount, err
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

	log.Infof("New block received at %s, PropagationHeight: %d, Hash: ...%s",
		block.BlockReceiveTime.Format(dateMiliTemplate), block.BlockHeight, block.BlockHash[len(block.BlockHash)-23:])
	return nil
}

func (pg *PgDb) SaveBlockFromSync(ctx context.Context, block interface{}) error {
	blockModel := blockDtoToModel(block.(mempool.Block))
	err := blockModel.Insert(ctx, pg.db, boil.Infer())
	if err != nil {
		if !strings.Contains(err.Error(), "unique constraint") { // Ignore duplicate entries
			return err
		}
	}
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
	blockSlice, err := models.Blocks(qm.OrderBy(fmt.Sprintf("%s DESC", models.BlockColumns.ReceiveTime)),
		qm.Offset(offset), qm.Limit(limit)).All(ctx, pg.db)

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

func (pg *PgDb) FetchBlockForSync(ctx context.Context, blockHeight int64, offtset int, limit int) ([]mempool.Block, int64, error) {
	blockSlice, err := models.Blocks(
		models.BlockWhere.Height.GT(int(blockHeight)),
		qm.OrderBy(models.BlockColumns.ReceiveTime),
		qm.Offset(offtset), qm.Limit(limit)).All(ctx, pg.db)
	if err != nil {
		return nil, 0, err
	}
	var result []mempool.Block
	for _, block := range blockSlice {
		result = append(result, mempool.Block{
			BlockHash:         block.Hash.String,
			BlockHeight:       uint32(block.Height),
			BlockInternalTime: block.InternalTimestamp.Time,
			BlockReceiveTime:  block.ReceiveTime.Time,
		})
	}
	totalCount, err := models.Blocks(models.BlockWhere.Height.GT(int(blockHeight))).Count(ctx, pg.db)

	return result, totalCount, err
}

func (pg *PgDb) SaveVote(ctx context.Context, vote mempool.Vote) error {
	voteModel := models.Vote{
		Hash:              vote.Hash,
		VotingOn:          null.Int64From(vote.VotingOn),
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

func (pg *PgDb) SaveVoteFromSync(ctx context.Context, voteData interface{}) error {
	vote := voteData.(mempool.Vote)
	voteModel := models.Vote{
		Hash:              vote.Hash,
		VotingOn:          null.Int64From(vote.VotingOn),
		BlockHash:         null.StringFrom(vote.BlockHash),
		ReceiveTime:       null.TimeFrom(vote.ReceiveTime),
		BlockReceiveTime:  null.TimeFrom(vote.BlockReceiveTime),
		TargetedBlockTime: null.TimeFrom(vote.TargetedBlockTime),
		ValidatorID:       null.IntFrom(vote.ValidatorId),
		Validity:          null.StringFrom(vote.Validity),
	}

	err := voteModel.Insert(ctx, pg.db, boil.Infer())
	if err != nil {
		if strings.Contains(err.Error(), "unique constraint") { // Ignore duplicate entries
			return nil
		}
	}
	return err
}

func (pg *PgDb) Votes(ctx context.Context, offset int, limit int) ([]mempool.VoteDto, error) {
	voteSlice, err := models.Votes(qm.OrderBy(fmt.Sprintf("%s DESC", models.BlockColumns.ReceiveTime)), qm.Offset(offset), qm.Limit(limit)).All(ctx, pg.db)
	if err != nil {
		return nil, err
	}

	var votes = make([]mempool.VoteDto, len(voteSlice))
	for i, vote := range voteSlice {
		votes[i] = pg.voteModelToDto(vote)
	}

	return votes, nil
}

func (pg *PgDb) VotesByBlock(ctx context.Context, blockHash string) ([]mempool.VoteDto, error) {
	voteSlice, err := models.Votes(
		models.VoteWhere.BlockHash.EQ(null.StringFrom(blockHash)),
		qm.OrderBy(fmt.Sprintf("%s DESC", models.BlockColumns.ReceiveTime)),
	).All(ctx, pg.db)
	if err != nil {
		return nil, err
	}

	var votes = make([]mempool.VoteDto, len(voteSlice))
	for i, vote := range voteSlice {
		votes[i] = pg.voteModelToDto(vote)
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

func (pg *PgDb) FetchVoteForSync(ctx context.Context, date time.Time, offtset int, limit int) ([]mempool.Vote, int64, error) {
	voteSlices, err := models.Votes(
		models.VoteWhere.ReceiveTime.GTE(null.TimeFrom(date)),
		qm.OrderBy(models.VoteColumns.ReceiveTime),
		qm.Offset(offtset), qm.Limit(limit)).All(ctx, pg.db)
	if err != nil {
		return nil, 0, err
	}
	var result []mempool.Vote
	for _, vote := range voteSlices {
		result = append(result, mempool.Vote{
			Hash:              vote.Hash,
			ReceiveTime:       vote.ReceiveTime.Time,
			TargetedBlockTime: vote.TargetedBlockTime.Time,
			BlockReceiveTime:  vote.BlockReceiveTime.Time,
			VotingOn:          vote.VotingOn.Int64,
			BlockHash:         vote.BlockHash.String,
			ValidatorId:       vote.ValidatorID.Int,
			Validity:          vote.Validity.String,
		})
	}
	totalCount, err := models.Votes(models.VoteWhere.ReceiveTime.GTE(null.TimeFrom(date))).Count(ctx, pg.db)
	if err != nil {
		return nil, 0, err
	}

	return result, totalCount, nil
}

func (pg *PgDb) propagationVoteChartDataByHeight(ctx context.Context, height int32) ([]mempool.PropagationChartData, error) {
	voteSlice, err := models.Votes(
		models.VoteWhere.VotingOn.GT(null.Int64From(int64(height))),
		qm.OrderBy(models.VoteColumns.VotingOn)).All(ctx, pg.db)
	if err != nil {
		return nil, err
	}

	var chartData = make([]mempool.PropagationChartData, len(voteSlice))
	for i, vote := range voteSlice {
		blockReceiveTimeDiff := vote.ReceiveTime.Time.Sub(vote.BlockReceiveTime.Time).Seconds()
		chartData[i] = mempool.PropagationChartData{
			BlockHeight: vote.VotingOn.Int64, TimeDifference: blockReceiveTimeDiff,
		}
	}

	return chartData, nil
}

func (pg *PgDb) propagationBlockChartData(ctx context.Context, height int) ([]mempool.PropagationChartData, error) {
	blockSlice, err := models.Blocks(
		models.BlockWhere.Height.GT(height),
		qm.OrderBy(models.BlockColumns.Height)).All(ctx, pg.db)
	if err != nil {
		return nil, err
	}

	var chartData = make([]mempool.PropagationChartData, len(blockSlice))
	for i, block := range blockSlice {
		blockReceiveTimeDiff := block.ReceiveTime.Time.Sub(block.InternalTimestamp.Time).Seconds()
		chartData[i] = mempool.PropagationChartData{
			BlockHeight:    int64(block.Height),
			TimeDifference: blockReceiveTimeDiff,
			BlockTime:      block.InternalTimestamp.Time,
		}
	}

	return chartData, nil
}

func (pg *PgDb) fetchBlockReceiveTimeByHeight(ctx context.Context, height int32) ([]mempool.BlockReceiveTime, error) {
	blockSlice, err := models.Blocks(
		models.BlockWhere.Height.GT(int(height)),
		qm.Select(models.BlockColumns.Height, models.BlockColumns.ReceiveTime),
		qm.OrderBy(models.BlockColumns.Height),
	).All(ctx, pg.db)

	if err != nil {
		return nil, err
	}

	var chartData []mempool.BlockReceiveTime
	for _, block := range blockSlice {
		chartData = append(chartData, mempool.BlockReceiveTime{
			BlockHeight: int64(block.Height),
			ReceiveTime: block.ReceiveTime.Time,
		})
	}

	return chartData, nil
}

// *****CHARTS******* //

func (pg *PgDb) fetchEncodeMempoolChart(ctx context.Context, charts *cache.Manager, dataType,
	_ string, binString string, _ ...string) ([]byte, error) {

	switch dataType {
	case cache.MempoolSize:
		return pg.fetchEncodeMempoolSize(ctx, charts, binString)
	case cache.MempoolFees:
		return pg.fetchEncodeMempoolFee(ctx, charts, binString)

	case cache.MempoolTxCount:
		return pg.fetchEncodeMempoolTxCount(ctx, charts, binString)
	}
	return nil, cache.UnknownChartErr
}

func (pg *PgDb) fetchEncodeMempoolSize(ctx context.Context, charts *cache.Manager, binString string) ([]byte, error) {
	if binString == string(cache.DefaultBin) {
		mempoolSlice, err := models.Mempools(
			qm.Select(models.MempoolColumns.Time, models.MempoolColumns.Size),
			qm.OrderBy(models.MempoolColumns.Time),
		).All(ctx, pg.db)
		if err != nil {
			return nil, err
		}
		var time = make(cache.ChartUints, len(mempoolSlice))
		var data = make(cache.ChartUints, len(mempoolSlice))
		for i, m := range mempoolSlice {
			time[i] = uint64(m.Time.UTC().Unix())
			data[i] = uint64(m.Size.Int)
		}
		return charts.Encode(nil, time, data)
	}

	mempoolSlice, err := models.MempoolBins(
		models.MempoolBinWhere.Bin.EQ(binString),
		qm.Select(models.MempoolBinColumns.Time, models.MempoolBinColumns.Size),
		qm.OrderBy(models.MempoolBinColumns.Time),
	).All(ctx, pg.db)
	if err != nil {
		return nil, err
	}
	var time = make(cache.ChartUints, len(mempoolSlice))
	var data = make(cache.ChartUints, len(mempoolSlice))
	for i, m := range mempoolSlice {
		time[i] = uint64(m.Time)
		data[i] = uint64(m.Size.Int)
	}
	return charts.Encode(nil, time, data)
}

func (pg *PgDb) fetchEncodeMempoolFee(ctx context.Context, charts *cache.Manager, binString string) ([]byte, error) {
	if binString == string(cache.DefaultBin) {
		mempoolSlice, err := models.Mempools(
			qm.Select(models.MempoolColumns.Time, models.MempoolColumns.TotalFee),
			qm.OrderBy(models.MempoolColumns.Time),
		).All(ctx, pg.db)
		if err != nil {
			return nil, err
		}
		var time = make(cache.ChartUints, len(mempoolSlice))
		var data = make(cache.ChartFloats, len(mempoolSlice))
		for i, m := range mempoolSlice {
			time[i] = uint64(m.Time.UTC().Unix())
			data[i] = m.TotalFee.Float64
		}
		return charts.Encode(nil, time, data)
	}

	mempoolSlice, err := models.MempoolBins(
		models.MempoolBinWhere.Bin.EQ(binString),
		qm.Select(models.MempoolBinColumns.Time, models.MempoolBinColumns.TotalFee),
		qm.OrderBy(models.MempoolBinColumns.Time),
	).All(ctx, pg.db)
	if err != nil {
		return nil, err
	}
	var time = make(cache.ChartUints, len(mempoolSlice))
	var data = make(cache.ChartFloats, len(mempoolSlice))
	for i, m := range mempoolSlice {
		time[i] = uint64(m.Time)
		data[i] = m.TotalFee.Float64
	}
	return charts.Encode(nil, time, data)
}

func (pg *PgDb) fetchEncodeMempoolTxCount(ctx context.Context, charts *cache.Manager, binString string) ([]byte, error) {
	if binString == string(cache.DefaultBin) {
		mempoolSlice, err := models.Mempools(
			qm.Select(models.MempoolColumns.Time, models.MempoolColumns.NumberOfTransactions),
			qm.OrderBy(models.MempoolColumns.Time),
		).All(ctx, pg.db)
		if err != nil {
			return nil, err
		}
		var time = make(cache.ChartUints, len(mempoolSlice))
		var data = make(cache.ChartUints, len(mempoolSlice))
		for i, m := range mempoolSlice {
			time[i] = uint64(m.Time.UTC().Unix())
			data[i] = uint64(m.NumberOfTransactions.Int)
		}
		return charts.Encode(nil, time, data)
	}

	mempoolSlice, err := models.MempoolBins(
		models.MempoolBinWhere.Bin.EQ(binString),
		qm.Select(models.MempoolBinColumns.Time, models.MempoolBinColumns.NumberOfTransactions),
		qm.OrderBy(models.MempoolBinColumns.Time),
	).All(ctx, pg.db)
	if err != nil {
		return nil, err
	}
	var time = make(cache.ChartUints, len(mempoolSlice))
	var data = make(cache.ChartUints, len(mempoolSlice))
	for i, m := range mempoolSlice {
		time[i] = uint64(m.Time)
		data[i] = uint64(m.NumberOfTransactions.Int)
	}
	return charts.Encode(nil, time, data)
}

// TODO: break down into individual chart type
func (pg *PgDb) fetchEncodePropagationChart(ctx context.Context, charts *cache.Manager, dataType, axis string, binString string, extras ...string) ([]byte, error) {
	blockDelays, err := pg.propagationBlockChartData(ctx, 0)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	var xAxis cache.ChartUints
	var blockDelay cache.ChartFloats
	localBlockReceiveTime := make(map[uint64]float64)
	for _, record := range blockDelays {
		if axis == string(cache.HeightAxis) {
			xAxis = append(xAxis, uint64(record.BlockHeight))
		} else {
			xAxis = append(xAxis, uint64(record.BlockTime.Unix()))
		}
		timeDifference, _ := strconv.ParseFloat(fmt.Sprintf("%04.2f", record.TimeDifference), 64)
		blockDelay = append(blockDelay, timeDifference)

		localBlockReceiveTime[uint64(record.BlockHeight)] = timeDifference
	}

	switch dataType {
	case cache.BlockPropagation:
		blockPropagation := make(map[string]cache.ChartFloats)
		var dates cache.ChartUints
		dateMap := make(map[int64]bool)
		for _, source := range pg.syncSources {
			data, err := models.Propagations(
				models.PropagationWhere.Source.EQ(source),
				models.PropagationWhere.Bin.EQ(binString),
			).All(ctx, pg.db)
			if err != nil {
				return nil, err
			}

			for _, rec := range data {
				if _, f := dateMap[rec.Height]; !f {
					if axis == string(cache.HeightAxis) {
						dates = append(dates, uint64(rec.Height))
					} else {
						dates = append(dates, uint64(rec.Time))
					}
					dates = append(dates, uint64(rec.Time))
					dateMap[rec.Height] = true
				}
				blockPropagation[source] = append(blockPropagation[source], rec.Deviation)
			}
		}
		var data = []cache.Lengther{dates}
		for _, d := range blockPropagation {
			data = append(data, d)
		}
		return charts.Encode(nil, data...)

	case cache.BlockTimestamp:
		if binString == string(cache.DefaultBin) {
			return charts.Encode(nil, xAxis, blockDelay)
		} else {
			blocks, err := models.BlockBins(
				models.BlockBinWhere.Bin.EQ(binString),
				qm.OrderBy(models.BlockBinColumns.InternalTimestamp),
			).All(ctx, pg.db)
			if err != nil {
				return nil, err
			}
			var xAxis cache.ChartUints
			var blockDelay cache.ChartFloats
			for _, block := range blocks {
				if axis == string(cache.HashrateAxis) {
					xAxis = append(xAxis, uint64(block.Height))
				} else {
					xAxis = append(xAxis, uint64(block.InternalTimestamp))
				}
				blockDelay = append(blockDelay, block.ReceiveTimeDiff)
			}
			return charts.Encode(nil, xAxis, blockDelay)
		}

	case cache.VotesReceiveTime:
		if binString == string(cache.DefaultBin) {
			votesReceiveTime, err := pg.propagationVoteChartDataByHeight(ctx, 0)
			if err != nil && err != sql.ErrNoRows {
				return nil, err
			}
			var votesTimeDeviations = make(map[int64]cache.ChartFloats)

			for _, record := range votesReceiveTime {
				votesTimeDeviations[record.BlockHeight] = append(votesTimeDeviations[record.BlockHeight], record.TimeDifference)
			}

			var voteReceiveTimeDeviations cache.ChartFloats
			for _, height := range xAxis {
				if deviations, found := votesTimeDeviations[int64(height)]; found {
					var totalTime float64
					for _, timeDiff := range deviations {
						totalTime += timeDiff
					}
					timeDifference, _ := strconv.ParseFloat(fmt.Sprintf("%04.2f", totalTime/float64(len(deviations))*1000), 64)
					voteReceiveTimeDeviations = append(voteReceiveTimeDeviations, timeDifference)
					continue
				}
				voteReceiveTimeDeviations = append(voteReceiveTimeDeviations, 0)
			}
			return charts.Encode(nil, xAxis, voteReceiveTimeDeviations)
		} else {
			records, err := models.VoteReceiveTimeDeviations(
				models.VoteReceiveTimeDeviationWhere.Bin.EQ(binString),
				qm.OrderBy(models.VoteReceiveTimeDeviationColumns.BlockTime),
			).All(ctx, pg.db)
			if err != nil {
				return nil, err
			}
			var xAxis cache.ChartUints
			var diffs cache.ChartFloats
			for _, rec := range records {
				if axis == string(cache.HeightAxis) {
					xAxis = append(xAxis, uint64(rec.BlockHeight))
				} else {
					xAxis = append(xAxis, uint64(rec.BlockTime))
				}
				diffs = append(diffs, rec.ReceiveTimeDifference)
			}
			return charts.Encode(nil, xAxis, diffs)
		}
	}
	return nil, cache.UnknownChartErr
}

func (pg PgDb) UpdateMempoolAggregateData(ctx context.Context) error {
	log.Info("Updating mempool bin data")
	// hour bin
	lastHourEntry, err := models.MempoolBins(
		models.MempoolBinWhere.Bin.EQ(string(cache.HourBin)),
		qm.OrderBy(fmt.Sprintf("%s desc", models.MempoolBinColumns.Time)),
	).One(ctx, pg.db)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	var nextHour = time.Time{}
	if lastHourEntry != nil {
		nextHour = time.Unix(lastHourEntry.Time, 0).Add(cache.AnHour * time.Second).UTC()
	}
	if time.Now().Before(nextHour) {
		return nil
	}

	mempoolSlice, err := models.Mempools(
		models.MempoolWhere.Time.GTE(nextHour),
		qm.OrderBy(models.MempoolColumns.Time),
	).All(ctx, pg.db)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	dLen := len(mempoolSlice)
	dates, txCounts, sizes := make(cache.ChartUints, dLen), make(cache.ChartUints, dLen), make(cache.ChartUints, dLen)
	fees := make(cache.ChartFloats, dLen)
	for i, m := range mempoolSlice {
		dates[i] = uint64(m.Time.Unix())
		txCounts[i] = uint64(m.NumberOfTransactions.Int)
		sizes[i] = uint64(m.Size.Int)
		fees[i] = m.TotalFee.Float64
	}

	tx, err := pg.db.Begin()
	if err != nil {
		return err
	}
	hours, _, hourIntervals := cache.GenerateHourBin(dates, nil)
	for i, interval := range hourIntervals {
		mempoolBin := models.MempoolBin{
			Time:                 int64(hours[i]),
			Bin:                  string(cache.HourBin),
			Size:                 null.IntFrom(int(sizes.Avg(interval[0], interval[1]))),
			TotalFee:             null.Float64From(fees.Avg(interval[0], interval[1])),
			NumberOfTransactions: null.IntFrom(int(txCounts.Avg(interval[0], interval[1]))),
		}
		if err = mempoolBin.Insert(ctx, tx, boil.Infer()); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}

	// day bin
	lastDayEntry, err := models.MempoolBins(
		models.MempoolBinWhere.Bin.EQ(string(cache.DayBin)),
		qm.OrderBy(fmt.Sprintf("%s desc", models.MempoolBinColumns.Time)),
	).One(ctx, pg.db)
	if err != nil && err != sql.ErrNoRows {
		_ = tx.Rollback()
		return err
	}

	var nextDay = time.Time{}
	if lastDayEntry != nil {
		nextDay = time.Unix(lastDayEntry.Time, 0).Add(cache.ADay * time.Second).UTC()
	}
	if time.Now().Before(nextDay) {
		return nil
	}

	mempoolSlice, err = models.Mempools(
		models.MempoolWhere.Time.GTE(nextDay),
		qm.OrderBy(models.MempoolColumns.Time),
	).All(ctx, pg.db)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	dLen = len(mempoolSlice)
	dates, txCounts, sizes = make(cache.ChartUints, dLen), make(cache.ChartUints, dLen), make(cache.ChartUints, dLen)
	fees = make(cache.ChartFloats, dLen)
	for i, m := range mempoolSlice {
		dates[i] = uint64(m.Time.Unix())
		txCounts[i] = uint64(m.NumberOfTransactions.Int)
		sizes[i] = uint64(m.Size.Int)
		fees[i] = m.TotalFee.Float64
	}

	if tx, err = pg.db.Begin(); err != nil {
		return err
	}
	days, _, dayIntervals := cache.GenerateDayBin(dates, nil)
	for i, interval := range dayIntervals {
		mempoolBin := models.MempoolBin{
			Time:                 int64(days[i]),
			Bin:                  string(cache.DayBin),
			Size:                 null.IntFrom(int(sizes.Avg(interval[0], interval[1]))),
			TotalFee:             null.Float64From(fees.Avg(interval[0], interval[1])),
			NumberOfTransactions: null.IntFrom(int(txCounts.Avg(interval[0], interval[1]))),
		}
		if err = mempoolBin.Insert(ctx, tx, boil.Infer()); err != nil {
			_ = tx.Rollback()
			spew.Dump(mempoolBin, nextDay)
			return err
		}
	}

	if err = tx.Commit(); err != nil {
		return err
	}
	log.Info("Mempool bin data updated")
	return nil
}

// UpdatePropagationData
func (pg PgDb) UpdatePropagationData(ctx context.Context) error {
	log.Info("Updating propagation data")
	tx, err := pg.db.Begin()
	if err != nil {
		return fmt.Errorf("cannot start transaction, %s", err.Error())
	}

	if len(pg.syncSources) == 0 {
		log.Info("Please add one or more propagation sources")
		return nil
	}

	for _, source := range pg.syncSources {
		log.Infof("Fetching propagation data for %s", source)
		// get the last entry for this source and prepage the propagation records
		lastEntry, err := models.Propagations(
			models.PropagationWhere.Source.EQ(source),
			models.PropagationWhere.Bin.EQ(string(cache.DefaultBin)),
			qm.OrderBy(fmt.Sprintf("%s desc", models.PropagationColumns.Time)),
		).One(ctx, pg.db)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		var lastHeight int64
		if lastEntry != nil {
			lastHeight = lastEntry.Height
		}

		chartsBlockHeight := int32(lastHeight)
		mainBlockDelays, err := pg.propagationBlockChartData(ctx, int(chartsBlockHeight))
		if err != nil && err != sql.ErrNoRows {
			return err
		}

		localBlockReceiveTime := make(map[int64]float64)
		for _, record := range mainBlockDelays {
			timeDifference, _ := strconv.ParseFloat(fmt.Sprintf("%04.2f", record.TimeDifference), 64)
			localBlockReceiveTime[record.BlockHeight] = timeDifference
		}

		db, err := pg.syncSourceDbProvider(source)
		if err != nil {
			return err
		}

		blockDelays, err := db.propagationBlockChartData(ctx, int(chartsBlockHeight))
		if err != nil && err != sql.ErrNoRows {
			return err
		}

		receiveTimeMap := make(map[int64]float64)
		for _, record := range blockDelays {
			receiveTimeMap[record.BlockHeight], _ = strconv.ParseFloat(fmt.Sprintf("%04.2f", record.TimeDifference), 64)
		}

		var dates, heights cache.ChartUints
		var deviations cache.ChartFloats

		for _, rec := range mainBlockDelays {
			var propagation = models.Propagation{
				Height: rec.BlockHeight,
				Time:   rec.BlockTime.Unix(),
				Bin:    string(cache.DefaultBin),
				Source: source,
			}
			if sourceTime, found := receiveTimeMap[rec.BlockHeight]; found {
				propagation.Deviation = localBlockReceiveTime[rec.BlockHeight] - sourceTime
			}
			if err = propagation.Insert(ctx, tx, boil.Infer()); err != nil {
				_ = tx.Rollback()
				return err
			}
			dates = append(dates, uint64(propagation.Time))
			heights = append(heights, uint64(propagation.Height))
			deviations = append(deviations, propagation.Deviation)
		}

		log.Info("Updating propagation hourly average")
		lastHourEntry, err := models.Propagations(
			models.PropagationWhere.Bin.EQ(string(cache.HourBin)),
			qm.OrderBy(fmt.Sprintf("%s desc", models.PropagationColumns.Time)),
		).One(ctx, pg.db)
		if err != nil && err != sql.ErrNoRows {
			return err
		}

		var nextHour = time.Time{}
		if lastHourEntry != nil {
			nextHour = time.Unix(lastHourEntry.Time, 0).Add(cache.AnHour * time.Second).UTC()
		}
		if time.Now().Before(nextHour) {
			if err = tx.Commit(); err != nil {
				return err
			}
			return nil
		}

		hours, hourHeights, hourIntervals := cache.GenerateHourBin(dates, heights)
		for i, interval := range hourIntervals {
			propagationBin := models.Propagation{
				Time:      int64(hours[i]),
				Height:    int64(hourHeights[i]),
				Bin:       string(cache.HourBin),
				Source:    source,
				Deviation: deviations.Avg(interval[0], interval[1]),
			}
			if err = propagationBin.Insert(ctx, tx, boil.Infer()); err != nil {
				_ = tx.Rollback()
				return err
			}
		}

		log.Info("Updating propagation daily average")
		lastDayEntry, err := models.Propagations(
			models.PropagationWhere.Bin.EQ(string(cache.DayBin)),
			qm.OrderBy(fmt.Sprintf("%s desc", models.PropagationColumns.Time)),
		).One(ctx, pg.db)
		if err != nil && err != sql.ErrNoRows {
			return err
		}

		var nextDay = time.Time{}
		if lastDayEntry != nil {
			nextDay = time.Unix(lastDayEntry.Time, 0).Add(cache.ADay * time.Second).UTC()
		}
		if time.Now().Before(nextDay) {
			if err = tx.Commit(); err != nil {
				return err
			}
			return nil
		}

		days, dayHeights, dayIntervals := cache.GenerateDayBin(dates, heights)
		for i, interval := range dayIntervals {
			propagationBin := models.Propagation{
				Time:      int64(days[i]),
				Height:    int64(dayHeights[i]),
				Bin:       string(cache.DayBin),
				Source:    source,
				Deviation: deviations.Avg(interval[0], interval[1]),
			}
			if err = propagationBin.Insert(ctx, tx, boil.Infer()); err != nil {
				_ = tx.Rollback()
				return err
			}
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	log.Info("Updated propagation data")
	return nil
}

// UpdateBlockBinData
func (pg *PgDb) UpdateBlockBinData(ctx context.Context) error {
	log.Info("Updating block bin data")
	if err := pg.updateBlockHourlyAvgData(ctx); err != nil {
		return err
	}
	if err := pg.updateBlockDailyAvgData(ctx); err != nil {
		return err
	}
	return nil
}

func (pg *PgDb) updateBlockHourlyAvgData(ctx context.Context) error {
	lastEntry, err := models.BlockBins(
		models.BlockBinWhere.Bin.EQ(string(cache.HourBin)),
		qm.OrderBy(fmt.Sprintf("%s desc", models.BlockBinColumns.Height)),
	).One(ctx, pg.db)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	var nextHour = time.Time{}
	var lastHeight int64
	if lastEntry != nil {
		lastHeight = lastEntry.Height
		nextHour = time.Unix(lastEntry.InternalTimestamp, 0).Add(cache.AnHour * time.Second).UTC()
	}

	if time.Now().Before(nextHour) {
		return nil
	}

	totalCount, err := models.Blocks(
		models.BlockWhere.Height.GT(int(lastHeight)),
	).Count(ctx, pg.db)
	if err != nil {
		return err
	}

	tx, err := pg.db.Begin()
	if err != nil {
		return err
	}

	const pageSize = 1000
	var processed int64
	for processed < totalCount {
		log.Infof("Processing hourly average deviation of %d to %f of %d blocks", processed+1,
			math.Min(float64(processed+pageSize), float64(totalCount)), totalCount)
		blockSlice, err := models.Blocks(
			models.BlockWhere.Height.GT(int(lastHeight)), // lastHeight is updated below to ensure appropriate pagination
			qm.OrderBy(models.BlockColumns.Height),
			qm.Limit(pageSize),
		).All(ctx, pg.db)
		if err != nil && err != sql.ErrNoRows {
			_ = tx.Rollback()
			return err
		}
		if err == sql.ErrNoRows {
			break
		}
		var dates, heights cache.ChartUints
		var diffs cache.ChartFloats
		for _, block := range blockSlice {
			dates = append(dates, uint64(block.InternalTimestamp.Time.Unix()))
			heights = append(heights, uint64(block.Height))
			blockReceiveTimeDiff := block.ReceiveTime.Time.Sub(block.InternalTimestamp.Time).Seconds()
			diffs = append(diffs, blockReceiveTimeDiff)
		}
		hours, hourHeights, hourIntervals := cache.GenerateHourBin(dates, heights)
		for i, interval := range hourIntervals {
			if int64(hours[i]) < nextHour.Unix() {
				continue
			}
			blockBin := models.BlockBin{
				InternalTimestamp: int64(hours[i]),
				Height:            int64(hourHeights[i]),
				Bin:               string(cache.HourBin),
				ReceiveTimeDiff:   diffs.Avg(interval[0], interval[1]),
			}
			if err = blockBin.Insert(ctx, tx, boil.Infer()); err != nil {
				_ = tx.Rollback()
				return err
			}
			lastHeight = blockBin.Height
		}
		processed += int64(len(blockSlice))
	}
	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (pg *PgDb) updateBlockDailyAvgData(ctx context.Context) error {
	lastEntry, err := models.BlockBins(
		models.BlockBinWhere.Bin.EQ(string(cache.DayBin)),
		qm.OrderBy(fmt.Sprintf("%s desc", models.BlockBinColumns.Height)),
	).One(ctx, pg.db)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	var nextDay = time.Time{}
	var lastHeight int64
	if lastEntry != nil {
		lastHeight = lastEntry.Height
		nextDay = time.Unix(lastEntry.InternalTimestamp, 0).Add(cache.ADay * time.Second).UTC()
	}

	if time.Now().Before(nextDay) {
		return nil
	}

	totalCount, err := models.Blocks(
		models.BlockWhere.Height.GT(int(lastHeight)),
	).Count(ctx, pg.db)
	if err != nil {
		return err
	}

	tx, err := pg.db.Begin()
	if err != nil {
		return err
	}

	const pageSize = 1000
	var processed int64
	for processed < totalCount {
		log.Infof("Processing daily average deviation of %d to %f of %d blocks", processed+1,
			math.Min(float64(processed+pageSize), float64(totalCount)), totalCount)
		blockSlice, err := models.Blocks(
			models.BlockWhere.Height.GT(int(lastHeight)), // lastHeight is updated below to ensure appropriate pagination
			qm.OrderBy(models.BlockColumns.Height),
			qm.Limit(pageSize),
		).All(ctx, pg.db)
		if err != nil && err != sql.ErrNoRows {
			_ = tx.Rollback()
			return err
		}
		if err == sql.ErrNoRows {
			break
		}
		var dates, heights cache.ChartUints
		var diffs cache.ChartFloats
		for _, block := range blockSlice {
			dates = append(dates, uint64(block.InternalTimestamp.Time.Unix()))
			heights = append(heights, uint64(block.Height))
			blockReceiveTimeDiff := block.ReceiveTime.Time.Sub(block.InternalTimestamp.Time).Seconds()
			diffs = append(diffs, blockReceiveTimeDiff)
		}
		days, dayHeights, dayIntervals := cache.GenerateDayBin(dates, heights)
		for i, interval := range dayIntervals {
			if int64(days[i]) < nextDay.Unix() {
				continue
			}
			blockBin := models.BlockBin{
				InternalTimestamp: int64(days[i]),
				Height:            int64(dayHeights[i]),
				Bin:               string(cache.DayBin),
				ReceiveTimeDiff:   diffs.Avg(interval[0], interval[1]),
			}
			if err = blockBin.Insert(ctx, tx, boil.Infer()); err != nil {
				_ = tx.Rollback()
				return err
			}
			lastHeight = blockBin.Height
		}
		processed += int64(len(blockSlice))
	}
	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

// UpdateVoteTimeDeviationData
func (pg *PgDb) UpdateVoteTimeDeviationData(ctx context.Context) error {
	log.Info("Updating vote time deviation data")
	if err := pg.updateVoteTimeDeviationHourlyAvgData(ctx); err != nil {
		return err
	}
	if err := pg.updateVoteTimeDeviationDailyAvgData(ctx); err != nil {
		return err
	}
	return nil
}

func (pg *PgDb) updateVoteTimeDeviationHourlyAvgData(ctx context.Context) error {
	lastEntry, err := models.VoteReceiveTimeDeviations(
		models.VoteReceiveTimeDeviationWhere.Bin.EQ(string(cache.HourBin)),
		qm.OrderBy(fmt.Sprintf("%s desc", models.VoteReceiveTimeDeviationColumns.BlockTime)),
		// The retrival process below ensure that all votes for the block is processed in one circle
		// qm.OrderBy(fmt.Sprintf("%s desc", models.VoteReceiveTimeDeviationColumns.Hash)),
	).One(ctx, pg.db)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	var lastHour, nextHour time.Time
	if lastEntry != nil {
		lastHour = time.Unix(lastEntry.BlockTime, 0)
		nextHour = time.Unix(lastEntry.BlockTime, 0).Add(cache.AnHour * time.Second).UTC()
	} else {
		firstBlock, err := models.Blocks(
			qm.OrderBy(models.BlockColumns.Height),
		).One(ctx, pg.db)
		if err != nil && err != sql.ErrNoRows {
			return nil
		}
		if firstBlock == nil {
			return nil
		}
		// the query uses <= so we are stepping back by 1 second
		lastHour = firstBlock.ReceiveTime.Time.Add(-1 * time.Second)
	}

	if time.Now().Before(nextHour) {
		return nil
	}

	totalCount, err := models.Votes(
		models.VoteWhere.TargetedBlockTime.GT(null.TimeFrom(lastHour)),
	).Count(ctx, pg.db)
	if err != nil {
		return err
	}

	if totalCount == 0 {
		return nil
	}

	tx, err := pg.db.Begin()
	if err != nil {
		return err
	}

	var processed int64
	for processed < totalCount {

		voteSlice, err := models.Votes(
			// lastHeight is updated below to ensure appropriate pagination
			models.VoteWhere.TargetedBlockTime.GT(null.TimeFrom(lastHour)),
			// Using block height to coordinate pagination to ensure the processing of all votes
			models.VoteWhere.TargetedBlockTime.LTE(null.TimeFrom(lastHour.Add(7*24*time.Hour))),
			qm.OrderBy(models.VoteColumns.VotingOn),
		).All(ctx, tx)

		if err != nil && err != sql.ErrNoRows {
			_ = tx.Rollback()
			return err
		}
		if err == sql.ErrNoRows {
			return nil
		}

		var dates, heights cache.ChartUints
		var diffs cache.ChartFloats
		for _, vote := range voteSlice {
			dates = append(dates, uint64(vote.TargetedBlockTime.Time.Unix()))
			heights = append(heights, uint64(vote.VotingOn.Int64))
			blockReceiveTimeDiff := vote.ReceiveTime.Time.Sub(vote.BlockReceiveTime.Time).Seconds()
			diffs = append(diffs, blockReceiveTimeDiff)
		}
		hours, hourHeights, hourIntervals := cache.GenerateHourBin(dates, heights)
		for i, interval := range hourIntervals {
			if int64(hours[i]) < nextHour.Unix() {
				continue
			}
			m := models.VoteReceiveTimeDeviation{
				BlockTime:             int64(hours[i]),
				BlockHeight:           int64(hourHeights[i]),
				Bin:                   string(cache.HourBin),
				ReceiveTimeDifference: diffs.Avg(interval[0], interval[1]),
			}
			if err = m.Insert(ctx, tx, boil.Infer()); err != nil {
				_ = tx.Rollback()
				return err
			}
			lastHour = time.Unix(m.BlockTime, 0)
			nextHour = lastHour.Add(1 * time.Hour)
		}

		log.Infof("Processed hourly average vote receive time deviation of %d to %d of %d votes", processed,
			processed+int64(len(voteSlice)), totalCount)
		processed += int64(len(voteSlice))
	}
	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (pg *PgDb) updateVoteTimeDeviationDailyAvgData(ctx context.Context) error {
	lastEntry, err := models.VoteReceiveTimeDeviations(
		models.VoteReceiveTimeDeviationWhere.Bin.EQ(string(cache.DayBin)),
		qm.OrderBy(fmt.Sprintf("%s desc", models.VoteReceiveTimeDeviationColumns.BlockTime)),
		// The retrival process below ensure that all votes for the block is processed in one circle
		// qm.OrderBy(fmt.Sprintf("%s desc", models.VoteReceiveTimeDeviationColumns.Hash)),
	).One(ctx, pg.db)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	var lastDay, nextDay time.Time
	if lastEntry != nil {
		lastDay = time.Unix(lastEntry.BlockTime, 0)
		nextDay = time.Unix(lastEntry.BlockTime, 0).Add(cache.ADay * time.Second).UTC()
	} else {
		firstBlock, err := models.Blocks(
			qm.OrderBy(models.BlockColumns.Height),
		).One(ctx, pg.db)
		if err != nil && err != sql.ErrNoRows {
			return nil
		}
		if firstBlock == nil {
			return nil
		}
		// the query uses <= so we are stepping back by 1 second
		lastDay = firstBlock.ReceiveTime.Time.Add(-1 * time.Second)
	}

	if time.Now().Before(nextDay) {
		return nil
	}

	totalCount, err := models.Votes(
		models.VoteWhere.TargetedBlockTime.GT(null.TimeFrom(lastDay)),
	).Count(ctx, pg.db)
	if err != nil {
		return err
	}

	if totalCount == 0 {
		return nil
	}

	tx, err := pg.db.Begin()
	if err != nil {
		return err
	}

	var processed int64
	for processed < totalCount {

		voteSlice, err := models.Votes(
			// lastHeight is updated below to ensure appropriate pagination
			models.VoteWhere.TargetedBlockTime.GT(null.TimeFrom(lastDay)),
			// Using block height to coordinate pagination to ensure the processing of all votes
			models.VoteWhere.TargetedBlockTime.LTE(null.TimeFrom(lastDay.Add(14*24*time.Hour))),
			qm.OrderBy(models.VoteColumns.VotingOn),
		).All(ctx, tx)

		if err != nil && err != sql.ErrNoRows {
			_ = tx.Rollback()
			return err
		}
		if err == sql.ErrNoRows {
			return nil
		}

		var dates, heights cache.ChartUints
		var diffs cache.ChartFloats
		for _, vote := range voteSlice {
			dates = append(dates, uint64(vote.TargetedBlockTime.Time.Unix()))
			heights = append(heights, uint64(vote.VotingOn.Int64))
			blockReceiveTimeDiff := vote.ReceiveTime.Time.Sub(vote.BlockReceiveTime.Time).Seconds()
			diffs = append(diffs, blockReceiveTimeDiff)
		}
		days, dayHeights, dayIntervals := cache.GenerateDayBin(dates, heights)
		for i, interval := range dayIntervals {
			if int64(days[i]) < nextDay.Unix() {
				continue
			}
			m := models.VoteReceiveTimeDeviation{
				BlockTime:             int64(days[i]),
				BlockHeight:           int64(dayHeights[i]),
				Bin:                   string(cache.DayBin),
				ReceiveTimeDifference: diffs.Avg(interval[0], interval[1]),
			}
			if err = m.Insert(ctx, tx, boil.Infer()); err != nil {
				_ = tx.Rollback()
				return err
			}
			lastDay = time.Unix(m.BlockTime, 0)
			nextDay = lastDay.Add(24 * time.Hour)
		}

		log.Infof("Processed daily average vote receive time deviation of %d to %d of %d votes", processed,
			processed+int64(len(voteSlice)), totalCount)
		processed += int64(len(voteSlice))
	}
	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}
