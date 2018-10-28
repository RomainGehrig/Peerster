package gossiper

import (
	"fmt"
	. "github.com/RomainGehrig/Peerster/messages"
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

type PeerStatusObserver (chan<- PeerStatus)

type Gossiper struct {
	address          *net.UDPAddr
	conn             *net.UDPConn
	uiAddress        *net.UDPAddr
	uiConn           *net.UDPConn
	rtimer           time.Duration
	Name             string
	simpleMode       bool
	knownPeers       *StringSet
	privateMsgs      []PrivateMessage  // TODO better datastructure ?
	routingTable     map[string]string // From Origin to ip:port
	sendChannel      chan<- *WrappedGossipPacket
	peerStatuses     map[string]PeerStatus
	rumorMsgs        map[PeerStatus]RumorMessage
	rumorLock        *sync.RWMutex
	peerWantList     map[string](map[string]PeerStatus)
	peerWantListLock *sync.RWMutex
	dispatcher       *Dispatcher
}

type ReceivedMessage struct {
	packetBytes []byte
	sender      *net.UDPAddr
}

type WrappedGossipPacket struct {
	sender    *net.UDPAddr
	gossipMsg *GossipPacket
}

type WrappedClientRequest struct {
	request *Request
	sender  *net.UDPAddr
}

type RegistrationMessageType int

const (
	Register RegistrationMessageType = iota
	Unregister
)

// TODO Better name
type StatusInterest struct {
	sender     string
	identifier string
}

type RegistrationMessage struct {
	observerChan PeerStatusObserver
	subject      StatusInterest
	msgType      RegistrationMessageType
}

// TODO better name
type LocalizedPeerStatuses struct {
	sender   string
	statuses []PeerStatus
}

type Dispatcher struct {
	statusChannel chan *LocalizedPeerStatuses
	registerChan  chan RegistrationMessage
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
		address:          udpAddr,
		conn:             udpConn,
		uiAddress:        udpUIAddr,
		uiConn:           udpUIConn,
		rtimer:           time.Duration(rtimer) * time.Second,
		simpleMode:       simple,
		Name:             name,
		knownPeers:       StringSetInit(peers),
		privateMsgs:      make([]PrivateMessage, 0),
		routingTable:     make(map[string]string),
		peerStatuses:     make(map[string]PeerStatus),
		rumorMsgs:        make(map[PeerStatus]RumorMessage),
		rumorLock:        &sync.RWMutex{},
		peerWantList:     make(map[string](map[string]PeerStatus)),
		peerWantListLock: &sync.RWMutex{},
	}
}

func (g *Gossiper) Run() {
	peerChan := g.PeersMessages()
	clientChan := g.ClientRequests()

	sendChan := make(chan *WrappedGossipPacket, BUFFERSIZE)
	g.sendChannel = sendChan

	go g.AntiEntropy()
	if g.rtimer != 0 {
		go g.RunRoutingMessages()
	}
	g.dispatcher = RunPeerStatusDispatcher()

	g.ListenForMessages(peerChan, clientChan, sendChan)
}

func (d *Dispatcher) mainLoop() {
	// Keep a "list" of observers per subject of interest
	// TODO Change bool to something else (empty struct ?)
	subjects := make(map[StatusInterest](map[PeerStatusObserver]bool))
	// Observers are a mapping from their channels to their interest (needed to find them in above map)
	observers := make(map[PeerStatusObserver]StatusInterest)

	for {
		select {
		case regMsg := <-d.registerChan:
			switch regMsg.msgType {
			case Register:
				subject := regMsg.subject
				mp, present := subjects[subject]
				if !present {
					mp = make(map[PeerStatusObserver]bool)
					subjects[subject] = mp
				}
				mp[regMsg.observerChan] = true
				observers[regMsg.observerChan] = subject
			case Unregister:
				toClose := regMsg.observerChan
				subject, present := observers[toClose]
				// TODO Find case where observer is not present
				// Concurrent bug ? Sending a unregister before the register ? :O
				if present {
					delete(observers, toClose)
					delete(subjects[subject], toClose)

					close(toClose)
				} else {
					// fmt.Println("Should only unregister after a registration !")
				}
			}
		case newStatuses := <-d.statusChannel:
			for _, peerStatus := range newStatuses.statuses {
				subject := StatusInterest{sender: newStatuses.sender, identifier: peerStatus.Identifier}
				interestedChans, present := subjects[subject]
				if !present {
					continue
				}

				for interestedChan, _ := range interestedChans {
					interestedChan <- peerStatus
				}
			}
		}
	}
}

