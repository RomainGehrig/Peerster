package network

import (
	. "github.com/RomainGehrig/Peerster/messages"
	. "github.com/RomainGehrig/Peerster/peers"
)

type receivedMessage struct {
	packetBytes []byte
	sender      PeerAddress
}

type wrappedGossipPacket struct {
	gossipMsg *GossipPacket
	sender    PeerAddress
}

type wrappedClientRequest struct {
	request *Request
	sender  PeerAddress
}
