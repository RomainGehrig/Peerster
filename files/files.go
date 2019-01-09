package files

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	. "github.com/RomainGehrig/Peerster/blockchain"
	. "github.com/RomainGehrig/Peerster/constants"
	. "github.com/RomainGehrig/Peerster/messages"
	. "github.com/RomainGehrig/Peerster/network"
	. "github.com/RomainGehrig/Peerster/peers"
	. "github.com/RomainGehrig/Peerster/reputation"
	. "github.com/RomainGehrig/Peerster/routing"
	. "github.com/RomainGehrig/Peerster/utils"
)

const SHARED_DIR_NAME = "_SharedFiles"
const DOWNLOAD_DIR_NAME = "_Downloads"
const CHUNK_SIZE = 8 * 1024 // == 8192 Bytes
const NO_ANSWER_TIMEOUT = 5 * time.Second
const MAX_RETRIES = 5

type FileState int

// TODO Where are they set ? Failed is not set
const (
	Owned               FileState = iota // Owned by us
	Replica                              // Replicated on this node
	DownloadingMetafile                  // First phase of the download: metafile
	Downloading                          // Second phase: chunks
	Downloaded                           // Third phase: end
	Failed
)

type LocalFile struct {
	Name         string      // Local name
	MetafileHash SHA256_HASH // TODO
	State        FileState
	Size         int64 // Type given by the Stat().Size()
	chunkCount   uint64
	metafile     []byte // TODO
	waitGroup    *sync.WaitGroup
}

type DownloadRequest struct {
	Hash SHA256_HASH
	Dest string
}

// TODO Add LRU Cache with callback on eviction to set "HasData" to false (or something else),
// (after a download)
type FileChunk struct {
	File    *LocalFile
	Number  uint64 // Chunk count: starts at 1 !
	Hash    SHA256_HASH
	HasData bool
	Data    []byte
}

type SearchedFile struct {
	firstPeer string
	maxChunk  uint64   // highest chunk number such that chunks [1..maxChunk] all have known owners
	chunks    []string // index is chunk number-1, value is the current owner
	// TODO: do we want first known owner or last known owner ? Do it at random ?
}

type FileHandler struct {
	files             map[SHA256_HASH]*LocalFile // Mapping from hashes to their corresponding file
	filesLock         *sync.RWMutex
	chunks            map[SHA256_HASH]*FileChunk
	chunksLock        *sync.RWMutex
	searchedFiles     map[SHA256_HASH]*SearchedFile
	searchedLock      *sync.RWMutex
	seenRequests      []SeenRequest
	queries           []*Query
	sharedDir         string
	downloadDir       string
	name              string
	downloadWorkers   uint
	downloadChannel   chan<- *DownloadRequest
	routing           *RoutingHandler
	peers             *PeersHandler
	net               *NetworkHandler
	blockchain        *BlockchainHandler
	dataDispatcher    *DataReplyDispatcher
	srepDispatcher    *SearchReplyDispatcher
	reputationhandler *ReputationHandler
}

func createDirIfNotExist(abspath string) error {
	if fileInfo, err := os.Stat(abspath); os.IsNotExist(err) {
		// Dir does not exist: we create it
		err = os.Mkdir(abspath, 0775)
		if err != nil {
			return err
		}
	} else if err != nil {
		// Other error => won't recover
		return err
	} else if !fileInfo.IsDir() {
		// Path points to a file instead of a directory, abort with message
		return errors.New(fmt.Sprint("Error: path ", abspath, " points to a file when it should be a directory! Aborting"))
	}

	// Good to go => error is nil
	return nil
}

