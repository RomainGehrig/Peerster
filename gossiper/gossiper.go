package gossiper

import (
	"fmt"
	. "github.com/RomainGehrig/Peerster/messages"
	. "github.com/RomainGehrig/Peerster/network"
	. "github.com/RomainGehrig/Peerster/peers"
	. "github.com/RomainGehrig/Peerster/routing"
	. "github.com/RomainGehrig/Peerster/rumors"
	. "github.com/RomainGehrig/Peerster/simple"
	"time"
)

// TODO Is is possible to remove this ?
const BUFFERSIZE int = 1024
const ANTIENTROPY_TIME = 1 * time.Second
const DEFAULT_HOP_LIMIT = 10

type Gossiper struct {
	Name        string
	simpleMode  bool
	peers       *PeersHandler
	routing     *RoutingHandler
	rumors      *RumorHandler
	net         *NetworkHandler
	simple      *SimpleHandler
	privateMsgs []PrivateMessage // TODO better datastructure ?
}

func NewGossiper(uiPort string, gossipAddr string, name string, peers []string, rtimer int, simple bool) *Gossiper {
	// fmt.Printf("Given arguments were: %s, %s, %s, %s, %s\n", uiPort, gossipAddr, name, peers[0], simple)
	return &Gossiper{
		simpleMode:  simple,
		Name:        name,
		peers:       NewPeersHandler(peers),
		routing:     NewRoutingHandler(time.Duration(rtimer) * time.Second),
		net:         NewNetworkHandler(uiPort, gossipAddr),
		simple:      NewSimpleHandler(),
		rumors:      NewRumorHandler(name),
		privateMsgs: make([]PrivateMessage, 0),
	}
}

// TODO BEGIN DELETE
func (g *Gossiper) GetName() string {
	return g.Name
}
func (g *Gossiper) GetAddress() string {
	return g.net.GetAddress()
}
func (g *Gossiper) WantList() []PeerStatus {
	return g.rumors.WantList()
}

// TODO END DELETE

func (g *Gossiper) Run() {
	// Small improvement: directly set ourself as the best route to get to ourself
	g.routing.UpdateRoutingTable(g.GetName(), StringAddress{g.GetAddress()})

	go g.AntiEntropy()
	go g.net.RunNetworkHandler(g)
	go g.simple.RunSimpleHandler(g, g.net, g.peers)
	go g.rumors.RunRumorHandler(g.net, g.peers, g.routing)
	go g.routing.RunRoutingHandler(g.peers, g.net, g.rumors)

	for {
		// Eternal wait
	}
}

func (g *Gossiper) AntiEntropy() {
	ticker := time.NewTicker(ANTIENTROPY_TIME)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			neighbor, present := g.peers.PickRandomNeighbor()
			if present {
				// TODO Move to rumors
				g.net.SendGossipPacket(g.rumors.CreateStatusMessage(), neighbor)
			}
		}
	}
}

func (g *Gossiper) DispatchClientRequest(req *Request, sender PeerAddress) {
	switch {
	case req.Get != nil:
		var resp Response
		resp.Type = req.Get.Type

		switch req.Get.Type {
		case NodeQuery:
			nodes := g.peers.AllPeersStr()
			resp.Nodes = nodes
		case MessageQuery:
			rumors := g.rumors.NonEmptyRumors()
			resp.Rumors = rumors
		case PeerIDQuery:
			resp.PeerID = g.GetName()
		case OriginsQuery:
			resp.Origins = g.routing.KnownOrigins()
		case PrivateMessageQuery:
			resp.PrivateMessages = g.privateMsgs
		}
		g.net.SendClientResponse(&resp, sender)
	case req.Post != nil:
		post := req.Post
		switch {
		case post.Node != nil:
			g.peers.AddPeer(ResolvePeerAddress(post.Node.Addr))
		case post.Message != nil:
			g.HandleClientMessage(post.Message)
			g.peers.PrintPeers()
		}
	}
}

func (g *Gossiper) DispatchPacket(packet *GossipPacket, sender PeerAddress) {
	g.peers.AddPeer(sender)
	switch {
	case packet.Simple != nil:
		fmt.Println(packet.Simple)
		g.simple.HandleSimpleMessage(packet.Simple)
	case packet.Rumor != nil:
		fmt.Println(packet.Rumor.StringWithSender(sender))
		g.rumors.HandleRumorMessage(packet.Rumor, sender)
	case packet.Status != nil:
		fmt.Println(packet.Status.StringWithSender(sender))
		g.rumors.HandleStatusMessage(packet.Status, sender)
	case packet.Private != nil:
		if packet.Private.Destination == g.Name {
			fmt.Println(packet.Private)
		}
		g.HandlePrivateMessage(packet.Private)
	}
	g.peers.PrintPeers()
}

func (g *Gossiper) HandleClientMessage(m *Message) {
	// TODO Print in case of PrivateMessage ?
	fmt.Println(m)
	if g.simpleMode {
		msg := g.simple.CreateSimpleMessage(m.Text)
		g.simple.BroadcastMessage(msg, nil)
	} else {
		// No destination specified => message is a rumor
		if m.Dest == "" {
			msg := g.rumors.CreateClientRumor(m.Text)
			g.rumors.HandleRumorMessage(msg, nil)
		} else { // Message is a private message
			msg := g.createPrivateMessage(m)
			g.HandlePrivateMessage(msg)
		}
	}
}

func (g *Gossiper) HandlePrivateMessage(p *PrivateMessage) {
	if p.Destination == g.Name {
		g.receivePrivateMessage(p)
	} else {
		// Save messages that are sent by us
		if p.Origin == g.Name {
			g.receivePrivateMessage(p)
		}

		newPM, shouldSend := g.preparePrivateMessage(p)
		if !shouldSend {
			return
		}
		nextHop, valid := g.routing.FindRouteTo(p.Destination)
		if valid {
			g.net.SendGossipPacket(newPM, nextHop)
		}
	}
}

func (g *Gossiper) receivePrivateMessage(p *PrivateMessage) {
	// TODO Locks ?
	// TODO Better datastructure ?
	g.privateMsgs = append(g.privateMsgs, *p)
}

/* Modifies in place the PrivateMessage given as argument */
func (g *Gossiper) preparePrivateMessage(p *PrivateMessage) (newPM *PrivateMessage, valid bool) {
	newPM = p
	// Won't forward
	if p.HopLimit <= 1 {
		p.HopLimit = 0
		valid = false
		return
	}

	// Will forward
	p.HopLimit -= 1
	valid = true
	return
}

func (g *Gossiper) createPrivateMessage(m *Message) *PrivateMessage {
	return &PrivateMessage{
		Origin:      g.Name,
		ID:          0, // TODO maybe do some kind of ordering
		Text:        m.Text,
		Destination: m.Dest,
		HopLimit:    DEFAULT_HOP_LIMIT + 1, // Add 1 because we are going to decrement it when sending
	}
}
