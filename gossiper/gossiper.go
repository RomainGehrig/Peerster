package gossiper

import (
	"fmt"
	. "github.com/RomainGehrig/Peerster/messages"
	. "github.com/RomainGehrig/Peerster/utils"
	"github.com/dedis/protobuf"
	"math/rand"
	"net"
	"strings"
)

// TODO Is is possible to remove this ?
const BUFFERSIZE int = 1024

type Gossiper struct {
	address      *net.UDPAddr
	conn         *net.UDPConn
	uiAddress    *net.UDPAddr
	uiConn       *net.UDPConn
	Name         string
	knownPeers   *StringSet
	peerStatuses map[string]PeerStatus
	rumorMsgs    map[PeerStatus]RumorMessage
}

type ReceivedMessage struct {
	packetBytes []byte
	sender      *net.UDPAddr
}

type WrappedGossipPacket struct {
	sender    *net.UDPAddr
	gossipMsg *GossipPacket
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
		address:      udpAddr,
		conn:         udpConn,
		uiAddress:    udpUIAddr,
		uiConn:       udpUIConn,
		Name:         name,
		knownPeers:   StringSetInit(peers),
		peerStatuses: make(map[string]PeerStatus),
		rumorMsgs:    make(map[PeerStatus]RumorMessage),
	}
}

func (g *Gossiper) Run() {
	peerChan := g.PeersMessages()
	clientChan := g.ClientMessages()

	g.ListenForMessages(peerChan, clientChan)
}

