package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/csv"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/google/uuid"
	"log/slog"
	"math/big"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

type SearcherResult struct {
	uuid          uuid.UUID
	Receipt       *types.Receipt
	latency       time.Duration
	latencyMargin time.Duration
	err           error
}

func NewSearcherResult(uuid uuid.UUID, receipt *types.Receipt, latency time.Duration, latencyMargin time.Duration, err error) SearcherResult {
	return SearcherResult{
		uuid:          uuid,
		Receipt:       receipt,
		latency:       latency,
		latencyMargin: latencyMargin,
		err:           err,
	}
}

type Searcher struct {
	privateKey    *ecdsa.PrivateKey
	searcherNonce atomic.Pointer[uint64]
	chainId       *big.Int
	client        *ethclient.Client
}

func NewSearcher(privateKeyHex string, chainId *big.Int, ethClient *ethclient.Client) (*Searcher, error) {
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		slog.Error("can not create private key", "err", err)
		return nil, err
	}

	s := &Searcher{
		privateKey: privateKey,
		chainId:    chainId,
		client:     ethClient,
	}

	initialNonce := uint64(0)
	s.searcherNonce.Store(&initialNonce)

	return s, nil
}

func (s *Searcher) FetchLatestNonce(nonceCh chan uint64) error {
	nonce, err := s.client.PendingNonceAt(context.Background(), crypto.PubkeyToAddress(s.privateKey.PublicKey))
	if err != nil {
		slog.Error("can not fetch latest nonce", "err", err)
		return err
	}
	nonceCh <- nonce

	return nil
}

// triggered when the optimistic block is received. We perform some work which is modelled by a sleepDelay
// and then submit the block to the sequencer. If in case before the sleepDelay we listen to the block commitment
// we reduce our sleep time by the latency margin to submit the tx on time to the auctioneer.
// TODO - add searcher task cancellation via a context.CancelFunc
func (s *Searcher) SearcherTask(uuid uuid.UUID, sleepDelay time.Duration, latencyMargin time.Duration, blockCommitmentCh chan optimisticBlockData, optimisticBlockInfo *OptimisticBlockInfo, searcherResultChan chan SearcherResult, wg *sync.WaitGroup) {
	slog.Info("searcher task started", "uuid", uuid, "delay", sleepDelay.String())
	defer func() {
		timer := time.NewTimer(5 * time.Second)

		select {
		case <-timer.C:
			slog.Info("block commitment not received in time")
		case _, ok := <-blockCommitmentCh:
			if !ok {
				slog.Info("block commitment already drained")
			} else {
				slog.Info("drained block commitment")
			}
		}

		wg.Done()
	}()

	validateBlockCommitment := func(blockCommitment *optimisticBlockData) error {
		if blockCommitment.blockNumber != optimisticBlockInfo.getBlockNumber() {
			// we are not interested in this block commitment
			return fmt.Errorf("block commitment number %v does not match with the optimistic block number %v", blockCommitment.blockNumber, optimisticBlockInfo.getBlockNumber())
		}
		return nil
	}

	timer := time.NewTimer(sleepDelay)
	nonceCh := make(chan uint64)
	go func() {
		err := s.FetchLatestNonce(nonceCh)
		if err != nil {
			slog.Error("can not fetch latest nonce", "err", err)
			return
		}
	}()

	blockCommitmentReceived := false
	select {
	case <-timer.C:
		slog.Info("searching completed on time! Now proceeding to submit tx to auctioneer")
	// we finished the work on time
	// exit and submit tx to auctioneer
	case blockCommitment := <-blockCommitmentCh:
		slog.Info("block commitment received! Stopping searching activity to submit!", "block_number", blockCommitment.blockNumber)
		// now we sleep only for latency margin
		// and submit tx to auctioneer

		// close the block commitment ch since we have finished reading from it
		close(blockCommitmentCh)

		err := validateBlockCommitment(&blockCommitment)
		if err != nil {
			slog.Error("failed block commitment validation", "err", err)
			searcherResultChan <- NewSearcherResult(uuid, nil, sleepDelay, latencyMargin, err)
			return
		}

		blockCommitmentReceived = true
	}

	if blockCommitmentReceived {
		// we still can sleep for latency margin amount
		slog.Info("Sleeping for latency margin time", "latency_margin", latencyMargin.String())
		time.Sleep(latencyMargin)
	}

	// wait for the latest nonce before proceeding to submitting the tx
	slog.Info("waiting for the nonce")
	select {
	case nonce := <-nonceCh:
		slog.Info("nonce received", "nonce", nonce)
		s.searcherNonce.Store(&nonce)
	}

	toAddress := common.HexToAddress("0x507056D8174aC4fb40Ec51578d18F853dFe000B2")
	currentNonce := *s.searcherNonce.Load()

	slog.Info("creating and signing the tx")
	txToSend := types.NewTx(&types.DynamicFeeTx{
		ChainID:   s.chainId,
		Nonce:     currentNonce,
		GasTipCap: big.NewInt(5000000000),
		GasFeeCap: big.NewInt(5000000000),
		Gas:       21000,
		To:        &toAddress,
		Value:     big.NewInt(10000000000000000),
	})
	signedTx, err := types.SignTx(txToSend, types.LatestSignerForChainID(s.chainId), s.privateKey)
	if err != nil {
		slog.Error("can not sign the tx", "err", err)
		searcherResultChan <- NewSearcherResult(uuid, nil, sleepDelay, latencyMargin, fmt.Errorf("can not sign the tx %v", err))
		return
	}

	slog.Info("sending the tx to the auctioneer")
	err = s.client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		slog.Error("can not send the tx", "err", err)
		searcherResultChan <- NewSearcherResult(uuid, nil, sleepDelay, latencyMargin, fmt.Errorf("can not send the tx %v", err))
		return
	}

	timeoutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	slog.Info("waiting for the tx to be mined")
	// we need to wait for one block for the tx to be mined
	receipt, err := bind.WaitMined(timeoutCtx, s.client, signedTx)
	if err != nil {
		slog.Error("can not wait for the tx to be mined", "err", err)
		searcherResultChan <- NewSearcherResult(uuid, nil, sleepDelay, latencyMargin, fmt.Errorf("can not wait for the tx to be mined %v", err))
		return
	}
	if receipt != nil {
		slog.Info("tx mined successfully")
		searcherResultChan <- NewSearcherResult(uuid, receipt, sleepDelay, latencyMargin, nil)
		return
	}
}

