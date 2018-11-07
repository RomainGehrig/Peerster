package gossiper

import (
	"fmt"
	. "github.com/RomainGehrig/Peerster/messages"
	. "github.com/RomainGehrig/Peerster/network"
	. "github.com/RomainGehrig/Peerster/peers"
	. "github.com/RomainGehrig/Peerster/private"
	. "github.com/RomainGehrig/Peerster/routing"
	. "github.com/RomainGehrig/Peerster/rumors"
	. "github.com/RomainGehrig/Peerster/simple"
	"time"
)

type Gossiper struct {
	Name       string
	simpleMode bool
	peers      *PeersHandler
	routing    *RoutingHandler
	rumors     *RumorHandler
	net        *NetworkHandler
	simple     *SimpleHandler
	private    *PrivateHandler
}

func NewGossiper(uiPort string, gossipAddr string, name string, peers []string, rtimer int, simple bool) *Gossiper {
	return &Gossiper{
		simpleMode: simple,
		Name:       name,
		peers:      NewPeersHandler(peers),
		routing:    NewRoutingHandler(time.Duration(rtimer) * time.Second),
		net:        NewNetworkHandler(uiPort, gossipAddr),
		simple:     NewSimpleHandler(name, gossipAddr),
		rumors:     NewRumorHandler(name),
		private:    NewPrivateHandler(name),
	}
}

// TODO DELETE
func (g *Gossiper) WantList() []PeerStatus {
	return g.rumors.WantList()
}

func (g *Gossiper) Run() {
	// Small improvement: directly set ourself as the best route to get to ourself
	g.routing.UpdateRoutingTable(g.Name, StringAddress{g.net.GetAddress()})

	go g.net.RunNetworkHandler(g)
	go g.simple.RunSimpleHandler(g.net, g.peers)
	go g.rumors.RunRumorHandler(g.net, g.peers, g.routing)
	go g.routing.RunRoutingHandler(g.peers, g.net, g.rumors)
	go g.private.RunPrivateHandler(g.routing, g.net)

	for {
		// Eternal wait
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
			resp.PeerID = g.Name
		case OriginsQuery:
			resp.Origins = g.routing.KnownOrigins()
		case PrivateMessageQuery:
			resp.PrivateMessages = g.private.GetPrivateMessages()
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
		g.private.HandlePrivateMessage(packet.Private)
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
			// TODO Put creation/handling directly inside of the private module
			msg := g.private.CreatePrivateMessage(m.Text, m.Dest)
			g.private.HandlePrivateMessage(msg)
		}
	}
}
