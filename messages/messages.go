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

func (p *GossipPacket) String() string {
	return fmt.Sprintf("SIMPLE MESSAGE origin %s from %s contents %s", p.Simple.OriginalName, p.Simple.RelayPeerAddr, p.Simple.Contents)
}
