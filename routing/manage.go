package routing

import (
	"fmt"
	. "github.com/RomainGehrig/Peerster/messages"
	. "github.com/RomainGehrig/Peerster/network"
	. "github.com/RomainGehrig/Peerster/peers"
	. "github.com/RomainGehrig/Peerster/rumors"
	"time"
)

type RoutingHandler struct {
	routingTable map[string]string // From Origin to ip:port
	rtimer       time.Duration

	// Modules needed by this handler
	net    *NetworkHandler
	peers  *PeersHandler
	rumors *RumorHandler
}

func NewRoutingHandler(rtimer time.Duration) *RoutingHandler {
	return &RoutingHandler{
		routingTable: make(map[string]string),
		rtimer:       rtimer,
	}
}

func (r *RoutingHandler) RunRoutingHandler(peers *PeersHandler, net *NetworkHandler, rumors *RumorHandler) {
	r.peers = peers
	r.net = net
	r.rumors = rumors
	go r.runRoutingMessages()
}

func (r *RoutingHandler) runRoutingMessages() {
	if r.rtimer == 0*time.Second {
		return
	}

	ticker := time.NewTicker(r.rtimer)
	defer ticker.Stop()

	// Wait a tiny bit before sending the first rumor
	time.Sleep(200 * time.Millisecond)

	// Start by sending a routing message to all neighbors
	r.sendRoutingMessage(r.peers.AllPeers()...)

	for {
		// Wait for ticker
		<-ticker.C

		neighbor, present := r.peers.PickRandomNeighbor()
		if present {
			r.sendRoutingMessage(neighbor)
		}
	}
}

func (r *RoutingHandler) KnownDestinations() []string {
	// TODO Locking
	origins := make([]string, 0)
	for origin, _ := range r.routingTable {
		origins = append(origins, origin)
	}

	return origins
}

func (r *RoutingHandler) UpdateRoutingTable(origin string, peerAddr PeerAddress) {
	newPeerAddr := peerAddr.String()
	// TODO Locks !
	prevPeerAddr, present := r.routingTable[origin]
	if !present || newPeerAddr != prevPeerAddr {
		r.routingTable[origin] = newPeerAddr
		fmt.Println("DSDV", origin, newPeerAddr)
	}
}

func (r *RoutingHandler) sendRoutingMessage(peers ...PeerAddress) {
	routingMessage := r.rumors.CreateClientRumor("")

	r.rumors.HandleRumorMessage(routingMessage, nil)

	for _, p := range peers {
		r.net.SendGossipPacket(routingMessage, p)
	}
}

func (r *RoutingHandler) SendPacketTowards(gp ToGossipPacket, dest string) bool {
	nextHop, valid := r.findRouteTo(dest)
	if !valid {
		return false
	}

	r.net.SendGossipPacket(gp, nextHop)

	return true
}

func (r *RoutingHandler) findRouteTo(dest string) (PeerAddress, bool) {
	// TODO Locking
	neighborAddr, valid := r.routingTable[dest]

	// For the moment, we can only send message to destination we know, ie that
	// are in the routing table. So logically, we know the route. If we don't,
	// someone tried to send a message to a destination that does not exist or
	// that we don't know yet.
	// TODO Fallback to sending to a random neighbor ?
	if !valid {
		fmt.Println("Destination", dest, "not found in the routing table.")
	}

	return StringAddress{neighborAddr}, valid
}
