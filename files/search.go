package files

import (
	"fmt"
	. "github.com/RomainGehrig/Peerster/constants"
	. "github.com/RomainGehrig/Peerster/messages"
	. "github.com/RomainGehrig/Peerster/peers"
	"strings"
	"time"
)

const MINIMUM_EXPONENTIAL_BUDGET = 2
const MAXIMUM_EXPONENTIAL_BUDGET = 32
const EXPONENTIAL_SEARCH_TIMEOUT = 1 * time.Second
const FULL_MATCHES_NEEDED_TO_STOP_SEARCH = 2

type Query struct {
	id        uint32
	keywords  []string
	replyChan chan<- *SearchReply
	results   []*File
}

func (q *Query) isCompleted() bool {
	return len(q.results) >= FULL_MATCHES_NEEDED_TO_STOP_SEARCH
}

func (q *Query) isMatchedByResult(sres *SearchResult) bool {
	for _, kw := range q.keywords {
		if strings.Contains(sres.FileName, kw) {
			return true
		}
	}

	return false
}

func (f *FileHandler) HandleSearchReply(srep *SearchReply) {
	// TODO Should not block !
	go func() {
		// Got a reply: it's either for us or we need to forward it
		if srep.Destination == f.name {
			f.srepDispatcher.srepChannel <- srep
		} else {
			if f.prepareSearchReply(srep) {
				f.routing.SendPacketTowards(srep, srep.Destination)
			}
		}
	}()
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

func (f *FileHandler) HandleSearchRequest(sreq *SearchRequest, sender ...PeerAddress) {
	go f.answerSearchRequest(sreq, sender...)
}

func (f *FileHandler) answerSearchRequest(sreq *SearchRequest, sender ...PeerAddress) {
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
	neighbors := f.peers.PickRandomNeighbors(int(newBudget), sender...)
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

func (f *FileHandler) addSearchResult(sres *SearchResult) (fileComplete bool) {
	// TODO Add result by hash: all chunks for this result hash should be updated
	// If the addition of those chunks completes the file, return true
	// TODO Locks !
	panic("not implemented")
}

func (f *FileHandler) newQueryWatcher(keywords []string) *Query {
	replyChan := make(chan *SearchReply)
	query := &Query{
		keywords:  keywords,
		replyChan: replyChan,
	}

	// registerQuery sets the id of the query
	f.registerQuery(query)

	go func() {

		defer f.unregisterQuery(query)

		for {
			rep := <-replyChan

			for _, result := range rep.Results {
				if query.isMatchedByResult(result) {
					chunks := make([]string, len(result.ChunkMap))
					for i, chunk := range result.ChunkMap {
						chunks[i] = fmt.Sprintf("%d", chunk)
					}
					fmt.Printf("FOUND match %s at %s metafile=%x chunks=%s\n", result.FileName, rep.Origin, result.MetafileHash, strings.Join(chunks, ","))
					if f.addSearchResult(result) && query.isCompleted() {
						fmt.Println("SEARCH FINISHED")
						return
					}
				}
			}
		}
	}()

	return query
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
	// TODO: setup the parts to receive SearchReply corresponding to this
	fmt.Println("New search for keywords", keywords)
	query := f.newQueryWatcher(keywords)

	if budget != 0 {
		fmt.Println("Normal search with budget", budget)
		// TODO: just start a search with SearchRequest if budget is fixed
		sr := &SearchRequest{
			Origin:   f.name,
			Budget:   budget,
			Keywords: keywords,
		}
		f.HandleSearchRequest(sr)

	} else {
		// TODO: if budget == 0 => start exponential search
		budget = MINIMUM_EXPONENTIAL_BUDGET

		// Careful to correctly send a query with a maximum of MAXIMUM_EXPONENTIAL_BUDGET
		for !(query.isCompleted() || budget > MAXIMUM_EXPONENTIAL_BUDGET) {
			fmt.Println("Exponential budget:", budget)
			sr := &SearchRequest{
				Origin:   f.name,
				Budget:   budget,
				Keywords: keywords,
			}
			f.HandleSearchRequest(sr)

			time.Sleep(EXPONENTIAL_SEARCH_TIMEOUT)
			budget *= 2
		}
	}
}
