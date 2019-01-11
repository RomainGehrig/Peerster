package files

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	. "github.com/RomainGehrig/Peerster/constants"
	. "github.com/RomainGehrig/Peerster/messages"
	. "github.com/RomainGehrig/Peerster/utils"
	"math/rand"
	"strings"
	"sync"
	"time"
)

const DEFAULT_REPLICATION_FACTOR = 3
const DELAY_BETWEEN_REPLICA_CHECKS = 10 * time.Second
const DELAY_BETWEEN_REPLICA_SEARCHES = 3 * time.Second
const DELAY_BETWEEN_REPLICATION_CHECKS = 1 * time.Second
const CHALLENGE_TIMEOUT = 5 * time.Second
const DELAY_BETWEEN_CHALLENGE_STATUS_CHECKS = 100 * time.Millisecond
const WAIT_FOR_REPLICATION_REPLIES = 1 * time.Second
const WAIT_BEFORE_CHALLENGE = 2 * time.Second

// Additional info needed when we own the file
type ReplicationData struct {
	factor         int                       // How many time the file should be replicated
	holders        []string                  // List of nodes that possess a replica of the file
	challenges     map[string]*ChallengeData // Map from replica name to their current challenge
	challengesLock *sync.RWMutex
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

func (f *FileHandler) HandleChallengeRequest(cr *ChallengeRequest) {
	go func() {
		if cr.Destination != f.name {
			if f.prepareChallengeRequest(cr) {
				f.routing.SendPacketTowards(cr, cr.Destination)
			}
		} else {
			f.filesLock.RLock()
			file, present := f.files[cr.FileHash]
			f.filesLock.RUnlock()

			if !present || !file.State.HaveWholeFile() {
				return
			}

			answer := f.AnswerChallenge(file, cr)
			reply := &ChallengeReply{
				Destination: cr.Source,
				Source:      f.name,
				FileHash:    cr.FileHash,
				Result:      answer,
				HopLimit:    DEFAULT_HOP_LIMIT,
			}

			f.routing.SendPacketTowards(reply, cr.Source)
		}
	}()
}

func (f *FileHandler) HandleChallengeReply(cr *ChallengeReply) {
	go func() {
		if cr.Destination != f.name {
			if f.prepareChallengeReply(cr) {
				f.routing.SendPacketTowards(cr, cr.Destination)
			}
		} else {
			// Process challenge reply
			f.filesLock.RLock()
			file, present := f.files[cr.FileHash]
			f.filesLock.RUnlock()

			if !present || file.State != Owned {
				return
			}

			replicaName := cr.Source

			// Check challenge
			file.replicationData.challengesLock.RLock()
			challenge, present := file.replicationData.challenges[replicaName]
			file.replicationData.challengesLock.RUnlock()

			// If the challenge already had an anwer or timed out, we won't change its state
			if challenge.ChallengeStatus != Waiting {
				return
			}

			if challenge.ExpectedResult == cr.Result {
				challenge.ChallengeStatus = CorrectlyAnswered
			} else {
				challenge.ChallengeStatus = IncorrectlyAnswered
			}
		}

	}()
}

func (f *FileHandler) HandleReplicationRequest(rr *ReplicationRequest) {
	// Should not block
	go func() {
		// Forward request
		if f.prepareReplicationRequest(rr) {
			f.simple.BroadcastMessage(rr, nil)
		}

		// Send a ReplicationReply if we are interesting in hosting the file
		// because we have it or if we have space for it or other arbitrary criteria

		// TODO For the moment, an arbitrary (but consistent) decision to choose
		// if we want to keep the file
		// if (rr.FileHash[0]+f.name[len(f.name)-1])%2 == 0 {
		if true {
			rep := &ReplicationReply{
				Source:      f.name,
				Destination: rr.Source,
				FileHash:    rr.FileHash,
				HopLimit:    DEFAULT_HOP_LIMIT,
			}

			f.routing.SendPacketTowards(rep, rr.Source)
		}
	}()
}

func (f *FileHandler) HandleReplicationReply(rr *ReplicationReply) {
	// Should not block
	go func() {
		if rr.Destination != f.name {
			if f.prepareReplicationReply(rr) {
				f.routing.SendPacketTowards(rr, rr.Destination)
			}
		} else {
			// Add the reply to the list of interested nodes for this particular file
			f.replicationRepliesLock.Lock()
			defer f.replicationRepliesLock.Unlock()

			lst, present := f.replicationReplies[rr.FileHash]
			// Should not happen
			if !present || lst == nil {
				return
			}

			f.replicationReplies[rr.FileHash] = append(lst, rr.Source)
		}
	}()
}

func (f *FileHandler) HandleReplicationACK(ra *ReplicationACK) {
	go func() {
		if ra.Destination != f.name {
			if f.prepareReplicationACK(ra) {
				f.routing.SendPacketTowards(ra, ra.Destination)
			}
		} else {
			fileHash := ra.FileHash

			// TODO Better local name for search purpose ?
			localName := fmt.Sprintf("%s_%x", f.name, fileHash)
			f.downloadFile(fileHash, ra.Source, localName, func(uint64) string { return ra.Source })

			f.filesLock.RLock()
			f.files[fileHash].State = Replica
			f.filesLock.RUnlock()
		}
	}()
}

// The challenge is essentially a verification to ensure the challengee has all chunks of the file
func (f *FileHandler) AnswerChallenge(file *LocalFile, req *ChallengeRequest) (out SHA256_HASH) {
	h := sha256.New()
	// Write the headers
	h.Write([]byte(req.Destination))
	h.Write([]byte(req.Source))
	h.Write(req.FileHash[:])
	binary.Write(h, binary.LittleEndian, req.Challenge)

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

// TODO Find a way to share code between all thes prepareXXX methods
func (f *FileHandler) prepareChallengeRequest(cr *ChallengeRequest) bool {
	if cr.HopLimit <= 1 {
		cr.HopLimit = 0
		return false
	}

	cr.HopLimit -= 1
	return true
}

func (f *FileHandler) prepareChallengeReply(cr *ChallengeReply) bool {
	if cr.HopLimit <= 1 {
		cr.HopLimit = 0
		return false
	}

	cr.HopLimit -= 1
	return true
}

func (f *FileHandler) prepareReplicationRequest(rr *ReplicationRequest) bool {
	if rr.HopLimit <= 1 {
		rr.HopLimit = 0
		return false
	}

	rr.HopLimit -= 1
	return true
}

func (f *FileHandler) prepareReplicationReply(rr *ReplicationReply) bool {
	if rr.HopLimit <= 1 {
		rr.HopLimit = 0
		return false
	}

	rr.HopLimit -= 1
	return true
}

func (f *FileHandler) prepareReplicationACK(ra *ReplicationACK) bool {
	if ra.HopLimit <= 1 {
		ra.HopLimit = 0
		return false
	}

	ra.HopLimit -= 1
	return true
}

/******* FUNCTIONS TO USE WHEN WE ARE THE OWNER OF A FILE *******/

// Should be run as a new go routine
// This function is used to do all the housekeeping of having to handle an owned file:
//  - making sure there are enough replicas
//  - if not, search for new
func (f *FileHandler) RunOwnedFileProcess(file *LocalFile) {
	// Check that we are the owner
	if file.State != Owned {
		fmt.Printf("We either don't have or don't own the file with hash %x. Aborting ownership process\n", file.MetafileHash)
		return
	}

	currentReplicas := make(map[string]chan bool)
	for _, replicaName := range file.replicationData.holders {
		replicaChannel := make(chan bool)
		currentReplicas[replicaName] = replicaChannel

		// Watch for this replica
		go f.ContinuouslyCheckReplicaStatus(replicaName, file, replicaChannel)
	}

	newReplicasChannel := make(chan string) // TODO Buffering ?
	// First replica search can be done immediately
	timeBetweenReplicaSearch := time.NewTimer(0 * time.Second)

	timeBetweenReplicationCheck := time.NewTicker(DELAY_BETWEEN_REPLICATION_CHECKS)

	// Infinite loop of continuously checking that the replicas are alive
	for {
		// Here we need to
		// 1) check that our current replica holders are still alive
		// 2) find new replica holders if we don't have enough
		fmt.Printf("Owned File: %x currently has %d/%d replicas: %s\n", file.MetafileHash, len(currentReplicas), file.replicationData.factor, strings.Join(file.replicationData.holders, ", "))
		updateHoldersList := false

		// Check replicas state
		for _, replicaName := range file.replicationData.holders {
			select {
			// Check if the replica failed the challenge, in which case we need
			// to update the state and maybe find some new replicas
			case <-currentReplicas[replicaName]:
				updateHoldersList = true
				close(currentReplicas[replicaName])
				delete(currentReplicas, replicaName)
			default:
			}
		}

		// Check if we have new replica coming
		for {
			select {
			case replicaName := <-newReplicasChannel:
				// Don't add a replica already there
				if _, present := currentReplicas[replicaName]; present {
					break
				}

				// TODO Share code between the first init and this one
				replicaChannel := make(chan bool)
				currentReplicas[replicaName] = replicaChannel
				go f.ContinuouslyCheckReplicaStatus(replicaName, file, replicaChannel)

				updateHoldersList = true
			default:
				// Need to break out of the loop
				goto endLoop
			}
		}
	endLoop:

		if updateHoldersList {
			newHolders := make([]string, 0)
			for replica, _ := range currentReplicas {
				newHolders = append(newHolders, replica)
			}
			file.replicationData.holders = newHolders
		}

		replicationsNeeded := file.replicationData.factor - len(currentReplicas)
		// Need to find more replicas !
		if replicationsNeeded > 0 {
			select {
			case <-timeBetweenReplicaSearch.C:
				// New search permitted
				f.FindNewReplicas(replicationsNeeded, file, StringSetInit(file.replicationData.holders), newReplicasChannel)
				timeBetweenReplicaSearch.Reset(DELAY_BETWEEN_REPLICA_SEARCHES)
			default:
				fmt.Printf("Too early to do a new search for file %x\n", file.MetafileHash)
			}
		}

		// TODO Distribute replication status (which replicas exist) ?

		// Sleep to avoid excessive looping
		<-timeBetweenReplicationCheck.C
	}
}

func (f *FileHandler) FindNewReplicas(count int, file *LocalFile, currentHolders *StringSet, resultChannel chan<- string) {
	req := &ReplicationRequest{
		Source:   f.name,
		FileHash: file.MetafileHash,
		FileSize: file.Size,
		HopLimit: SMALL_FLOOD_HOP_LIMIT,
	}

	fileHash := file.MetafileHash

	f.replicationRepliesLock.Lock()
	f.replicationReplies[fileHash] = make([]string, 0)
	f.replicationRepliesLock.Unlock()

	// Send query to network
	f.simple.BroadcastMessage(req, nil)

	// Wait for answers
	time.Sleep(WAIT_FOR_REPLICATION_REPLIES)

	potentialReplicas := make([]string, 0)

	// Select `count` of them and ACK
	f.replicationRepliesLock.Lock()
	repliesSet := StringSetInit(f.replicationReplies[fileHash])
	for _, replySource := range repliesSet.ToSlice() {
		// Don't add owner or current replica
		if replySource == f.name || currentHolders.Has(replySource) {
			continue
		}

		if len(potentialReplicas) >= count {
			break
		}

		potentialReplicas = append(potentialReplicas, replySource)

		ack := &ReplicationACK{
			Source:      f.name,
			Destination: replySource,
			FileHash:    fileHash,
			HopLimit:    DEFAULT_HOP_LIMIT,
		}
		f.routing.SendPacketTowards(ack, replySource)
	}
	delete(f.replicationReplies, fileHash)
	f.replicationRepliesLock.Unlock()

	// Wait N seconds for them to download the file
	time.Sleep(WAIT_BEFORE_CHALLENGE)

	// Challenge each and send them the successful ones to the channel
	for _, replica := range potentialReplicas {
		go func(replica string) {
			if f.ChallengeReplica(replica, file, true) {
				resultChannel <- replica
			}
		}(replica)
	}
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

	// Send the request
	f.routing.SendPacketTowards(req, replicaName)

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

	return request, f.AnswerChallenge(file, request)
}
