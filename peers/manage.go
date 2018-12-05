package peers

import (
	"fmt"
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
}

func NewPeersHandler(peers []string) *PeersHandler {
	return &PeersHandler{
		knownPeers:       AsStringAddresses(peers...),
		peerWantList:     make(map[string](map[string]PeerStatus)),
		peerWantListLock: &sync.RWMutex{},
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

// Pick up to N different neighbors, without any that is in the excluded list
func (p *PeersHandler) PickRandomNeighbors(n int, excludedAddresses ...PeerAddress) (peers []PeerAddress) {
	peers = make([]PeerAddress, 0)
	excluded := StringSetInit(AsStrings(excludedAddresses...))

	for _, i := range rand.Perm(len(p.knownPeers)) {
		peer := p.knownPeers[i]
		if excluded.Has(peer.String()) {
			continue
		}

		peers = append(peers, peer)
		if len(peers) == n {
			break
		}
	}

	return peers
}

func (p *PeersHandler) PickRandomNeighbor(excludedAddresses ...PeerAddress) (PeerAddress, bool) {
	excludedSet := StringSetInit(AsStrings(excludedAddresses...))
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
