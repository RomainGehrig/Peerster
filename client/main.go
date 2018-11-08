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
	flag.Usage = func() {
		fmt.Printf("Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
	}

	uiPort := flag.String("UIPort", "8080", "port for the UI client")
	dest := flag.String("Dest", "", "destination for the private message")
	file := flag.String("file", "", "file to be indexed by the gossiper")
	msg := flag.String("msg", "", "message to be sent")

	flag.Parse()

	address := fmt.Sprintf(":%s", *uiPort)

	postReq := &PostRequest{}
	toSend := &Request{Post: postReq}
	// Be careful with the order of the conditions: most restrictive first, then more general
	switch {
	case *msg != "" && *dest != "":
		postReq.Message = &Message{Text: *msg, Dest: *dest}
	case *msg != "":
		postReq.Message = &Message{Text: *msg}
	// TODO FileRequest
	case *file != "":
		postReq.FileIndex = &FileIndex{Filename: *file}
	default:
		fmt.Println("Arguments don't make sense. :(")
		flag.Usage()
		os.Exit(1)
	}

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
}
