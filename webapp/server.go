package main

import (
	// "github.com/gorilla/handlers"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	. "github.com/RomainGehrig/Peerster/webapp/lib"
	"github.com/gorilla/mux"
)

func main() {
	r := mux.NewRouter()

	r.HandleFunc("/message", MessageHandler).Methods("GET", "POST")
	r.HandleFunc("/node", NodeHandler).Methods("GET", "POST")
	r.HandleFunc("/id", IdHandler).Methods("GET")
	r.HandleFunc("/destinations", DestinationsHandler).Methods("GET")
	r.HandleFunc("/pmessage", PrivateMessageHandler).Methods("GET", "POST")
	r.HandleFunc("/sharedFiles", SharedFileHandler).Methods("GET", "POST")
	r.HandleFunc("/files", FileRequestHandler).Methods("POST")
	r.HandleFunc("/search", SearchRequestHandler).Methods("GET", "POST")
	r.HandleFunc("/reputations", ReputationHandler).Methods("GET")

	r.PathPrefix("/").Handler(http.StripPrefix("/", http.FileServer(http.Dir("static/"))))

	flag.Usage = func() {
		fmt.Printf("Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
	}

	uiPort := flag.Int("UIPort", 8080, "Gossiper's UI port and HTTP port")
	flag.Parse()

	Config.SetGUIPort(*uiPort)

	fmt.Printf("Starting webapp on address http://127.0.0.1:%d (connecting to Gossiper on GUI port %d)\n", *uiPort, *uiPort)

	srv := &http.Server{
		Handler:      r,
		Addr:         fmt.Sprintf("127.0.0.1:%d", *uiPort),
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}
