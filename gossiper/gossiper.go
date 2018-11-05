package gossiper

import (
	"fmt"
	. "github.com/RomainGehrig/Peerster/messages"
	. "github.com/RomainGehrig/Peerster/peers"
	. "github.com/RomainGehrig/Peerster/utils"
	"github.com/dedis/protobuf"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"
)

// TODO Is is possible to remove this ?
const BUFFERSIZE int = 1024
const STATUS_MESSAGE_TIMEOUT = 1 * time.Second
const ANTIENTROPY_TIME = 1 * time.Second
const DEFAULT_HOP_LIMIT = 10

type Gossiper struct {
	address      *net.UDPAddr
	conn         *net.UDPConn
	uiAddress    *net.UDPAddr
	uiConn       *net.UDPConn
	rtimer       time.Duration
	Name         string
	simpleMode   bool
	peers        *PeersHandler
	peerStatuses map[string]PeerStatus
	privateMsgs  []PrivateMessage  // TODO better datastructure ?
	routingTable map[string]string // From Origin to ip:port
	sendChannel  chan<- *WrappedGossipPacket
	rumorMsgs    map[PeerStatus]RumorMessage
	rumorLock    *sync.RWMutex
}

type ReceivedMessage struct {
	packetBytes []byte
	sender      PeerAddress
}

type WrappedGossipPacket struct {
	gossipMsg *GossipPacket
	sender    PeerAddress
}

type WrappedClientRequest struct {
	request *Request
	sender  PeerAddress
}

func NewGossiper(uiPort string, gossipAddr string, name string, peers []string, rtimer int, simple bool) *Gossiper {
	// fmt.Printf("Given arguments were: %s, %s, %s, %s, %s\n", uiPort, gossipAddr, name, peers[0], simple)
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
		rtimer:       time.Duration(rtimer) * time.Second,
		simpleMode:   simple,
		Name:         name,
		peers:        NewPeersHandler(peers),
		peerStatuses: make(map[string]PeerStatus),
		privateMsgs:  make([]PrivateMessage, 0),
		routingTable: make(map[string]string),
		rumorMsgs:    make(map[PeerStatus]RumorMessage),
		rumorLock:    &sync.RWMutex{},
	}
}

func (g *Gossiper) Run() {
	peerChan := g.PeersMessages()
	clientChan := g.ClientRequests()

	sendChan := make(chan *WrappedGossipPacket, BUFFERSIZE)
	g.sendChannel = sendChan

	// Small improvement: directly set ourself as the best route to get to ourself
	g.updateRoutingTable(g.Name, UDPAddress{g.address})

	go g.AntiEntropy()
	if g.rtimer != 0 {
		go g.RunRoutingMessages()
	}

	g.ListenForMessages(peerChan, clientChan, sendChan)
}

func (g *Gossiper) RunRoutingMessages() {
	ticker := time.NewTicker(g.rtimer)
	defer ticker.Stop()

	// Start by sending a routing message to all neighbors
	g.SendRoutingMessage(g.peers.AllPeers()...)

	for {
		// Wait for ticker
		<-ticker.C

		neighbor, present := g.peers.PickRandomNeighbor()
		if present {
			g.SendRoutingMessage(neighbor)
		}
	}
}

func (g *Gossiper) SendRoutingMessage(peers ...PeerAddress) {
	routingMessage := g.createClientRumor(&Message{Text: ""})

	for _, p := range peers {
		g.SendGossipPacket(routingMessage, p)
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
				g.SendGossipPacket(g.createStatusMessage(), neighbor)
			}
		}
	}
}

