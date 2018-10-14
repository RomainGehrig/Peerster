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

func getMessages() []RumorMessage {
	toSend := &Request{Get: &GetRequest{Type: MessageQuery}}
	packetBytes, err := protobuf.Encode(toSend)
	if err != nil {
		fmt.Println(err)
	}
	conn, err := net.Dial("udp4", ":8080")
	if err != nil {
		fmt.Println(err)
	}
	conn.Write(packetBytes)

	resp := waitForResponse(conn)
	return resp.Rumors
}

func NodeHandler(w http.ResponseWriter, r *http.Request) {

}

func IdHandler(w http.ResponseWriter, r *http.Request) {

}

func MessageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		json.NewEncoder(w).Encode(getMessages())
	} else if r.Method == "POST" {
		json.NewEncoder(w).Encode("POST")
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
