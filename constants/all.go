package constants

import "crypto/sha256"

const UDP_DATAGRAM_MAX_SIZE int = 65507 // Maximum payload for UDP

// Should be at least 2 for the gossiper to work
const CHANNEL_BUFFERSIZE int = 16

const DEFAULT_HOP_LIMIT = 10
const SMALL_FLOOD_HOP_LIMIT = 3

type SHA256_HASH [sha256.Size]byte

var ZERO_SHA256_HASH SHA256_HASH = [32]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

// TODO Put somewhere else ?
type RegistrationMessageType int

const (
	Register RegistrationMessageType = iota
	Unregister
)
