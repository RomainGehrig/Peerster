package files

import (
	. "github.com/RomainGehrig/Peerster/constants"
	. "github.com/RomainGehrig/Peerster/messages"
	. "github.com/RomainGehrig/Peerster/peers"
	"strings"
	"time"
)

const MAXIMUM_EXPONENTIAL_BUDGET = 32
const EXPONENTIAL_SEARCH_TIMEOUT = 1 * time.Second
const FULL_MATCHES_NEEDED_TO_STOP_SEARCH = 2

func (f *FileHandler) HandleSearchReply(srep *SearchReply) {
	// TODO Should not block !
	go func() {
		// Got a reply: it's either for us (TODO) or needs to forward it
		if srep.Destination == f.name {
			// TODO
			panic("Received a message for us: not implemented !")
		} else {
			if valid := f.prepareSearchReply(srep); valid {
				f.routing.SendPacketTowards(srep, srep.Destination)
			}
		}
	}()
}

func (f *FileHandler) HandleSearchRequest(sreq *SearchRequest, sender PeerAddress) {
	// TODO Should not block !
	go f.answerSearchRequest(sreq, sender)
}

// Modifies in-place the search reply
func (f *FileHandler) prepareSearchReply(srep *SearchReply) bool {
	if srep.HopLimit <= 1 {
		srep.HopLimit = 0
		return false
	}

	srep.HopLimit -= 1
	return true
}

func (f *FileHandler) answerSearchRequest(sreq *SearchRequest, sender PeerAddress) {
	// If search matches some local files (ie. downloading or sharing), send back a reply
	matches := make([]*SearchResult, 0)

	// TODO LOCKS
	for _, file := range f.files {
		// We cannot provide chunks if we don't have any
		if file.State == DownloadingMetafile || file.State == Failed {
			continue
		}

		for _, kw := range sreq.Keywords {
			if strings.Contains(file.Name, kw) {
				matches = append(matches, &SearchResult{
					FileName:     file.Name,
					MetafileHash: file.MetafileHash[:],
					ChunkMap:     f.chunkMap(file),
					ChunkCount:   file.chunkCount,
				})
				break
			}
		}
	}

	// Send SearchReply if we have answers
	if len(matches) > 0 {
		sr := &SearchReply{
			Origin:      f.name,
			Destination: sreq.Origin,
			HopLimit:    DEFAULT_HOP_LIMIT,
			Results:     matches,
		}

		f.routing.SendPacketTowards(sr, sr.Destination)
	}

	// No forwarding if there is no budget
	if sreq.Budget <= 1 {
		return
	}

	// Update budget
	newBudget := sreq.Budget - 1

	// Forward the packet elsewhere
	neighbors := f.peers.PickRandomNeighbors(int(newBudget), sender)
	if len(neighbors) == 0 {
		return
	}

	// Number of neighbors to have an additional budget point
	additional := newBudget % uint64(len(neighbors))
	part := newBudget / uint64(len(neighbors))

	// Distribute budget evenly
	for i, neighbor := range neighbors {
		budget := part
		if uint64(i) < additional {
			budget += 1
		}

		nreq := &SearchRequest{
			Origin:   sreq.Origin,
			Budget:   budget,
			Keywords: sreq.Keywords,
		}

		// Send packet
		f.net.SendGossipPacket(nreq, neighbor)
	}
}

func (f *FileHandler) chunkMap(file *File) []uint64 {
	chunks := make([]uint64, 0)
	for _, hash := range MetaFileToHashes(file.metafile) {
		// TODO LOCKS
		if chunk := f.chunks[hash]; chunk.HasData {
			chunks = append(chunks, chunk.Number)
		}
	}
	return chunks
}

func (f *FileHandler) StartSearch(keywords []string, budget uint64) {
	// TODO: if budget == 0 => start exponential search
	if budget == 0 {
		panic("Exponential search not implemented yet")
	} else {
		// TODO: else: just start a search with SearchRequest
		// sr := &SearchRequest{
		// 	Origin:   f.name,
		// 	Budget:   budget,
		// 	Keywords: keywords,
		// }

		// TODO: setup the parts to receive SearchReply corresponding to this
	}
}
