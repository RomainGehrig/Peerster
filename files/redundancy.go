package files

import (
	. "github.com/RomainGehrig/Peerster/constants"
)

// Handle a timeout in the network
func (f *FileHandler) HandleTimeout(hostName string) {
	// Should not block
	go func() {
		// TODO Check if host was the owner of a file => download and claim file ?
		// TODO Check if host was the replica of file we own => need to redistribute to other
		//      and tell other replicas of the new replica name
	}()
}


// Try to claim the file with the
func (f *FileHandler) ClaimOwnership(oldOwner string, fileHash SHA256_HASH) {
	// TODO Make sure we have the complete file

	// TODO Send a transaction of OwnershipTransfert from previous owner to new owner
	// TODO Wait a bit so the blockchain converges
	// TODO See if we are the effective owner -> if not, quit the process
	// TODO If there are no new owner (ie transactions were lost or something), repeat

	// We are now the new owner
	// TODO Start the OwnershipProcess
}


/******* FUNCTIONS TO USE WHERE WE ARE THE OWNER OF A FILE *******/

// Should be run as a new go routine
// This function is used to do all the housekeeping of having to handle an owned file:
//  - making sure there are enough replicas
//  - if not, search for new
func (f *FileHandler) RunOwnedFileProcess(fileHash SHA256_HASH) {
	// TODO Check replicas state
	// TODO Find new if needed
	// TODO Distribute replication status (which replicas exist)


	// TODO Don't forget: handle timeouts
}

// Should be run as a new go routine
func (f *FileHandler) RunSearchNewReplicas(n int, fileHash SHA256_HASH) bool {
	// TODO Send query to nodes that aren't replicas already
	// TODO Wait for answers
	// TODO ACK up to n nodes (their download should not count for reputation spendings !)

	// TODO Return true if we have enough answers
	return true
}

// Insure that a replica has the file we gave it
func (f *FileHandler) ChallengeReplica(replicaName string, fileHash SHA256_HASH) bool {
	// TODO Don't challenge the replica if we don't have the file

	// TODO Create a challenge that we know the answer to
	// TODO Send the challenge to the replica and wait for the answer (or timeout)
	// TODO Malus if replica doesn't answer: return false in this case

	// TODO Return true if the replica has answered
	return true
}
