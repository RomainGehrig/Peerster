package peers

import (
	. "github.com/RomainGehrig/Peerster/messages"
)

func (p *PeersHandler) GetPeerWantList(peer PeerAddress) []PeerStatus {
	p.peerWantListLock.RLock()
	defer p.peerWantListLock.RUnlock()

	statusList, present := p.peerWantList[peer.String()]
	if !present {
		return nil
	}

	wantList := make([]PeerStatus, 0)
	for _, peerStatus := range statusList {
		wantList = append(wantList, peerStatus)
	}

	return wantList
}

/* Function that update the view we have of our neighbors */
func (p *PeersHandler) UpdatePeerState(peer PeerAddress, newPeerStatus PeerStatus) (updated bool) {
	updated = true

	p.peerWantListLock.Lock()
	defer p.peerWantListLock.Unlock()
	mp, present := p.peerWantList[peer.String()]
	if !present {
		mp = make(map[string]PeerStatus)
		p.peerWantList[peer.String()] = mp
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
