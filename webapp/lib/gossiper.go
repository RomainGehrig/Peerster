package lib

import (
	// "github.com/gorilla/handlers"
	"fmt"
	. "github.com/RomainGehrig/Peerster/constants"
	. "github.com/RomainGehrig/Peerster/messages"
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

func getSharedFiles() []FileInfo {
	toSend := &Request{Get: &GetRequest{Type: SharedFilesQuery}}
	return reqToResp(toSend).Files
}

func postNewMessage(text string) {
	toSend := &Request{Post: &PostRequest{Message: &Message{Text: text}}}
	sendQuery(toSend).Close()
}

func postNewPrivateMessage(text string, dest string) {
	toSend := &Request{Post: &PostRequest{Message: &Message{Text: text, Dest: dest}}}
	sendQuery(toSend).Close()
}

func postFileName(filename string) {
	toSend := &Request{Post: &PostRequest{FileIndex: &FileIndex{Filename: filename}}}
	sendQuery(toSend).Close()
}

func addNewNode(addr string) {
	toSend := &Request{Post: &PostRequest{Node: &Node{Addr: addr}}}
	sendQuery(toSend).Close()
}
