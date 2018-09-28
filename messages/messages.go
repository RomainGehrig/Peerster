package messages

import "fmt"

type Message struct {
	Text string
}

type SimpleMessage struct {
	OriginalName  string
	RelayPeerAddr string
	Contents      string
}

type GossipPacket struct {
	Simple *SimpleMessage
}

func (m *Message) String() string {
	return fmt.Sprintf("CLIENT MESSAGE %s", m.Text)
}

func (p *GossipPacket) String() string {
	return fmt.Sprintf("SIMPLE MESSAGE origin %s from %s contents %s", p.Simple.OriginalName, p.Simple.RelayPeerAddr, p.Simple.Contents)
}
