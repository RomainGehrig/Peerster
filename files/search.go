package files

import (
	"fmt"
	. "github.com/RomainGehrig/Peerster/constants"
	. "github.com/RomainGehrig/Peerster/messages"
	. "github.com/RomainGehrig/Peerster/peers"
	. "github.com/RomainGehrig/Peerster/utils"
	"strings"
	"time"
)

const MINIMUM_EXPONENTIAL_BUDGET = 2
const MAXIMUM_EXPONENTIAL_BUDGET = 32
const EXPONENTIAL_SEARCH_TIMEOUT = 1 * time.Second
const SEARCH_REQUEST_IGNORE_TIMEOUT = 500 * time.Millisecond
const FULL_MATCHES_NEEDED_TO_STOP_SEARCH = 2

type Query struct {
	id        uint32
	keywords  []string
	replyChan chan<- *SearchReply
	results   []*FileInfo
}

type SeenRequest struct {
	Origin    string
	Keywords  []string
	Timestamp time.Time
}

func (sf *SearchedFile) isComplete() bool {
	return sf.maxChunk == uint64(len(sf.chunks))
}

// Update maxChunk to its max value such that every previous chunk (incl. maxChunk) has an owner
func (sf *SearchedFile) updateMaxChunk() {
	for sf.maxChunk < uint64(len(sf.chunks)) && sf.chunks[sf.maxChunk] != "" {
		sf.maxChunk += 1
	}
}

func (q *Query) isCompleted() bool {
	return len(q.results) >= FULL_MATCHES_NEEDED_TO_STOP_SEARCH
}

