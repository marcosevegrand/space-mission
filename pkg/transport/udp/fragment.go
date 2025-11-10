package udp

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
)

const (
	// HeaderSize defines the total size of the frag header in bytes:
	// 1 byte for flags, 4 bytes for sequence number, 2 byte for fragment ID,
	// 2 byte for total fragments, and 4 bytes for checksum.
	HeaderSize int = 13

	// FlagACK indicates that the frag is an acknowledgment.
	FlagACK uint8 = 0x01

	// FlagDATA indicates that the frag contains data.
	FlagDATA uint8 = 0x02
)

// frag represents the structure of a UDP reliability protocol frag,
// including flags, sequence number, checksum, and optional payload.
type Fragment struct {
	flags    uint8  // frag type flags (ACK, DATA, etc.)
	seqNum   uint32 // Sequence number to identify frags
	fragID   uint16 // Fragment number to identify fragments
	numFrags uint16 // Total number of fragments in the frag
	checksum uint32 // CRC32 checksum over header and payload for integrity
	payload  []byte // Payload data for data frags
}

// BuildDatafrag constructs a byte slice representing a data frag
// with header fields and checksum, ready for transmission.
func BuildDataFrag(seqNum uint32, fragID uint16, numFrags uint16, payload []byte) []byte {
	flags := FlagDATA
	// Allocate byte slice with space for header and payload
	frag := make([]byte, HeaderSize+len(payload))

	// Set flags byte
	frag[0] = flags

	// Encode sequence number using big-endian byte order
	binary.BigEndian.PutUint32(frag[1:5], seqNum)

	// Encode fragment ID and total fragments using big-endian byte order
	binary.BigEndian.PutUint16(frag[5:7], fragID)
	binary.BigEndian.PutUint16(frag[7:9], numFrags)

	// Payload placed immediately after header
	copy(frag[HeaderSize:], payload)

	// Calculate checksum over flags, sequence number, fragment ID, total fragments, and payload
	checksum := ComputeChecksum(flags, seqNum, fragID, numFrags, payload)

	// Store checksum in header in big-endian format
	binary.BigEndian.PutUint32(frag[9:13], checksum)

	return frag
}

// BuildAckfrag constructs a byte slice representing an acknowledgment frag
// with header fields and checksum, containing no payload.
func BuildAckFrag(seqNum uint32, fragID uint16, numFrags uint16) []byte {
	flags := FlagACK
	frag := make([]byte, HeaderSize)

	frag[0] = flags
	binary.BigEndian.PutUint32(frag[1:5], seqNum)
	binary.BigEndian.PutUint16(frag[5:7], fragID)
	binary.BigEndian.PutUint16(frag[7:9], numFrags)

	// Checksum computed only over flags, sequence number, and fragment ID (no payload)
	checksum := ComputeChecksum(flags, seqNum, fragID, numFrags, nil)
	binary.BigEndian.PutUint32(frag[9:13], checksum)

	return frag
}

// Parsefrag converts a raw byte slice received over UDP into a frag struct,
// validating that the slice is at least as long as the header.
func ParseFrag(rawFrag []byte) (*Fragment, error) {
	if len(rawFrag) < HeaderSize {
		return nil, fmt.Errorf("frag too small: %d bytes, minimum %d", len(rawFrag), HeaderSize)
	}

	flags := rawFrag[0]
	seqNum := binary.BigEndian.Uint32(rawFrag[1:5])
	fragID := binary.BigEndian.Uint16(rawFrag[5:7])
	numFrags := binary.BigEndian.Uint16(rawFrag[7:9])
	checksum := binary.BigEndian.Uint32(rawFrag[9:13])
	payload := rawFrag[HeaderSize:]

	return &Fragment{
		flags:    flags,
		seqNum:   seqNum,
		fragID:   fragID,
		numFrags: numFrags,
		checksum: checksum,
		payload:  payload,
	}, nil
}

// ValidateChecksum recomputes the checksum and compares it to the frag's checksum field,
// returning true if they match and the frag data is intact.
func ValidateChecksum(frag *Fragment) bool {
	computedChecksum := ComputeChecksum(frag.flags, frag.seqNum, frag.fragID, frag.numFrags, frag.payload)
	return computedChecksum == frag.checksum
}

// ComputeChecksum calculates the CRC32 checksum over the frag fields:
// flags, sequence number, fragment ID, total fragments, and optional payload bytes.
func ComputeChecksum(flags uint8, seqNum uint32, fragID uint16, numFrags uint16, payload []byte) uint32 {
	// Prepare a byte slice to accumulate checksum input data efficiently
	checksumData := make([]byte, 0, HeaderSize-4+len(payload))

	// Convert header fields to big-endian byte slices and append
	headerBytes := make([]byte, HeaderSize-4)
	headerBytes[0] = flags
	binary.BigEndian.PutUint32(headerBytes[1:5], seqNum)
	binary.BigEndian.PutUint16(headerBytes[5:7], fragID)
	binary.BigEndian.PutUint16(headerBytes[7:9], numFrags)
	checksumData = append(checksumData, headerBytes...)

	// Append payload if present
	if payload != nil {
		checksumData = append(checksumData, payload...)
	}

	// Compute and return CRC32 checksum using IEEE polynomial
	return crc32.ChecksumIEEE(checksumData)
}

// IsAck returns true if the frag's flags indicate it is an acknowledgment.
func (p *Fragment) IsAck() bool {
	return p.flags&FlagACK != 0
}

// IsData returns true if the frag's flags indicate it is a data frag.
func (p *Fragment) IsData() bool {
	return p.flags&FlagDATA != 0
}
