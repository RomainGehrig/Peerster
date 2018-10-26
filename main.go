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

	uiPort := flag.String("UIPort", "8080", "port for the UI client")
	gossipAddr := flag.String("gossipAddr", "127.0.0.1:5000", "ip:addr for the gossiper")
	name := flag.String("name", "", "name of the gossiper")
	peersList := flag.String("peers", "", "comma separated list of peers of the form ip:port")
	rtimer := flag.Int("rtimer", 0, "route rumors sending period in seconds, 0 to disable sending of route rumors (default 0)")
	simple := flag.Bool("simple", false, "run gossiper in simple broadcast mode")

	flag.Parse()

	// Read the list of peers from the string of peers
	var peers []string
	if *peersList != "" {
		peers = strings.Split(*peersList, ",")
	} else {
		peers = nil
	}

	gossiper := NewGossiper(*uiPort, *gossipAddr, *name, peers, *rtimer, *simple)

	gossiper.Run()
}
