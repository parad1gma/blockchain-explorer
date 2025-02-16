package syncer

import (
	"context"
	"ethernal/explorer/db"
	"ethernal/explorer/eth"
	"math"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/sirupsen/logrus"
	"github.com/uptrace/bun"
)

type JobArgs struct {
	BlockNumbers         []uint64
	Client               *rpc.Client
	Db                   *bun.DB
	Step                 uint
	CallTimeoutInSeconds uint
	EthLogs              bool
}

type JobResult struct {
	Blocks       []*db.Block
	Transactions []*db.Transaction
	Logs         []*db.Log
	Nfts         []*db.Nft
	Contracts    []db.Contract
}

var (
	execFn = func(ctx context.Context, args interface{}) interface{} {
		jobArgs, ok := args.(JobArgs)
		if !ok {
			logrus.Panic("Wrong type for args parameter")
		}

		blocks := GetBlocks(jobArgs, ctx)
		if blocks == nil {
			return nil
		}
		transactions, receipts := GetTransactions(blocks, jobArgs, ctx)
		if transactions == nil || receipts == nil {
			return nil
		}

		dbBlocks := make([]*db.Block, len(blocks))
		for i, b := range blocks {
			dbBlocks[i] = eth.CreateDbBlock(b)
		}

		dbTransactions := make([]*db.Transaction, len(transactions))
		dbLogs := []*db.Log{}
		dbContracts := []db.Contract{}
		dbNfts := []*db.Nft{}

		for i, t := range transactions {
			dbTransactions[i] = eth.CreateDbTransaction(t, receipts[i])
			if receipts[i].ContractAddress != "" {
				dbContracts = append(dbContracts, eth.CreateDbContract(receipts[i]))
			}
			if jobArgs.EthLogs {
				dbLogs = append(dbLogs, eth.CreateDbLog(t, receipts[i])...)
				nfts, err := eth.CreateDbNfts(t, receipts[i])
				if err != nil {
					logrus.Error("Error while parsing logs for transaction ", t.Hash, " , err: ", err)
					return nil
				}
				dbNfts = append(dbNfts, nfts...)
			}
		}

		return JobResult{Blocks: dbBlocks, Transactions: dbTransactions, Logs: dbLogs, Nfts: dbNfts, Contracts: dbContracts}
	}
)

func GetTransactions(blocks []*eth.Block, jobArgs JobArgs, ctx context.Context) ([]*eth.Transaction, []*eth.TransactionReceipt) {
	transactions := []*eth.Transaction{}
	receipts := []*eth.TransactionReceipt{}
	var elems []rpc.BatchElem

	for _, block := range blocks {
		if len(block.Transactions) == 0 {
			continue
		}

		for _, transHash := range block.Transactions {
			transaction := &eth.Transaction{
				Timestamp: block.Timestamp,
			}
			receipt := &eth.TransactionReceipt{}

			elems = append(elems, rpc.BatchElem{
				Method: "eth_getTransactionByHash",
				Args:   []interface{}{transHash},
				Result: transaction,
			})
			elems = append(elems, rpc.BatchElem{
				Method: "eth_getTransactionReceipt",
				Args:   []interface{}{transHash},
				Result: receipt,
			})

			transactions = append(transactions, transaction)
			receipts = append(receipts, receipt)
		}
	}

	step := jobArgs.Step
	if len(elems) != 0 {
		totalCounter := uint(math.Ceil(float64(len(elems)) / float64(step)))
		var i uint
		for i = 0; i < totalCounter; i++ {

			from := i * step
			to := int(math.Min(float64(len(elems)), float64((i+1)*step)))

			elemSlice := elems[from:to]
			ioErr := batchCallWithTimeout(&elemSlice, *jobArgs.Client, jobArgs.CallTimeoutInSeconds, ctx)
			if ioErr != nil {
				logrus.Error("Cannot get transactions from blockchain, err: ", ioErr)
				return nil, nil
			}

			for _, e := range elemSlice {
				if e.Error != nil {
					logrus.Error("Error during batch call, err: ", e.Error.Error())
					return nil, nil
				}
			}
		}
	}

	return transactions, receipts
}

func GetBlocks(jobArgs JobArgs, ctx context.Context) []*eth.Block {
	blocks := []*eth.Block{}
	elems := make([]rpc.BatchElem, 0, len(jobArgs.BlockNumbers))

	for _, blockNumber := range jobArgs.BlockNumbers {
		block := &eth.Block{}

		elems = append(elems, rpc.BatchElem{
			Method: "eth_getBlockByNumber",
			Args:   []interface{}{string(hexutil.EncodeBig(big.NewInt(int64(blockNumber)))), false},
			Result: block,
		})

		blocks = append(blocks, block)
	}

	ioErr := batchCallWithTimeout(&elems, *jobArgs.Client, jobArgs.CallTimeoutInSeconds, ctx)
	if ioErr != nil {
		logrus.Error("Cannot get blocks from blockchain, err: ", ioErr)
		return nil
	}

	for _, e := range elems {
		if e.Error != nil {
			logrus.Error("Error during batch call, err: ", e.Error.Error())
			return nil
		}
	}

	return blocks
}

func batchCallWithTimeout(elems *[]rpc.BatchElem, client rpc.Client, callTimeoutInSeconds uint, ctx context.Context) error {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, time.Duration(callTimeoutInSeconds)*time.Second)
	defer cancel()
	return client.BatchCallContext(ctxWithTimeout, *elems)
}
