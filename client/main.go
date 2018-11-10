package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	. "github.com/RomainGehrig/Peerster/messages"
	. "github.com/RomainGehrig/Peerster/utils"
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
	dest := flag.String("dest", "", "destination for the private message")
	file := flag.String("file", "", "file to be indexed by the gossiper, or filename of the requested file")
	msg := flag.String("msg", "", "message to be sent")
	request := flag.String("request", "", "request a chunk or metafile of this hash")

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
	case *file != "" && *request != "" && *dest != "":
		decoded, err := hex.DecodeString(*request)
		if err != nil {
			fmt.Println("Couldn't decode -request argument...")
			os.Exit(1)
		}
		hash, err := ToHash(decoded)
		postReq.FileDownload = &FileDownload{
			Filename:    *file,
			Hash:        hash,
			Destination: *dest}
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
