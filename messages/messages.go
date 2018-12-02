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

/// File transfer
type DataRequest struct {
	Origin      string
	Destination string
	HopLimit    uint32
	HashValue   []byte
}

type DataReply struct {
	Origin      string
	Destination string
	HopLimit    uint32
	HashValue   []byte
	Data        []byte
}

/// File search
type SearchRequest struct {
	Origin   string
	Budget   uint64
	Keywords []string
}

type SearchResult struct {
	FileName     string
	MetafileHash []byte
	ChunkMap     []uint64
	ChunkCount   uint64
}

type SearchReply struct {
	Origin      string
	Destination string
	HopLimit    uint32
	Results     []*SearchResult
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
	Simple        *SimpleMessage
	Rumor         *RumorMessage
	Status        *StatusPacket
	Private       *PrivateMessage
	DataRequest   *DataRequest
	DataReply     *DataReply
	SearchRequest *SearchRequest
	SearchReply   *SearchReply
}

/// Print functions
func (simple *SimpleMessage) String() string {
	return fmt.Sprintf("SIMPLE MESSAGE origin %s from %s contents %s", simple.OriginalName, simple.RelayPeerAddr, simple.Contents)
}

func (rumor *RumorMessage) StringWithSender(sender fmt.Stringer) string {
	return fmt.Sprintf("RUMOR origin %s from %s ID %d contents %s", rumor.Origin, sender, rumor.ID, rumor.Text)
}

func (p *PrivateMessage) String() string {
	return fmt.Sprintf("PRIVATE origin %s hop-limit %d contents %s", p.Origin, p.HopLimit, p.Text)
}

func (status *StatusPacket) StringWithSender(sender fmt.Stringer) string {
	wantStr := make([]string, 0)
	for _, peerStatus := range status.Want {
		wantStr = append(wantStr, peerStatus.String())
	}
	return fmt.Sprintf("STATUS from %s %s", sender, strings.Trim(strings.Join(wantStr, " "), " "))
}

func (p *PeerStatus) String() string {
	return fmt.Sprintf("peer %s nextID %d", p.Identifier, p.NextID)
}

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

func (dataReq *DataRequest) ToGossipPacket() *GossipPacket {
	return &GossipPacket{DataRequest: dataReq}
}

func (dataRep *DataReply) ToGossipPacket() *GossipPacket {
	return &GossipPacket{DataReply: dataRep}
}