func (g *Gossiper) ListenForMessages(peerMsgs <-chan *WrappedGossipPacket, clientRequests <-chan *WrappedClientRequest, sendGossipPacket <-chan *WrappedGossipPacket) {
	for {
		select {
		case wgp := <-peerMsgs:
			g.DispatchPacket(wgp)
			g.PrintPeers()
		case cliReq := <-clientRequests:
			g.DispatchClientRequest(cliReq)
		case wgp := <-sendGossipPacket:
			gp := wgp.gossipMsg
			peerAddr := wgp.sender
			packetBytes, err := protobuf.Encode(gp)
			if err != nil {
				fmt.Println(err)
			}
			_, err = g.conn.WriteToUDP(packetBytes, peerAddr.ToUDPAddr())
			if err != nil {
				fmt.Println(err)
			}
		}
	}
}

func (g *Gossiper) SendClientResponse(resp *Response, peerAddr PeerAddress) {
	packetBytes, err := protobuf.Encode(resp)
	if err != nil {
		fmt.Println(err)
	}
	_, err = g.uiConn.WriteToUDP(packetBytes, peerAddr.ToUDPAddr())
	if err != nil {
		fmt.Println(err)
	}
}

func (g *Gossiper) Origins() []string {
	// TODO Locking
	origins := make([]string, 0)
	for origin, _ := range g.routingTable {
		origins = append(origins, origin)
	}

	return origins
}

func (g *Gossiper) DispatchClientRequest(wreq *WrappedClientRequest) {
	req := wreq.request
	sender := wreq.sender

	switch {
	case req.Get != nil:
		var resp Response
		resp.Type = req.Get.Type

		switch req.Get.Type {
		case NodeQuery:
			nodes := g.peers.AllPeersStr()
			resp.Nodes = nodes
		case MessageQuery:
			rumors := g.AllRumorsForClient()
			resp.Rumors = rumors
		case PeerIDQuery:
			resp.PeerID = g.Name
		case OriginsQuery:
			resp.Origins = g.Origins()
		case PrivateMessageQuery:
			resp.PrivateMessages = g.privateMsgs
		}
		g.SendClientResponse(&resp, sender)
	case req.Post != nil:
		post := req.Post
		switch {
		case post.Node != nil:
			g.peers.AddPeer(ResolvePeerAddress(post.Node.Addr))
		case post.Message != nil:
			g.HandleClientMessage(post.Message)
			g.PrintPeers()
		}
	}
}

func (g *Gossiper) AllRumorsForClient() []RumorMessage {
	out := make([]RumorMessage, 0)

	g.rumorLock.RLock()
	defer g.rumorLock.RUnlock()
	for _, msg := range g.rumorMsgs {
		if msg.Text != "" {
			out = append(out, msg)
		}
	}

	return out
}

func (g *Gossiper) ClientRequests() <-chan *WrappedClientRequest {
	out := make(chan *WrappedClientRequest)
	receivedMsgs := MessageReceiver(g.uiConn)
	go func() {
		for {
			var request Request
			receivedMsg := <-receivedMsgs
			protobuf.Decode(receivedMsg.packetBytes, &request)
			out <- &WrappedClientRequest{request: &request, sender: receivedMsg.sender}
		}
	}()
	return out
}

func (g *Gossiper) PeersMessages() <-chan *WrappedGossipPacket {
	out := make(chan *WrappedGossipPacket, BUFFERSIZE)
	receivedMsgs := MessageReceiver(g.conn)
	go func() {
		for {
			var packet GossipPacket
			receivedMsg := <-receivedMsgs
			protobuf.Decode(receivedMsg.packetBytes, &packet)
			out <- &WrappedGossipPacket{&packet, receivedMsg.sender}
		}
	}()
	return out
}

func MessageReceiver(conn *net.UDPConn) <-chan *ReceivedMessage {
	out := make(chan *ReceivedMessage, BUFFERSIZE)
	go func() {
		for {
			packetBytes := make([]byte, BUFFERSIZE)
			_, sender, _ := conn.ReadFromUDP(packetBytes)
			out <- &ReceivedMessage{packetBytes, UDPAddress{sender}}
		}
	}()
	return out
}

