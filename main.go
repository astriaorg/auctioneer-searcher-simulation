// main.go
package main

import (
	"auctioneer-searcher-simulation/sequencer_client"
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/ethclient"
	"io"
	"log/slog"
	"os"
	"sync"
)

type optimisticBlockData struct {
	blockHash   []byte
	blockNumber uint64
}

func main() {
	config, err := readConfigFromCmdLine()
	if err != nil {
		fmt.Printf("can not read config from env: err: %s\n", err.Error())
		return
	}

	sequencerClient, err := sequencer_client.NewSequencerClient(config.sequencerUrl)
	if err != nil {
		fmt.Printf("can not connect with server: err: %s\n", err.Error())
		return
	}

	optimisticStreamingInfo := sequencer_client.NewOptimisticStreamConnectionInfo(sequencerClient, config.rollupName)
	optimisticStream, err := optimisticStreamingInfo.GetOptimisticStream()
	if err != nil {
		fmt.Printf("can not create optimistic stream: err: %s\n", err.Error())
		return
	}

	blockCommitmentStreamInfo := sequencer_client.NewBlockCommitmentStreamConnectionInfo(sequencerClient)
	blockCommitmentStream, err := blockCommitmentStreamInfo.GetBlockCommitmentStream()
	if err != nil {
		fmt.Printf("can not create block commitment stream: err: %s", err.Error())
		return
	}

	client, err := ethclient.Dial(config.ethRpcUrl)
	if err != nil {
		fmt.Printf("can not connect to eth client: err: %s", err.Error())
		return
	}
	chainId, err := client.ChainID(context.Background())
	if err != nil {
		fmt.Printf("can not get chain id: err: %s", err.Error())
		return
	}

	done := make(chan bool)

	searcher, err := NewSearcher(config.searcherPrivateKey, chainId, client)
	if err != nil {
		fmt.Printf("can not create searcher: err: %s", err.Error())
		return
	}

	searcherTasksWaitGroup := sync.WaitGroup{}

	optimisticBlockChannel := make(chan optimisticBlockData)
	blockCommitmentChannel := make(chan optimisticBlockData)

	go func(optimisticBlockChannel chan optimisticBlockData) {
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

	go func(blockCommitmentData chan optimisticBlockData) {
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

	// ignore the first optimistic block and block commitment. this is a sync step so that we always start the loop with an optimistic block
	<-optimisticBlockChannel

	<-blockCommitmentChannel

	txMiningInfoRes := make(chan TxMiningInfo)

	select {
	case <-optimisticBlockChannel:
		// the auction starts, trigger the searcher task
		searcherTasksWaitGroup.Add(1)

		go searcher.SearcherTask(&searcherTasksWaitGroup, txMiningInfoRes, config.addressToSend, config.amountToSend)
	case <-done:
		fmt.Printf("exiting due to an unexpected error!")
	}

	txMiningInfo := <-txMiningInfoRes
	if txMiningInfo.err != nil {
		os.Exit(1)
	} else {
		fmt.Printf("Successfully mined tx %s at block: %d\n", txMiningInfo.txHash.String(), txMiningInfo.blockNumber)
	}

	searcherTasksWaitGroup.Wait()
}
