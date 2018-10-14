package messages

import "fmt"

///// Client messages
type Message struct {
	Text string
}

///// Internode messages
/// Status messages
type PeerStatus struct {
	Identifier string
	NextID     uint32
}

type StatusPacket struct {
	Want []PeerStatus
}

/// Normal messages
type SimpleMessage struct {
	OriginalName  string
	RelayPeerAddr string
	Contents      string
}

/// Rumor messages
type RumorMessage struct {
	Origin string
	ID     uint32
	Text   string
}

//// Actual packet sent
type GossipPacket struct {
	Simple *SimpleMessage
	Rumor  *RumorMessage
	Status *StatusPacket
}

/// Print functions
func (m *Message) String() string {
	return fmt.Sprintf("CLIENT MESSAGE %s", m.Text)
}

func (simple *SimpleMessage) String() string {
	return fmt.Sprintf("SIMPLE MESSAGE origin %s from %s contents %s", simple.OriginalName, simple.RelayPeerAddr, simple.Contents)
}

func (rumor *RumorMessage) String() string {
	// TODO Rumor relay_addr !
	return fmt.Sprintf("RUMOR origin %s from %s ID %d contents %s", rumor.Origin, rumor.Origin, rumor.ID, rumor.Text)
}

// TODO Could add the interface "ToUDPAddr" to convert string to udpaddr

type ToGossipPacket interface {
	ToGossipPacket() *GossipPacket
}

func (gp *GossipPacket) ToGossipPacket() *GossipPacket {
	return gp
}

func (s *SimpleMessage) ToGossipPacket() *GossipPacket {
	return &GossipPacket{Simple: s}
}

func (r *RumorMessage) ToGossipPacket() *GossipPacket {
	return &GossipPacket{Rumor: r}
}

func (sp *StatusPacket) ToGossipPacket() *GossipPacket {
	return &GossipPacket{Status: sp}
}
