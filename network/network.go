package network

import (
	"fmt"
	"github.com/RomainGehrig/Peerster/interfaces"
	. "github.com/RomainGehrig/Peerster/messages"
	. "github.com/RomainGehrig/Peerster/peers"
	"github.com/dedis/protobuf"
	"net"
)

type NetworkHandler struct {
	address     *net.UDPAddr
	conn        *net.UDPConn
	uiAddress   *net.UDPAddr
	uiConn      *net.UDPConn
	tmpGossiper interfaces.GossiperLike
	sendChannel chan<- *wrappedGossipPacket
}

const CHANNEL_BUFFERSIZE int = 1024

// TODO delete
func (n *NetworkHandler) GetAddress() string {
	return n.address.String()
}

func NewNetworkHandler(uiPort string, gossipAddr string) *NetworkHandler {
	udpAddr, err := net.ResolveUDPAddr("udp4", gossipAddr)
	if err != nil {
		panic(fmt.Sprintln("Error when creating udpAddr", err))
	}
	udpConn, err := net.ListenUDP("udp4", udpAddr)
	if err != nil {
		panic(fmt.Sprintln("Error when creating udpConn", err))
	}

	uiAddr := fmt.Sprintf("127.0.0.1:%s", uiPort)
	udpUIAddr, err := net.ResolveUDPAddr("udp4", uiAddr)
	if err != nil {
		panic(fmt.Sprintln("Error when creating udpUIAddr", err))
	}
	udpUIConn, err := net.ListenUDP("udp4", udpUIAddr)
	if err != nil {
		panic(fmt.Sprintln("Error when creating udpUIConn", err))
	}

	return &NetworkHandler{
		address:   udpAddr,
		conn:      udpConn,
		uiAddress: udpUIAddr,
		uiConn:    udpUIConn,
	}
}

func (n *NetworkHandler) RunNetworkHandler(goss interfaces.GossiperLike) {
	peerChan := n.peerMessagesReceiver()
	clientChan := n.clientMessagesReceiver()
	n.tmpGossiper = goss

	sendChan := make(chan *wrappedGossipPacket, CHANNEL_BUFFERSIZE)
	n.sendChannel = sendChan

	go n.listenForMessages(peerChan, clientChan, sendChan)
}

func (n *NetworkHandler) SendClientResponse(resp *Response, peerAddr PeerAddress) {
	packetBytes, err := protobuf.Encode(resp)
	if err != nil {
		fmt.Println(err)
	}
	_, err = n.uiConn.WriteToUDP(packetBytes, peerAddr.ToUDPAddr())
	if err != nil {
		fmt.Println(err)
	}
}

func (n *NetworkHandler) listenForMessages(peerMsgs <-chan *wrappedGossipPacket, clientRequests <-chan *wrappedClientRequest, messagesFromModules <-chan *wrappedGossipPacket) {
	for {
		select {
		case wgp := <-peerMsgs:
			gp := wgp.gossipMsg
			sender := wgp.sender
			n.tmpGossiper.DispatchPacket(gp, sender)
			// TODO Print peers was changed
			// g.peers.PrintPeers()
		case cliReq := <-clientRequests:
			req := cliReq.request
			sender := cliReq.sender
			n.tmpGossiper.DispatchClientRequest(req, sender)
		case wgp := <-messagesFromModules:
			gp := wgp.gossipMsg
			peerAddr := wgp.sender
			packetBytes, err := protobuf.Encode(gp)
			if err != nil {
				fmt.Println(err)
			}
			_, err = n.conn.WriteToUDP(packetBytes, peerAddr.ToUDPAddr())
			if err != nil {
				fmt.Println(err)
			}
		}
	}
}

// Manage receiving and sending messages
func (n *NetworkHandler) SendGossipPacket(tgp ToGossipPacket, peerAddr PeerAddress) {
	n.sendChannel <- &wrappedGossipPacket{sender: peerAddr, gossipMsg: tgp.ToGossipPacket()}
}

func (n *NetworkHandler) clientMessagesReceiver() <-chan *wrappedClientRequest {
	out := make(chan *wrappedClientRequest)
	receivedMsgs := messageReceiver(n.uiConn)
	go func() {
		for {
			var request Request
			receivedMsg := <-receivedMsgs
			protobuf.Decode(receivedMsg.packetBytes, &request)
			out <- &wrappedClientRequest{request: &request, sender: receivedMsg.sender}
		}
	}()
	return out
}

func (n *NetworkHandler) peerMessagesReceiver() <-chan *wrappedGossipPacket {
	out := make(chan *wrappedGossipPacket, CHANNEL_BUFFERSIZE)
	receivedMsgs := messageReceiver(n.conn)
	go func() {
		for {
			var packet GossipPacket
			receivedMsg := <-receivedMsgs
			protobuf.Decode(receivedMsg.packetBytes, &packet)
			out <- &wrappedGossipPacket{&packet, receivedMsg.sender}
		}
	}()
	return out
}

func messageReceiver(conn *net.UDPConn) <-chan *receivedMessage {
	out := make(chan *receivedMessage, CHANNEL_BUFFERSIZE)
	go func() {
		for {
			packetBytes := make([]byte, CHANNEL_BUFFERSIZE)
			_, sender, _ := conn.ReadFromUDP(packetBytes)
			out <- &receivedMessage{packetBytes, UDPAddress{sender}}
		}
	}()
	return out
}
