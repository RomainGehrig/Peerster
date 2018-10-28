package messages

import "fmt"
import "strings"

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
	Origin string `json:"origin"`
	ID     uint32 `json:"id"`
	Text   string `json:"text"`
}

/// Private messages
type PrivateMessage struct {
	Origin      string `json:"origin"`
	ID          uint32 `json:"id"`
	Text        string `json:"text"`
	Destination string `json:"destination"`
	HopLimit    uint32
}

//// Actual packet sent
type GossipPacket struct {
	Simple  *SimpleMessage
	Rumor   *RumorMessage
	Status  *StatusPacket
	Private *PrivateMessage
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

func (p *PrivateMessage) String() string {
	return fmt.Sprintf("PRIVATE origin %s hop-limit %d contents %s", p.Origin, p.HopLimit, p.Text)
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

func (p *PrivateMessage) ToGossipPacket() *GossipPacket {
	return &GossipPacket{Private: p}
}
