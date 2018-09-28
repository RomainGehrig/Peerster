package main

import (
	"flag"
	"fmt"
	. "github.com/RomainGehrig/Peerster/messages"
	"github.com/dedis/protobuf"
	"net"
	"os"
)

func main() {
	Usage := func() {
		fmt.Printf("Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
	}

	uiPort := flag.String("UIPort", "8080", "port for the UI client")
	msg := flag.String("msg", "", "message to be sent")

	Usage()
	flag.Parse()

	address := fmt.Sprintf(":%s", *uiPort)

	toSend := &Message{*msg}
	packetBytes, err := protobuf.Encode(toSend)
	if err != nil {
		fmt.Println(err)
	}
	// TODO Handle err

	conn, err := net.Dial("udp4", address)
	if err != nil {
		fmt.Println(err)
	}

	conn.Write(packetBytes)

	// udpAddr, err := net.ResolveUDPAddr("udp4", address)
	// if err != nil {
	// 	fmt.Println(err)
	// }
	// udpConn, err := net.DialUDP("udp4", "", udpAddr)
	// if err != nil {
	// 	fmt.Println(err)
	// }

	// u, err := udpConn.WriteToUDP(packetBytes, udpAddr)
	// if err != nil {
	// 	fmt.Println(err)
	// }

	// fmt.Println(u)
}
