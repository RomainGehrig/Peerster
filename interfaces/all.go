package interfaces

import (
	. "github.com/RomainGehrig/Peerster/messages"
	. "github.com/RomainGehrig/Peerster/peers"
)

// Iterface are needed when there are bidirectionnal interactions between two modules

type MessageDispatcher interface {
	DispatchClientRequest(req *Request, sender PeerAddress)
	DispatchPacket(packet *GossipPacket, sender PeerAddress)
}

type UpdatableRouter interface {
	UpdateRoutingTable(origin string, peerAddr PeerAddress)
}
