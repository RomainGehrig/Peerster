package blockchain

import (
	"fmt"
	"strings"
	"sync"

	. "github.com/RomainGehrig/Peerster/constants"
	. "github.com/RomainGehrig/Peerster/messages"
	. "github.com/RomainGehrig/Peerster/reputation"
	. "github.com/RomainGehrig/Peerster/simple"
	. "github.com/RomainGehrig/Peerster/utils"
)

const TX_PUBLISH_HOP_LIMIT = 10
const BLOCK_PUBLISH_HOP_LIMIT = 20
const PRINT_CHAIN_OPS = false

type BlockAugmented struct {
	block  *Block
	height uint
}

type MissingBlock struct {
	tail *Block
	head *Block
}

// To avoid deadlocks: always take blocksLock -> mappingLock -> pendingTxLock
type BlockchainHandler struct {
	blocks            map[SHA256_HASH]*BlockAugmented
	blocksLock        *sync.RWMutex
	mapping           map[string]SHA256_HASH
	mappingLock       *sync.RWMutex
	pendingTx         map[string]*TxPublish
	pendingTxLock     *sync.RWMutex
	lastBlockHash     SHA256_HASH
	simple            *SimpleHandler
	reputationHandler *ReputationHandler
}

func NewBlockchainHandler(rep *ReputationHandler) *BlockchainHandler {
	return &BlockchainHandler{
		pendingTx:         make(map[string]*TxPublish),
		pendingTxLock:     &sync.RWMutex{},
		blocks:            make(map[SHA256_HASH]*BlockAugmented),
		blocksLock:        &sync.RWMutex{},
		mapping:           make(map[string]SHA256_HASH),
		mappingLock:       &sync.RWMutex{},
		lastBlockHash:     ZERO_SHA256_HASH,
		reputationHandler: rep,
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
	fmt.Println("Received a transaction")
	fmt.Println(tx)

	go func() {
		if !b.isTXValid(tx) {
			return
		}

		b.pendingTxLock.Lock()
		defer b.pendingTxLock.Unlock()

		b.pendingTx[tx.File.Name] = tx

		// Flood Tx if there is still budget
		if b.prepareTxPublish(tx) {
			b.simple.BroadcastMessage(tx, nil)
		}
	}()
}

func (b *BlockchainHandler) blockIsAcceptable(blk *Block) bool {
	b.blocksLock.RLock()
	defer b.blocksLock.RUnlock()

	blkHash := blk.Hash()
	_, blockIsKnown := b.blocks[blkHash]

	return !blockIsKnown && blk.HasValidPoW()
}

func (b *BlockchainHandler) HandleBlockPublish(blockPub *BlockPublish) {

	// Should not block
	go func() {
		blk := &blockPub.Block

		// Skip blocks we can't take
		if !b.blockIsAcceptable(blk) {
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

// Have locks on blocks map !
// `dangling` means the block/chain is not attached to the main chain
func (b *BlockchainHandler) getBlockHeight(newBlk *Block) (height uint, dangling bool) {

	// Parent block was present and with a known height
	prevBlock, present := b.blocks[newBlk.PrevHash]
	if present && prevBlock.height != 0 {
		return prevBlock.height + 1, false
	}

	// Block is genesis
	if newBlk.PrevHash == ZERO_SHA256_HASH {
		return 1, false
	}

	// Otherwise, we make an estimation of the real height because
	// we don't know all blocks
	dangling = true
	height = 1
	curr := newBlk

	for {
		if prevBlock, present := b.blocks[curr.PrevHash]; !present {
			break
		} else {
			curr = prevBlock.block
			height += 1
		}
	}

	return
}

func (b *BlockchainHandler) acceptBlock(newBlk *Block) {
	// Careful with deadlocks !
	b.blocksLock.Lock()
	defer b.blocksLock.Unlock()
	b.mappingLock.Lock()
	defer b.mappingLock.Unlock()
	b.pendingTxLock.Lock()
	defer b.pendingTxLock.Unlock()

	// New block is saved with height
	estimatedHeight, dangling := b.getBlockHeight(newBlk)
	var newHeight uint
	if dangling {
		newHeight = 0
	} else {
		newHeight = estimatedHeight
	}
	newBlkAug := &BlockAugmented{
		block:  newBlk,
		height: newHeight,
	}
	blkHash := newBlk.Hash()
	b.blocks[blkHash] = newBlkAug

	currBlkAug, initialized := b.blocks[b.lastBlockHash]

	// Print "CHAIN ..." only if something changes:
	// - grows current main chain
	// - makes a side chain bigger than the main chain

	// If we grow the current chain
	if b.lastBlockHash == newBlk.PrevHash || !initialized {

		b.lastBlockHash = blkHash
		b.applyBlockTx(newBlk)
		if PRINT_CHAIN_OPS {
			fmt.Println(b.ChainString())
		}

		return
	}

	// We know local we have some blocks in the blockchain
	// => currBlkAug exists
	currHeight, _ := b.getBlockHeight(currBlkAug.block)

	// Side-chain became bigger than current chain
	if estimatedHeight > currHeight {
		rewind, apply := b.blockRewind(currBlkAug, newBlkAug)
		for _, rewBlk := range rewind {
			b.unapplyBlockTx(rewBlk)
		}
		for _, appBlk := range apply {
			b.applyBlockTx(appBlk)
		}
		b.lastBlockHash = blkHash
		if PRINT_CHAIN_OPS {
			fmt.Printf("FORK-LONGER rewind %d blocks\n", len(rewind))
			fmt.Println(b.ChainString())
		}
	} else { // New block in side-chain
		if PRINT_CHAIN_OPS {
			fmt.Printf("FORK-SHORTER %x\n", blkHash)
		}
	}
}

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

	// 1) Rewind to the same height
	for {
		// We don't care if blocks are dangling, we just want the height
		prevHeight, _ := b.getBlockHeight(currPrev.block)
		newHeight, _ := b.getBlockHeight(currNew.block)
		if prevHeight == newHeight {
			break
		}

		if prevHeight > newHeight {
			rewind = append(rewind, currPrev.block)
			currPrev = b.blocks[currPrev.block.PrevHash]
		} else {
			apply = append(apply, currNew.block)
			currNew = b.blocks[currNew.block.PrevHash]
		}
	}

	// 2) Find common ancestor (or if there is none, all the transactions we know of)
	for currPrev.block.PrevHash != currPrev.block.PrevHash {
		// Add both blocks
		rewind = append(rewind, currPrev.block)
		apply = append(apply, currNew.block)

		// Rewind both
		newPrev, prevPresent := b.blocks[currPrev.block.PrevHash]
		newNew, newPresent := b.blocks[currNew.block.PrevHash]
		currPrev = newPrev
		currNew = newNew

		// Stop if we can't find previous blocks, ie. no common ancestor because at least
		// one block is dangling
		if !prevPresent || !newPresent {
			break
		}
	}

	// Reverse apply as it was constructed backwards
	for left, right := 0, len(apply)-1; left < right; left, right = left+1, right-1 {
		apply[left], apply[right] = apply[right], apply[left]
	}

	return
}

// Be sure to have all the needed locks to apply the transactions
func (b *BlockchainHandler) applyBlockTx(blk *Block) {
	// Impact on the reputation
	b.reputationHandler.AcceptNewBlock(*blk)

	for _, newTx := range blk.Transactions {
		file := newTx.File
		fileHash, _ := ToHash(file.MetafileHash)
		b.mapping[file.Name] = fileHash

		// Delete transactions that are invalidated by block
		if _, present := b.pendingTx[file.Name]; present {
			delete(b.pendingTx, file.Name)
		}
	}
}

// Be sure to have all the needed locks to apply the transactions
func (b *BlockchainHandler) unapplyBlockTx(blk *Block) {
	// Impact on reputation
	b.reputationHandler.UndoBlock(*blk)

	for _, oldTx := range blk.Transactions {
		file := oldTx.File
		delete(b.mapping, file.Name)
	}
}

func (b *BlockchainHandler) LongestChainPrevHash() SHA256_HASH {
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

func (b *BlockchainHandler) PublishBlock(blk *Block) {
	blkPub := &BlockPublish{
		Block:    *blk,
		HopLimit: BLOCK_PUBLISH_HOP_LIMIT + 1,
	}

	b.simple.BroadcastMessage(blkPub, nil)
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
