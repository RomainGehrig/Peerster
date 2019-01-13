package gossiper

import (
	"fmt"

	. "github.com/RomainGehrig/Peerster/messages"
	. "github.com/RomainGehrig/Peerster/peers"
)

func (g *Gossiper) DispatchClientRequest(req *Request, sender PeerAddress) {
	switch {
	case req.Get != nil:
		var resp Response
		resp.Type = req.Get.Type

		switch req.Get.Type {
		case NodeQuery:
			nodes := g.peers.AllPeersStr()
			resp.Nodes = nodes
		case MessageQuery:
			rumors := g.rumors.NonEmptyRumors()
			resp.Rumors = rumors
		case PeerIDQuery:
			resp.PeerID = g.Name
		case DestinationsQuery:
			resp.Destinations = g.routing.KnownDestinations()
		case PrivateMessageQuery:
			resp.PrivateMessages = g.private.GetPrivateMessages()
		case SharedFilesQuery:
			resp.Files = g.files.SharedFiles()
		case FileSearchResultQuery:
			resp.FileSearchResult = g.files.LastQueryResults()
		case ReputationQuery:
			resp.Reputations = g.reputation.AllReputations
		case TimedOutQuery:
			resp.Nodes = g.failure.NodesDown
		}
		g.net.SendClientResponse(&resp, sender)
	case req.Post != nil:
		post := req.Post
		switch {
		case post.Node != nil:
			g.peers.AddPeer(ResolvePeerAddress(post.Node.Addr))
			g.peers.PrintPeers() // TODO Do we print the new peers here ?
		case post.Message != nil:
			g.HandleClientMessage(post.Message)
			g.peers.PrintPeers()
		case post.FileIndex != nil:
			g.files.RequestFileIndexing(post.FileIndex.Filename)
		case post.FileDownload != nil:
			dl := post.FileDownload
			if dl.Destination != "" {
				g.files.RequestFileDownload(dl.Destination, dl.Hash, dl.Filename)
			} else {
				g.files.RequestSearchedFileDownload(dl.Hash, dl.Filename)
			}
		case post.FileSearch != nil:
			fs := post.FileSearch
			g.files.StartSearch(fs.Keywords, fs.Budget)
		case post.FileRedundancyFactor != nil:
			frf := post.FileRedundancyFactor
			g.files.SetRedundancyFactor(frf.Hash, frf.Factor)
		}
	}
}

func (g *Gossiper) DispatchPacket(packet *GossipPacket, sender PeerAddress) {
	g.peers.AddPeer(sender)
	switch {
	case packet.Simple != nil:
		fmt.Println(packet.Simple)
		g.simple.HandleSimpleMessage(packet.Simple)
		g.peers.PrintPeers()
	case packet.Rumor != nil:
		fmt.Println(packet.Rumor.StringWithSender(sender))
		g.rumors.HandleRumorMessage(packet.Rumor, sender)
		g.peers.PrintPeers()
	case packet.Status != nil:
		fmt.Println(packet.Status.StringWithSender(sender))
		g.rumors.HandleStatusMessage(packet.Status, sender)
		g.peers.PrintPeers()
	case packet.Private != nil:
		if packet.Private.Destination == g.Name {
			fmt.Println(packet.Private)
		}
		g.private.HandlePrivateMessage(packet.Private)
		g.peers.PrintPeers()
	case packet.DataReply != nil:
		g.files.HandleDataReply(packet.DataReply)
	case packet.DataRequest != nil:
		g.files.HandleDataRequest(packet.DataRequest)
	case packet.SearchReply != nil:
		g.files.HandleSearchReply(packet.SearchReply)
	case packet.SearchRequest != nil:
		g.files.HandleSearchRequest(packet.SearchRequest, sender)
	case packet.TxPublish != nil:
		g.blockchain.HandleTxPublish(packet.TxPublish)
	case packet.BlockPublish != nil:
		g.blockchain.HandleBlockPublish(packet.BlockPublish)
	case packet.OnlineMessage != nil:
		g.failure.HandleOnlineMessage(packet.OnlineMessage)
	case packet.RequestHasReplica != nil:
		g.failure.HandleRequestReplica(packet.RequestHasReplica)
	case packet.AnswerReplicaFile != nil:
		g.files.HandleAnswer(packet.AnswerReplicaFile)
	case packet.ReplicationRequest != nil:
		g.files.HandleReplicationRequest(packet.ReplicationRequest)
	case packet.ReplicationReply != nil:
		g.files.HandleReplicationReply(packet.ReplicationReply)
	case packet.ReplicationACK != nil:
		g.files.HandleReplicationACK(packet.ReplicationACK)
	case packet.ChallengeRequest != nil:
		g.files.HandleChallengeRequest(packet.ChallengeRequest)
	case packet.ChallengeReply != nil:
		g.files.HandleChallengeReply(packet.ChallengeReply)
	}
}

func (g *Gossiper) HandleClientMessage(m *Message) {
	// TODO Print in case of PrivateMessage ?
	fmt.Println(m)
	if g.simpleMode {
		msg := g.simple.CreateSimpleMessage(m.Text)
		g.simple.BroadcastMessage(msg, nil)
	} else {
		// No destination specified => message is a rumor
		if m.Dest == "" {
			msg := g.rumors.CreateClientRumor(m.Text)
			g.rumors.HandleRumorMessage(msg, nil)
		} else { // Message is a private message
			// TODO Put creation/handling directly inside of the private module
			msg := g.private.CreatePrivateMessage(m.Text, m.Dest)
			g.private.HandlePrivateMessage(msg)
		}
	}
}
