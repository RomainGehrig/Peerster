package gossiper

import (
	"time"

	. "github.com/RomainGehrig/Peerster/blockchain"
	. "github.com/RomainGehrig/Peerster/failure"
	. "github.com/RomainGehrig/Peerster/files"
	. "github.com/RomainGehrig/Peerster/network"
	. "github.com/RomainGehrig/Peerster/peers"
	. "github.com/RomainGehrig/Peerster/private"
	. "github.com/RomainGehrig/Peerster/reputation"
	. "github.com/RomainGehrig/Peerster/routing"
	. "github.com/RomainGehrig/Peerster/rumors"
	. "github.com/RomainGehrig/Peerster/simple"
)

type Gossiper struct {
	Name       string
	simpleMode bool
	peers      *PeersHandler
	routing    *RoutingHandler
	rumors     *RumorHandler
	net        *NetworkHandler
	simple     *SimpleHandler
	private    *PrivateHandler
	files      *FileHandler
	blockchain *BlockchainHandler
	failure    *FailureHandler
	reputation *ReputationHandler
}

const DEFAULT_DOWNLOADING_WORKER_COUNT uint = 10

func NewGossiper(uiPort string, gossipAddr string, name string, peers []string, rtimer int, simple bool) *Gossiper {
	simpleHandler := NewSimpleHandler(name, gossipAddr)
	reputationHandler := NewReputationHandler()
	reputationHandler.IncreaseOrCreate(name, 0) // Local initialization of the node with our reputation added
	fileHandler := NewFileHandler(name, DEFAULT_DOWNLOADING_WORKER_COUNT, reputationHandler)
	routingHandler := NewRoutingHandler(time.Duration(rtimer) * time.Second)
	blockchainHandler := NewBlockchainHandler(reputationHandler, name)
	return &Gossiper{
		simpleMode: simple,
		Name:       name,
		peers:      NewPeersHandler(peers),
		routing:    routingHandler,
		net:        NewNetworkHandler(uiPort, gossipAddr),
		simple:     simpleHandler,
		rumors:     NewRumorHandler(name),
		private:    NewPrivateHandler(name),
		failure:    NewFailureHandler(name, simpleHandler, fileHandler, routingHandler, blockchainHandler),
		files:      fileHandler,
		blockchain: blockchainHandler,
		reputation: reputationHandler,
	}
}

func (g *Gossiper) Run() {
	// Small improvement: directly set ourself as the best route to get to ourself
	g.routing.UpdateRoutingTable(g.Name, StringAddress{g.net.GetAddress()})

	go g.net.RunNetworkHandler(g)
	go g.simple.RunSimpleHandler(g.net, g.peers)
	go g.rumors.RunRumorHandler(g.net, g.peers, g.routing)
	go g.routing.RunRoutingHandler(g.peers, g.net, g.rumors)
	go g.private.RunPrivateHandler(g.routing, g.net)
	go g.files.RunFileHandler(g.net, g.peers, g.routing, g.blockchain, g.simple)
	go g.blockchain.RunBlockchainHandler(g.simple)
	go g.failure.RunFailureHandler()

	for {
		// Eternal wait
		time.Sleep(5 * time.Minute)
	}
}
