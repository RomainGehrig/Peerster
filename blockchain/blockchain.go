package blockchain

import (
	"fmt"
	. "github.com/RomainGehrig/Peerster/constants"
	. "github.com/RomainGehrig/Peerster/messages"
	. "github.com/RomainGehrig/Peerster/simple"
	. "github.com/RomainGehrig/Peerster/utils"
	"strings"
	"sync"
)

const TX_PUBLISH_HOP_LIMIT = 10
const BLOCK_PUBLISH_HOP_LIMIT = 20

type BlockAugmented struct {
	block  *Block
	height uint
}

// To avoid deadlocks: always take blocksLock -> mappingLock -> pendingTxLock
type BlockchainHandler struct {
	blocks        map[SHA256_HASH]*BlockAugmented
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
		blocks:        make(map[SHA256_HASH]*BlockAugmented),
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

// Make sure the caller has locks on the map !
func (b *BlockchainHandler) ChainString() string {
	blocks := make([]string, 0)
	currHash := b.LongestChainPrevHash()

	for currHash != ZERO_SHA256_HASH {
		blk, present := b.blocks[currHash]
		if !present {
			break
		}
		block := blk.block
		blocks = append(blocks, block.String())
		currHash = block.PrevHash
	}

	return fmt.Sprintf("CHAIN %s", strings.Join(blocks, " "))
}

func (b *BlockchainHandler) acceptBlock(newBlk *Block) {
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

/*
  Compute what has to be done to go from `prev` to `new`.

  Return block splice are ordered to facilitate iteration:
  `rewind` is ordered from newer to older blocks
  `apply` is ordered from older to newer blocks
*/
func (b *BlockchainHandler) blockRewind(prev *BlockAugmented, new *BlockAugmented) (rewind []*Block, apply []*Block) {
	rewind = make([]*Block, 0)
	// Apply is constructed in the wrong order and then inversed
	apply = make([]*Block, 0)

	currPrev := prev
	currNew := new

	// Rewind to the same height
	for currPrev.height != currNew.height {
		if currPrev.height > currNew.height {
			rewind = append(rewind, currPrev.block)
			currPrev = b.blocks[currPrev.block.PrevHash]
		} else {
			apply = append(apply, currNew.block)
			currNew = b.blocks[currNew.block.PrevHash]
		}
	}

	// Find common ancestor
	for currPrev.block.PrevHash != currPrev.block.PrevHash {
		currPrev = b.blocks[currPrev.block.PrevHash]
		currNew = b.blocks[currNew.block.PrevHash]
		rewind = append(rewind, currPrev.block)
		apply = append(apply, currNew.block)
	}

	// Reverse apply as it was constructed backwards
	for left, right := 0, len(apply)-1; left < right; left, right = left+1, right-1 {
		apply[left], apply[right] = apply[right], apply[left]
	}

	return
}

// Be sure to have all the needed locks to apply the transactions
func (b *BlockchainHandler) applyBlockTx(blk *Block) {
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
