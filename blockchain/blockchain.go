package blockchain

import (
	"fmt"
	. "github.com/RomainGehrig/Peerster/constants"
	. "github.com/RomainGehrig/Peerster/messages"
	. "github.com/RomainGehrig/Peerster/simple"
)

const TX_PUBLISH_HOP_LIMIT = 10
const BLOCK_PUBLISH_HOP_LIMIT = 20
type BlockchainHandler struct {
	simple  *SimpleHandler
}

func NewBlockchainHandler() *BlockchainHandler {
	return &BlockchainHandler{}
}

func (b *BlockchainHandler) RunBlockchainHandler(simple *SimpleHandler) {
	b.simple = simple
}

func (b *BlockchainHandler) HandleTxPublish(tx *TxPublish) {
	// Should not block
	fmt.Println("Got txpublish:", tx)
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
}
