package failure

import (
	"sync"
	"time"

	. "github.com/RomainGehrig/Peerster/constants"
	. "github.com/RomainGehrig/Peerster/files"
	. "github.com/RomainGehrig/Peerster/messages"
	. "github.com/RomainGehrig/Peerster/routing"
	. "github.com/RomainGehrig/Peerster/simple"
)

type FailureHandler struct {
	Name           string
	Nodes          map[string]*OnlineMessage
	NodesLock      *sync.RWMutex
	NodesDelay     map[string]int64
	NodesDelayLock *sync.RWMutex
	MaxDelay       int64
	NodesDown      []string
	Dispatch       *SimpleHandler
	File           *FileHandler
	Adresses       *RoutingHandler
}

func NewFailureHandler(name string, net *SimpleHandler, file *FileHandler, addresses *RoutingHandler) *FailureHandler {
	return &FailureHandler{
		Name:           name,
		Nodes:          make(map[string]*OnlineMessage),
		NodesLock:      &sync.RWMutex{},
		NodesDelay:     make(map[string]int64),
		NodesDelayLock: &sync.RWMutex{},
		MaxDelay:       0,
		NodesDown:      make([]string, 0),
		Dispatch:       net,
		File:           file,
		Adresses:       addresses,
	}
}

func (b *FailureHandler) RunFailureHandler() {
	for {
		msg := b.createOnlineMsg()
		b.Dispatch.BroadcastMessage(msg, nil)
		time.Sleep(time.Second * 5)
	}
}

func (b *FailureHandler) createOnlineMsg() *OnlineMessage {
	filesInfo := b.File.SharedFiles()
	filesOwned := make([]SHA256_HASH, 0)
	for _, f := range filesInfo {
		filesOwned = append(filesOwned, f.Hash)
	}
	return &OnlineMessage{Name: b.Name,
		Hosting:   filesOwned,
		TimeStamp: time.Now().Unix(),
		HopLimit:  20}
}

func (b *FailureHandler) HandleOnlineMessage(msg *OnlineMessage) {
	b.updateTables(msg)
}

func (b *FailureHandler) HandleRequestReplica(msg *RequestHasReplica) {
	if b.File.ReplicatesFile(msg.FileHash) {
		ans := AnswerReplicaFile{
			Origin:   b.Name,
			Dest:     msg.HostName,
			FileHash: msg.FileHash,
			HopLimit: 20,
		}
		b.Adresses.SendPacketTowards(&ans, msg.HostName)
	}
}

func (b *FailureHandler) updateTables(msg *OnlineMessage) {
	b.NodesLock.Lock()
	defer b.NodesLock.Unlock()

	b.NodesDelayLock.Lock()
	defer b.NodesDelayLock.Unlock()

	_, present := b.Nodes[msg.Name]

	b.Nodes[msg.Name] = msg

	if !present {
		b.NodesDelay[msg.Name] = time.Now().Unix() - msg.TimeStamp
		if time.Now().Unix()-msg.TimeStamp > b.MaxDelay {
			b.MaxDelay = time.Now().Unix() - msg.TimeStamp
		}
		go b.detectFailure(msg.Name)
	}
}

func (b *FailureHandler) detectFailure(name string) {
	for {
		b.NodesLock.RLock()

		b.NodesDelayLock.RLock()

		lastTime, present := b.Nodes[name]
		if !present {
			panic("the node disapeared from our table")
		}
		delay, present := b.NodesDelay[name]
		if !present {
			panic("the node disapeared from our table")
		}
		b.NodesLock.RUnlock()
		b.NodesDelayLock.RUnlock()
		//the frequency of onlineMessage is 5 seconds so we can miss one message before considering the node is offline
		if time.Now().Unix()-lastTime.TimeStamp < delay*2+420 {
			time.Sleep(time.Second * 5)
		} else {
			b.NodesDown = append(b.NodesDown, name)
			b.checkIfAHost(name)
			break
		}
	}
}

func (b *FailureHandler) checkIfAHost(name string) {
	b.NodesLock.RLock()
	defer b.NodesLock.RUnlock()

	node, present := b.Nodes[name]
	if !present {
		panic("the node disapeared from our table")
	}
	if len(node.Hosting) > 0 {
		for _, file := range node.Hosting {
			if b.File.ReplicatesFile(file) {
				go b.File.BecomeTheHost(file)
			}
		}
	}
}
