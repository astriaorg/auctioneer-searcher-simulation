package main

import (
	"sync/atomic"
)

type OptimisticBlockInfo struct {
	blockHash   atomic.Pointer[[]byte]
	blockNumber atomic.Pointer[uint64]
}

func (o *OptimisticBlockInfo) getBlockHash() []byte {
	return *o.blockHash.Load()
}

func (o *OptimisticBlockInfo) getBlockNumber() uint64 {
	return *o.blockNumber.Load()
}

func (o *OptimisticBlockInfo) SetBlockHash(blockHash []byte) {
	o.blockHash.Store(&blockHash)
}

func (o *OptimisticBlockInfo) SetBlockNumber(blockNumber uint64) {
	o.blockNumber.Store(&blockNumber)
}
