package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"math/big"
	"sync"
	"sync/atomic"
	"time"
)

type TxMiningInfo struct {
	txHash      common.Hash
	blockNumber uint64
	err         error
}

func NewTxMiningInfo(receipt *types.Receipt) TxMiningInfo {
	return TxMiningInfo{
		blockNumber: receipt.BlockNumber.Uint64(),
		txHash:      receipt.TxHash,
		err:         nil,
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
		fmt.Printf("can not create private key: err: %v\n", err.Error())
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

func (s *Searcher) FetchLatestNonce() (uint64, error) {
	nonce, err := s.client.PendingNonceAt(context.Background(), crypto.PubkeyToAddress(s.privateKey.PublicKey))
	if err != nil {
		fmt.Printf("can not fetch latest nonce: %v\n", err.Error())
		return 0, err
	}

	return nonce, nil
}

// triggered when the optimistic block is received. We perform some work which is modelled by a sleepDelay
// and then submit the block to the sequencer. If in case before the sleepDelay we listen to the block commitment
// we reduce our sleep time by the latency margin to submit the tx on time to the auctioneer.
func (s *Searcher) SearcherTask(wg *sync.WaitGroup, txMiningInfoResult chan TxMiningInfo, addressToSend common.Address, amountToSend *big.Int) {
	defer wg.Done()

	type nonceResult struct {
		nonce uint64
		err   error
	}

	nonceCh := make(chan nonceResult)
	go func() {
		nonce, err := s.FetchLatestNonce()
		if err != nil {
			nonceCh <- nonceResult{
				err: err,
			}
			fmt.Printf("can not fetch latest nonce: %s\n", err.Error())
			return
		}

		nonceCh <- nonceResult{nonce: nonce, err: err}
	}()

	// give time for auctioneer to receive the executed optimistic block
	time.Sleep(50 * time.Millisecond)

	// wait for the latest nonce before proceeding to submitting the tx
	nonce := <-nonceCh
	if nonce.err != nil {
		txMiningInfoResult <- TxMiningInfo{
			err: nonce.err,
		}
		return
	}

	s.searcherNonce.Store(&nonce.nonce)

	currentNonce := *s.searcherNonce.Load()

	txToSend := types.NewTx(&types.DynamicFeeTx{
		ChainID:   s.chainId,
		Nonce:     currentNonce,
		GasTipCap: big.NewInt(5000000000),
		GasFeeCap: big.NewInt(5000000000),
		Gas:       21000,
		To:        &addressToSend,
		Value:     amountToSend,
	})
	signedTx, err := types.SignTx(txToSend, types.LatestSignerForChainID(s.chainId), s.privateKey)
	if err != nil {
		txMiningInfoResult <- TxMiningInfo{
			err: err,
		}
		fmt.Printf("can not sign the tx: %s\n", err.Error())
		return
	}

	fmt.Printf("Submitting tx hash: %s with bid: %d\n", signedTx.Hash().Hex(), signedTx.GasTipCap())

	err = s.client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		txMiningInfoResult <- TxMiningInfo{
			err: err,
		}
		fmt.Printf("can not send the tx: err: %s\n", err.Error())
		return
	}

	timeoutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// we need to wait for one block for the tx to be mined
	receipt, err := bind.WaitMined(timeoutCtx, s.client, signedTx)
	if err != nil {
		txMiningInfoResult <- TxMiningInfo{
			err: err,
		}
		fmt.Printf("can not wait for the tx to be mined: %s\n", err.Error())
		return
	}
	if receipt != nil {
		txMiningInfo := NewTxMiningInfo(receipt)
		txMiningInfoResult <- txMiningInfo
		return
	}
}