func NewFileHandler(name string, downloadWorkers uint, r *ReputationHandler) *FileHandler {
	exe, err := os.Executable()
	if err != nil {
		panic(err)
	}

	sharedDir := filepath.Join(filepath.Dir(exe), SHARED_DIR_NAME)
	if err = createDirIfNotExist(sharedDir); err != nil {
		panic(err)
	}

	downloadDir := filepath.Join(filepath.Dir(exe), DOWNLOAD_DIR_NAME)
	if err = createDirIfNotExist(downloadDir); err != nil {
		panic(err)
	}

	return &FileHandler{
		files:             make(map[SHA256_HASH]*LocalFile), // MetafileHash to file
		filesLock:         &sync.RWMutex{},
		chunks:            make(map[SHA256_HASH]*FileChunk), // Hash to file chunk
		chunksLock:        &sync.RWMutex{},
		searchedFiles:     make(map[SHA256_HASH]*SearchedFile), // Hash to searched files
		searchedLock:      &sync.RWMutex{},
		seenRequests:      make([]SeenRequest, 0),
		queries:           make([]*Query, 0),
		sharedDir:         sharedDir,
		downloadDir:       downloadDir,
		downloadWorkers:   downloadWorkers,
		name:              name,
		reputationhandler: r,
	}
}

// Files only directly shared by us (not replicated)
func (f *FileHandler) SharedFiles() []FileInfo {
	f.filesLock.RLock()
	defer f.filesLock.RUnlock()

	files := make([]FileInfo, 0)
	for _, file := range f.files {
		if file.State == Owned {
			files = append(files, FileInfo{Filename: file.Name, Hash: file.MetafileHash, Size: file.Size})
		}
	}
	return files
}

func (f *FileHandler) RunFileHandler(net *NetworkHandler, peers *PeersHandler, routing *RoutingHandler, blockchain *BlockchainHandler) {
	f.routing = routing
	f.dataDispatcher = runDataReplyDispatcher()
	f.srepDispatcher = runSearchReplyDispatcher()
	f.downloadChannel = f.runDownloadGroup(f.downloadWorkers)
	f.net = net
	f.peers = peers
	f.blockchain = blockchain
}

/* We just got some new chunk */
func (f *FileHandler) HandleDataReply(dataRep *DataReply) {
	// Important: should not block
	if dataRep.Destination == f.name {

		// Impact on the reputation, must add a TxPublish to the blockchain
		// First create the new TxPublish to be "mined"
		filename := fmt.Sprintf("%x", dataRep.Data)
		transaction := TxPublish{
			Type: DownloadSuccess,
			File: File{Name: filename,
				Size:         0,
				MetafileHash: []byte{}},
			NodeOrigin:      dataRep.Destination,
			NodeDestination: dataRep.Origin,
			TargetHash:      dataRep.Data,
			HopLimit:        10,
		}
		// Handle the transaction in the blockchain
		go f.blockchain.HandleTxPublish(&transaction)

		go func() {
			f.dataDispatcher.dataReplyChan <- dataRep
		}()
	} else {
		if valid := f.prepareDataReply(dataRep); valid {
			f.routing.SendPacketTowards(dataRep, dataRep.Destination)
		}
	}
}

/* A data request came by */
func (f *FileHandler) HandleDataRequest(dataReq *DataRequest) {
	// Should not block as it is called by the dispatcher
	go func() {
		// TODO Should we differentiate between when we are the destination and
		// when we just happen to have the chunk already ?
		if dataRep, valid := f.answerTo(dataReq); valid {
			if f.reputationhandler.CanDownloadChunk(dataReq.Destination) {
				f.routing.SendPacketTowards(dataRep, dataRep.Destination)
			}
		} else if dataReq.Destination == f.name {
			// The destination is us but we can't reply because we don't have the data
			fmt.Printf("We don't have data for %x. Dropping the request.\n", dataReq.HashValue)
		} else { // Cannot answer so we forward the request
			if valid := f.prepareDataRequest(dataReq); valid {
				f.routing.SendPacketTowards(dataReq, dataReq.Destination)
			}
		}
	}()
}

func (f *FileHandler) answerTo(dataReq *DataRequest) (*DataReply, bool) {
	hash, err := ToHash(dataReq.HashValue)
	if err != nil {
		return nil, false
	}

	reply := &DataReply{
		Origin:      dataReq.Destination,
		Destination: dataReq.Origin,
		HopLimit:    DEFAULT_HOP_LIMIT,
		HashValue:   dataReq.HashValue,
	}

	f.filesLock.RLock()
	defer f.filesLock.RUnlock()

	// Reply can either be a metafile or a chunk
	if metafile, present := f.files[hash]; present {
		reply.Data = metafile.metafile
		return reply, true
	}

	f.chunksLock.RLock()
	defer f.chunksLock.RUnlock()

	if chunk, present := f.chunks[hash]; present && chunk.HasData {
		reply.Data = chunk.Data
		return reply, true
	}

	return nil, false
}

