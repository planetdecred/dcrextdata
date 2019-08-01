// Copyright (c) 2018-2019 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mempool

import (
	"context"
	"database/sql"
	"math"
	"sync"
	"time"

	"github.com/decred/dcrd/chaincfg"
	"github.com/decred/dcrd/chaincfg/chainhash"
	"github.com/decred/dcrd/dcrjson"
	"github.com/decred/dcrd/rpcclient"
	"github.com/decred/dcrd/wire"
	exptypes "github.com/decred/dcrdata/explorer/types"
	"github.com/decred/dcrdata/txhelpers/v2"
	"github.com/raedahgroup/dcrextdata/app/helpers"
)

func NewCollector(interval float64, activeChain *chaincfg.Params, dataStore DataStore) *Collector {
	return &Collector{
		collectionInterval: interval,
		dataStore:          dataStore,
		activeChain:        activeChain,
	}
}

func (c *Collector) SetClient(client *rpcclient.Client) {
	c.dcrClient = client
}

func (c *Collector) DcrdHandlers(ctx context.Context) *rpcclient.NotificationHandlers {
	var ticketIndsMutex sync.Mutex
	ticketInds := make(exptypes.BlockValidatorIndex)

	return &rpcclient.NotificationHandlers{
		OnTxAcceptedVerbose: func(txDetails *dcrjson.TxRawResult) {
			go func() {
				if ctx.Err() != nil {
					return
				}
				if !c.syncIsDone {
					return
				}
				receiveTime := helpers.NowUtc()

				msgTx, err := txhelpers.MsgTxFromHex(txDetails.Hex)
				if err != nil {
					log.Errorf("Failed to decode transaction hex: %v", err)
					return
				}

				if txType := txhelpers.DetermineTxTypeString(msgTx); txType != "Vote" {
					return
				}

				var voteInfo *exptypes.VoteInfo
				validation, version, bits, choices, err := txhelpers.SSGenVoteChoices(msgTx, c.activeChain)
				if err != nil {
					log.Errorf("Error in getting vote choice: %s", err.Error())
					return
				}

				voteInfo = &exptypes.VoteInfo{
					Validation: exptypes.BlockValidation{
						Hash:     validation.Hash.String(),
						Height:   validation.Height,
						Validity: validation.Validity,
					},
					Version:     version,
					Bits:        bits,
					Choices:     choices,
					TicketSpent: msgTx.TxIn[1].PreviousOutPoint.Hash.String(),
				}

				ticketIndsMutex.Lock()
				voteInfo.SetTicketIndex(ticketInds)
				ticketIndsMutex.Unlock()

				vote := Vote{
					ReceiveTime: receiveTime,
					VotingOn:    validation.Height,
					Hash:        txDetails.Txid,
					ValidatorId: voteInfo.MempoolTicketIndex,
				}

				if voteInfo.Validation.Validity {
					vote.Validity = "Valid"
				} else {
					vote.Validity = "Invalid"
				}

				var retries = 3
				var targetedBlock *wire.MsgBlock

				// try to get the block from the blockchain until the number of retries has elapsed
				for i := 0; i <= retries; i++ {
					targetedBlock, err = c.dcrClient.GetBlock(&validation.Hash)
					if err == nil {
						break
					}
					time.Sleep(2 * time.Second)
				}

				// err is ignored since the vote will be updated when the block becomes available
				if targetedBlock != nil {
					vote.TargetedBlockTime = targetedBlock.Header.Timestamp.UTC()
					vote.BlockHash = targetedBlock.Header.BlockHash().String()
				}

				if err = c.dataStore.SaveVote(ctx, vote); err != nil {
					log.Error(err)
				}
			}()
		},

		OnBlockConnected: func(blockHeaderSerialized []byte, transactions [][]byte) {
			if !c.syncIsDone {
				return
			}

			if ctx.Err() != nil {
				return
			}

			blockHeader := new(wire.BlockHeader)
			err := blockHeader.FromBytes(blockHeaderSerialized)
			if err != nil {
				log.Error("Failed to deserialize blockHeader in new block notification: %v", err)
				return
			}

			block := Block{
				BlockInternalTime: blockHeader.Timestamp.UTC(),
				BlockReceiveTime:  helpers.NowUtc(),
				BlockHash:         blockHeader.BlockHash().String(),
				BlockHeight:       blockHeader.Height,
			}
			if err = c.dataStore.SaveBlock(ctx, block); err != nil {
				log.Error(err)
			}
		},
	}
}

func (c *Collector) StartMonitoring(ctx context.Context) {
	var mu sync.Mutex

	collectMempool := func() {
		mu.Lock()
		defer mu.Unlock()

		mempoolTransactionMap, err := c.dcrClient.GetRawMempoolVerbose(dcrjson.GRMAll)
		if err != nil {
			log.Error(err)
			return
		}

		if len(mempoolTransactionMap) == 0 {
			return
		}

		// there wont be transactions in the mempool while sync is going on
		c.syncIsDone = true // todo: we need a better way to determine the sync status of dcrd

		mempoolDto := Mempool{
			NumberOfTransactions: len(mempoolTransactionMap),
			Time:                 helpers.NowUtc(),
			FirstSeenTime:        helpers.NowUtc(), //todo: use the time of the first tx in the mempool
		}

		for hashString, tx := range mempoolTransactionMap {
			hash, err := chainhash.NewHashFromStr(hashString)
			if err != nil {
				log.Error(err)
				continue
			}
			rawTx, err := c.dcrClient.GetRawTransactionVerbose(hash)
			if err != nil {
				log.Error(err)
				continue
			}

			totalOut := 0.0
			for _, v := range rawTx.Vout {
				totalOut += v.Value
			}

			mempoolDto.Total += totalOut
			mempoolDto.TotalFee += tx.Fee
			mempoolDto.Size += tx.Size
			if mempoolDto.FirstSeenTime.Unix() > tx.Time {
				mempoolDto.FirstSeenTime = time.Unix(tx.Time, 0)
			}

		}

		votes, err := c.dcrClient.GetRawMempool(dcrjson.GRMVotes)
		if err != nil {
			log.Error(err)
			return
		}
		mempoolDto.Voters = len(votes)

		tickets, err := c.dcrClient.GetRawMempool(dcrjson.GRMTickets)
		if err != nil {
			log.Error(err)
			return
		}
		mempoolDto.Tickets = len(tickets)

		revocations, err := c.dcrClient.GetRawMempool(dcrjson.GRMRevocations)
		if err != nil {
			log.Error(err)
			return
		}
		mempoolDto.Revocations = len(revocations)

		err = c.dataStore.StoreMempool(ctx, mempoolDto)
		log.Info("Creating Mempool table")
		if err != nil {
			log.Error(err)
		}
	}

	lastMempoolTime, err := c.dataStore.LastMempoolTime()
	if err != nil {
		if err != sql.ErrNoRows {
			log.Errorf("Unable to get last mempool entry time: %s", err.Error())
		}
	} else {
		sencodsPassed := math.Abs(time.Since(lastMempoolTime).Seconds())
		if sencodsPassed < c.collectionInterval {
			timeLeft := c.collectionInterval - sencodsPassed
			log.Infof("Fetching mempool every %dm, collected %0.2f ago, will fetch in %0.2f.", 1, sencodsPassed,
				timeLeft)
			time.Sleep(time.Duration(timeLeft) * time.Second)
		}
	}
	collectMempool()
	ticker := time.NewTicker(time.Duration(c.collectionInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			collectMempool()
			break
		case <-ctx.Done():
			return
		}
	}
}
