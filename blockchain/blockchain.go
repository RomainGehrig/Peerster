package blockchain

import (
	"fmt"
	. "github.com/RomainGehrig/Peerster/constants"
	. "github.com/RomainGehrig/Peerster/messages"
	. "github.com/RomainGehrig/Peerster/simple"
	. "github.com/RomainGehrig/Peerster/utils"
	"sync"
)

const TX_PUBLISH_HOP_LIMIT = 10
const BLOCK_PUBLISH_HOP_LIMIT = 20

// To avoid deadlocks: always take blocksLock -> mappingLock -> pendingTxLock
type BlockchainHandler struct {
	blocks        map[SHA256_HASH]*Block
	blocksLock    *sync.RWMutex
	mapping       map[string]SHA256_HASH
	mappingLock   *sync.RWMutex
	pendingTx     map[string]*File
	pendingTxLock *sync.RWMutex
	lastBlockHash SHA256_HASH
	simple        *SimpleHandler
}

func NewBlockchainHandler() *BlockchainHandler {
	return &BlockchainHandler{
		pendingTx:     make(map[string]*File),
		pendingTxLock: &sync.RWMutex{},
		blocks:        make(map[SHA256_HASH]*Block),
		blocksLock:    &sync.RWMutex{},
		mapping:       make(map[string]SHA256_HASH),
		mappingLock:   &sync.RWMutex{},
		lastBlockHash: ZERO_SHA256_HASH,
	}
}

func (b *BlockchainHandler) RunBlockchainHandler(simple *SimpleHandler) {
	b.simple = simple

	go b.runMiner()
}

func (b *BlockchainHandler) isTXValid(tx *TxPublish) bool {
	// Careful with deadlocks !
	b.mappingLock.RLock()
	defer b.mappingLock.RUnlock()

	b.pendingTxLock.RLock()
	defer b.pendingTxLock.RUnlock()

	txName := tx.File.Name
	_, pending := b.pendingTx[txName]
	_, present := b.mapping[txName]

	return !(pending || present)
}

func (b *BlockchainHandler) HandleTxPublish(tx *TxPublish) {
	// Should not block
	fmt.Println("Got txpublish:", tx)

	go func() {
		if !b.isTXValid(tx) {
			return
		}

		b.pendingTxLock.Lock()
		defer b.pendingTxLock.Unlock()

		b.pendingTx[tx.File.Name] = &tx.File

		// Flood Tx if there is still budget
		if b.prepareTxPublish(tx) {
			b.simple.BroadcastMessage(tx, nil)
		}
	}()
}

func (b *BlockchainHandler) blockIsAcceptable(blk *Block) bool {
	// TODO Exercise 1: can only add to last seen block
	return b.lastBlockHash == ZERO_SHA256_HASH || blk.PrevHash == b.lastBlockHash
}

func (b *BlockchainHandler) HandleBlockPublish(blockPub *BlockPublish) {
	// Should not block
	fmt.Println("Got blockpublish:", blockPub)
	go func() {
		blk := &blockPub.Block

		// Skip blocks we can't take
		if !blk.HasValidPoW() || !b.blockIsAcceptable(blk) {
			return
		}

		b.acceptBlock(blk)

		// Forward if enough budget
		if b.prepareBlockPublish(blockPub) {
			b.simple.BroadcastMessage(blockPub, nil)
		}
	}()
}

func (b *BlockchainHandler) acceptBlock(blk *Block) {
	// Careful with deadlocks !
	b.blocksLock.Lock()
	defer b.blocksLock.Unlock()

	b.mappingLock.Lock()
	defer b.mappingLock.Unlock()

	b.pendingTxLock.Lock()
	defer b.pendingTxLock.Unlock()

	blkHash := blk.Hash()
	b.blocks[blkHash] = blk

	// TODO Exercise 2, change this:
	// Update last block
	b.lastBlockHash = blkHash

	for _, newTx := range blk.Transactions {
		file := newTx.File
		fileHash, _ := ToHash(file.MetafileHash)
		b.mapping[file.Name] = fileHash

		// TODO Exercise 2: Only invalidate transactions that are invalidated
		// by longest chain, ie. if the new block is the new head of the chain

		// Delete transactions that are invalidated by block
		if _, present := b.pendingTx[file.Name]; present {
			delete(b.pendingTx, file.Name)
		}
	}
}

func (b *BlockchainHandler) LongestChainPrevHash() SHA256_HASH {
	// TODO Exercise 2: hash of longest chain
	return b.lastBlockHash
}

func (b *BlockchainHandler) PublishBindingForFile(file *File) {
	tx := &TxPublish{
		File:     *file,
		HopLimit: TX_PUBLISH_HOP_LIMIT + 1,
	}

	// Handle our own transaction
	b.HandleTxPublish(tx)
}

func (b *BlockchainHandler) prepareTxPublish(tx *TxPublish) bool {
	if tx.HopLimit <= 1 {
		tx.HopLimit = 0
		return false
	}

	tx.HopLimit -= 1
	return true

}

func (b *BlockchainHandler) prepareBlockPublish(blk *BlockPublish) bool {
	if blk.HopLimit <= 1 {
		blk.HopLimit = 0
		return false
	}

	blk.HopLimit -= 1
	return true

}
