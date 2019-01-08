package reputation

import (
	"sync"

	. "github.com/RomainGehrig/Peerster/messages"
)

// Constants to use when modifying the reputation a peer
const (
	INDEX    = 5
	DOWNLOAD = 2
	SERVE    = 4
	FAIL     = 10
	STARTING = 15
)

// ReputationHandler is a struct that will represents all informations related to the reputation part of the project
type ReputationHandler struct {
	allReputations map[string]int64
	lock           *sync.RWMutex
}

func (r *ReputationHandler) CanDownloadChunk(nodename string) bool {
	r.increaseOrCreate(nodename, 0)
	return r.allReputations[nodename] > 0
}

// undoTransaction will undo the effect of the given transaction on the reputation
func (r *ReputationHandler) UndoTransaction(transaction TxPublish) {
	r.AffectTransaction(transaction, true)
}

// acceptNewTransaction will do the modifications with a new transaction being accepted
func (r *ReputationHandler) AcceptNewTransaction(transaction TxPublish) {
	r.AffectTransaction(transaction, false)
}

func (r *ReputationHandler) increaseOrCreate(name string, amount int64) {
	// Small lock to avoid concurrence problems with modifications coming from different threads
	r.lock.Lock()
	defer r.lock.Unlock()

	res, ok := r.allReputations[name]
	if !ok {
		r.allReputations[name] = STARTING
	}
	r.allReputations[name] = res + amount
}

// affectTransaction is a method that will modify the mapping for a transaction
func (r *ReputationHandler) AffectTransaction(transaction TxPublish, reversed bool) {
	coef := int64(1)
	if reversed {
		coef = int64(-1)
	}
	// Case of a new chunk being indexed
	if transaction.Type == IndexChunk {
		indexer := transaction.NodeOrigin
		r.increaseOrCreate(indexer, coef*INDEX)
	}

	// A chunk was downloaded successfully
	if transaction.Type == DownloadSuccess {
		downloader := transaction.NodeOrigin
		seeder := transaction.NodeDestination
		r.increaseOrCreate(seeder, coef*SERVE)
		r.increaseOrCreate(downloader, coef*(-DOWNLOAD))
	}

	// A chunk wasn't downloaded successfully
	if transaction.Type == DownloadFail {
		seeder := transaction.NodeDestination
		r.increaseOrCreate(seeder, coef*(-FAIL))
	}
}

func (r *ReputationHandler) AffectBlock(block Block, reversed bool) {
	for i := 0; i < len(block.Transactions); i++ {
		r.AffectTransaction(block.Transactions[i], reversed)
	}
}

func (r *ReputationHandler) AcceptNewBlock(block Block) {
	r.AffectBlock(block, false)
}

func (r *ReputationHandler) UndoBlock(block Block) {
	r.AffectBlock(block, true)
}
