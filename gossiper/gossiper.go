package gossiper

import (
	"fmt"
	. "github.com/RomainGehrig/Peerster/messages"
	. "github.com/RomainGehrig/Peerster/utils"
	"github.com/dedis/protobuf"
	"net"
	"strings"
)

// TODO Is is possible to remove this ?
const BUFFERSIZE int = 1024

type Gossiper struct {
	address    *net.UDPAddr
	conn       *net.UDPConn
	uiAddress  *net.UDPAddr
	uiConn     *net.UDPConn
	Name       string
	knownPeers *StringSet
}

func NewGossiper(uiPort string, gossipAddr string, name string, peers []string, simple bool) *Gossiper {
	// fmt.Printf("Given arguments where: %s, %s, %s, %s, %s\n", uiPort, gossipAddr, name, peers[0], simple)
	udpAddr, err := net.ResolveUDPAddr("udp4", gossipAddr)
	if err != nil {
		fmt.Println("Error when creating udpAddr", err)
	}
	udpConn, err := net.ListenUDP("udp4", udpAddr)
	if err != nil {
		// TODO Handle err
		fmt.Println("Error when creating udpConn", err)
	}

	uiAddr := fmt.Sprintf("127.0.0.1:%s", uiPort)
	udpUIAddr, err := net.ResolveUDPAddr("udp4", uiAddr)
	if err != nil {
		fmt.Println("Error when creating udpUIAddr", err)
	}
	udpUIConn, err := net.ListenUDP("udp4", udpUIAddr)
	if err != nil {
		// TODO Handle err
		fmt.Println("Error when creating udpUIConn", err)
	}

	return &Gossiper{
		address:    udpAddr,
		conn:       udpConn,
		uiAddress:  udpUIAddr,
		uiConn:     udpUIConn,
		Name:       name,
		knownPeers: StringSetInit(peers),
	}
}

func (g *Gossiper) Run() {
	peerChan := g.PeersMessages()
	clientChan := g.ClientMessages()

	g.ListenForMessages(peerChan, clientChan)
}

func (g *Gossiper) ListenForMessages(peerMsgs <-chan *GossipPacket, clientMsgs <-chan *Message) {
	for {
		select {
		case gp := <-peerMsgs:
			g.DispatchPacket(gp)
		case msg := <-clientMsgs:
			g.HandleClientMessage(msg)
			fmt.Println(msg)
		}
		g.PrintPeers()
	}
}

func (g *Gossiper) ClientMessages() <-chan *Message {
	var message Message
	out := make(chan *Message)
	packets := MessageReceiver(g.uiConn)
	go func() {
		for {
			packetBytes := <-packets
			protobuf.Decode(packetBytes, &message)
			out <- &message
		}
	}()
	return out
}

func (g *Gossiper) PeersMessages() <-chan *GossipPacket {
	var packet GossipPacket
	out := make(chan *GossipPacket)
	packets := MessageReceiver(g.conn)
	go func() {
		for {
			packetBytes := <-packets
			protobuf.Decode(packetBytes, &packet)
			out <- &packet
		}
	}()
	return out
}

func MessageReceiver(conn *net.UDPConn) <-chan []byte {
	packetBytes := make([]byte, BUFFERSIZE)
	out := make(chan []byte)
	go func() {
		for {
			_, sender, _ := conn.ReadFromUDP(packetBytes)
			out <- packetBytes
		}
	}()
	return out
}

func (g *Gossiper) DispatchPacket(packet *GossipPacket) {
	switch {
	case packet.Simple != nil:
		// TODO Is is always a node message ?
		g.HandleNodeMessage(packet.Simple)
		fmt.Println(packet.Simple)
	case packet.Rumor != nil:
		g.HandleRumorMessage(packet.Rumor)
	case packet.Status != nil:
		// TODO
	}
}

func (g *Gossiper) HandleClientMessage(m *Message) {
	msg := g.createClientMessage(m)
	g.BroadcastMessage(msg, nil)
}

func (g *Gossiper) HandleRumorMessage(rumor *RumorMessage) {
	// TODO
}

func (g *Gossiper) HandleNodeMessage(simple *SimpleMessage) {
	msg := g.createForwardedMessage(simple)
	// Add msg peer to peers
	g.AddPeer(simple.RelayPeerAddr)
	// Broadcast to everyone but sender
	g.BroadcastMessage(msg, StringSetInitSingleton(simple.RelayPeerAddr))
}

func (g *Gossiper) AddPeer(peer string) {
	g.knownPeers.Add(peer)
}

// A forwarded message is a message where we put our address
// as the sender of the message
func (g *Gossiper) createForwardedMessage(m *SimpleMessage) *SimpleMessage {
	return &SimpleMessage{
		OriginalName:  m.OriginalName,
		RelayPeerAddr: g.address.String(),
		Contents:      m.Contents}
}

func (g *Gossiper) createClientMessage(m *Message) *SimpleMessage {
	return &SimpleMessage{
		OriginalName:  g.Name,
		RelayPeerAddr: g.address.String(),
		Contents:      m.Text}
}

func (g *Gossiper) SendGossipPacket(gp *GossipPacket, peerAddr string) {
	packetBytes, err := protobuf.Encode(gp)
	if err != nil {
		fmt.Println(err)
	}

	peerUDPAddr, err := net.ResolveUDPAddr("udp4", peerAddr)
	_, err = g.conn.WriteToUDP(packetBytes, peerUDPAddr)
	if err != nil {
		fmt.Println(err)
	}
}

func (g *Gossiper) SendMessage(m *SimpleMessage, peerAddr string) {
	toSend := &GossipPacket{Simple: m}
	g.SendGossipPacket(toSend, peerAddr)
}

// TODO Thread safety of knownPeers
func (g *Gossiper) BroadcastMessage(m *SimpleMessage, excludedPeers *StringSet) {
	for peer := range g.knownPeers.Iterate() {
		if excludedPeers == nil || !excludedPeers.Has(peer) {
			g.SendMessage(m, peer)
		}
	}
}

func (g *Gossiper) PrintPeers() {
	fmt.Printf("PEERS %s\n", strings.Join(g.knownPeers.ToSlice(), ","))
}
