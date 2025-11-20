package udplink

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
)

const (
	// HeaderSize defines the total size of the fragment header in bytes.
	// 1 byte:  Flags
	// 4 bytes: Sequence Number
	// 2 bytes: Fragment ID
	// 2 bytes: Total Data Fragments (K)
	// 2 bytes: Total Parity Fragments (M)
	// 4 bytes: Checksum
	HeaderSize int = 15

	// FlagACK indicates that the fragment is an acknowledgment.
	FlagACK uint8 = 0x01
	// FlagDATA indicates that the fragment contains data.
	FlagDATA uint8 = 0x02
	// FlagFEC indicates that the fragment is a Forward Error Correction (parity) shard.
	FlagFEC uint8 = 0x04
)

// Fragment represents the structure of a single packet in the reliable UDP protocol.
// It includes metadata for sequencing, fragmentation, error correction, and integrity checks.
type Fragment struct {
	flags        uint8  // Fragment type flags (ACK, DATA, FEC).
	seqNum       uint32 // Sequence number for ordering entire messages.
	fragmentID   uint16 // ID of this fragment within a sequence (0 to K+M-1).
	dataShards   uint16 // The number of data fragments (K) for FEC.
	parityShards uint16 // The number of parity fragments (M) for FEC.
	checksum     uint32 // CRC32 checksum of the header and payload for integrity.
	payload      []byte // Payload data.
}

// BuildDataFragment constructs a byte slice representing a data or FEC fragment.
// The resulting slice is ready for transmission over UDP.
func BuildDataFragment(seqNum uint32, fragmentID, dataShards, parityShards uint16, isFEC bool, payload []byte) []byte {
	flags := FlagDATA
	if isFEC {
		flags |= FlagFEC
	}

	// Allocate a byte slice with space for the header and payload.
	fragmentBytes := make([]byte, HeaderSize+len(payload))

	// 1. Flags
	fragmentBytes[0] = flags
	// 2. Sequence Number
	binary.BigEndian.PutUint32(fragmentBytes[1:5], seqNum)
	// 3. Fragment ID
	binary.BigEndian.PutUint16(fragmentBytes[5:7], fragmentID)
	// 4. Data Shards (K)
	binary.BigEndian.PutUint16(fragmentBytes[7:9], dataShards)
	// 5. Parity Shards (M)
	binary.BigEndian.PutUint16(fragmentBytes[9:11], parityShards)

	// Copy payload immediately after the header fields (checksum is last).
	copy(fragmentBytes[HeaderSize:], payload)

	// Calculate checksum over the header fields and payload.
	checksum := computeChecksum(flags, seqNum, fragmentID, dataShards, parityShards, payload)
	// 6. Checksum
	binary.BigEndian.PutUint32(fragmentBytes[11:15], checksum)

	return fragmentBytes
}

// BuildAckFragment constructs a byte slice representing an acknowledgment (ACK) fragment.
// ACKs are sent by the receiver to confirm receipt of a specific fragment.
func BuildAckFragment(seqNum uint32, fragmentID, dataShards, parityShards uint16) []byte {
	flags := FlagACK
	fragmentBytes := make([]byte, HeaderSize)

	fragmentBytes[0] = flags
	binary.BigEndian.PutUint32(fragmentBytes[1:5], seqNum)
	binary.BigEndian.PutUint16(fragmentBytes[5:7], fragmentID)
	binary.BigEndian.PutUint16(fragmentBytes[7:9], dataShards)
	binary.BigEndian.PutUint16(fragmentBytes[9:11], parityShards)

	// Checksum is computed over the header fields (no payload for ACKs).
	checksum := computeChecksum(flags, seqNum, fragmentID, dataShards, parityShards, nil)
	binary.BigEndian.PutUint32(fragmentBytes[11:15], checksum)

	return fragmentBytes
}

// ParseFragment converts a raw byte slice received over UDP into a Fragment struct.
// It performs a basic size check to ensure the slice can contain a valid header.
func ParseFragment(rawFragment []byte) (*Fragment, error) {
	if len(rawFragment) < HeaderSize {
		return nil, fmt.Errorf("fragment too small: %d bytes, minimum %d", len(rawFragment), HeaderSize)
	}

	return &Fragment{
		flags:        rawFragment[0],
		seqNum:       binary.BigEndian.Uint32(rawFragment[1:5]),
		fragmentID:   binary.BigEndian.Uint16(rawFragment[5:7]),
		dataShards:   binary.BigEndian.Uint16(rawFragment[7:9]),
		parityShards: binary.BigEndian.Uint16(rawFragment[9:11]),
		checksum:     binary.BigEndian.Uint32(rawFragment[11:15]),
		payload:      rawFragment[HeaderSize:],
	}, nil
}

// ValidateChecksum recomputes the fragment's checksum and compares it to the checksum field.
// This verifies the integrity of the fragment's data.
func (f *Fragment) ValidateChecksum() bool {
	computedChecksum := computeChecksum(f.flags, f.seqNum, f.fragmentID, f.dataShards, f.parityShards, f.payload)
	return computedChecksum == f.checksum
}

// computeChecksum calculates the CRC32 checksum over all fragment fields except the checksum itself.
func computeChecksum(flags uint8, seqNum uint32, fragmentID, dataShards, parityShards uint16, payload []byte) uint32 {
	// Prepare a byte slice to hold all data for checksum calculation.
	// The size is HeaderSize (15) - ChecksumSize (4) + payload length.
	checksumData := make([]byte, 0, HeaderSize-4+len(payload))

	// Use a temporary buffer for header fields to avoid multiple appends.
	headerBytes := make([]byte, HeaderSize-4)
	headerBytes[0] = flags
	binary.BigEndian.PutUint32(headerBytes[1:5], seqNum)
	binary.BigEndian.PutUint16(headerBytes[5:7], fragmentID)
	binary.BigEndian.PutUint16(headerBytes[7:9], dataShards)
	binary.BigEndian.PutUint16(headerBytes[9:11], parityShards)

	checksumData = append(checksumData, headerBytes...)

	if payload != nil {
		checksumData = append(checksumData, payload...)
	}

	return crc32.ChecksumIEEE(checksumData)
}

// IsAck returns true if the fragment is an acknowledgment.
func (f *Fragment) IsAck() bool {
	return f.flags&FlagACK != 0
}

// IsData returns true if the fragment contains data (either original or FEC).
func (f *Fragment) IsData() bool {
	return f.flags&FlagDATA != 0
}
