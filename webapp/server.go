package main

import (
	// "github.com/gorilla/handlers"
	"encoding/json"
	"fmt"
	. "github.com/RomainGehrig/Peerster/messages"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"time"

	"github.com/dedis/protobuf"
	"net"
)

func waitForResponse(conn net.Conn) *Response {
	packetBytes := make([]byte, 1024)

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

func main() {
	r := mux.NewRouter()

	// r.Handle("/admin", handlers.LoggingHandler(os.Stdout, http.HandlerFunc(ShowAdminDashboard)))
	r.HandleFunc("/message", MessageHandler).Methods("GET", "POST")
	r.HandleFunc("/node", NodeHandler).Methods("GET", "POST")
	r.HandleFunc("/id", IdHandler).Methods("GET")

	r.PathPrefix("/").Handler(http.StripPrefix("/", http.FileServer(http.Dir("static/"))))

	srv := &http.Server{
		Handler: r,
		Addr:    "127.0.0.1:8080",
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}
