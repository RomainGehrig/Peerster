package peers

import (
	"fmt"
	. "github.com/RomainGehrig/Peerster/constants"
	. "github.com/RomainGehrig/Peerster/messages"
	. "github.com/RomainGehrig/Peerster/utils"
	"math/rand"
	"strings"
	"sync"
)

type PeersHandler struct {
	knownPeers       []PeerAddress
	peerWantList     map[string](map[string]PeerStatus)
	peerWantListLock *sync.RWMutex
	dispatcher       *Dispatcher
}

type PeerStatusObserver (chan<- PeerStatus)

// TODO Better name
type StatusInterest struct {
	Sender     string
	Identifier string
}

// TODO better name
type LocalizedPeerStatuses struct {
	Sender   PeerAddress
	Statuses []PeerStatus
}

func NewPeersHandler(peers []string) *PeersHandler {
	return &PeersHandler{
		knownPeers:       AsStringAddresses(peers...),
		peerWantList:     make(map[string](map[string]PeerStatus)),
		peerWantListLock: &sync.RWMutex{},
		dispatcher:       runPeerStatusDispatcher(),
	}
}

func (p *PeersHandler) AllPeers() []PeerAddress {
	return p.knownPeers
}

func (p *PeersHandler) AllPeersStr() []string {
	peers := make([]string, 0)
	for _, peer := range p.knownPeers {
		peers = append(peers, peer.String())
	}
	return peers
}

func (p *PeersHandler) AddPeer(peerAddr PeerAddress) {
	peer := peerAddr.String()
	for _, p := range p.knownPeers {
		if p.String() == peer {
			return
		}
	}
	p.knownPeers = append(p.knownPeers, peerAddr)
}

func (p *PeersHandler) PrintPeers() {
	fmt.Printf("PEERS %s\n", strings.Join(p.AllPeersStr(), ","))
}

func (p *PeersHandler) PickRandomNeighbor(excludedAddresses ...PeerAddress) (PeerAddress, bool) {
	// TODO Find an alternative to the conversion to strings
	excluded := make([]string, 0)
	for _, exAddr := range excludedAddresses {
		excluded = append(excluded, exAddr.String())
	}
	excludedSet := StringSetInit(excluded)
	notExcluded := make([]string, 0)

	for _, peer := range p.knownPeers {
		peerStr := peer.String()
		if !excludedSet.Has(peerStr) {
			notExcluded = append(notExcluded, peerStr)
		}
	}

	// Cannot pick a random neighbor that is not excluded
	if len(notExcluded) == 0 {
		return nil, false
	}

	// Return a random peer
	return StringAddress{notExcluded[rand.Intn(len(notExcluded))]}, true
}

func (p *PeersHandler) InformStatusReception(lps *LocalizedPeerStatuses) {
	p.dispatcher.statusChannel <- lps
}

func (p *PeersHandler) RegisterChannel(observerChan PeerStatusObserver, interest StatusInterest) {
	p.dispatcher.registerChannel <- registrationMessage{
		observerChan: observerChan,
		subject:      interest,
		msgType:      Register,
	}
}

func (p *PeersHandler) UnregisterChannel(observerChan PeerStatusObserver) {
	p.dispatcher.registerChannel <- registrationMessage{
		observerChan: observerChan,
		msgType:      Unregister,
	}
}