func (g *Gossiper) ListenForMessages(peerMsgs <-chan *WrappedGossipPacket, clientMsgs <-chan *Message) {
	for {
		select {
		case wgp := <-peerMsgs:
			g.DispatchPacket(wgp)
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
	receivedMsgs := MessageReceiver(g.uiConn)
	go func() {
		for {
			receivedMsg := <-receivedMsgs
			protobuf.Decode(receivedMsg.packetBytes, &message)
			out <- &message
		}
	}()
	return out
}

func (g *Gossiper) PeersMessages() <-chan *WrappedGossipPacket {
	var packet GossipPacket
	out := make(chan *WrappedGossipPacket)
	receivedMsgs := MessageReceiver(g.conn)
	go func() {
		for {
			receivedMsg := <-receivedMsgs
			protobuf.Decode(receivedMsg.packetBytes, &packet)
			out <- &WrappedGossipPacket{receivedMsg.sender, &packet}
		}
	}()
	return out
}

func MessageReceiver(conn *net.UDPConn) <-chan *ReceivedMessage {
	packetBytes := make([]byte, BUFFERSIZE)
	out := make(chan *ReceivedMessage)
	go func() {
		for {
			_, sender, _ := conn.ReadFromUDP(packetBytes)
			// TODO Is it safe to only pass a reference to the byte array?
			out <- &ReceivedMessage{packetBytes, sender}
		}
	}()
	return out
}

func (g *Gossiper) DispatchPacket(wpacket *WrappedGossipPacket) {
	packet := wpacket.gossipMsg
	sender := wpacket.sender
	switch {
	case packet.Simple != nil:
		// TODO Is is always a node message ?
		g.HandleNodeMessage(packet.Simple)
		fmt.Println(packet.Simple)
	case packet.Rumor != nil:
		g.HandleRumorMessage(packet.Rumor, sender)
	case packet.Status != nil:
		// TODO
	}
}

func (g *Gossiper) HandleClientMessage(m *Message) {
	msg := g.createClientMessage(m)
	g.BroadcastMessage(msg, nil)
}

func (g *Gossiper) HandleRumorMessage(rumor *RumorMessage, sender *net.UDPAddr) {
	// Received a rumor message from sender
	diff := g.SelfDiffRumorID(rumor)
	// Send back the status ACK for the message
	// Want list is different in only one case: we accept the new rumor message
	defer func() {
		g.SendGossipPacket(g.createStatusMessage(), sender)
	}()
	switch {
	case diff == 0: // TODO => we are in sync, ie. the rumor is not new
	case diff > 0: // TODO => we are in advance, peer should send us a statusmessage afterwards in order to sync
	case diff < 0: // TODO => we are late, add message to list iff diff == -1
		if diff == -1 {
			g.acceptRumorMessage(rumor)
			// New message, so we monger
			// TODO Is sender.String() the right way to compare with other?
			randNeighbor := g.pickRandomNeighbor(sender.String())
			g.SendGossipPacketStr(rumor, randNeighbor)
		}
	}
}

func (g *Gossiper) WantList() []PeerStatus {
	peerStatuses := make([]PeerStatus, len(g.peerStatuses))
	i := 0
	for _, peerStatus := range g.peerStatuses {
		peerStatuses[i] = peerStatus
		i += 1
	}
	return peerStatuses
}

// A return value =0 means we are sync, >0 we are in advance, <0 we are late
func (g *Gossiper) SelfDiffRumorID(rumor *RumorMessage) int {
	// TODO put initialization elsewhere
	peerStatus, present := g.peerStatuses[rumor.Origin]
	if !present {
		peerStatus = PeerStatus{
			Identifier: rumor.Origin,
			NextID:     0,
		}
		g.peerStatuses[rumor.Origin] = peerStatus
	}

	return int(peerStatus.NextID) - int(rumor.ID)
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

func (g *Gossiper) acceptRumorMessage(r *RumorMessage) {
	oldPeerStatus, present := g.peerStatuses[r.Origin]
	if !present {
		// TODO assert that r.ID == currentPeerStatus.NextID
		fmt.Println("Old rumor was not present inside peerStatus", r.Origin, r.ID)
	}
	newPeerStatus := PeerStatus{Identifier: r.Origin, NextID: r.ID + 1}
	g.rumorMsgs[oldPeerStatus] = *r
	g.peerStatuses[r.Origin] = newPeerStatus
}

func (g *Gossiper) pickRandomNeighbor(excluded string) string {
	peers := g.knownPeers.ToSlice()
	peer := excluded
	// TODO validate that if excluded is nil it works
	for peer == excluded {
		fmt.Println("Peer is", peer, "excluded is", excluded)
		peer = peers[rand.Intn(len(peers))]
	}
	return peer
}

func (g *Gossiper) createStatusMessage() *StatusPacket {
	return &StatusPacket{Want: g.WantList()}
}

// A forwarded message is a message where we put our address
// as the sender of the message
func (g *Gossiper) createForwardedMessage(m *SimpleMessage) *SimpleMessage {
	return &SimpleMessage{
		OriginalName:  m.OriginalName,
		RelayPeerAddr: g.address.String(),
		Contents:      m.Contents}
}

// We are the original sender of this message
func (g *Gossiper) createClientMessage(m *Message) *SimpleMessage {
	return &SimpleMessage{
		OriginalName:  g.Name,
		RelayPeerAddr: g.address.String(),
		Contents:      m.Text}
}

func (g *Gossiper) SendGossipPacket(tgp ToGossipPacket, peerUDPAddr *net.UDPAddr) {
	gp := tgp.ToGossipPacket()

	packetBytes, err := protobuf.Encode(gp)
	if err != nil {
		fmt.Println(err)
	}
	_, err = g.conn.WriteToUDP(packetBytes, peerUDPAddr)
	if err != nil {
		fmt.Println(err)
	}
}

func (g *Gossiper) SendGossipPacketStr(tgp ToGossipPacket, peerAddr string) {
	peerUDPAddr, _ := net.ResolveUDPAddr("udp4", peerAddr)
	g.SendGossipPacket(tgp, peerUDPAddr)
}

// TODO Thread safety of knownPeers
func (g *Gossiper) BroadcastMessage(m *SimpleMessage, excludedPeers *StringSet) {
	for peer := range g.knownPeers.Iterate() {
		if excludedPeers == nil || !excludedPeers.Has(peer) {
			g.SendGossipPacketStr(m, peer)
		}
	}
}

func (g *Gossiper) PrintPeers() {
	fmt.Printf("PEERS %s\n", strings.Join(g.knownPeers.ToSlice(), ","))
}
