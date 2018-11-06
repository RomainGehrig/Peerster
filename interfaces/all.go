package interfaces

import (
	. "github.com/RomainGehrig/Peerster/messages"
	. "github.com/RomainGehrig/Peerster/peers"
)

type GossiperLike interface {
	DispatchClientRequest(req *Request, sender PeerAddress)
	DispatchPacket(packet *GossipPacket, sender PeerAddress)
	WantList() []PeerStatus
	// TODO Delete
	GetName() string
	GetAddress() string
}

type UpdatableRouter interface {
	UpdateRoutingTable(origin string, peerAddr PeerAddress)
}
