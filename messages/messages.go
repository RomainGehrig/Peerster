package messages

import "fmt"
import "strings"

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

func (rumor *RumorMessage) StringWithSender(sender string) string {
	return fmt.Sprintf("RUMOR origin %s from %s ID %d contents %s", rumor.Origin, sender, rumor.ID, rumor.Text)
}

func (status *StatusPacket) StringWithSender(sender string) string {
	wantStr := make([]string, 0)
	for _, peerStatus := range status.Want {
		wantStr = append(wantStr, peerStatus.String())
	}
	return fmt.Sprintf("STATUS from %s %s", sender, strings.Trim(strings.Join(wantStr, " "), " "))
}

func (p *PeerStatus) String() string {
	return fmt.Sprintf("peer %s nextID %d", p.Identifier, p.NextID)
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
