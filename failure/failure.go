package failure

import (
	"sync"
	"time"

	. "github.com/RomainGehrig/Peerster/messages"
	. "github.com/RomainGehrig/Peerster/simple"
)

type FailureHandler struct {
	Name           string
	Nodes          map[string]*OnlineMessage
	NodesLock      *sync.RWMutex
	NodesDelay     map[string]int64
	NodesDelayLock *sync.RWMutex
	Hosting        []string
	MaxDelay       int64
	FileMap        map[string]map[uint64][]string
	FileMapLock    *sync.RWMutex
	Dispatch       *SimpleHandler
}

func NewFailureHandler(name string, net *SimpleHandler) *FailureHandler {
	return &FailureHandler{
		Name:           name,
		Nodes:          make(map[string]*OnlineMessage),
		NodesLock:      &sync.RWMutex{},
		NodesDelay:     make(map[string]int64),
		NodesDelayLock: &sync.RWMutex{},
		Hosting:        make([]string, 0),
		MaxDelay:       0,
		FileMap:        make(map[string]map[uint64][]string),
		FileMapLock:    &sync.RWMutex{},
		Dispatch:       net,
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
	return &OnlineMessage{Name: b.Name,
		//TODO
		//Hosting: the files that we host,
		TimeStamp: time.Now().Unix(),
		HopLimit:  20}
}

func (b *FailureHandler) HandleOnlineMessage(msg *OnlineMessage) {
	b.updateTables(msg)
}

func (b *FailureHandler) HandleReqChunkList(msg *RequestChunkList) {
	//TODO
	//chunks = getChunksOfFile(msg.FileHash)
	//sendBack(chunks, msg.HostName)
}

//Fill the FileMap with the answer
func (b *FailureHandler) HandleAnswer(msg *AnswerChunkList) {
	b.FileMapLock.Lock()
	defer b.FileMapLock.Unlock()

	_, present := b.FileMap[msg.FileHash]
	if !present {
		b.FileMap[msg.FileHash] = make(map[uint64][]string)
	}
	for _, chunkNumber := range msg.ChunkList {
		list, present := b.FileMap[msg.FileHash][chunkNumber]
		if present {
			if alreadyView(list, msg.Origin) {
				b.FileMap[msg.FileHash][chunkNumber] = append(list, msg.Origin)
			}
		} else {
			singleList := make([]string, 1)
			singleList[0] = msg.Origin
			b.FileMap[msg.FileHash][chunkNumber] = append(singleList, msg.Origin)
		}
	}
}

func alreadyView(list []string, origin string) bool {
	new := true
	for _, name := range list {
		if name == origin {
			new = false
		}
	}
	return new
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
		defer b.NodesLock.RUnlock()

		b.NodesDelayLock.RLock()
		defer b.NodesDelayLock.RUnlock()

		lastTime, present := b.Nodes[name]
		if !present {
			panic("the node disapeared from our table")
		}
		delay, present := b.NodesDelay[name]
		if !present {
			panic("the node disapeared from our table")
		}
		//the frequency of onlineMessage is 5 seconds so we can miss on message before considering the node is offline
		if time.Now().Unix()-lastTime.TimeStamp < delay*2+420 {
			time.Sleep(time.Second * 5)
		} else {
			b.checkIfHosting(name)
			b.checkIfItHost(name)
			break
		}
	}
}

func (b *FailureHandler) checkIfHosting(name string) {
	/*for file := range filehosted {
		for chunks := range file.chunkmap {
			for node := range chunks {
				if node == name {
					redistributedChunk(chunk)
				}
			}
		}
	}*/
}

func (b *FailureHandler) checkIfItHost(name string) {
	b.NodesLock.RLock()
	defer b.NodesLock.RUnlock()

	node, present := b.Nodes[name]
	if !present {
		panic("the node disapeared from our table")
	}
	if len(node.Hosting) > 0 {
		for _, file := range node.Hosting {
			go b.becomeTheHost(file)
		}
	}
}

func (b *FailureHandler) becomeTheHost(hash string) {
	/*dl the file
	if finished {
		put it in the blockchain
	}
	if published and we are the new host {
		hostingSetup(hash)
	}*/
}

func (b *FailureHandler) hostingSetup(hash string) {
	b.Hosting = append(b.Hosting, hash)

	req := RequestChunkList{HostName: b.Name,
		FileHash: hash,
		HopLimit: 20,
	}

	b.Dispatch.BroadcastMessage(&req, nil)

	time.Sleep(3 * time.Second * time.Duration(b.MaxDelay))
	b.createMap(hash)
}

func (b *FailureHandler) createMap(hash string) {
	//use FileMap to create a new map chunk-user
	//check redundancy and distributed missing chunk
}
