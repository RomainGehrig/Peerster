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

type PeerStatusObserver (chan<- PeerStatus)

type Gossiper struct {
	address    *net.UDPAddr
	conn       *net.UDPConn
	uiAddress  *net.UDPAddr
	uiConn     *net.UDPConn
	Name       string
	simpleMode bool
	// TODO RWMutex on peerstatuses, rumormsgs, knownPeers ?
	knownPeers       *StringSet
	peerStatuses     map[string]PeerStatus
	rumorMsgs        map[PeerStatus]RumorMessage
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
	input        chan LocalizedPeerStatuses
	registerChan chan RegistrationMessage
}

func NewGossiper(uiPort string, gossipAddr string, name string, peers []string, simple bool) *Gossiper {
	// fmt.Printf("Given arguments were: %s, %s, %s, %s, %s\n", uiPort, gossipAddr, name, peers[0], simple)
	// TODO Handle empty peers list
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
		simpleMode:       simple,
		Name:             name,
		knownPeers:       StringSetInit(peers),
		peerStatuses:     make(map[string]PeerStatus),
		rumorMsgs:        make(map[PeerStatus]RumorMessage),
		peerWantList:     make(map[string](map[string]PeerStatus)),
		peerWantListLock: &sync.RWMutex{},
	}
}

func (g *Gossiper) Run() {
	peerChan := g.PeersMessages()
	clientChan := g.ClientRequests()

	go g.AntiEntropy()
	g.dispatcher = RunPeerStatusDispatcher()
	g.ListenForMessages(peerChan, clientChan)
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
				// TODO
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
				if !present {
					panic("Should only unregister after a registration !")
				}
				delete(observers, toClose)
				delete(subjects[subject], toClose)

				close(toClose)

			}
		case newStatuses := <-d.input:
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
	// TODO Add buffering
	inputChan := make(chan LocalizedPeerStatuses, BUFFERSIZE)
	registerChan := make(chan RegistrationMessage, BUFFERSIZE)
	dispatcher := &Dispatcher{input: inputChan, registerChan: registerChan}

	go dispatcher.mainLoop()

	return dispatcher
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

func (g *Gossiper) ListenForMessages(peerMsgs <-chan *WrappedGossipPacket, clientRequests <-chan *Request) {
	for {
		select {
		case wgp := <-peerMsgs:
			g.DispatchPacket(wgp)
		case cliReq := <-clientRequests:
			g.DispatchClientRequest(cliReq)
		}
		g.PrintPeers()
	}
}

func (g *Gossiper) DispatchClientRequest(req *Request) {
	switch {
	case req.Get != nil:
		switch req.Get.Type {
		case NodeQuery:
		case MessageQuery:
		case PeerIDQuery:
		}
	case req.Post != nil:
		post := req.Post
		switch {
		case post.Node != nil:
		// TODO
		case post.Message != nil:
			g.HandleClientMessage(post.Message)
		}
	}
}

