package messages

import (
	"fmt"

	. "github.com/RomainGehrig/Peerster/constants"
)

///// Get Request
type ResourceType int

const (
	NodeQuery ResourceType = iota
	MessageQuery
	PeerIDQuery
	DestinationsQuery
	PrivateMessageQuery
	SharedFilesQuery
	FileSearchResultQuery
	ReputationQuery
	TimedOutQuery
)

type GetRequest struct {
	Type ResourceType
}

///// Post Request

// If Dest is left empty, the message is considered a Rumor
type Message struct {
	Text string
	Dest string
}

type Node struct {
	Addr string
}

// Files
type FileInfo struct {
	Filename          string      `json:"filename"`
	Hash              SHA256_HASH `json:"hash"`
	Size              int64       `json:"size"` // In bytes
	IsOwner           bool        `json:"isOwner"`
	ReplicaNames      []string    `json:"replicas"`
	ReplicationFactor int         `json:"replicationFactor"`
}

type FileIndex struct {
	Filename string
}

type FileRedundancyFactor struct {
	Hash   SHA256_HASH `file:"hash"`
	Factor int         `file:"factor"`
}

// If Destination is "", the file will be downloaded from all
// known peers that have chunks of the file
type FileDownload struct {
	Destination string `json:"destination"`
	FileInfo
}

type FileSearch struct {
	Budget   uint64
	Keywords []string
}

type FileSearchResult struct {
	Keywords []string    `json:"keywords"`
	Files    []*FileInfo `json:"files"`
}

type PostRequest struct {
	Node                 *Node
	Message              *Message
	FileIndex            *FileIndex
	FileDownload         *FileDownload
	FileSearch           *FileSearch
	FileRedundancyFactor *FileRedundancyFactor
}

type Request struct {
	Get  *GetRequest
	Post *PostRequest
}

type Response struct {
	Type             ResourceType
	Rumors           []RumorMessage
	PrivateMessages  []PrivateMessage
	PeerID           string
	Nodes            []string
	Destinations     []string
	Files            []FileInfo
	FileSearchResult *FileSearchResult
	Reputations      map[string]int64
}

func (m *Message) String() string {
	return fmt.Sprintf("CLIENT MESSAGE %s", m.Text)
}

func (r ResourceType) String() string {
	return [...]string{"NodeQuery", "MessageQuery", "PeerIDQuery", "OriginsQuery", "PrivateMessageQuery", "SharedFilesQuery", "SearchResultQuery"}[r]
}

func (g *GetRequest) String() string {
	return fmt.Sprintf("GetRequest{Type: %s}", g.Type)
}
func (p *PostRequest) String() string {
	return fmt.Sprintf("PostRequest{Node: %s, Message: %s}", p.Node, p.Message)
}

func (r *Request) String() string {
	return fmt.Sprintf("Request{Get: %s, Post: %s}", r.Get, r.Post)
}