func RunPeerStatusDispatcher() *Dispatcher {
	inputChan := make(chan *LocalizedPeerStatuses, BUFFERSIZE)
	// Register chan is unbuffered to prevent sending an unregister message registration
	registerChan := make(chan RegistrationMessage)
	dispatcher := &Dispatcher{statusChannel: inputChan, registerChan: registerChan}

	go dispatcher.mainLoop()

	return dispatcher
}

func (g *Gossiper) RunRoutingMessages() {
	ticker := time.NewTicker(g.rtimer)
	defer ticker.Stop()

	// Start by sending a routing message to all neighbors
	g.SendRoutingMessage(g.knownPeers.ToSlice()...)

	for {
		// Wait for ticker
		<-ticker.C

		neighbor, present := g.pickRandomNeighbor()
		if present {
			g.SendRoutingMessage(neighbor)
		}
	}
}

func (g *Gossiper) SendRoutingMessage(peers ...string) {
	routingMessage := g.createClientRumor(&Message{Text: ""})

	for _, p := range peers {
		g.SendGossipPacketStr(routingMessage, p)
	}
}

func (g *Gossiper) AntiEntropy() {
	ticker := time.NewTicker(ANTIENTROPY_TIME)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			neighbor, present := g.pickRandomNeighbor()
			if present {
				g.SendGossipPacketStr(g.createStatusMessage(), neighbor)
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
			peerUDPAddr := wgp.sender
			packetBytes, err := protobuf.Encode(gp)
			if err != nil {
				fmt.Println(err)
			}
			_, err = g.conn.WriteToUDP(packetBytes, peerUDPAddr)
			if err != nil {
				fmt.Println(err)
			}
		}
	}
}

func (g *Gossiper) SendClientResponse(resp *Response, addr *net.UDPAddr) {
	packetBytes, err := protobuf.Encode(resp)
	if err != nil {
		fmt.Println(err)
	}
	_, err = g.uiConn.WriteToUDP(packetBytes, addr)
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
			nodes := g.knownPeers.ToSlice()
			resp.Nodes = nodes
		case MessageQuery:
			rumors := g.AllRumors()
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
			g.AddPeer(post.Node.Addr)
		case post.Message != nil:
			g.HandleClientMessage(post.Message)
			g.PrintPeers()
		}
	}
}

func (g *Gossiper) AllRumors() []RumorMessage {
	out := make([]RumorMessage, 0)

	g.rumorLock.RLock()
	defer g.rumorLock.RUnlock()
	for _, msg := range g.rumorMsgs {
		out = append(out, msg)
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
			out <- &WrappedGossipPacket{receivedMsg.sender, &packet}
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
			out <- &ReceivedMessage{packetBytes, sender}
		}
	}()
	return out
}

func (g *Gossiper) DispatchPacket(wpacket *WrappedGossipPacket) {
	packet := wpacket.gossipMsg
	sender := wpacket.sender
	g.AddPeer(sender.String())
	switch {
	case packet.Simple != nil:
		fmt.Println(packet.Simple)
		g.HandleNodeMessage(packet.Simple)
	case packet.Rumor != nil:
		fmt.Println(packet.Rumor.StringWithSender(sender.String()))
		g.HandleRumorMessage(packet.Rumor, sender)
	case packet.Status != nil:
		fmt.Println(packet.Status.StringWithSender(sender.String()))
		g.HandleStatusMessage(packet.Status, sender)
	case packet.Private != nil:
		if packet.Private.Destination == g.Name {
			fmt.Println(packet.Private)
		}
		g.HandlePrivateMessage(packet.Private)
	}
}

