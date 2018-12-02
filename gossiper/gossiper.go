package gossiper

import (
	. "github.com/RomainGehrig/Peerster/files"
	. "github.com/RomainGehrig/Peerster/network"
	. "github.com/RomainGehrig/Peerster/peers"
	. "github.com/RomainGehrig/Peerster/private"
	. "github.com/RomainGehrig/Peerster/routing"
	. "github.com/RomainGehrig/Peerster/rumors"
	. "github.com/RomainGehrig/Peerster/simple"
	"time"
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
}

const DEFAULT_DOWNLOADING_WORKER_COUNT uint = 10

func NewGossiper(uiPort string, gossipAddr string, name string, peers []string, rtimer int, simple bool) *Gossiper {
	return &Gossiper{
		simpleMode: simple,
		Name:       name,
		peers:      NewPeersHandler(peers),
		routing:    NewRoutingHandler(time.Duration(rtimer) * time.Second),
		net:        NewNetworkHandler(uiPort, gossipAddr),
		simple:     NewSimpleHandler(name, gossipAddr),
		rumors:     NewRumorHandler(name),
		private:    NewPrivateHandler(name),
		files:      NewFileHandler(name, DEFAULT_DOWNLOADING_WORKER_COUNT),
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
	go g.files.RunFileHandler(g.peers, g.routing)

	for {
		// Eternal wait
		time.Sleep(5 * time.Minute)
	}
}
