package private

import (
	. "github.com/RomainGehrig/Peerster/constants"
	. "github.com/RomainGehrig/Peerster/messages"
	. "github.com/RomainGehrig/Peerster/network"
	. "github.com/RomainGehrig/Peerster/routing"
)

type PrivateHandler struct {
	// TODO Get name from somewhere else
	name        string
	routing     *RoutingHandler
	net         *NetworkHandler
	privateMsgs []PrivateMessage // TODO better datastructure ?
}

func NewPrivateHandler(name string) *PrivateHandler {
	return &PrivateHandler{
		name:        name,
		privateMsgs: make([]PrivateMessage, 0),
	}
}

func (p *PrivateHandler) GetPrivateMessages() []PrivateMessage {
	return p.privateMsgs
}

func (p *PrivateHandler) RunPrivateHandler(routing *RoutingHandler, net *NetworkHandler) {
	p.routing = routing
	p.net = net
}

func (p *PrivateHandler) HandlePrivateMessage(pm *PrivateMessage) {
	if pm.Destination == p.name {
		p.receivePrivateMessage(pm)
	} else {
		// Save messages that are sent by us
		if pm.Origin == p.name {
			p.receivePrivateMessage(pm)
		}

		shouldSend := p.preparePrivateMessage(pm)
		if !shouldSend {
			return
		}

		p.routing.SendPacketTo(pm, pm.Destination)
	}
}

func (p *PrivateHandler) receivePrivateMessage(pm *PrivateMessage) {
	// TODO Locks ?
	// TODO Better datastructure ?
	p.privateMsgs = append(p.privateMsgs, *pm)
}

/* Modifies in place the PrivateMessage given as argument */
func (p *PrivateHandler) preparePrivateMessage(pm *PrivateMessage) bool {
	// Won't forward
	if pm.HopLimit <= 1 {
		pm.HopLimit = 0
		return false
	}

	// Will forward
	pm.HopLimit -= 1
	return true
}

func (p *PrivateHandler) CreatePrivateMessage(text string, dest string) *PrivateMessage {
	return &PrivateMessage{
		Origin:      p.name,
		ID:          0, // TODO maybe do some kind of ordering
		Text:        text,
		Destination: dest,
		HopLimit:    DEFAULT_HOP_LIMIT + 1, // Add 1 because we are going to decrement it when sending
	}
}
