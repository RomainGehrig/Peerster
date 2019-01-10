package files

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	. "github.com/RomainGehrig/Peerster/constants"
	. "github.com/RomainGehrig/Peerster/utils"
	"math/rand"
	"sync"
	"time"
)

const DEFAULT_REPLICATION_FACTOR = 3
const DELAY_BETWEEN_REPLICA_CHECKS = 10 * time.Second
const CHALLENGE_TIMEOUT = 5 * time.Second
const DELAY_BETWEEN_CHALLENGE_STATUS_CHECKS = 100 * time.Millisecond

// Additional info needed when we own the file
type ReplicationData struct {
	factor         int                       // How many time the file should be replicated
	holders        []string                  // List of nodes that possess a replica of the file
	challenges     map[string]*ChallengeData // Map from replica name to their current challenge
	challengesLock *sync.RWMutex
}

// TODO Move into messages
type ChallengeRequest struct {
	Destination string
	Source      string
	FileHash    SHA256_HASH
	Challenge   uint64
	HopLimit    int
}

type ChallengeReply struct {
	Destination string
	Source      string
	FileHash    SHA256_HASH
	Result      SHA256_HASH
	HopLimit    int
}

type ChallengeStatus int

const (
	Waiting ChallengeStatus = iota
	IncorrectlyAnswered
	CorrectlyAnswered
	Timeout
)

type ChallengeData struct {
	Challenge       uint64
	ExpectedResult  SHA256_HASH
	ChallengeStatus ChallengeStatus
}

func (lf *LocalFile) InitializeReplicationData() {
	// Initialization when we are the owner
	if lf.replicationData == nil && lf.State == Owned {
		lf.replicationData = &ReplicationData{
			factor:         DEFAULT_REPLICATION_FACTOR,
			holders:        make([]string, 0),
			challenges:     make(map[string]*ChallengeData),
			challengesLock: &sync.RWMutex{},
		}
	}
	// TODO Init we switch from replica to owner
	// TODO Init we become a replica (from Downloaded state)
}

// Handle a timeout in the network
func (f *FileHandler) HandleTimeout(hostName string) {
	// Should not block
	go func() {
		// TODO Check if host was the owner of a file => download and claim file ?
		// TODO Check if host was the replica of file we own => need to redistribute to other
		//      and tell other replicas of the new replica name
	}()
}

// Try to claim the file
func (f *FileHandler) ClaimOwnership(oldOwner string, fileHash SHA256_HASH) {
	// TODO Make sure we have the complete file

	// TODO Send a transaction of OwnershipTransfert from previous owner to new owner
	// TODO Wait a bit so the blockchain converges
	// TODO See if we are the effective owner -> if not, quit the process
	// TODO If there are no new owner (ie transactions were lost or something), repeat

	// We are now the new owner
	// TODO Start the OwnershipProcess
}

/******* FUNCTIONS TO USE WHEN WE ARE THE OWNER OF A FILE *******/

// Should be run as a new go routine
// This function is used to do all the housekeeping of having to handle an owned file:
//  - making sure there are enough replicas
//  - if not, search for new
func (f *FileHandler) RunOwnedFileProcess(fileHash SHA256_HASH) {
	f.filesLock.RLock()
	file, present := f.files[fileHash]
	f.filesLock.RUnlock()

	// Check that we are the owner
	if !present || file.State != Owned {
		fmt.Printf("We either don't have or don't own the file with hash %x. Aborting ownership process\n", fileHash)
		return
	}

	currentReplicas := make(map[string]chan bool)
	for _, replicaName := range file.replicationData.holders {
		replicaChannel := make(chan bool)
		currentReplicas[replicaName] = replicaChannel

		// Watch for this replica
		go f.ContinuouslyCheckReplicaStatus(replicaName, file, replicaChannel)
	}

	// Infinite loop of continuously checking that the replicas are alive
	for {
		// Here we need to
		// 1) check that our current replica holders are still alive
		// 2) find new replica holders if we don't have enough

		updateHoldersList := false

		// Check replicas state
		for _, replicaName := range file.replicationData.holders {
			select {
			// Some replica failed the challenge, need to update the state
			// and maybe find some new replicas
			case <-currentReplicas[replicaName]:
				updateHoldersList = true
				delete(currentReplicas, replicaName)
			default:
			}
		}

		if updateHoldersList {
			newHolders := make([]string, 0)
			for replica, _ := range currentReplicas {
				newHolders = append(newHolders, replica)
			}
			file.replicationData.holders = newHolders
			// TODO Inform others that the holders have changed
		}

		replicationsNeeded := file.replicationData.factor - len(currentReplicas)
		// Need to find more replicas !
		if replicationsNeeded > 0 {
			f.FindNewReplicas(replicationsNeeded, file, StringSetInit(file.replicationData.holders))
		}

		// TODO Find new replica that doesn't already host the file
		// TODO Distribute replication status (which replicas exist)

		// TODO Sleep or ticker to avoid excessive looping ?
	}

	// TODO Don't forget: handle timeouts
}

