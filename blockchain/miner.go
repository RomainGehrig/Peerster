package blockchain

import (
	"fmt"
	. "github.com/RomainGehrig/Peerster/constants"
	. "github.com/RomainGehrig/Peerster/messages"
	"math/rand"
	"time"
)

const SLEEP_DURATION_AFTER_GENESIS = 5 * time.Second

func (b *BlockchainHandler) runMiner() {
	var nonce SHA256_HASH

	startTime := time.Now()
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
			// Time from start to mine
			miningDuration := time.Now().Sub(startTime)

			fmt.Printf("FOUND-BLOCK %x\n", block.Hash())
			b.acceptBlock(block)

			go func() {
				// Sleep to add more forks in the system

				// Wait more before publishing the genesis block
				if prevHash == ZERO_SHA256_HASH {
					time.Sleep(SLEEP_DURATION_AFTER_GENESIS)
				} else {
					time.Sleep(2 * miningDuration)
				}

				b.PublishBlock(block)
			}()

			startTime = time.Now()
		}
		// Repeat
	}
}