// TODO Find a way to share code between private messages and datarequest/reply and searchrequest/reply and Tx/Block Publish?
func (f *FileHandler) prepareDataRequest(dataReq *DataRequest) bool {
	if dataReq.HopLimit <= 1 {
		dataReq.HopLimit = 0
		return false
	}

	dataReq.HopLimit -= 1
	return true
}

func (f *FileHandler) prepareDataReply(dataRep *DataReply) bool {
	if dataRep.HopLimit <= 1 {
		dataRep.HopLimit = 0
		return false
	}

	dataRep.HopLimit -= 1
	return true
}

func (f *FileHandler) downloadFile(metafileHash SHA256_HASH, metafileOwner string, localName string, chunkResolver func(chunkNumber uint64) string) {
	// TODO locks
	// TODO Reenable check ?
	// if metafile, present := f.files[metafileHash]; present && metafile.State == Owned {
	// 	fmt.Printf("File is already shared (metafile is present). Hash: %x \n", metafileHash)
	// 	return
	// }

	// Create an entry for this file
	file := &LocalFile{
		Name:         localName,
		MetafileHash: metafileHash,
		State:        DownloadingMetafile,
	}

	f.filesLock.Lock()
	f.files[metafileHash] = file
	f.filesLock.Unlock()

	fmt.Println("DOWNLOADING metafile of", localName, "from", metafileOwner)
	// Request for metafile
	metafile, success := f.chunkDownloader(&DownloadRequest{
		Hash: metafileHash,
		Dest: metafileOwner,
	})
	if !success {
		fmt.Printf("Downloading metafile %x from %s was unsuccessful. \n", metafileHash, metafileOwner)
		return
	}

	// Receive and validate
	hashes, err := f.addMetafileInfo(metafileHash, metafile.Data)
	if err != nil {
		fmt.Println("Metafile", metafileHash, "from", metafileOwner, "was invalid")
		file.State = Failed
		return
	}
	// Query each chunk
	for i, hash := range hashes {
		f.downloadChannel <- &DownloadRequest{Hash: hash, Dest: chunkResolver(uint64(i + 1))}
	}

	file.waitGroup.Wait()
	file.State = Downloaded

	// WaitGroup is done => can save file
	// TODO Good file flags ?
	outputFile, err := os.OpenFile(filepath.Join(f.downloadDir, localName), os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		fmt.Println("Could not open file for writing:", err)
		return
	}

	f.chunksLock.RLock()
	defer f.chunksLock.RUnlock()

	totalWritten := int64(0)
	for _, hash := range hashes {
		data := f.chunks[hash].Data
		n, _ := outputFile.Write(data)
		totalWritten += int64(n)
	}
	file.Size = totalWritten

	outputFile.Close()
	fmt.Println("RECONSTRUCTED file", localName)

}

/* Some client wants to download a file */
func (f *FileHandler) RequestFileDownload(dest string, metafileHash SHA256_HASH, localName string) {
	// Should not block
	go func() {
		f.downloadFile(metafileHash, dest, localName, func(i uint64) string { return dest })
	}()
}

/* Download a chunk: handles timeouts, as well as invalid hashes.
   Returns (*DataReply, true) if the download was successful and (nil, false)
   if the download failed for any reason. Blocking function */
