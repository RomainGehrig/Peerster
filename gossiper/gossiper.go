package gossiper

import (
	"fmt"
	. "github.com/RomainGehrig/Peerster/messages"
	. "github.com/RomainGehrig/Peerster/utils"
	"github.com/dedis/protobuf"
	"net"
	"strings"
)

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
	fmt.Printf("Given arguments where: %s, %s, %s, %s, %s\n", uiPort, gossipAddr, name, peers[0], simple)
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
	fmt.Println(udpUIAddr, uiAddr)
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

func (g *Gossiper) ListenForClientMessages() {
	msgBytes := make([]byte, BUFFERSIZE)
	var msg Message
	for {
		g.uiConn.ReadFromUDP(msgBytes)
		protobuf.Decode(msgBytes, &msg)
		g.HandleClientMessage(&msg)
	}
}

func (g *Gossiper) ListenForNodeMessages() {
	packetBytes := make([]byte, BUFFERSIZE)
	var packet GossipPacket
	for {
		g.conn.ReadFromUDP(packetBytes)
		protobuf.Decode(packetBytes, &packet)
		g.HandleNodeMessage(&packet)
	}
}

func (g *Gossiper) HandleClientMessage(m *Message) {
	msg := g.createClientMessage(m)
	g.BroadcastMessage(msg, nil)

	fmt.Println(m)
	g.PrintPeers()
}

func (g *Gossiper) HandleNodeMessage(p *GossipPacket) {
	msg := g.createForwardedMessage(p.Simple)
	// Add msg peer to peers
	g.AddPeer(p.Simple.RelayPeerAddr)
	// Broadcast to everyone but sender
	g.BroadcastMessage(msg, StringSetInitSingleton(p.Simple.RelayPeerAddr))

	fmt.Println(p)
	g.PrintPeers()
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

func (g *Gossiper) SendMessage(m *SimpleMessage, peerAddr string) {
	toSend := &GossipPacket{Simple: m}
	packetBytes, err := protobuf.Encode(toSend)
	if err != nil {
		fmt.Println(err)
	}
	// TODO Handle err

	conn, err := net.Dial("udp4", peerAddr)

	conn.Write(packetBytes)
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