func (g *Gossiper) HandleClientMessage(m *Message) {
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
		newPM, shouldSend := g.preparePrivateMessage(p)
		if !shouldSend {
			return
		}
		nextHop, valid := g.FindRouteTo(p.Destination)
		if valid {
			g.SendGossipPacketStr(newPM, nextHop)
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

func (g *Gossiper) FindRouteTo(dest string) (neighbor string, valid bool) {
	// TODO Locking
	neighbor, valid = g.routingTable[dest]

	// For the moment, we can only send message to destination we know, ie that
	// are in the routing table. So logically, we know the route. If we don't,
	// someone tried to send a message to a destination that does not exist or
	// that we don't know yet.
	// TODO Fallback to sending to a random neighbor ?
	if !valid {
		fmt.Println("Destination", dest, "not found in the routing table.")
	}

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

func (g *Gossiper) HandleStatusMessage(status *StatusPacket, sender *net.UDPAddr) {
	rumorsToSend, rumorsToAsk := g.computeRumorStatusDiff(status.Want)
	// newlyReceivedByPeer, newlyAvailableFromPeer := g.updatePeerViewedStatus(status.Want, sender.String())

	// The priority is first to send rumors, then to ask for rumors
	if len(rumorsToSend) > 0 {
		// Start rumormongering
		g.rumorLock.RLock()
		var rumor RumorMessage = g.rumorMsgs[rumorsToSend[0]]
		g.rumorLock.RUnlock()

		g.InformStatusReception(&LocalizedPeerStatuses{sender: sender.String(), statuses: status.Want})

		g.StartRumormongering(&rumor, sender)
	} else if len(rumorsToAsk) > 0 {
		// Send a StatusPacket
		g.SendGossipPacket(g.createStatusMessage(), sender)
	} else {
		fmt.Println("IN SYNC WITH", sender)
	}
}

func (g *Gossiper) InformStatusReception(lps *LocalizedPeerStatuses) {
	g.dispatcher.statusChannel <- lps
}

func (g *Gossiper) StartRumormongeringStr(rumor *RumorMessage, peerAddr string) {
	peerUDPAddr, _ := net.ResolveUDPAddr("udp4", peerAddr)
	g.StartRumormongering(rumor, peerUDPAddr)
}

func (g *Gossiper) StartRumorMongeringProcess(rumor *RumorMessage, excluded ...string) (string, bool) {
	// Pick a random neighbor and send rumor
	randNeighbor, present := g.pickRandomNeighbor(excluded...)

	if present {
		g.StartRumormongeringStr(rumor, randNeighbor)
	}

	return randNeighbor, present
}

func (g *Gossiper) getPeerWantList(peer string) []PeerStatus {
	g.peerWantListLock.RLock()
	defer g.peerWantListLock.RUnlock()

	statusList, present := g.peerWantList[peer]
	if !present {
		return nil
	}

	wantList := make([]PeerStatus, len(statusList))
	for _, peerStatus := range statusList {
		wantList = append(wantList, peerStatus)
	}

	return wantList
}

func (g *Gossiper) peerIsInSync(peer string) bool {
	// TODO Do an optimized version of it
	rumorsToSend, rumorsToAsk := g.computeRumorStatusDiff(g.getPeerWantList(peer))
	return rumorsToAsk == nil && rumorsToSend == nil
}

/* Function that update the view we have of our neighbors */
func (g *Gossiper) updatePeerState(peer string, newPeerStatus PeerStatus) (updated bool) {
	updated = true

	g.peerWantListLock.Lock()
	defer g.peerWantListLock.Unlock()
	mp, present := g.peerWantList[peer]
	if !present {
		mp = make(map[string]PeerStatus)
		g.peerWantList[peer] = mp
	}

	oldPeerStatus, present := mp[newPeerStatus.Identifier]
	// In the case where the old status was more recent than the new one (could happen)
	// bc of concurrency => we keep the old status
	if present {
		if oldPeerStatus.NextID > newPeerStatus.NextID {
			newPeerStatus = oldPeerStatus
			updated = false
		}
	}
	mp[newPeerStatus.Identifier] = newPeerStatus

	return
}

func (g *Gossiper) StartRumormongering(rumor *RumorMessage, peerUDPAddr *net.UDPAddr) {
	go func() {
		observerChan := make(chan PeerStatus, BUFFERSIZE)
		timer := time.NewTimer(STATUS_MESSAGE_TIMEOUT)
		peer := peerUDPAddr.String()

		// Register our channel to receive updates
		g.dispatcher.registerChan <- RegistrationMessage{
			observerChan: observerChan,
			subject: StatusInterest{
				sender:     peer,
				identifier: rumor.Origin},
			msgType: Register,
		}

		// Function to call to unregister our channel
		unregister := func() {
			timer.Stop()
			g.dispatcher.registerChan <- RegistrationMessage{
				observerChan: observerChan,
				msgType:      Unregister,
			}
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
					g.updatePeerState(peer, peerStatus)

					if g.peerIsInSync(peer) {
						g.coinFlipRumorMongering(rumor, peer)
					}

					unregister()
				}

			case <-timer.C: // Timed out
				// Resend the rumor to another neighbor with prob 1/2
				g.coinFlipRumorMongering(rumor, peer)
				unregister()
			}
		}
	}()

	fmt.Println("MONGERING with", peerUDPAddr)
	g.SendGossipPacket(rumor, peerUDPAddr)
}

func (g *Gossiper) coinFlipRumorMongering(rumor *RumorMessage, excludedPeers ...string) {
	if rand.Intn(2) == 0 {
		neighbor, sent := g.StartRumorMongeringProcess(rumor, excludedPeers...)
		if sent {
			fmt.Println("FLIPPED COIN sending rumor to", neighbor)
		}
	}
}

func (g *Gossiper) computeRumorStatusDiff(otherStatus []PeerStatus) (rumorsToSend, rumorsToAsk []PeerStatus) {
	// Need to find messages that are new from the sender => easy: for each message, ask if message is in g.msgs
	// Need to find messages that are new to the sender => messages status that are not in status:
	//   Create a set with all the other's origins: iterate over self.origins and check if self.origin in set of others.origins

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

/* Sender can be nil, meaning the rumor was sent by a client */
func (g *Gossiper) HandleRumorMessage(rumor *RumorMessage, sender *net.UDPAddr) {
	// Received a rumor message from sender
	diff := g.SelfDiffRumorID(rumor)

	// No matter what, we will send a status back:
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
			g.updateRoutingTable(rumor.Origin, sender.String())
			g.StartRumorMongeringProcess(rumor, sender.String())
		} else {
			g.StartRumorMongeringProcess(rumor)
		}
	}

	// Other cases are:
	// - diff > 0: We are in advance, peer should send us a statusmessage afterwards in order to sync
	// - diff < 0: We are late => we will send status message
}

