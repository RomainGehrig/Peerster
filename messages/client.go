package messages

import (
	"fmt"
	. "github.com/RomainGehrig/Peerster/constants"
	. "github.com/RomainGehrig/Peerster/files"
)

///// Get Request
type ResourceType int

const (
	NodeQuery ResourceType = iota
	MessageQuery
	PeerIDQuery
	OriginsQuery
	PrivateMessageQuery
	SharedFilesQuery
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
type FileIndex struct {
	Filename string
}

type FileDownload struct {
	Destination string
	File
}

type PostRequest struct {
	Node         *Node
	Message      *Message
	FileIndex    *FileIndex
	FileDownload *FileDownload
}

type Request struct {
	Get  *GetRequest
	Post *PostRequest
}

type Response struct {
	Type            ResourceType
	Rumors          []RumorMessage
	PrivateMessages []PrivateMessage
	PeerID          string
	Nodes           []string
	Origins         []string
	Files           []File
}

func (m *Message) String() string {
	return fmt.Sprintf("CLIENT MESSAGE %s", m.Text)
}

func (r ResourceType) String() string {
	return [...]string{"NodeQuery", "MessageQuery", "PeerIDQuery", "OriginsQuery", "PrivateMessageQuery"}[r]
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