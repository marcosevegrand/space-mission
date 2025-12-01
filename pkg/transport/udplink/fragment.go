package udplink

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
)

const (
	// Header Field Sizes (in bytes)
	sizeFlags        = 1
	sizeSessionID    = 4
	sizeSeqNum       = 4
	sizeFragmentID   = 2
	sizeDataShards   = 2
	sizeParityShards = 2
	sizeChecksum     = 4

	// HeaderSize defines the total size of the fragment header.
	HeaderSize = sizeFlags + sizeSessionID + sizeSeqNum + sizeFragmentID + sizeDataShards + sizeParityShards + sizeChecksum

	// Header Offsets
	offsetFlags        = 0
	offsetSessionID    = offsetFlags + sizeFlags
	offsetSeqNum       = offsetSessionID + sizeSessionID
	offsetFragmentID   = offsetSeqNum + sizeSeqNum
	offsetDataShards   = offsetFragmentID + sizeFragmentID
	offsetParityShards = offsetDataShards + sizeDataShards
	offsetChecksum     = offsetParityShards + sizeParityShards
	offsetPayload      = offsetChecksum + sizeChecksum

	// Flags
	FlagACK  uint8 = 0x01 // Fragment is an acknowledgment
	FlagDATA uint8 = 0x02 // Fragment contains payload data
	FlagFEC  uint8 = 0x04 // Fragment is a parity shard
)

// Fragment represents a single UDP packet unit including protocol metadata.
type Fragment struct {
	flags        uint8
	sessionID    uint32
	seqNum       uint32
	fragmentID   uint16
	dataShards   uint16
	parityShards uint16
	checksum     uint32
	payload      []byte
}

// BuildDataFragment constructs a binary packet for data transmission.
func BuildDataFragment(sessionID, seqNum uint32, fragmentID, dataShards, parityShards uint16, isFEC bool, payload []byte) []byte {
	flags := FlagDATA
	if isFEC {
		flags |= FlagFEC
	}

	buf := make([]byte, HeaderSize+len(payload))

	// Write Header
	buf[offsetFlags] = flags
	binary.BigEndian.PutUint32(buf[offsetSessionID:], sessionID)
	binary.BigEndian.PutUint32(buf[offsetSeqNum:], seqNum)
	binary.BigEndian.PutUint16(buf[offsetFragmentID:], fragmentID)
	binary.BigEndian.PutUint16(buf[offsetDataShards:], dataShards)
	binary.BigEndian.PutUint16(buf[offsetParityShards:], parityShards)

	// Compute and Write Checksum
	// We pass 'buf' (header so far) and 'payload' separately to avoid an extra allocation
	checksum := computeChecksum(buf[:offsetChecksum], payload)
	binary.BigEndian.PutUint32(buf[offsetChecksum:], checksum)

	// Write Payload
	copy(buf[offsetPayload:], payload)

	return buf
}

// BuildAckFragment constructs a binary packet for acknowledgment.
func BuildAckFragment(sessionID, seqNum uint32, fragmentID, dataShards, parityShards uint16) []byte {
	flags := FlagACK
	buf := make([]byte, HeaderSize)

	buf[offsetFlags] = flags
	binary.BigEndian.PutUint32(buf[offsetSessionID:], sessionID)
	binary.BigEndian.PutUint32(buf[offsetSeqNum:], seqNum)
	binary.BigEndian.PutUint16(buf[offsetFragmentID:], fragmentID)
	binary.BigEndian.PutUint16(buf[offsetDataShards:], dataShards)
	binary.BigEndian.PutUint16(buf[offsetParityShards:], parityShards)

	checksum := computeChecksum(buf[:offsetChecksum], nil)
	binary.BigEndian.PutUint32(buf[offsetChecksum:], checksum)

	return buf
}

// ParseFragment decodes a raw byte slice into a Fragment struct.
func ParseFragment(raw []byte) (*Fragment, error) {
	if len(raw) < HeaderSize {
		return nil, fmt.Errorf("fragment too small: got %d bytes, need %d", len(raw), HeaderSize)
	}

	return &Fragment{
		flags:        raw[offsetFlags],
		sessionID:    binary.BigEndian.Uint32(raw[offsetSessionID:]),
		seqNum:       binary.BigEndian.Uint32(raw[offsetSeqNum:]),
		fragmentID:   binary.BigEndian.Uint16(raw[offsetFragmentID:]),
		dataShards:   binary.BigEndian.Uint16(raw[offsetDataShards:]),
		parityShards: binary.BigEndian.Uint16(raw[offsetParityShards:]),
		checksum:     binary.BigEndian.Uint32(raw[offsetChecksum:]),
		payload:      raw[offsetPayload:],
	}, nil
}

// ValidateChecksum verifies data integrity.
func (f *Fragment) ValidateChecksum() bool {
	// Reconstruct the header bytes for checksum calculation
	// Note: In a highly optimized hot path, you might pass the raw buffer to this method
	// to avoid rebuilding it, but this is safe and clean.
	header := make([]byte, offsetChecksum)
	header[offsetFlags] = f.flags
	binary.BigEndian.PutUint32(header[offsetSessionID:], f.sessionID)
	binary.BigEndian.PutUint32(header[offsetSeqNum:], f.seqNum)
	binary.BigEndian.PutUint16(header[offsetFragmentID:], f.fragmentID)
	binary.BigEndian.PutUint16(header[offsetDataShards:], f.dataShards)
	binary.BigEndian.PutUint16(header[offsetParityShards:], f.parityShards)

	expected := computeChecksum(header, f.payload)
	return expected == f.checksum
}

func computeChecksum(headerPartial []byte, payload []byte) uint32 {
	c := crc32.NewIEEE()
	c.Write(headerPartial)
	if payload != nil {
		c.Write(payload)
	}
	return c.Sum32()
}

func (f *Fragment) IsAck() bool  { return f.flags&FlagACK != 0 }
func (f *Fragment) IsData() bool { return f.flags&FlagDATA != 0 }
