package messages

///// Get Request
type ResourceType int

const (
	NodeQuery ResourceType = iota
	MessageQuery
	PeerIDQuery
)

type GetRequest struct {
	Type ResourceType
}

///// Post Request
type Message struct {
	Text string
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
	Type   ResourceType
	Rumors []RumorMessage
	PeerID string
	Nodes  []string
}