func (g *Gossiper) ClientRequests() <-chan *Request {
	out := make(chan *Request)
	receivedMsgs := MessageReceiver(g.uiConn)
	go func() {
		for {
			var request Request
			receivedMsg := <-receivedMsgs
			protobuf.Decode(receivedMsg.packetBytes, &request)
			out <- &request
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
			// TODO Is it safe to only pass a reference to the byte array?
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
	}
}

func (g *Gossiper) HandleClientMessage(m *Message) {
	fmt.Println(m)
	if g.simpleMode {
		msg := g.createClientMessage(m)
		g.BroadcastMessage(msg, nil)
	} else {
		msg := g.createClientRumor(m)
		g.HandleRumorMessage(msg, nil)
	}
}

func (g *Gossiper) HandleStatusMessage(status *StatusPacket, sender *net.UDPAddr) {
	rumorsToSend, rumorsToAsk := g.computeRumorStatusDiff(status.Want)
	// newlyReceivedByPeer, newlyAvailableFromPeer := g.updatePeerViewedStatus(status.Want, sender.String())

	// The priority is first to send rumors, then to ask for rumors
	if len(rumorsToSend) > 0 {
		// Start rumormongering
		var rumor RumorMessage = g.rumorMsgs[rumorsToSend[0]]
		g.dispatcher.input <- LocalizedPeerStatuses{sender: sender.String(), statuses: status.Want}
		g.StartRumormongering(&rumor, sender)
	} else if len(rumorsToAsk) > 0 {
		// Send a StatusPacket
		g.SendGossipPacket(g.createStatusMessage(), sender)
	} else {
		fmt.Println("IN SYNC WITH", sender)
	}
}

func (g *Gossiper) StartRumormongeringStr(rumor *RumorMessage, peerAddr string) {
	peerUDPAddr, _ := net.ResolveUDPAddr("udp4", peerAddr)
	g.StartRumormongering(rumor, peerUDPAddr)
}

func (g *Gossiper) StartRumorMongeringProcess(rumor *RumorMessage, excluded ...string) (string, bool) {
	// Pick a random neighbor and send rumor
	// TODO Is sender.String() the right way to compare with other?
	// TODO If we only have as single neighbor, we stop
	randNeighbor, present := g.pickRandomNeighbor(excluded...)

	if present {
		g.StartRumormongeringStr(rumor, randNeighbor)
	}

	return randNeighbor, present
}

func (g *Gossiper) getPeerWantList(peer string) []PeerStatus {
	g.peerWantListLock.RLock()
	defer g.peerWantListLock.RUnlock()

	statusList, _ := g.peerWantList[peer]
	wantList := make([]PeerStatus, len(statusList))
	for _, peerStatus := range statusList {
		wantList = append(wantList, peerStatus)
	}
	// TODO if not present, are we returning a nil slice ?
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
	// TODO Send message and create a goroutine waiting for the ack
	// TODO Start timer for the timeout
	// TODO Have to intercept the status packet that is relevant to the timeout
	// TODO Act on StatusPacket -> left to reception ?
	// TODO if timer is triggered, flip coin and restart rumormongering with a different peer
	go func() {
		observerChan := make(chan PeerStatus, BUFFERSIZE)
		timer := time.NewTimer(STATUS_MESSAGE_TIMEOUT)
		peer := peerUDPAddr.String()

		unregister := func() {
			timer.Stop()
			g.dispatcher.registerChan <- RegistrationMessage{
				observerChan: observerChan,
				msgType:      Unregister,
			}
		}

		// Register our channel to receive updates
		g.dispatcher.registerChan <- RegistrationMessage{
			observerChan: observerChan,
			subject: StatusInterest{
				sender:     peer,
				identifier: rumor.Origin},
			msgType: Register,
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
			// TODO NextID starts at zero ?
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
			// TODO NextID starts at zero?
			rumorsToSend = append(rumorsToSend, PeerStatus{Identifier: peerID, NextID: 1})
		}
	}

	return
}

/* Sender can be nil, meaning the rumor was sent by a client */
func (g *Gossiper) HandleRumorMessage(rumor *RumorMessage, sender *net.UDPAddr) {
	// Received a rumor message from sender
	diff := g.SelfDiffRumorID(rumor)
	// Send back the status ACK for the message
	defer func() {
		if sender != nil {
			g.SendGossipPacket(g.createStatusMessage(), sender)
		}
	}()
	switch {
	case diff == 0: // TODO => This rumor is what we wanted, so we accept it
		g.acceptRumorMessage(rumor)
		g.StartRumorMongeringProcess(rumor, sender.String())
	case diff > 0: // TODO => we are in advance, peer should send us a statusmessage afterwards in order to sync
	case diff < 0: // TODO => we are late => we will send status message
	}
}

func (g *Gossiper) WantList() []PeerStatus {
	peerStatuses := make([]PeerStatus, 0)
	// TODO RLock ?
	for _, peerStatus := range g.peerStatuses {
		peerStatuses = append(peerStatuses, peerStatus)
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
	g.rumorMsgs[oldPeerStatus] = *r
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
	// TODO NextID starts at zero ?
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
	gp := tgp.ToGossipPacket()

	packetBytes, err := protobuf.Encode(gp)
	if err != nil {
		fmt.Println(err)
	}
	_, err = g.conn.WriteToUDP(packetBytes, peerUDPAddr)
	if err != nil {
		fmt.Println(err)
	}

	// TODO Handle the prints occuring when sending a packet
	// => Only when mongering
	switch tgp.(type) {
	case *RumorMessage:
		fmt.Println("MONGERING with", peerUDPAddr)
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
