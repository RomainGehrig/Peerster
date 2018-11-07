package peers

import (
	. "github.com/RomainGehrig/Peerster/constants"
)

type registrationMessageType int

const (
	Register registrationMessageType = iota
	Unregister
)

type registrationMessage struct {
	observerChan PeerStatusObserver
	subject      StatusInterest
	msgType      registrationMessageType
}

type Dispatcher struct {
	statusChannel   chan *LocalizedPeerStatuses
	registerChannel chan registrationMessage
}

func runPeerStatusDispatcher() *Dispatcher {
	inputChan := make(chan *LocalizedPeerStatuses, CHANNEL_BUFFERSIZE)
	// Register chan is unbuffered to prevent sending an unregister message registration
	// TODO Is that true (?)
	registerChannel := make(chan registrationMessage)
	dispatcher := &Dispatcher{statusChannel: inputChan, registerChannel: registerChannel}

	go dispatcher.watchForMessages()

	return dispatcher
}

// The meaty function:
//   We basically wait for statuses to be communicated through the `statusChannel`
//   and we dispatch them to the goroutines that were interested in knowing their
//   status.
func (d *Dispatcher) watchForMessages() {
	// Keep a "list" of observers per subject of interest
	// TODO Change bool to something else (empty struct ?)
	subjects := make(map[StatusInterest](map[PeerStatusObserver]bool))
	// Observers are a mapping from their channels to their interest (needed to find them in above map)
	observers := make(map[PeerStatusObserver]StatusInterest)

	for {
		select {
		case regMsg := <-d.registerChannel:
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
			for _, peerStatus := range newStatuses.Statuses {
				subject := StatusInterest{Sender: newStatuses.Sender.String(), Identifier: peerStatus.Identifier}
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
