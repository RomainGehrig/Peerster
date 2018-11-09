package files

import (
	"fmt"
	. "github.com/RomainGehrig/Peerster/constants"
	. "github.com/RomainGehrig/Peerster/messages"
)

type DataReplyObserver (chan<- *DataReply)

type DataReplyDispatcher struct {
	dataReplyChan chan *DataReply
	registerChan  chan *registrationMessage
}

type registrationMessage struct {
	observer DataReplyObserver
	subject  SHA256_HASH
	msgType  RegistrationMessageType
}

func runDataReplyDispatcher() *DataReplyDispatcher {
	dataReplyChan := make(chan *DataReply, CHANNEL_BUFFERSIZE)
	registerChan := make(chan *registrationMessage, CHANNEL_BUFFERSIZE)
	dispatcher := &DataReplyDispatcher{
		dataReplyChan: dataReplyChan,
		registerChan:  registerChan,
	}

	go dispatcher.watchForMessages()

	return dispatcher
}

func (d *DataReplyDispatcher) watchForMessages() {

	// We assume that a chunk has a unique SHA256 (because deterministic function)
	// AND that a SHA256 uniquely identifies a chunk (non collision). That means
	// we can only have one channel interested in it.
	subjects := make(map[SHA256_HASH]DataReplyObserver)

	go func() {
		for {
			select {
			case reg := <-d.registerChan:
				switch reg.msgType {
				case Register:
					// TODO checks that the subject does not already exist
					if prevObs, present := subjects[reg.subject]; present {
						fmt.Println("Hash", reg.subject, "was already being watched by another observer. Old:", prevObs, ", new: ", reg.observer)
						close(reg.observer)
						break
					}
					subjects[reg.subject] = reg.observer
				case Unregister:
					if obs, present := subjects[reg.subject]; present {
						delete(subjects, reg.subject)
						close(obs)
					} else {
						panic("Need to provide a valid subject (hash) to unregister")
					}
				}
			case dataRep := <-d.dataReplyChan:
				hash, err := toHash(dataRep.HashValue)
				if err != nil {
					fmt.Println(err)
				}
				if obs, present := subjects[hash]; present {
					obs <- dataRep
				}
			}
		}
	}()
}
