package rumors

import (
	"fmt"
	. "github.com/RomainGehrig/Peerster/messages"
	. "github.com/RomainGehrig/Peerster/peers"
	. "github.com/RomainGehrig/Peerster/utils"
	"math/rand"
	"time"
)

func (r *RumorHandler) StartRumorMongeringProcess(rumor *RumorMessage, excluded ...PeerAddress) (PeerAddress, bool) {
	// Pick a random neighbor and send rumor
	randNeighbor, present := r.peers.PickRandomNeighbor(excluded...)

	if present {
		r.StartRumormongering(rumor, randNeighbor)
	}

	return randNeighbor, present
}

func (r *RumorHandler) StartRumormongering(rumor *RumorMessage, peerAddr PeerAddress) {
	go func() {
		observerChan := make(chan PeerStatus, BUFFERSIZE_TMP)
		timer := time.NewTimer(STATUS_MESSAGE_TIMEOUT)

		// Register our channel to receive updates
		r.peers.RegisterChannel(observerChan, StatusInterest{
			Sender:     peerAddr.String(),
			Identifier: rumor.Origin})

		// Function to call to unregister our channel
		unregister := func() {
			timer.Stop()
			r.peers.UnregisterChannel(observerChan)
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
					r.peers.UpdatePeerState(peerAddr, peerStatus)

					if r.PeerIsInSync(peerAddr) {
						r.coinFlipRumorMongering(rumor, peerAddr)
					}

					unregister()
				}

			case <-timer.C: // Timed out
				// Resend the rumor to another neighbor with prob 1/2
				r.coinFlipRumorMongering(rumor, peerAddr)
				unregister()
			}
		}
	}()

	fmt.Println("MONGERING with", peerAddr)
	r.net.SendGossipPacket(rumor, peerAddr)
}

func (r *RumorHandler) coinFlipRumorMongering(rumor *RumorMessage, excludedPeers ...PeerAddress) {
	if rand.Intn(2) == 0 {
		neighbor, sent := r.StartRumorMongeringProcess(rumor, excludedPeers...)
		if sent {
			fmt.Println("FLIPPED COIN sending rumor to", neighbor)
		}
	}
}

func (r *RumorHandler) PeerIsInSync(peer PeerAddress) bool {
	// TODO Do an optimized version of it
	rumorsToSend, rumorsToAsk := r.ComputeRumorStatusDiff(r.peers.GetPeerWantList(peer))
	return rumorsToAsk == nil && rumorsToSend == nil
}

/* Current implementation may have multiple times the same rumor in rumorsToSend/rumorsToAsk */
func (r *RumorHandler) ComputeRumorStatusDiff(otherStatus []PeerStatus) (rumorsToSend, rumorsToAsk []PeerStatus) {
	rumorsToSend = make([]PeerStatus, 0)
	rumorsToAsk = make([]PeerStatus, 0)
	otherOrigins := make([]string, 0)
	// Check that we have all rumors already
	for _, peerStatus := range otherStatus {
		// Create the list of origins we will use to do the symmetric diff
		otherOrigins = append(otherOrigins, peerStatus.Identifier)

		currStatus, present := r.peerStatuses[peerStatus.Identifier]
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
	for peerID, _ := range r.peerStatuses {
		if !otherOriginsSet.Has(peerID) {
			rumorsToSend = append(rumorsToSend, PeerStatus{Identifier: peerID, NextID: 1})
		}
	}

	return
}