func (g *Gossiper) WantList() []PeerStatus {
	peerStatuses := make([]PeerStatus, 0)
	for _, peerStatus := range g.peerStatuses {
		peerStatuses = append(peerStatuses, peerStatus)
	}
	return peerStatuses
}

func (g *Gossiper) updateRoutingTable(origin string, newPeerAddr string) {
	// TODO Locks !
	prevPeerAddr, present := g.routingTable[origin]
	if !present || newPeerAddr != prevPeerAddr {
		g.routingTable[origin] = newPeerAddr
		fmt.Println("DSDV", origin, newPeerAddr)
	}
}

// A return value =0 means we are sync, >0 we are in advance, <0 we are late
func (g *Gossiper) SelfDiffRumorID(rumor *RumorMessage) int {
	// TODO put initialization elsewhere
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
	oldPeerStatus, _ := g.peerStatuses[r.Origin]
	newPeerStatus := PeerStatus{Identifier: r.Origin, NextID: r.ID + 1}

	g.rumorLock.Lock()
	g.rumorMsgs[oldPeerStatus] = *r
	g.rumorLock.Unlock()
	g.peerStatuses[r.Origin] = newPeerStatus
}

func (g *Gossiper) pickRandomNeighbor(excluded ...string) (string, bool) {
	peers := g.knownPeers.ToSlice()
	excludedSet := StringSetInit(excluded)
	notExcluded := make([]string, 0)

	for _, peer := range peers {
		if !excludedSet.Has(peer) {
			notExcluded = append(notExcluded, peer)
		}
	}

	// Cannot pick a random neighbor that is not excluded
	if len(notExcluded) == 0 {
		return "", false
	}

	// Return a random peer
	return notExcluded[rand.Intn(len(notExcluded))], true
}

func (g *Gossiper) createStatusMessage() *StatusPacket {
	wantList := g.WantList()
	// fmt.Println("Wantlist is of size", len(wantList), "before sending")
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

func (g *Gossiper) SendGossipPacket(tgp ToGossipPacket, peerUDPAddr *net.UDPAddr) {
	g.sendChannel <- &WrappedGossipPacket{sender: peerUDPAddr, gossipMsg: tgp.ToGossipPacket()}
}

func (g *Gossiper) SendGossipPacketStr(tgp ToGossipPacket, peerAddr string) {
	peerUDPAddr, _ := net.ResolveUDPAddr("udp4", peerAddr)
	g.SendGossipPacket(tgp, peerUDPAddr)
}

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
