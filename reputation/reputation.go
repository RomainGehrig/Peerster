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

func NewReputationHandler() *ReputationHandler {
	return &ReputationHandler{
		AllReputations: make(map[string]int64),
		lock:           &sync.RWMutex{},
	}
}

// ReputationHandler is a struct that will represents all informations related to the reputation part of the project
type ReputationHandler struct {
	AllReputations map[string]int64
	lock           *sync.RWMutex
}

func (r *ReputationHandler) CanDownloadChunk(nodename string) bool {
	r.IncreaseOrCreate(nodename, 0)
	return r.AllReputations[nodename] > 0
}

// undoTransaction will undo the effect of the given transaction on the reputation
func (r *ReputationHandler) UndoTransaction(transaction TxPublish) {
	r.AffectTransaction(transaction, true)
}

// acceptNewTransaction will do the modifications with a new transaction being accepted
func (r *ReputationHandler) AcceptNewTransaction(transaction TxPublish) {
	r.AffectTransaction(transaction, false)
}

func (r *ReputationHandler) IncreaseOrCreate(name string, amount int64) {
	// Small lock to avoid concurrence problems with modifications coming from different threads
	r.lock.Lock()
	defer r.lock.Unlock()

	_, ok := r.AllReputations[name]
	if !ok {
		r.AllReputations[name] = STARTING
	}
	r.AllReputations[name] = r.AllReputations[name] + amount
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
		r.IncreaseOrCreate(indexer, coef*INDEX)
	}

	// A chunk was downloaded successfully
	if transaction.Type == DownloadSuccess {
		downloader := transaction.NodeOrigin
		seeder := transaction.NodeDestination
		r.IncreaseOrCreate(seeder, coef*SERVE)
		r.IncreaseOrCreate(downloader, coef*(-DOWNLOAD))
	}

	// A chunk wasn't downloaded successfully
	if transaction.Type == DownloadFail {
		seeder := transaction.NodeDestination
		r.IncreaseOrCreate(seeder, coef*(-FAIL))
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

func (r *ReputationHandler) CreateTimeoutTransaction(fname, localnodename, peername string) *TxPublish {
	return &TxPublish{Type: DownloadFail, NodeOrigin: localnodename, NodeDestination: peername,
		File: File{Name: fname, Size: 0, MetafileHash: []byte{}}}
}
