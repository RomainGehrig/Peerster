package messages

///// Get Request
type ResourceType int

const (
	NodeQuery ResourceType = iota
	MessageQuery
	PeerIDQuery
	OriginsQuery
)

type GetRequest struct {
	Type ResourceType
}

///// Post Request
type Message struct {
	Text string
	Dest string
}

type Node struct {
	Addr string
}

type PostRequest struct {
	Node    *Node
	Message *Message
}

type Request struct {
	Get  *GetRequest
	Post *PostRequest
}

type Response struct {
	Type    ResourceType
	Rumors  []RumorMessage
	PeerID  string
	Nodes   []string
	Origins []string
}
