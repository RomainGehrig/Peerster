package blockchain

import (
	"fmt"
	. "github.com/RomainGehrig/Peerster/constants"
	. "github.com/RomainGehrig/Peerster/messages"
	"math/rand"
)

func (b *BlockchainHandler) runMiner() {
	// TODO Timings for publishing
	var nonce SHA256_HASH

	for {
		// Get pending transactions
		b.pendingTxLock.RLock()
		pendingTx := make([]TxPublish, 0)
		for _, file := range b.pendingTx {
			pendingTx = append(pendingTx, TxPublish{
				HopLimit: TX_PUBLISH_HOP_LIMIT,
				File:     *file,
			})
		}
		b.pendingTxLock.RUnlock()

		// Get previous block hash
		prevHash := b.LongestChainPrevHash()

		// TODO Performance improvement: repeat the next instructions 1000 times before reloading the transactions/prevHash

		// Modify a bit the nonce (a random byte)
		randVal := byte(rand.Int())
		nonce[rand.Intn(len(nonce))] = randVal

		// Make a block
		block := &Block{
			PrevHash:     prevHash,
			Nonce:        nonce,
			Transactions: pendingTx,
		}

		// If valid => publish after x seconds
		if block.HasValidPoW() {
			fmt.Printf("BLOCK HAS VALID HASH (%d tx): %x\n", len(pendingTx), block.Hash())
		}

		// Repeat
	}
}
