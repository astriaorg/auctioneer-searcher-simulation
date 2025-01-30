package main

import (
	"auctioneer-searcher-simulation/sequencer_client"
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/google/uuid"
	"github.com/lmittmann/tint"
	"io"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
)

type optimisticBlockData struct {
	blockHash   []byte
	blockNumber uint64
}

func getConfigFile() (string, error) {
	env := os.Getenv("ENV")

	if env == "" {
		return ".env.dev", nil
	}

	if env != "mainnet" && env != "testnet" && env != "dev" {
		return "", fmt.Errorf("invalid env")
	}

	return fmt.Sprintf(".env.%s", env), nil
}

// TODO - supporting having multiple searcher instances
func main() {
	detailedLogHandler := NewDetailedLogHandler(tint.NewHandler(os.Stdout, &tint.Options{}))
	slog.SetDefault(slog.New(&detailedLogHandler))

	configFile, err := getConfigFile()
	if err != nil {
		slog.Error("can not get config file", "err", err)
		return
	}

	slog.Info("config file", "file", configFile)

	config, err := readConfigFromEnv(configFile)
	if err != nil {
		slog.Error("can not read config from env", "err", err)
		return
	}

	slog.Info("creating sequencer client")
	sequencerClient, err := sequencer_client.NewSequencerClient(config.sequencerUrl)
	if err != nil {
		slog.Error("can not connect with server", "err", err)
		return
	}

	slog.Info("creating optimistic stream")
	optimisticStreamingInfo := sequencer_client.NewOptimisticStreamConnectionInfo(sequencerClient, config.rollupName)
	optimisticStream, err := optimisticStreamingInfo.GetOptimisticStream()
	if err != nil {
		slog.Error("can not create optimistic stream", "err", err)
		return
	}

	slog.Info("creating block commitment stream")
	blockCommitmentStreamInfo := sequencer_client.NewBlockCommitmentStreamConnectionInfo(sequencerClient)
	blockCommitmentStream, err := blockCommitmentStreamInfo.GetBlockCommitmentStream()
	if err != nil {
		slog.Error("can not create block commitment stream", "err", err)
		return
	}

	client, err := ethclient.Dial(config.ethRpcUrl)
	if err != nil {
		slog.Error("can not connect to eth client", "err", err)
		return
	}
	chainId, err := client.ChainID(context.Background())
	if err != nil {
		slog.Error("can not get chain id", "err", err)
		return
	}

	done := make(chan bool)

	searcher, err := NewSearcher(config.searcherPrivateKey, chainId, client)
	if err != nil {
		slog.Error("can not create searcher", "err", err)
		return
	}

	searcherTasksWaitGroup := sync.WaitGroup{}
	searcherResultChan := make(chan SearcherResult)
	searcherResultStore := NewSearcherResultStore()
	searcherResultTaskRes := make(chan bool)

	optimisticBlockInfo := OptimisticBlockInfo{}
	optimisticBlockChannel := make(chan optimisticBlockData)
	blockCommitmentChannel := make(chan optimisticBlockData)
	searcherBlockCommitmentChannelMap := map[uint64]chan optimisticBlockData{}

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
				slog.Error("can not receive", "err", err)
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
				slog.Error("can not receive", "err", err)
			}
			block := resp.GetCommitment()

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

	// ignore the first optimistic block and block commitment. this is a sync step so that we always start the loop with an optimistic block
	select {
	case optimisticBlock := <-optimisticBlockChannel:
		slog.Info("Ignoring the first Optimistic Block", "block_number", optimisticBlock.blockNumber)
	}

	select {
	case blockCommitment := <-blockCommitmentChannel:
		slog.Info("Ignoring the first block commitment", "block_number", blockCommitment.blockNumber)
	}

	blockCounter := atomic.Uint64{}
	interval := config.searchingTimeStart
	intervalDelta := config.searchingTimeIncreaseDelta

loop:
	for {
		select {
		case optimisticBlock := <-optimisticBlockChannel:
			if uint64(searcherTasksSpawned) >= config.searcherTasksToSpinUp {
				slog.Info("All searcher tasks are spawned, breaking out of the loop!")
				break loop
			}

			// send every 4th block. We can avoid this by maintaining multiple searcher instances
			if blockCounter.Load()%4 == 0 {
				// the auction starts, trigger the searcher task
				slog.Info("Received Optimistic Block", "block_number", optimisticBlock.blockNumber)
				optimisticBlockInfo.SetBlockNumber(optimisticBlock.blockNumber)
				optimisticBlockInfo.SetBlockHash(optimisticBlock.blockHash)

				searcherTasksSpawned += 1

				searcherId, err := uuid.NewUUID()
				if err != nil {
					panic(fmt.Sprintf("can not create uuid %v", err))
				}
				searcherTasksWaitGroup.Add(1)

				// create the block commitment channel
				searcherBlockCommitmentCh := make(chan optimisticBlockData)
				searcherBlockCommitmentChannelMap[optimisticBlock.blockNumber] = searcherBlockCommitmentCh

				go searcher.SearcherTask(searcherId, interval, config.latencyMargin, searcherBlockCommitmentCh, &optimisticBlockInfo, searcherResultChan, &searcherTasksWaitGroup)
				interval += intervalDelta
				if interval > config.searchingTimeEnd {
					interval = config.searchingTimeEnd
				}
			}
			blockCounter.Add(1)
		case blockCommitment := <-blockCommitmentChannel:
			blockCommitmentCh, ok := searcherBlockCommitmentChannelMap[blockCommitment.blockNumber]
			if ok {
				slog.Info("Received corresponding block commitment", "block_number", blockCommitment.blockNumber)
				blockCommitmentCh <- blockCommitment
			}
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
	err = searcherResultStore.WriteResultsToCsvFile(config.resultFile)
	if err != nil {
		slog.Error("can not write searcher results to csv file", "err", err)
		return
	}
}