func (q *Query) addResult(sres *SearchResult) {
	hash, _ := ToHash(sres.MetafileHash)
	newFile := &FileInfo{
		Filename: sres.FileName,
		Hash:     hash,
		Size:     int64(sres.ChunkCount * CHUNK_SIZE), // Approx of the size
	}

	// Check that we don't double add the same result
	// Uniqueness is determined by fileName & hash
	for _, file := range q.results {
		// Stop if non-unique
		if file.Filename == newFile.Filename && file.Hash == newFile.Hash {
			return
		}
	}

	// Unique => add result
	q.results = append(q.results, newFile)
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

func (f *FileHandler) tidySeenRequests() {
	now := time.Now()
	newSeen := f.seenRequests[:0]
	for _, seenReq := range f.seenRequests {
		// Seen request is still not timeouted
		if now.Sub(seenReq.Timestamp) < SEARCH_REQUEST_IGNORE_TIMEOUT {
			newSeen = append(newSeen, seenReq)
		}
	}
	f.seenRequests = newSeen
}

func (f *FileHandler) searchRequestShouldBeIgnored(newReq SeenRequest) bool {
	now := time.Now()
	for _, req := range f.seenRequests {
		if req.Origin == newReq.Origin && now.Sub(req.Timestamp) < SEARCH_REQUEST_IGNORE_TIMEOUT {
			// Not the same keywords
			if len(req.Keywords) != len(newReq.Keywords) {
				return false
			}

			// check if req.Keywords == newReq.Keywords
			for i, kw := range req.Keywords {
				if kw != newReq.Keywords[i] {
					return false
				}
			}

			// Same queries
			return true
		}
	}
	return false
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

func (f *FileHandler) LastQueryResults() *FileSearchResult {
	// No query, yet
	if len(f.queries) == 0 {
		return nil
	}

	lastQuery := f.queries[len(f.queries)-1]
	return &FileSearchResult{
		Keywords: lastQuery.keywords,
		Files:    lastQuery.results,
	}
}

func (f *FileHandler) HandleSearchRequest(sreq *SearchRequest, sender ...PeerAddress) {
	defer f.tidySeenRequests()
	seenReq := SeenRequest{
		Origin:    sreq.Origin,
		Keywords:  sreq.Keywords[:],
		Timestamp: time.Now(),
	}

	if f.searchRequestShouldBeIgnored(seenReq) {
		return
	}

	f.seenRequests = append(f.seenRequests, seenReq)

	go f.answerSearchRequest(sreq, sender...)

}

func (f *FileHandler) RequestSearchedFileDownload(metafileHash SHA256_HASH, localName string) {
	// Should not block
	go func() {
		f.searchedLock.RLock()
		file, present := f.searchedFiles[metafileHash]
		f.searchedLock.RUnlock()

		if !present || !file.isComplete() {
			fmt.Println("Cannot download a file that was not search (and matched) before")
			return
		}

		metafileOwner := file.chunks[0]
		resolver := func(chunkNumber uint64) string {
			return file.chunks[chunkNumber-1]
		}
		f.downloadFile(metafileHash, metafileOwner, localName, resolver)
	}()
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

		// Don't send a packet if we are the target
		if sreq.Origin == f.name {
			f.HandleSearchReply(sr)
		} else {
			f.routing.SendPacketTowards(sr, sr.Destination)
		}
	}

	// No forwarding if there is no budget
	if sreq.Budget <= 1 {
		return
	}

	f.forwardSearchRequest(sreq, sreq.Budget-1, sender...)
}

func (f *FileHandler) forwardSearchRequest(sreq *SearchRequest, budget uint64, excluded ...PeerAddress) {
	// Forward the packet elsewhere
	neighbors := f.peers.PickRandomNeighbors(int(budget), excluded...)
	if len(neighbors) == 0 {
		return
	}

	// Number of neighbors to have an additional budget point
	additional := budget % uint64(len(neighbors))
	part := budget / uint64(len(neighbors))

	// Distribute budget evenly
	for i, neighbor := range neighbors {
		peerBudget := part
		if uint64(i) < additional {
			peerBudget += 1
		}

		nreq := &SearchRequest{
			Origin:   sreq.Origin,
			Budget:   peerBudget,
			Keywords: sreq.Keywords,
		}

		// Send packet
		f.net.SendGossipPacket(nreq, neighbor)
	}

}

func (f *FileHandler) addSearchResult(sres *SearchResult, origin string) (fileComplete bool) {
	// TODO Add result by hash: all chunks for this result hash should be updated

	hash, _ := ToHash(sres.MetafileHash)

	// Lock everything so we don't run into issues where two goroutines add files/chunks at the same time
	f.searchedLock.Lock()
	defer f.searchedLock.Unlock()

	file, present := f.searchedFiles[hash]
	if !present {
		file = &SearchedFile{
			firstPeer: origin,
			maxChunk:  0,
			chunks:    make([]string, sres.ChunkCount),
		}

		f.searchedFiles[hash] = file
	}

	for _, chunk := range sres.ChunkMap {
		// TODO Eviction of old chunks/new chunks: do it at random ?
		file.chunks[chunk-1] = origin

		// Update maxChunk if we have a chunk that fills the "hole"
		if chunk == file.maxChunk+1 {
			file.maxChunk = chunk

			file.updateMaxChunk()
		}
	}

	// Return true if the file is complete. Important -> means that when we
	// receive multiple times the same answer (due to expanding ring) we return
	// true each time (if file is completed the first time)
	return file.isComplete()
}

func (f *FileHandler) newQueryWatcher(keywords []string) *Query {
	replyChan := make(chan *SearchReply)
	query := &Query{
		keywords:  keywords,
		replyChan: replyChan,
	}

	f.queries = append(f.queries, query)

	// registerQuery sets the id of the query
	f.registerQuery(query)

	go func() {

		defer f.unregisterQuery(query)

		for {
			rep := <-replyChan

			for _, result := range rep.Results {
				if query.isMatchedByResult(result) {
					// Print the chunks
					{
						chunks := make([]string, len(result.ChunkMap))
						for i, chunk := range result.ChunkMap {
							chunks[i] = fmt.Sprintf("%d", chunk)
						}
						fmt.Printf("FOUND match %s at %s metafile=%x chunks=%s\n", result.FileName, rep.Origin, result.MetafileHash, strings.Join(chunks, ","))
					}

					if f.addSearchResult(result, rep.Origin) {
						query.addResult(result)

						if query.isCompleted() {
							fmt.Println("SEARCH FINISHED")
							return
						}
					}

				}
			}
		}
	}()

	return query
}

func (f *FileHandler) chunkMap(file *LocalFile) []uint64 {
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
		// Start the request in another goroutine
		go func() {
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
		}()
	}
}
