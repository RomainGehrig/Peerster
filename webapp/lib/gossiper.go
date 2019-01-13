package lib

import (
	// "github.com/gorilla/handlers"
	"fmt"
	"net"

	. "github.com/RomainGehrig/Peerster/constants"
	. "github.com/RomainGehrig/Peerster/messages"
	"github.com/dedis/protobuf"
)

type Configuration struct {
	UIPort int
}

var Config Configuration = Configuration{}

func (c *Configuration) SetGUIPort(newPort int) {
	c.UIPort = newPort
}

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
	conn, err := net.Dial("udp4", fmt.Sprintf(":%d", Config.UIPort))
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

func getDestinations() []string {
	toSend := &Request{Get: &GetRequest{Type: DestinationsQuery}}
	return reqToResp(toSend).Destinations
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

func postNewRedundancyFactor(hash SHA256_HASH, newFactor int) {
	fmt.Printf("New redundancy factor. For %x: new factor is %d\n", hash, newFactor)
	toSend := &Request{Post: &PostRequest{FileRedundancyFactor: &FileRedundancyFactor{Hash: hash, Factor: newFactor}}}
	sendQuery(toSend).Close()
}

func postSearchRequest(keywords []string) {
	toSend := &Request{Post: &PostRequest{FileSearch: &FileSearch{Keywords: keywords}}}
	sendQuery(toSend).Close()
}

func getLastSearchRequest() *FileSearchResult {
	toSend := &Request{Get: &GetRequest{Type: FileSearchResultQuery}}
	return reqToResp(toSend).FileSearchResult
}

func postNewPrivateMessage(text string, dest string) {
	toSend := &Request{Post: &PostRequest{Message: &Message{Text: text, Dest: dest}}}
	sendQuery(toSend).Close()
}

func postFileName(filename string) {
	toSend := &Request{Post: &PostRequest{FileIndex: &FileIndex{Filename: filename}}}
	sendQuery(toSend).Close()
}

func postFileDownloadRequest(destination string, hash SHA256_HASH, filename string) {
	toSend := &Request{Post: &PostRequest{FileDownload: &FileDownload{destination, FileInfo{Hash: hash, Filename: filename}}}}
	sendQuery(toSend).Close()
}

func getAllDead() []string {
	toSend := &Request{Get: &GetRequest{Type: TimedOutQuery}}
	return reqToResp(toSend).Nodes
}

func addNewNode(addr string) {
	toSend := &Request{Post: &PostRequest{Node: &Node{Addr: addr}}}
	sendQuery(toSend).Close()
}

func getReputation() map[string]int64 {
	toSend := &Request{Get: &GetRequest{Type: ReputationQuery}}
	return reqToResp(toSend).Reputations
}
