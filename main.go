package main

import (
	"auctioneer-searcher-simulation/sequencer_client"
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/google/uuid"
	"io"
	"log/slog"
	"sync"
	"time"
)

var (
	// TODO - fetch this from a .env file
	SEQUENCER_URL             = "127.0.0.1:49171"
	SEARCHER_PRIVATE_KEY      = "528a032646bf3498e48994778a0c1f9a535541142a6356808c9234dd3614882e"
	ETH_RPC_URL               = "http://executor.astria.localdev.me"
	SEARCHER_TASKS_TO_SPIN_UP = 5
)

type optimisticBlockData struct {
	blockHash   []byte
	blockNumber uint64
}

// TODO - supporting having multiple searcher instances
func main() {
	config, err := readConfigFromEnv(".env.local")
	if err != nil {
		slog.Error("can not read config from env %v", err)
		return
	}

	config.PrintConfig()

	slog.Info("creating sequencer client")
	sequencerClient, err := sequencer_client.NewSequencerClient(config.sequencerUrl)
	if err != nil {
		slog.Error("can not connect with server %v", err)
		return
	}

	slog.Info("creating optimistic stream")
	optimisticStreamingInfo := sequencer_client.NewOptimisticStreamConnectionInfo(sequencerClient, config.rollupName)
	optimisticStream, err := optimisticStreamingInfo.GetOptimisticStream()
	if err != nil {
		slog.Error("can not create optimistic stream %v", err)
		return
	}

	slog.Info("creating block commitment stream")
	blockCommitmentStreamInfo := sequencer_client.NewBlockCommitmentStreamConnectionInfo(sequencerClient)
	blockCommitmentStream, err := blockCommitmentStreamInfo.GetBlockCommitmentStream()
	if err != nil {
		slog.Error("can not create block commitment stream %v", err)
		return
	}

	client, err := ethclient.Dial(config.ethRpcUrl)
	if err != nil {
		slog.Error("can not connect to eth client %v", err)
		return
	}
	chainId, err := client.ChainID(context.Background())
	if err != nil {
		slog.Error("can not get chain id %v", err)
		return
	}
	slog.Info("chain id is %v", chainId)

	done := make(chan bool)

	searcher, err := NewSearcher(config.searcherPrivateKey, chainId, client)
	if err != nil {
		slog.Error("can not create searcher %v", err)
		return
	}

	searcherTasksWaitGroup := sync.WaitGroup{}
	searcherResultChan := make(chan SearcherResult)
	searcherResultStore := NewSearcherResultStore()
	searcherResultTaskRes := make(chan bool)

	optimisticBlockInfo := OptimisticBlockInfo{}
	optimisticBlockChannel := make(chan optimisticBlockData)
	blockCommitmentChannel := make(chan optimisticBlockData)

	slog.Info("Starting optimistic block stream")
	go func(optimisticBlockChannel chan optimisticBlockData) {
		slog.Info("Starting optimistic block stream")
		for {
			resp, err := optimisticStream.GetStreamClient().Recv()
			if err == io.EOF {
				close(done)
				return
			}
			if err != nil {
				slog.Error("can not receive %v", err)
			}
			block := resp.GetBlock()

			opData := optimisticBlockData{
				blockHash:   block.GetBlockHash(),
				blockNumber: block.GetHeader().GetHeight(),
			}

			optimisticBlockChannel <- opData
		}
	}(optimisticBlockChannel)

	slog.Info("Starting block commitment stream")
	go func(blockCommitmentData chan optimisticBlockData) {
		slog.Info("Starting block commitment stream")
		for {
			resp, err := blockCommitmentStream.GetStreamClient().Recv()
			if err == io.EOF {
				close(done)
				return
			}
			if err != nil {
				slog.Error("can not receive %v", err)
			}
			block := resp.GetCommitment()

			slog.Info("Received block commitment in go routine!")
			opData := optimisticBlockData{
				blockHash:   block.GetBlockHash(),
				blockNumber: block.GetHeight(),
			}
			blockCommitmentData <- opData
		}
	}(blockCommitmentChannel)

	slog.Info("Starting searcher result task collector")
	go func(searcherResultStore *SearcherResultStore, resultCh chan SearcherResult, resCh chan bool, doneCh chan bool) {
		for {
			select {
			case searcherResult := <-resultCh:
				searcherResultStore.AddSearcherResult(searcherResult)
				if searcherResultStore.SearcherResultCount() == config.searcherTasksToSpinUp {
					resCh <- true
					slog.Info("All searcher task results have been collected!")
				}

			case <-doneCh:
				resCh <- true
				slog.Info("Received result channel close signal")
				return
			}
		}

	}(searcherResultStore, searcherResultChan, searcherResultTaskRes, done)

	searcherTasksSpawned := 0

	// ignore the first block commitment
	select {
	case <-blockCommitmentChannel:
		slog.Info("Ignoring the first block commitment")
	}

	blockCounter := 0

loop:
	for {
		select {
		case optimisticBlock := <-optimisticBlockChannel:
			if uint64(searcherTasksSpawned) >= config.searcherTasksToSpinUp {
				slog.Info("All searcher tasks are spawned, breaking out of the loop!")
				break loop
			}

			// send every 4th block. We can avoid this by maintaining multiple searcher instances
			if blockCounter%4 == 0 {
				// the auction starts, trigger the searcher task
				slog.Info("Received Optimistic Block: %v", optimisticBlock)
				optimisticBlockInfo.SetBlockNumber(optimisticBlock.blockNumber)
				optimisticBlockInfo.SetBlockHash(optimisticBlock.blockHash)

				searcherTasksSpawned += 1

				searcherId, err := uuid.NewUUID()
				if err != nil {
					panic(fmt.Sprintf("can not create uuid %v", err))
				}
				searcherTasksWaitGroup.Add(1)
				go searcher.SearcherTask(searcherId, 200*time.Millisecond, blockCommitmentChannel, &optimisticBlockInfo, searcherResultChan, config.latencyMargin, &searcherTasksWaitGroup)
			}
			blockCounter += 1
		case <-blockCommitmentChannel:
			// drain the block commitments received, otherwise the searcher channel will receive stale block commitments
			slog.Info("Received block commitment in main routine!")
		case <-done:
			slog.Error("exiting due to an unexpected error!")
			break loop
		}
	}

	slog.Info("Waiting for result task collector to end")
	select {
	case <-searcherResultTaskRes:
		slog.Info("Searcher result task collector finished!")
	}

	slog.Info("Waiting for searcher tasks to finish")
	searcherTasksWaitGroup.Wait()

	slog.Info("All searcher tasks are finished")

	searcherResultStore.PrintSearcherResults()
}