func (f *FileHandler) FindNewReplicas(count int, file *LocalFile, currentHolders *StringSet) {
	// TODO
}

func (f *FileHandler) ContinuouslyCheckReplicaStatus(replicaName string, file *LocalFile, stopChan chan<- bool) {
	ticker := time.NewTicker(DELAY_BETWEEN_REPLICA_CHECKS)

	go func() {
		for {
			select {
			case <-ticker.C:
				// Challenge the replica and stop if it fails (we don't tolerate failure)
				// TODO Do we need to keep the challenge in the map ?
				if !f.ChallengeReplica(replicaName, file, true) {
					stopChan <- false
					return
				}
			}
		}
	}()
}

// Should be run as a new go routine
func (f *FileHandler) RunSearchNewReplicas(n int, file *LocalFile) bool {
	// TODO Send query to nodes that aren't replicas already
	// TODO Wait for answers
	// TODO ACK up to n nodes (their download should not count for reputation spendings !)

	// TODO Return true if we have enough answers
	return true
}

// Insure that a replica has the file we gave it: return true if the replica
// correctly answered the challenge or false if it timed out or answered incorrectly
//
// Blocking function
func (f *FileHandler) ChallengeReplica(replicaName string, file *LocalFile, cleanupAfter bool) bool {
	file.replicationData.challengesLock.RLock()
	challenge, present := file.replicationData.challenges[replicaName]
	file.replicationData.challengesLock.RUnlock()

	// Don't rerun a challenge if it's pending, or wrong, or timed out
	if present && challenge.ChallengeStatus != CorrectlyAnswered {
		fmt.Println("Previous challenge for replica", replicaName, "is either not ended or ended badly")
		return false
	}

	// Create the challenge and its answer
	req, expected := f.CreateChallenge(replicaName, file)
	challenge = &ChallengeData{
		Challenge:       req.Challenge,
		ExpectedResult:  expected,
		ChallengeStatus: Waiting,
	}

	file.replicationData.challengesLock.Lock()
	file.replicationData.challenges[replicaName] = challenge
	file.replicationData.challengesLock.Unlock()

	// Delete the challenge (keeping the map tidy) if wanted
	if cleanupAfter {
		defer func() {
			file.replicationData.challengesLock.Lock()
			delete(file.replicationData.challenges, replicaName)
			file.replicationData.challengesLock.Unlock()
		}()
	}

	timer := time.NewTimer(CHALLENGE_TIMEOUT)
	defer timer.Stop()

	checkTicker := time.NewTicker(DELAY_BETWEEN_CHALLENGE_STATUS_CHECKS)
	defer checkTicker.Stop()

	// Send the challenge to the replica and wait for the answer (or timeout)
	for {
		select {
		case <-timer.C:
			// Challenge has timed out
			challenge.ChallengeStatus = Timeout
			return false
		case <-checkTicker.C:
			if challenge.ChallengeStatus != Waiting {
				return challenge.ChallengeStatus == CorrectlyAnswered
			}
		}
	}
}

func (f *FileHandler) CreateChallenge(target string, file *LocalFile) (*ChallengeRequest, SHA256_HASH) {
	challenge := rand.Uint64()
	request := &ChallengeRequest{
		Source:      f.name,
		Destination: target,
		FileHash:    file.MetafileHash,
		Challenge:   challenge,
		HopLimit:    DEFAULT_HOP_LIMIT,
	}

	return request, f.AnswerChallenge(request)
}

// The challenge is essentially a verification to ensure the challengee has all chunks of the file
func (f *FileHandler) AnswerChallenge(req *ChallengeRequest) (out SHA256_HASH) {
	h := sha256.New()
	// Write the headers
	h.Write([]byte(req.Destination))
	h.Write([]byte(req.Source))
	h.Write(req.FileHash[:])
	binary.Write(h, binary.LittleEndian, req.Challenge)

	f.filesLock.RLock()
	file, present := f.files[req.FileHash]
	f.filesLock.RUnlock()

	if !present {
		fmt.Printf("We don't have the file we are challenged for ! FileHash: %x\n", req.FileHash)
		return ZERO_SHA256_HASH
	}

	f.chunksLock.RLock()
	for _, chunkHash := range MetaFileToHashes(file.metafile) {
		chunk, present := f.chunks[chunkHash]
		if !present {
			return ZERO_SHA256_HASH
		}
		h.Write(chunk.Data)
	}
	f.chunksLock.RUnlock()

	copy(out[:], h.Sum(nil))
	return
}
