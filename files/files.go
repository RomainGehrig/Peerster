package files

import (
	"crypto/sha256"
	"fmt"
	. "github.com/RomainGehrig/Peerster/routing"
	"io"
	"os"
	"path/filepath"
)

const SHARED_DIR_NAME = "_SharedDir"
const BYTES_IN_8KB = 8 * 1024 // == 8192 Bytes

type SHA256_HASH [sha256.Size]byte

type File struct {
	Name         string
	Size         int64       // Type given by the Stat().Size()
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
		panic(fmt.Sprint("Error: path", sharedDir, "points to a file when it should be a directory! Aborting"))
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
	abspath := filepath.Join(f.sharedDir, filename)
	indexed, err := f.toIndexedFile(abspath)
	if err != nil {
		fmt.Println("File", abspath, "does not exist. Won't index it.")
		return
	}
	// Put back the filename as Name instead of the abspath
	indexed.Name = filename

	f.files[indexed.MetafileHash] = indexed
}

/* Resulting *File has its absolute path as Name. */
func (f *FileHandler) toIndexedFile(abspath string) (*File, error) {
	// Open file
	file, err := os.Open(abspath)
	if err != nil {
		return nil, err
	}
	fileStats, _ := file.Stat()

	// Read in chunk of 8KB
	metafile := make([]byte, 0)
	chunk := make([]byte, BYTES_IN_8KB)
	for {
		read, err := file.Read(chunk)
		if err != nil {
			if err != io.EOF {
				return nil, err
			}
			break
		}
		// Hash chunk and append it to metafile
		hash := sha256.Sum256(chunk[:read])
		// fmt.Printf("[% x] \n", hash)
		metafile = append(metafile, hash[:]...)
	}

	if len(metafile) > BYTES_IN_8KB {
		fmt.Println("Metafile is bigger than 8KB! Size:", len(metafile), "Bytes")
		// TODO err ?
	}

	metafileHash := sha256.Sum256(metafile)

	indexedFile := &File{
		Name:         abspath, // Change if you want
		Size:         fileStats.Size(),
		Metafile:     metafile,
		MetafileHash: metafileHash,
	}

	return indexedFile, nil
}
