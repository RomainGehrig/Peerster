package files

import (
	"crypto/sha256"
	"fmt"
	. "github.com/RomainGehrig/Peerster/routing"
)

type SHA256_HASH [sha256.Size]byte

type File struct {
	Name         string
	Size         uint32      // May need to upgrade to uint64 if files > 4GiB
	Metafile     []byte      // TODO
	MetafileHash SHA256_HASH // TODO
}

type FileHandler struct {
	files   map[SHA256_HASH]File
	routing *RoutingHandler
}

func NewFileHandler() *FileHandler {
	return &FileHandler{
		files: make(map[SHA256_HASH]File),
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
