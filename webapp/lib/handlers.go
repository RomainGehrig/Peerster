package lib

import (
	"encoding/hex"
	"encoding/json"
	"net/http"

	. "github.com/RomainGehrig/Peerster/messages"
	. "github.com/RomainGehrig/Peerster/utils"
)

func ackPOST(success bool, w http.ResponseWriter) {
	var response struct {
		Success bool `json:"success"`
	}
	response.Success = success
	json.NewEncoder(w).Encode(response)
}

func ReputationHandler(w http.ResponseWriter, r *http.Request) {
	allReputs := getReputation()
	json.NewEncoder(w).Encode(allReputs)
}

func NodeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		var nodes struct {
			Nodes []string `json:"nodes"`
		}
		nodes.Nodes = getNodes()
		allAlive := []string{}
		allDead := getAllDead()
		for _, elem := range nodes.Nodes {
			alive := true
			for _, dead := range allDead {
				if dead == elem {
					alive = false
					break
				}
			}
			if alive {
				allAlive = append(allAlive, elem)
			}
		}

		json.NewEncoder(w).Encode(allAlive)
	} else if r.Method == "POST" {
		var node struct {
			Addr string `json:"addr"`
		}
		json.NewDecoder(r.Body).Decode(&node)
		addNewNode(node.Addr)
		ackPOST(true, w)
	}
}

func IdHandler(w http.ResponseWriter, r *http.Request) {
	var peerID struct {
		PeerID string `json:"id"`
	}
	peerID.PeerID = getPeerID()
	json.NewEncoder(w).Encode(peerID)
}

func DestinationsHandler(w http.ResponseWriter, r *http.Request) {
	var destinations struct {
		Destinations []string `json:"destinations"`
	}
	destinations.Destinations = getDestinations()
	json.NewEncoder(w).Encode(destinations)
}

func MessageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		var messages struct {
			Messages []RumorMessage `json:"messages"`
		}
		messages.Messages = getMessages()
		json.NewEncoder(w).Encode(messages)
	} else if r.Method == "POST" {
		var message struct {
			Text string `json:"text"`
		}
		json.NewDecoder(r.Body).Decode(&message)
		postNewMessage(message.Text)
		ackPOST(true, w)
	}
}

func PrivateMessageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		origins := make(map[string][]PrivateMessage)
		// TODO Save the name so we don't need to request it every time ?
		selfName := getPeerID()
		for _, msg := range getPrivateMessages() {

			// We want to group the messages we send to a destination
			// with the messages we received from it (the "nodeOfInterest")
			var nodeOfInterest string
			// If we sent the message, the destination is the one that is important
			if selfName == msg.Origin {
				nodeOfInterest = msg.Destination
			} else {
				nodeOfInterest = msg.Origin
			}

			lst, present := origins[nodeOfInterest]
			if !present {
				lst = make([]PrivateMessage, 0)
			}
			origins[nodeOfInterest] = append(lst, msg)
		}
		json.NewEncoder(w).Encode(origins)
	} else if r.Method == "POST" {
		var message struct {
			Text string `json:"text"`
			Dest string `json:"dest"`
		}
		json.NewDecoder(r.Body).Decode(&message)
		postNewPrivateMessage(message.Text, message.Dest)
		ackPOST(true, w)
	}
}

func RedundancyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		var redundancyFactor struct {
			Hash   string `json:"hash"`
			Factor int    `json:"factor"`
		}
		json.NewDecoder(r.Body).Decode(&redundancyFactor)

		decoded, err := hex.DecodeString(redundancyFactor.Hash)
		if err != nil {
			ackPOST(false, w)
		} else if hash, err := ToHash(decoded); err == nil {
			postNewRedundancyFactor(hash, redundancyFactor.Factor)
			ackPOST(true, w)
		} else {
			ackPOST(false, w)
		}
	}
}

func SharedFileHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		var files struct {
			Files []FileInfo `json:"files"`
		}
		files.Files = getSharedFiles()
		json.NewEncoder(w).Encode(files)
	} else if r.Method == "POST" {
		var filename struct {
			Filename string `json:"filename"`
		}
		json.NewDecoder(r.Body).Decode(&filename)
		postFileName(filename.Filename)
		ackPOST(true, w)
	}
}

func FileRequestHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		var fileinfo struct {
			Filename    string `json:"filename"`
			Hash        string `json:"hash"`
			Destination string `json:"destination"`
		}

		json.NewDecoder(r.Body).Decode(&fileinfo)

		decoded, err := hex.DecodeString(fileinfo.Hash)
		if err != nil {
			ackPOST(false, w)
		} else if hash, err := ToHash(decoded); err == nil {
			postFileDownloadRequest(fileinfo.Destination, hash, fileinfo.Filename)
			ackPOST(true, w)
		} else {
			ackPOST(false, w)
		}
	}
}

func SearchRequestHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		fileSearchResult := getLastSearchRequest()
		json.NewEncoder(w).Encode(fileSearchResult)
	} else if r.Method == "POST" {
		var request struct {
			Keywords []string `json:"keywords"`
		}
		json.NewDecoder(r.Body).Decode(&request)
		postSearchRequest(request.Keywords)
		ackPOST(true, w)
	}
}
