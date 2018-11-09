package files

import (
	"crypto/sha256"
	"errors"
	"fmt"
	. "github.com/RomainGehrig/Peerster/messages"
	. "github.com/RomainGehrig/Peerster/routing"
	"io"
	"os"
	"path/filepath"
	"time"
)

const SHARED_DIR_NAME = "_SharedDir"
const DOWNLOAD_DIR_NAME = "_Downloads"
const BYTES_IN_8KB = 8 * 1024 // == 8192 Bytes
const NO_ANSWER_TIMEOUT = 5 * time.Second

type SHA256_HASH [sha256.Size]byte
type FileSource int

const (
	Shared FileSource = iota
	Download
)

type File struct {
	Name         string
	Size         int64       // Type given by the Stat().Size()
	Metafile     []byte      // TODO
	MetafileHash SHA256_HASH // TODO
	Source       FileSource
}

// TODO Add LRU Cache with callback on eviction to set "HasData" to false (or something else)
type FileChunk struct {
	File    *File
	Number  uint32 // Chunk count
	Hash    SHA256_HASH
	HasData bool
	Data    []byte
}

type FileHandler struct {
	// filesLock     *sync.RWMutex // TODO
	// chunksLock     *sync.RWMutex // TODO
	files          map[SHA256_HASH]*File // Mapping from hashes to their corresponding file
	chunks         map[SHA256_HASH]*FileChunk
	sharedDir      string
	downloadDir    string
	routing        *RoutingHandler
	dataDispatcher *DataReplyDispatcher
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

func NewFileHandler() *FileHandler {
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
		files:       make(map[SHA256_HASH]*File),
		chunks:      make(map[SHA256_HASH]*FileChunk),
		sharedDir:   sharedDir,
		downloadDir: downloadDir,
	}
}

func (f *FileHandler) RunFileHandler(routing *RoutingHandler) {
	f.routing = routing
	f.dataDispatcher = runDataReplyDispatcher()
}

// We just got some new chunk
func (f *FileHandler) HandleDataReply(dataRep *DataReply) {
	// TODO SHOULD NOT BLOCK
}

func (f *FileHandler) HandleDataRequest(dataReq *DataRequest) {
	// TODO SHOULD NOT BLOCK
	// TODO Answer (we are target or we have hash) or reroute
}

func (f *FileHandler) RequestFileDownload(dest string, metafileHash SHA256_HASH, localName string) {
	// Timeouts: if no answer

	// Request for metafile
	// Receive and validate
	// Query each chunk

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

	f.files[indexed.MetafileHash] = indexed
}

/* Resulting *File has its absolute path as Name. */
func (f *FileHandler) toIndexedFile(abspath string) (*File, map[SHA256_HASH]*FileChunk, error) {
	// Open file
	file, err := os.Open(abspath)
	if err != nil {
		return nil, nil, err
	}
	fileStats, _ := file.Stat()

	indexedFile := &File{
		Name: abspath, // Changeable if needed
		Size: fileStats.Size(),
	}

	fileChunks := make(map[SHA256_HASH]*FileChunk)

	// Read in chunk of 8KB
	metafile := make([]byte, 0)
	chunk := make([]byte, BYTES_IN_8KB)
	var chunkCount uint32 = 0
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

	if len(metafile) > BYTES_IN_8KB {
		fmt.Println("Metafile is bigger than 8KB! Size:", len(metafile), "Bytes")
		// TODO err ?
	}

	metafileHash := sha256.Sum256(metafile)

	indexedFile.Metafile = metafile
	indexedFile.MetafileHash = metafileHash

	return indexedFile, fileChunks, nil
}
