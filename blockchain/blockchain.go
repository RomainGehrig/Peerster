package blockchain

import (
	"fmt"
	"strings"
	"sync"

	. "github.com/RomainGehrig/Peerster/constants"
	. "github.com/RomainGehrig/Peerster/messages"
	. "github.com/RomainGehrig/Peerster/reputation"
	. "github.com/RomainGehrig/Peerster/simple"
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
	mapping           map[SHA256_HASH]struct{}
	mappingLock       *sync.RWMutex
	NewOwner          map[SHA256_HASH]bool
	NewOwnerLock      *sync.RWMutex
	DeleteOwner       map[SHA256_HASH]bool
	DeleteOwnerLock   *sync.RWMutex
	OwnerID           map[SHA256_HASH]uint64
	OwnerIDLock       *sync.RWMutex
	pendingID         map[SHA256_HASH]uint64
	pendingIDLock     *sync.RWMutex
	pendingTx         map[SHA256_HASH]*TxPublish
	pendingTxLock     *sync.RWMutex
	lastBlockHash     SHA256_HASH
	name              string
	simple            *SimpleHandler
	reputationHandler *ReputationHandler
}

func NewBlockchainHandler(rep *ReputationHandler, designation string) *BlockchainHandler {
	return &BlockchainHandler{
		pendingTx:         make(map[SHA256_HASH]*TxPublish),
		pendingTxLock:     &sync.RWMutex{},
		blocks:            make(map[SHA256_HASH]*BlockAugmented),
		blocksLock:        &sync.RWMutex{},
		mapping:           make(map[SHA256_HASH]struct{}),
		mappingLock:       &sync.RWMutex{},
		NewOwner:          make(map[SHA256_HASH]bool),
		NewOwnerLock:      &sync.RWMutex{},
		DeleteOwner:       make(map[SHA256_HASH]bool),
		DeleteOwnerLock:   &sync.RWMutex{},
		OwnerID:           make(map[SHA256_HASH]uint64),
		OwnerIDLock:       &sync.RWMutex{},
		pendingID:         make(map[SHA256_HASH]uint64),
		pendingIDLock:     &sync.RWMutex{},
		lastBlockHash:     ZERO_SHA256_HASH,
		name:              designation,
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

	b.pendingIDLock.RLock()
	defer b.pendingIDLock.RUnlock()

	//Special case if it is a block to announce a new main host
	if tx.Type == NewMaster {
		var hash_sha [32]byte
		copy(hash_sha[:], tx.TargetHash)

		id, present := b.pendingID[hash_sha]
		if present {
			return tx.ID > id
		} else {
			registerID, present := b.OwnerID[hash_sha]
			if present {
				return tx.ID > registerID
			} else {
				return tx.ID > 0
			}
		}
	} else {
		txName := tx.Hash()
		_, pending := b.pendingTx[txName]
		_, present := b.mapping[txName]

		return !(pending || present)
	}
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

		b.pendingTx[tx.Hash()] = tx

		if tx.Type == NewMaster {
			b.pendingIDLock.Lock()
			defer b.pendingIDLock.Unlock()
			var hash_sha [32]byte
			copy(hash_sha[:], tx.TargetHash)
			b.pendingID[hash_sha] = tx.ID
		}
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

	b.OwnerIDLock.Lock()
	defer b.OwnerIDLock.Unlock()

	b.pendingIDLock.Lock()
	defer b.pendingIDLock.Unlock()

	b.NewOwnerLock.Lock()
	defer b.NewOwnerLock.Unlock()

	for _, newTx := range blk.Transactions {

		mappingName := newTx.Hash()
		b.mapping[mappingName] = struct{}{}

		//Special case if it is a block to announce a new main host
		if newTx.Type == NewMaster {
			var hash_sha [32]byte
			copy(hash_sha[:], newTx.TargetHash)
			b.OwnerID[hash_sha] = newTx.ID
			if newTx.NodeOrigin == b.name {
				b.NewOwner[hash_sha] = true
			}

			// Delete transactions that are invalidated by block
			ID, present := b.pendingID[hash_sha]
			if present && ID <= newTx.ID {
				for k, v := range b.pendingTx {
					if v.Type == NewMaster {
						var pending_hash_sha [32]byte
						copy(pending_hash_sha[:], v.TargetHash)
						if pending_hash_sha == hash_sha && v.ID <= newTx.ID {
							delete(b.pendingTx, k)
						}
					}
				}

				delete(b.pendingID, hash_sha)
			}
		}

		// Delete transactions that are invalidated by block
		if _, present := b.pendingTx[mappingName]; present {
			delete(b.pendingTx, mappingName)
		}
	}
}

// Be sure to have all the needed locks to apply the transactions
func (b *BlockchainHandler) unapplyBlockTx(blk *Block) {
	// Impact on reputation
	b.reputationHandler.UndoBlock(*blk)
	b.NewOwnerLock.Lock()
	defer b.NewOwnerLock.Unlock()

	b.DeleteOwnerLock.Lock()
	defer b.DeleteOwnerLock.Unlock()
	for _, oldTx := range blk.Transactions {
		//Special case if it is a block to announce a new main host
		if oldTx.Type == NewMaster && oldTx.NodeOrigin == b.name {
			var hash_sha [32]byte
			copy(hash_sha[:], oldTx.TargetHash)
			if _, present := b.NewOwner[hash_sha]; present {
				delete(b.NewOwner, hash_sha)
			}
			b.DeleteOwner[hash_sha] = true
		}
		delete(b.mapping, oldTx.Hash())
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
