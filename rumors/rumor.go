package rumors

import (
	"fmt"
	"github.com/RomainGehrig/Peerster/interfaces"
	. "github.com/RomainGehrig/Peerster/messages"
	. "github.com/RomainGehrig/Peerster/network"
	. "github.com/RomainGehrig/Peerster/peers"
	"sync"
	"time"
)

const BUFFERSIZE_TMP int = 1024
const STATUS_MESSAGE_TIMEOUT = 1 * time.Second

type RumorHandler struct {
	peerStatuses map[string]PeerStatus
	rumorMsgs    map[PeerStatus]RumorMessage
	rumorLock    *sync.RWMutex
	net          *NetworkHandler
	peers        *PeersHandler
	routing      interfaces.UpdatableRouter
	// TODO get
	name string
}

func NewRumorHandler(name string) *RumorHandler {
	return &RumorHandler{
		name:         name,
		peerStatuses: make(map[string]PeerStatus),
		rumorMsgs:    make(map[PeerStatus]RumorMessage),
		rumorLock:    &sync.RWMutex{},
	}
}

func (r *RumorHandler) RunRumorHandler(net *NetworkHandler, peers *PeersHandler, routing interfaces.UpdatableRouter) {
	r.net = net
	r.peers = peers
	r.routing = routing
}

func (r *RumorHandler) CreateClientRumor(text string) *RumorMessage {
	selfStatus, present := r.peerStatuses[r.name]
	var nextID uint32
	if !present {
		nextID = 1
	} else {
		nextID = selfStatus.NextID
	}
	return &RumorMessage{
		Origin: r.name,
		ID:     nextID,
		Text:   text,
	}
}

func (r *RumorHandler) NonEmptyRumors() []RumorMessage {
	out := make([]RumorMessage, 0)

	r.rumorLock.RLock()
	defer r.rumorLock.RUnlock()
	for _, msg := range r.rumorMsgs {
		if msg.Text != "" {
			out = append(out, msg)
		}
	}

	return out
}

// A return value =0 means we are sync, >0 we are in advance, <0 we are late
func (r *RumorHandler) SelfDiffRumorID(rumor *RumorMessage) int {
	// TODO put initialization elsewhere ?
	peerStatus, present := r.peerStatuses[rumor.Origin]
	if !present {
		peerStatus = PeerStatus{
			Identifier: rumor.Origin,
			NextID:     1,
		}
		// TODO Lock ?
		r.peerStatuses[rumor.Origin] = peerStatus
	}

	return int(peerStatus.NextID) - int(rumor.ID)
}

func (r *RumorHandler) CreateStatusMessage() *StatusPacket {
	return &StatusPacket{Want: r.WantList()}
}

func (r *RumorHandler) WantList() []PeerStatus {
	peerStatuses := make([]PeerStatus, 0)
	for _, peerStatus := range r.peerStatuses {
		peerStatuses = append(peerStatuses, peerStatus)
	}
	return peerStatuses
}

/* Sender can be nil, meaning the rumor was sent by a client */
// TODO May have a better option than setting setter to nil to indicate this ?
//      Ex: a PeerAddress where IsValid() == false when sent by client ?
func (r *RumorHandler) HandleRumorMessage(rumor *RumorMessage, sender PeerAddress) {
	// Received a rumor message from sender
	diff := r.SelfDiffRumorID(rumor)

	// No matter the diff, we will send a status back:
	defer func() {
		// Send back the status ACK for the message if it wasn't sent by the local client
		if sender != nil {
			r.net.SendGossipPacket(r.CreateStatusMessage(), sender)
		}
	}()

	// This rumor is what we wanted, so we accept it
	if diff == 0 {
		r.acceptRumorMessage(rumor)
		if sender != nil {
			// TODO Routing only if
			r.routing.UpdateRoutingTable(rumor.Origin, sender)
			r.StartRumorMongeringProcess(rumor, sender)
		} else {
			r.StartRumorMongeringProcess(rumor)
		}
	}

	// Other cases are:
	// - diff > 0: We are in advance, peer should send us a statusmessage afterwards in order to sync
	// - diff < 0: We are late => we will send status message
}

func (r *RumorHandler) acceptRumorMessage(rumor *RumorMessage) {
	oldPeerStatus, _ := r.peerStatuses[rumor.Origin]
	newPeerStatus := PeerStatus{Identifier: rumor.Origin, NextID: rumor.ID + 1}

	r.rumorLock.Lock()
	r.rumorMsgs[oldPeerStatus] = *rumor
	r.rumorLock.Unlock()

	r.peerStatuses[rumor.Origin] = newPeerStatus
}

func (r *RumorHandler) HandleStatusMessage(status *StatusPacket, sender PeerAddress) {
	rumorsToSend, rumorsToAsk := r.ComputeRumorStatusDiff(status.Want)
	// newlyReceivedByPeer, newlyAvailableFromPeer := g.updatePeerViewedStatus(status.Want, sender.String())

	// The priority is first to send rumors, then to ask for rumors
	if len(rumorsToSend) > 0 {
		// Start rumormongering
		r.rumorLock.RLock()
		var rumor RumorMessage = r.rumorMsgs[rumorsToSend[0]]
		r.rumorLock.RUnlock()

		r.peers.InformStatusReception(&LocalizedPeerStatuses{Sender: sender, Statuses: status.Want})

		r.StartRumormongering(&rumor, sender)
	} else if len(rumorsToAsk) > 0 {
		// Send a StatusPacket
		// TODO Move to rumors
		r.net.SendGossipPacket(r.CreateStatusMessage(), sender)
	} else {
		fmt.Println("IN SYNC WITH", sender)
	}
}