func (g *Gossiper) DispatchPacket(wpacket *WrappedGossipPacket) {
	packet := wpacket.gossipMsg
	sender := wpacket.sender
	g.peers.AddPeer(sender)
	switch {
	case packet.Simple != nil:
		fmt.Println(packet.Simple)
		g.HandleNodeMessage(packet.Simple)
	case packet.Rumor != nil:
		fmt.Println(packet.Rumor.StringWithSender(sender))
		g.HandleRumorMessage(packet.Rumor, sender)
	case packet.Status != nil:
		fmt.Println(packet.Status.StringWithSender(sender))
		g.HandleStatusMessage(packet.Status, sender)
	case packet.Private != nil:
		if packet.Private.Destination == g.Name {
			fmt.Println(packet.Private)
		}
		g.HandlePrivateMessage(packet.Private)
	}
}

func (g *Gossiper) HandleClientMessage(m *Message) {
	// TODO Print in case of PrivateMessage ?
	fmt.Println(m)
	if g.simpleMode {
		msg := g.createClientMessage(m)
		g.BroadcastMessage(msg, nil)
	} else {
		// No destination specified => message is a rumor
		if m.Dest == "" {
			msg := g.createClientRumor(m)
			g.HandleRumorMessage(msg, nil)
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
		nextHop, valid := g.FindRouteTo(p.Destination)
		if valid {
			g.SendGossipPacket(newPM, nextHop)
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

func (g *Gossiper) FindRouteTo(dest string) (PeerAddress, bool) {
	// TODO Locking
	neighborAddr, valid := g.routingTable[dest]

	// For the moment, we can only send message to destination we know, ie that
	// are in the routing table. So logically, we know the route. If we don't,
	// someone tried to send a message to a destination that does not exist or
	// that we don't know yet.
	// TODO Fallback to sending to a random neighbor ?
	if !valid {
		fmt.Println("Destination", dest, "not found in the routing table.")
	}

	return StringAddress{neighborAddr}, valid
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

func (g *Gossiper) HandleStatusMessage(status *StatusPacket, sender PeerAddress) {
	rumorsToSend, rumorsToAsk := g.ComputeRumorStatusDiff(status.Want)
	// newlyReceivedByPeer, newlyAvailableFromPeer := g.updatePeerViewedStatus(status.Want, sender.String())

	// The priority is first to send rumors, then to ask for rumors
	if len(rumorsToSend) > 0 {
		// Start rumormongering
		g.rumorLock.RLock()
		var rumor RumorMessage = g.rumorMsgs[rumorsToSend[0]]
		g.rumorLock.RUnlock()

		g.peers.InformStatusReception(&LocalizedPeerStatuses{Sender: sender, Statuses: status.Want})

		g.StartRumormongering(&rumor, sender)
	} else if len(rumorsToAsk) > 0 {
		// Send a StatusPacket
		g.SendGossipPacket(g.createStatusMessage(), sender)
	} else {
		fmt.Println("IN SYNC WITH", sender)
	}
}

func (g *Gossiper) StartRumorMongeringProcess(rumor *RumorMessage, excluded ...PeerAddress) (PeerAddress, bool) {
	// Pick a random neighbor and send rumor
	randNeighbor, present := g.peers.PickRandomNeighbor(excluded...)

	if present {
		g.StartRumormongering(rumor, randNeighbor)
	}

	return randNeighbor, present
}

func (g *Gossiper) StartRumormongering(rumor *RumorMessage, peerAddr PeerAddress) {
	go func() {
		observerChan := make(chan PeerStatus, BUFFERSIZE)
		timer := time.NewTimer(STATUS_MESSAGE_TIMEOUT)

		// Register our channel to receive updates
		g.peers.RegisterChannel(observerChan, StatusInterest{
			Sender:     peerAddr.String(),
			Identifier: rumor.Origin})

		// Function to call to unregister our channel
		unregister := func() {
			timer.Stop()
			g.peers.UnregisterChannel(observerChan)
		}

		for {
			select {
			case peerStatus, ok := <-observerChan:
				// Channel was closed by the dispatcher
				if !ok {
					return
				}

				// Accept all status that would validate the rumor ACK
				if peerStatus.NextID >= rumor.ID {
					g.peers.UpdatePeerState(peerAddr, peerStatus)

					if g.PeerIsInSync(peerAddr) {
						g.coinFlipRumorMongering(rumor, peerAddr)
					}

					unregister()
				}

			case <-timer.C: // Timed out
				// Resend the rumor to another neighbor with prob 1/2
				g.coinFlipRumorMongering(rumor, peerAddr)
				unregister()
			}
		}
	}()

	fmt.Println("MONGERING with", peerAddr)
	g.SendGossipPacket(rumor, peerAddr)
}

func (g *Gossiper) coinFlipRumorMongering(rumor *RumorMessage, excludedPeers ...PeerAddress) {
	if rand.Intn(2) == 0 {
		neighbor, sent := g.StartRumorMongeringProcess(rumor, excludedPeers...)
		if sent {
			fmt.Println("FLIPPED COIN sending rumor to", neighbor)
		}
	}
}

/* Sender can be nil, meaning the rumor was sent by a client */
// TODO May have a better option than setting setter to nil to indicate this ?
//      Ex: a PeerAddress where IsValid() == false when sent by client ?
func (g *Gossiper) HandleRumorMessage(rumor *RumorMessage, sender PeerAddress) {
	// Received a rumor message from sender
	diff := g.SelfDiffRumorID(rumor)

	// No matter the diff, we will send a status back:
	defer func() {
		// Send back the status ACK for the message if it wasn't sent by the local client
		if sender != nil {
			g.SendGossipPacket(g.createStatusMessage(), sender)
		}
	}()

	// This rumor is what we wanted, so we accept it
	if diff == 0 {
		g.acceptRumorMessage(rumor)
		if sender != nil {
			g.updateRoutingTable(rumor.Origin, sender)
			g.StartRumorMongeringProcess(rumor, sender)
		} else {
			g.StartRumorMongeringProcess(rumor)
		}
	}

	// Other cases are:
	// - diff > 0: We are in advance, peer should send us a statusmessage afterwards in order to sync
	// - diff < 0: We are late => we will send status message
}

func (g *Gossiper) updateRoutingTable(origin string, peerAddr PeerAddress) {
	newPeerAddr := peerAddr.String()
	// TODO Locks !
	prevPeerAddr, present := g.routingTable[origin]
	if !present || newPeerAddr != prevPeerAddr {
		g.routingTable[origin] = newPeerAddr
		fmt.Println("DSDV", origin, newPeerAddr)
	}
}

// A return value =0 means we are sync, >0 we are in advance, <0 we are late
func (g *Gossiper) SelfDiffRumorID(rumor *RumorMessage) int {
	// TODO put initialization elsewhere ?
	peerStatus, present := g.peerStatuses[rumor.Origin]
	if !present {
		peerStatus = PeerStatus{
			Identifier: rumor.Origin,
			NextID:     1,
		}
		g.peerStatuses[rumor.Origin] = peerStatus
	}

	return int(peerStatus.NextID) - int(rumor.ID)
}

func (g *Gossiper) WantList() []PeerStatus {
	peerStatuses := make([]PeerStatus, 0)
	for _, peerStatus := range g.peerStatuses {
		peerStatuses = append(peerStatuses, peerStatus)
	}
	return peerStatuses
}

func (g *Gossiper) PeerIsInSync(peer PeerAddress) bool {
	// TODO Do an optimized version of it
	rumorsToSend, rumorsToAsk := g.ComputeRumorStatusDiff(g.peers.GetPeerWantList(peer))
	return rumorsToAsk == nil && rumorsToSend == nil
}

/* Current implementation may have multiple times the same rumor in rumorsToSend/rumorsToAsk */
func (g *Gossiper) ComputeRumorStatusDiff(otherStatus []PeerStatus) (rumorsToSend, rumorsToAsk []PeerStatus) {
	rumorsToSend = make([]PeerStatus, 0)
	rumorsToAsk = make([]PeerStatus, 0)
	otherOrigins := make([]string, 0)
	// Check that we have all rumors already
	for _, peerStatus := range otherStatus {
		// Create the list of origins we will use to do the symmetric diff
		otherOrigins = append(otherOrigins, peerStatus.Identifier)

		currStatus, present := g.peerStatuses[peerStatus.Identifier]
		if !present {
			// We didn't know this origin before
			rumorsToAsk = append(rumorsToAsk, PeerStatus{Identifier: peerStatus.Identifier, NextID: 1})
		} else if currStatus.NextID < peerStatus.NextID {
			// We are late on rumors from this origin
			// /!\ We have to ask for OUR peerStatus, because we can only accept the next message
			rumorsToAsk = append(rumorsToAsk, currStatus)
		} else if currStatus.NextID > peerStatus.NextID {
			// We are more in advance than them
			// /!\ We have to send the rumor corresponding to their peerStatus (peer can't accept another)
			rumorsToSend = append(rumorsToSend, peerStatus)
		}
	}

	otherOriginsSet := StringSetInit(otherOrigins)

	// See if we have origins that they don't have (the other cases are handled already)
	for peerID, _ := range g.peerStatuses {
		if !otherOriginsSet.Has(peerID) {
			rumorsToSend = append(rumorsToSend, PeerStatus{Identifier: peerID, NextID: 1})
		}
	}

	return
}

func (g *Gossiper) HandleNodeMessage(simple *SimpleMessage) {
	msg := g.createForwardedMessage(simple)
	// Add msg peer to peers
	g.peers.AddPeer(StringAddress{simple.RelayPeerAddr})
	// Broadcast to everyone but sender
	g.BroadcastMessage(msg, StringSetInitSingleton(simple.RelayPeerAddr))
}

func (g *Gossiper) acceptRumorMessage(r *RumorMessage) {
	oldPeerStatus, _ := g.peerStatuses[r.Origin]
	newPeerStatus := PeerStatus{Identifier: r.Origin, NextID: r.ID + 1}

	g.rumorLock.Lock()
	g.rumorMsgs[oldPeerStatus] = *r
	g.rumorLock.Unlock()
	g.peerStatuses[r.Origin] = newPeerStatus
}

func (g *Gossiper) createStatusMessage() *StatusPacket {
	wantList := g.WantList()
	return &StatusPacket{Want: wantList}
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

func (g *Gossiper) createClientRumor(m *Message) *RumorMessage {
	selfStatus, present := g.peerStatuses[g.Name]
	var nextID uint32
	if !present {
		nextID = 1
	} else {
		nextID = selfStatus.NextID
	}
	return &RumorMessage{
		Origin: g.Name,
		ID:     nextID,
		Text:   m.Text,
	}
}

func (g *Gossiper) SendGossipPacket(tgp ToGossipPacket, peerAddr PeerAddress) {
	g.sendChannel <- &WrappedGossipPacket{sender: peerAddr, gossipMsg: tgp.ToGossipPacket()}
}

// TODO Excluded as PeerAddress... ?
func (g *Gossiper) BroadcastMessage(m *SimpleMessage, excludedPeers *StringSet) {
	for _, peer := range g.peers.AllPeers() {
		if excludedPeers == nil || !excludedPeers.Has(peer.String()) {
			g.SendGossipPacket(m, peer)
		}
	}
}

func (g *Gossiper) PrintPeers() {
	fmt.Printf("PEERS %s\n", strings.Join(g.peers.AllPeersStr(), ","))
}
