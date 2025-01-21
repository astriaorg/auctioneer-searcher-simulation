package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/google/uuid"
	"log/slog"
	"math/big"
	"sync"
	"sync/atomic"
	"time"
)

type SearcherResult struct {
	uuid    uuid.UUID
	Receipt *types.Receipt
	err     error
}

func NewSearcherResult(uuid uuid.UUID, receipt *types.Receipt, err error) SearcherResult {
	return SearcherResult{
		uuid:    uuid,
		Receipt: receipt,
		err:     err,
	}
}

type Searcher struct {
	privateKey    *ecdsa.PrivateKey
	searcherNonce atomic.Pointer[uint64]
	chainId       *big.Int
	client        *ethclient.Client
}

func NewSearcher(privateKeyHex string, chainId *big.Int, ethClient *ethclient.Client) (*Searcher, error) {
	privateKey, err := crypto.HexToECDSA("528a032646bf3498e48994778a0c1f9a535541142a6356808c9234dd3614882e")
	if err != nil {
		slog.Error("can not create private key %v", err)
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
func (s *Searcher) SearcherTask(uuid uuid.UUID, sleepDelay time.Duration, blockCommitmentCh chan optimisticBlockData, optimisticBlockInfo *OptimisticBlockInfo, searcherResultChan chan SearcherResult, latencyMargin uint64, wg *sync.WaitGroup) {
	slog.Info("searcher task started", "uuid", uuid, "delay", sleepDelay.String())
	defer wg.Done()

	timer := time.NewTimer(sleepDelay)
	nonceCh := make(chan uint64)
	go func() {
		err := s.FetchLatestNonce(nonceCh)
		if err != nil {
			slog.Error("can not fetch latest nonce", "err", err)
			return
		}
	}()

	searchingCompletedOnTime := false
	select {
	case <-timer.C:
		slog.Info("searching completed on time! Now proceeding to submit tx to auctioneer")
		// we finished the work on time
		// exit and submit tx to auctioneer
		searchingCompletedOnTime = true
	case blockCommitment := <-blockCommitmentCh:
		slog.Info("block commitment received! Now proceeding to submit tx to auctioneer after latency margin")
		// now we sleep only for latency margin
		// and submit tx to auctioneer
		if blockCommitment.blockNumber != optimisticBlockInfo.getBlockNumber() {
			slog.Error("block commitment number does not match with the optimistic block number", "block commitment number", blockCommitment.blockNumber, "optimistic block number", optimisticBlockInfo.getBlockNumber())
			// we are not interested in this block commitment
			searcherResultChan <- NewSearcherResult(uuid, nil, fmt.Errorf("block commitment number %v does not match with the optimistic block number %v", blockCommitment.blockNumber, optimisticBlockInfo.getBlockNumber()))
			return
		}
		if bytes.Equal(blockCommitment.blockHash, optimisticBlockInfo.getBlockHash()) {
			slog.Error("block commitment hash does not match with the optimistic block hash", "block commitment hash", blockCommitment.blockHash, "optimistic block hash", optimisticBlockInfo.getBlockHash())
			// we are not interested in this block commitment
			searcherResultChan <- NewSearcherResult(uuid, nil, fmt.Errorf("block commitment hash %v does not match with the optimistic block hash %v", blockCommitment.blockHash, optimisticBlockInfo.getBlockHash()))
			return
		}
	}
	if !searchingCompletedOnTime {
		slog.Info("waiting for latency margin! proceeding to submit tx to auctioneer after latency margin")
		time.Sleep(time.Duration(latencyMargin) * time.Millisecond)
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
		searcherResultChan <- NewSearcherResult(uuid, nil, fmt.Errorf("can not sign the tx %v", err))
		return
	}

	slog.Info("sending the tx to the auctioneer")
	err = s.client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		slog.Error("can not send the tx", "err", err)
		searcherResultChan <- NewSearcherResult(uuid, nil, fmt.Errorf("can not send the tx %v", err))
		return
	}

	timeoutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	slog.Info("waiting for the tx to be mined")
	// we need to wait for one block for the tx to be mined
	receipt, err := bind.WaitMined(timeoutCtx, s.client, signedTx)
	if err != nil {
		slog.Error("can not wait for the tx to be mined", "err", err)
		slog.Info("sending searcher result to searcher result chan!")
		searcherResultChan <- NewSearcherResult(uuid, nil, fmt.Errorf("can not wait for the tx to be mined %v", err))
		return
	}
	if receipt != nil {
		slog.Info("tx mined successfully")
		searcherResultChan <- NewSearcherResult(uuid, receipt, nil)
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
		slog.Info("searcher result", "uuid", k)
		if v.err != nil {
			slog.Error("searcher result error", "err", v.err)
			continue
		}

		if v.Receipt != nil {
			slog.Info("searcher result", "uuid", k, "tx hash", v.Receipt.TxHash.String(), "block number", v.Receipt.BlockNumber.Uint64())
		}
	}
}

func (s *SearcherResultStore) SearcherResultCount() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return uint64(len(s.searcherResultMap))
}
