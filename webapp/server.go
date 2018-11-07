package main

import (
	// "github.com/gorilla/handlers"
	"encoding/json"
	"fmt"
	. "github.com/RomainGehrig/Peerster/constants"
	. "github.com/RomainGehrig/Peerster/messages"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"time"

	"github.com/dedis/protobuf"
	"net"
)

func waitForResponse(conn net.Conn) *Response {
	packetBytes := make([]byte, UDP_DATAGRAM_MAX_SIZE)

	_, err := conn.Read(packetBytes)
	if err != nil {
		fmt.Println(err)
	}

	var resp Response
	protobuf.Decode(packetBytes, &resp)
	return &resp
}

func sendQuery(req *Request) net.Conn {
	packetBytes, err := protobuf.Encode(req)
	if err != nil {
		fmt.Println(err)
	}
	conn, err := net.Dial("udp4", ":8080")
	if err != nil {
		fmt.Println(err)
	}

	conn.Write(packetBytes)
	return conn
}

func reqToResp(req *Request) *Response {
	conn := sendQuery(req)
	defer conn.Close()
	return waitForResponse(conn)
}

func getOrigins() []string {
	toSend := &Request{Get: &GetRequest{Type: OriginsQuery}}
	return reqToResp(toSend).Origins
}

func getPrivateMessages() []PrivateMessage {
	toSend := &Request{Get: &GetRequest{Type: PrivateMessageQuery}}
	return reqToResp(toSend).PrivateMessages
}

func getMessages() []RumorMessage {
	toSend := &Request{Get: &GetRequest{Type: MessageQuery}}
	return reqToResp(toSend).Rumors
}

func getNodes() []string {
	toSend := &Request{Get: &GetRequest{Type: NodeQuery}}
	return reqToResp(toSend).Nodes
}

func getPeerID() string {
	toSend := &Request{Get: &GetRequest{Type: PeerIDQuery}}
	return reqToResp(toSend).PeerID
}

func postNewMessage(text string) {
	toSend := &Request{Post: &PostRequest{Message: &Message{Text: text}}}
	sendQuery(toSend).Close()
}

func postNewPrivateMessage(text string, dest string) {
	toSend := &Request{Post: &PostRequest{Message: &Message{Text: text, Dest: dest}}}
	sendQuery(toSend).Close()
}

func addNewNode(addr string) {
	toSend := &Request{Post: &PostRequest{Node: &Node{Addr: addr}}}
	sendQuery(toSend).Close()
}

func ackPOST(success bool, w http.ResponseWriter) {
	var response struct {
		Success bool `json:"success"`
	}
	response.Success = success
	json.NewEncoder(w).Encode(response)
}

func NodeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		var nodes struct {
			Nodes []string `json:"nodes"`
		}
		nodes.Nodes = getNodes()
		json.NewEncoder(w).Encode(nodes)
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

func OriginsHandler(w http.ResponseWriter, r *http.Request) {
	var origins struct {
		Origins []string `json:"origins"`
	}
	origins.Origins = getOrigins()
	json.NewEncoder(w).Encode(origins)
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

func main() {
	r := mux.NewRouter()

	r.HandleFunc("/message", MessageHandler).Methods("GET", "POST")
	r.HandleFunc("/node", NodeHandler).Methods("GET", "POST")
	r.HandleFunc("/id", IdHandler).Methods("GET")
	r.HandleFunc("/origins", OriginsHandler).Methods("GET")
	r.HandleFunc("/pmessage", PrivateMessageHandler).Methods("GET", "POST")

	r.PathPrefix("/").Handler(http.StripPrefix("/", http.FileServer(http.Dir("static/"))))

	srv := &http.Server{
		Handler:      r,
		Addr:         "127.0.0.1:8080",
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}
