package files

import (
	"crypto/sha256"
	"fmt"
	. "github.com/RomainGehrig/Peerster/routing"
	"os"
	"path/filepath"
)

const SHARED_DIR_NAME = "_SharedDir"

type SHA256_HASH [sha256.Size]byte

type File struct {
	Name         string
	Size         uint32      // May need to upgrade to uint64 if files > 4GiB
	Metafile     []byte      // TODO
	MetafileHash SHA256_HASH // TODO
}

type FileHandler struct {
	files     map[SHA256_HASH]*File // Mapping from hashes to their corresponding file
	sharedDir string
	routing   *RoutingHandler
}

func NewFileHandler() *FileHandler {
	exe, err := os.Executable()
	if err != nil {
		panic(err)
	}

	sharedDir := filepath.Join(filepath.Dir(exe), SHARED_DIR_NAME)
	if fileInfo, err := os.Stat(sharedDir); os.IsNotExist(err) {
		// Dir does not exist: we create it
		fmt.Println("Creating the", SHARED_DIR_NAME, "directory here:", sharedDir)
		err = os.Mkdir(sharedDir, 0775)
		if err != nil {
			panic(err)
		}
	} else if err != nil {
		// Other error => won't recover
		panic(err)
	} else if !fileInfo.IsDir() {
		// Path points to a file instead of a directory, abort with message
		panic(fmt.Sprintf("Error: path", sharedDir, "points to a file when it should be a directory! Aborting"))
	}

	return &FileHandler{
		files:     make(map[SHA256_HASH]*File),
		sharedDir: sharedDir,
	}
}

func (f *FileHandler) RunFileHandler(routing *RoutingHandler) {
	f.routing = routing
}

func (f *FileHandler) RequestFileIndexing(filename string) {
	indexed, err := f.indexFile(filename)
	if err {
		fmt.Println(indexed)
	}
}

func (f *FileHandler) indexFile(filename string) (*File, bool) {
	return nil, false
}
