package main

import (
	"flag"
	"fmt"
	. "github.com/RomainGehrig/Peerster/gossiper"
	"os"
	"strings"
)

func main() {
	flag.Usage = func() {
		fmt.Printf("Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
	}

	var uiPort = flag.String("UIPort", "8080", "port for the UI client")
	var gossipAddr = flag.String("gossipAddr", "127.0.0.1:5000", "ip:addr for the gossiper")
	var name = flag.String("name", "", "name of the gossiper")
	var peersList = flag.String("peers", "", "comma separated list of peers of the form ip:port")
	var simple = flag.Bool("simple", false, "run gossiper in simple broadcast mode")

	flag.Parse()

	var peers = strings.Split(*peersList, ",")

	gossiper := NewGossiper(*uiPort, *gossipAddr, *name, peers, *simple)

	fmt.Printf("Node starting\n")
	go gossiper.ListenForClientMessages()
	gossiper.ListenForNodeMessages()
}
