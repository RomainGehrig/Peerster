package simple

import (
	"github.com/RomainGehrig/Peerster/interfaces"
	. "github.com/RomainGehrig/Peerster/messages"
	. "github.com/RomainGehrig/Peerster/network"
	. "github.com/RomainGehrig/Peerster/peers"
	. "github.com/RomainGehrig/Peerster/utils"
)

type SimpleHandler struct {
	// TODO GOssiperLike is bad
	tmpGoss interfaces.GossiperLike
	net     *NetworkHandler
	peers   *PeersHandler
}

func NewSimpleHandler() *SimpleHandler {
	return &SimpleHandler{}
}

func (s *SimpleHandler) RunSimpleHandler(goss interfaces.GossiperLike, net *NetworkHandler, peers *PeersHandler) {
	s.tmpGoss = goss
	s.net = net
	s.peers = peers
}

// A forwarded message is a message where we put our address
// as the sender of the message
func (s *SimpleHandler) CreateForwardedMessage(msg *SimpleMessage) *SimpleMessage {
	return &SimpleMessage{
		OriginalName:  msg.OriginalName,
		RelayPeerAddr: s.tmpGoss.GetAddress(),
		Contents:      msg.Contents}
}

// We are the original sender of this message
func (s *SimpleHandler) CreateSimpleMessage(text string) *SimpleMessage {
	return &SimpleMessage{
		OriginalName:  s.tmpGoss.GetName(),
		RelayPeerAddr: s.tmpGoss.GetAddress(),
		Contents:      text}
}

// TODO Excluded as PeerAddress... ?
func (s *SimpleHandler) BroadcastMessage(m *SimpleMessage, excludedPeers *StringSet) {
	for _, peer := range s.peers.AllPeers() {
		if excludedPeers == nil || !excludedPeers.Has(peer.String()) {
			s.net.SendGossipPacket(m, peer)
		}
	}
}

func (s *SimpleHandler) HandleSimpleMessage(simple *SimpleMessage) {
	msg := s.CreateForwardedMessage(simple)
	// Add msg peer to peers
	s.peers.AddPeer(StringAddress{simple.RelayPeerAddr})
	// Broadcast to everyone but sender
	s.BroadcastMessage(msg, StringSetInitSingleton(simple.RelayPeerAddr))
}