type SearcherResultStore struct {
	searcherResultMap map[uuid.UUID]SearcherResult
	mu                sync.Mutex
}

func NewSearcherResultStore() *SearcherResultStore {
	return &SearcherResultStore{
		searcherResultMap: make(map[uuid.UUID]SearcherResult),
		mu:                sync.Mutex{},
	}
}

func (s *SearcherResultStore) AddSearcherResult(searcherResult SearcherResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.searcherResultMap[searcherResult.uuid]
	if ok {
		panic(fmt.Sprintf("searcher result already exists for uuid %v", searcherResult.uuid))
	}

	s.searcherResultMap[searcherResult.uuid] = searcherResult
}

func (s *SearcherResultStore) GetSearcherResult(uuid uuid.UUID) (SearcherResult, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	searcherResult, ok := s.searcherResultMap[uuid]
	return searcherResult, ok
}

func (s *SearcherResultStore) PrintSearcherResults() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.searcherResultMap) == 0 {
		slog.Info("no searcher results to print")
		return
	}

	for k, v := range s.searcherResultMap {
		if v.err != nil {
			slog.Error("searcher result error", "uuid", k, "err", v.err, "latency", v.latency.String())
			continue
		}

		if v.Receipt != nil {
			slog.Info("searcher result", "uuid", k, "tx hash", v.Receipt.TxHash.String(), "block number", v.Receipt.BlockNumber.Uint64(), "latency", v.latency.String(), "latency_margin", v.latencyMargin.String())
		}
	}
}

func (s *SearcherResultStore) WriteResultsToCsvFile(fileName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if FileExists(fileName) {
		return fmt.Errorf("file %v already exists", fileName)
	}

	file, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// write headers
	err = writer.Write([]string{"uuid", "success", "tx_hash", "block_number", "latency", "latency_margin"})
	if err != nil {
		slog.Error("can not write headers to csv file", "err", err)
		return fmt.Errorf("can not write headers to csv file %v", err)
	}

	for k, v := range s.searcherResultMap {
		success := "false"
		txHash := ""
		blockNumber := ""
		if v.err == nil {
			success = "true"
			if v.Receipt != nil {
				txHash = v.Receipt.TxHash.String()
				blockNumber = fmt.Sprintf("%v", v.Receipt.BlockNumber.Uint64())
			}
		}

		err = writer.Write([]string{k.String(), success, txHash, blockNumber, v.latency.String(), v.latencyMargin.String()})
		if err != nil {
			slog.Error("can not write row to csv file", "err", err)
			return fmt.Errorf("can not write row to csv file %v", err)
		}
	}

	return nil

}

func (s *SearcherResultStore) SearcherResultCount() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return uint64(len(s.searcherResultMap))
}