func (f *FileHandler) chunkDownloader(req *DownloadRequest) (*DataReply, bool) {
	dest := req.Dest
	chunkHash := req.Hash

	receiver := make(chan *DataReply, CHANNEL_BUFFERSIZE)
	f.registerChannel(receiver, chunkHash)
	dataRequest := &DataRequest{
		Origin:      f.name,
		Destination: dest,
		HopLimit:    DEFAULT_HOP_LIMIT,
		HashValue:   chunkHash[:],
	}

	f.routing.SendPacketTowards(dataRequest, dest)
	ticker := time.NewTicker(NO_ANSWER_TIMEOUT)
	timeouts := 0

	defer ticker.Stop()
	defer f.unregisterChannel(receiver, chunkHash)

	for {
		select {
		case dataRep, ok := <-receiver:
			// Channel was closed by dispatcher
			if !ok {
				// TODO
				// panic("Downloader was finished by dispatcher")
				return nil, false
			}

			f.chunksLock.RLock()
			chunkInfo, present := f.chunks[req.Hash]
			f.chunksLock.RUnlock()

			if present {
				fmt.Printf("DOWNLOADING %s chunk %d from %s\n", chunkInfo.File.Name, chunkInfo.Number, req.Dest)
			}

			// Drop the packet if it isn't valid
			if !isDataReplyValid(dataRep) {
				fmt.Printf("Received Hash (%x) didn't match the hash of the received content: (%x)\n", dataRep.HashValue, sha256.Sum256(dataRep.Data))
				break
			}

			return dataRep, true

		case <-ticker.C:
			timeouts += 1
			if timeouts > MAX_RETRIES {
				fmt.Printf("Maximum retries reached. Aborting download of %x\n", chunkHash)
				return nil, false
			}

			// Retry if we timeout
			f.routing.SendPacketTowards(dataRequest, dest)
		}
	}
}

func isDataReplyValid(dataRep *DataReply) bool {
	hash, err := ToHash(dataRep.HashValue)
	if err != nil {
		fmt.Println(err)
		return false
	}
	return sha256.Sum256(dataRep.Data) == hash
}

func (f *FileHandler) addMetafileInfo(hash SHA256_HASH, hashes []byte) ([]SHA256_HASH, error) {
	if len(hashes)%sha256.Size != 0 {
		return nil, errors.New(fmt.Sprint("Metafile content was invalid: received ", len(hashes), " bytes, not a multiple of ", sha256.Size))
	}

	// Add metafile to file
	f.filesLock.RLock()
	file, present := f.files[hash]
	f.filesLock.RUnlock()

	// We didn't ask for this file
	if !present {
		return nil, errors.New(fmt.Sprint("Received a metafile we didn't ask for. Hash: ", hash))
	} else if file.State == Owned {
		return nil, errors.New(fmt.Sprint("We won't download file that was shared by us. File is: ", file.Name))
		// TODO STATE = Replica
	} else {
		file.MetafileHash = hash
		file.metafile = hashes
		file.waitGroup = &sync.WaitGroup{}
		totalChunks := len(hashes) / sha256.Size
		file.chunkCount = uint64(totalChunks)
		file.waitGroup.Add(totalChunks)

		file.State = Downloading
	}

	// Add all hashes to the download list
	// TODO Locks
	separatedHashes := MetaFileToHashes(hashes)

	f.chunksLock.Lock()
	defer f.chunksLock.Unlock()

	for chunkCount, chunkHash := range separatedHashes {
		f.chunks[chunkHash] = &FileChunk{
			File:    file,
			Number:  uint64(chunkCount + 1), // Starts at 1 !
			Hash:    chunkHash,
			HasData: false,
			Data:    nil,
		}
	}

	// No error
	return separatedHashes, nil
}

func MetaFileToHashes(hashes []byte) []SHA256_HASH {
	separatedHashes := make([]SHA256_HASH, 0)
	for start := 0; start < len(hashes); start += sha256.Size {
		chunkHash, _ := ToHash(hashes[start : start+sha256.Size])
		separatedHashes = append(separatedHashes, chunkHash)
	}
	return separatedHashes
}

/* Create a worker pool that will handle the downloads of chunks
   (only chunks, not metafile). Returns a channel to submit download
   requests */
