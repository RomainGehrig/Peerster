package blockchain

import (
	"fmt"
	. "github.com/RomainGehrig/Peerster/constants"
	. "github.com/RomainGehrig/Peerster/messages"
	. "github.com/RomainGehrig/Peerster/simple"
	"sync"
)

const TX_PUBLISH_HOP_LIMIT = 10
const BLOCK_PUBLISH_HOP_LIMIT = 20

type BlockchainHandler struct {
	pendingTx     map[string]*File
	blocks        map[SHA256_HASH]*Block
	mapping       map[string]SHA256_HASH
	mappingLock   *sync.RWMutex
	pendingTxLock *sync.RWMutex
	simple        *SimpleHandler
}

func NewBlockchainHandler() *BlockchainHandler {
	return &BlockchainHandler{
		pendingTx:     make(map[string]*File),
		blocks:        make(map[SHA256_HASH]*Block),
		mapping:       make(map[string]SHA256_HASH),
		mappingLock:   &sync.RWMutex{},
		pendingTxLock: &sync.RWMutex{},
	}
}

func (b *BlockchainHandler) RunBlockchainHandler(simple *SimpleHandler) {
	b.simple = simple
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

func (b *BlockchainHandler) HandleBlockPublish(blockPub *BlockPublish) {
	// Should not block
	fmt.Println("Got blockpublish:", blockPub)
	blk := blockPub.Block

	// TODO
	if !blk.HasValidPoW() {
		return
	}
	// TODO
}

func (b *BlockchainHandler) PublishBindingForFile(file *File) {
	tx := &TxPublish{
		File:     *file,
		HopLimit: TX_PUBLISH_HOP_LIMIT,
	}

	b.simple.BroadcastMessage(tx, nil)

	// TODO add transaction to self
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
