package files

import (
	"fmt"
	. "github.com/RomainGehrig/Peerster/constants"
	. "github.com/RomainGehrig/Peerster/messages"
	"sync/atomic"
)

type SearchReplyObserver (chan<- *SearchReply)

type searchRegistrationMessage struct {
	observer SearchReplyObserver
	queryID  uint32
	msgType  RegistrationMessageType
}

type SearchReplyDispatcher struct {
	srepChannel    chan *SearchReply
	regChannel     chan *searchRegistrationMessage
	currentQueryID uint32
}

func runSearchReplyDispatcher() *SearchReplyDispatcher {
	srepChannel := make(chan *SearchReply, CHANNEL_BUFFERSIZE)
	regChannel := make(chan *searchRegistrationMessage, CHANNEL_BUFFERSIZE)

	dispatcher := &SearchReplyDispatcher{
		srepChannel: srepChannel,
		regChannel:  regChannel,
	}

	go dispatcher.watchForSearchReplies()

	return dispatcher
}

func (f *FileHandler) registerQuery(query *Query) {
	query.id = f.srepDispatcher.newQueryID()

	f.srepDispatcher.regChannel <- &searchRegistrationMessage{
		observer: query.replyChan,
		queryID:  query.id,
		msgType:  Register,
	}
}

func (f *FileHandler) unregisterQuery(query *Query) {
	f.srepDispatcher.regChannel <- &searchRegistrationMessage{
		queryID:  query.id,
		observer: query.replyChan,
		msgType:  Unregister,
	}
}

func (s *SearchReplyDispatcher) newQueryID() uint32 {
	return atomic.AddUint32(&s.currentQueryID, 1)
}

func (s *SearchReplyDispatcher) watchForSearchReplies() {
	go func() {
		queries := make(map[uint32]SearchReplyObserver)

		for {
			select {
			case reg := <-s.regChannel:
				// TODO double register?
				if reg.msgType == Register {
					queries[reg.queryID] = reg.observer
				} else {
					// TODO Close channel ?
					delete(queries, reg.queryID)
				}
			case rep := <-s.srepChannel:
				for _, queryChan := range queries {
					queryChan <- rep
				}
			}
		}
	}()

}