func (f *FileHandler) runDownloadGroup(workers uint) chan<- *DownloadRequest {
	dlChan := make(chan *DownloadRequest, CHANNEL_BUFFERSIZE)

	for i := uint(0); i < workers; i += 1 {
		go func() {
			for {
				select {
				case req, ok := <-dlChan:
					// Stop goroutine if channel is closed
					if !ok {
						return
					}

					if dataRep, success := f.chunkDownloader(req); success {
						// TODO Download
						// fmt.Printf("Downloaded chunk: %x\n", dataRep.HashValue)
						f.acceptDataChunk(req.Hash, dataRep.Data)
					} else {
						// TODO abort download
					}
				}
			}
		}()
	}

	return dlChan
}

/* We assume the chunk was validated already */
func (f *FileHandler) acceptDataChunk(hash SHA256_HASH, data []byte) {
	// TODO Locks
	// file.waitGroup.Done()
	// Add chunk to downloaded

	// TODO presence checking should be unnecessary

	f.chunksLock.RLock()
	chunk, present := f.chunks[hash]
	f.chunksLock.RUnlock()

	if !present {
		panic("Chunk not present")
	}
	chunk.Data = data
	chunk.HasData = true

	// fmt.Println("Chunk", chunk.Number, "was downloaded")

	// TODO Timeout on waitgroup
	chunk.File.waitGroup.Done()
}

func (f *FileHandler) RequestFileIndexing(filename string) {
	abspath := filepath.Join(f.sharedDir, filename)
	indexed, fileChunks, err := f.toIndexedFile(abspath)
	if err != nil {
		fmt.Println("File", abspath, "does not exist. Won't index it.")
		return
	}
	// Put back the filename as Name instead of the abspath
	indexed.Name = filename

	f.chunksLock.Lock()
	// Save all file chunks into our global chunk map
	for h, v := range fileChunks {
		f.chunks[h] = v
	}
	f.chunksLock.Unlock()

	fmt.Printf("Indexed file %s (%d chunks) with hash %x\n", indexed.Name, len(fileChunks), indexed.MetafileHash)

	f.filesLock.Lock()
	f.files[indexed.MetafileHash] = indexed
	f.filesLock.Unlock()

	// Claim the file's name on the blockchain
	bFile := &File{
		Name:         indexed.Name,
		Size:         indexed.Size,
		MetafileHash: indexed.MetafileHash[:],
	}
	f.blockchain.PublishBindingForFile(bFile)
}

/* Resulting *File has its absolute path as Name. */
func (f *FileHandler) toIndexedFile(abspath string) (*LocalFile, map[SHA256_HASH]*FileChunk, error) {
	// Open file
	file, err := os.Open(abspath)
	if err != nil {
		return nil, nil, err
	}
	fileStats, _ := file.Stat()

	indexedFile := &LocalFile{
		Name:  abspath, // Changeable if needed
		Size:  fileStats.Size(),
		State: Owned,
	}

	fileChunks := make(map[SHA256_HASH]*FileChunk)

	// Read in chunk of 8KB
	metafile := make([]byte, 0)
	chunk := make([]byte, CHUNK_SIZE)
	var chunkCount uint64 = 1 // Starts at 1 !
	for {
		read, err := file.Read(chunk)
		if err != nil {
			if err != io.EOF {
				return nil, nil, err
			}
			break
		}
		// Hash chunk and append it to metafile
		hash := sha256.Sum256(chunk[:read])
		// fmt.Printf("[% x] \n", hash)
		metafile = append(metafile, hash[:]...)

		chunkData := make([]byte, read)
		copy(chunkData, chunk)
		newChunk := &FileChunk{
			File:    indexedFile,
			Number:  chunkCount,
			Hash:    hash,
			HasData: true,
			Data:    chunkData,
		}
		fileChunks[hash] = newChunk

		chunkCount += 1
	}

	if len(metafile) > CHUNK_SIZE {
		fmt.Println("Metafile is bigger than 8KB! Size:", len(metafile), "Bytes")
		// TODO err ?
	}

	metafileHash := sha256.Sum256(metafile)

	indexedFile.metafile = metafile
	indexedFile.MetafileHash = metafileHash
	indexedFile.chunkCount = uint64(len(fileChunks))

	return indexedFile, fileChunks, nil
}
