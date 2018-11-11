package main

import (
	// "github.com/gorilla/handlers"
	. "github.com/RomainGehrig/Peerster/webapp/lib"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"time"
)

func main() {
	r := mux.NewRouter()

	r.HandleFunc("/message", MessageHandler).Methods("GET", "POST")
	r.HandleFunc("/node", NodeHandler).Methods("GET", "POST")
	r.HandleFunc("/id", IdHandler).Methods("GET")
	r.HandleFunc("/destinations", DestinationsHandler).Methods("GET")
	r.HandleFunc("/pmessage", PrivateMessageHandler).Methods("GET", "POST")
	r.HandleFunc("/file", FileHandler).Methods("GET", "POST")

	r.PathPrefix("/").Handler(http.StripPrefix("/", http.FileServer(http.Dir("static/"))))

	srv := &http.Server{
		Handler:      r,
		Addr:         "127.0.0.1:8080",
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}
