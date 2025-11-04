package udp

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
)

const (
	// HeaderSize defines the total size of the packet header in bytes:
	// 1 byte for flags, 4 bytes for sequence number, and 4 bytes for checksum.
	HeaderSize int = 9

	// FlagACK indicates that the packet is an acknowledgment.
	FlagACK uint8 = 0x01

	// FlagDATA indicates that the packet contains data.
	FlagDATA uint8 = 0x02
)

// Packet represents the structure of a UDP reliability protocol packet,
// including flags, sequence number, checksum, and optional payload.
type Packet struct {
	Flags    uint8  // Packet type flags (ACK, DATA, etc.)
	SeqNum   uint32 // Sequence number to identify packets
	Checksum uint32 // CRC32 checksum over header and payload for integrity
	Payload  []byte // Payload data for data packets
}

// BuildDataPacket constructs a byte slice representing a data packet
// with header fields and checksum, ready for transmission.
func BuildDataPacket(seqNum uint32, payload []byte) []byte {
	flags := FlagDATA
	// Allocate byte slice with space for header and payload
	packet := make([]byte, HeaderSize+len(payload))

	// Set flags byte
	packet[0] = flags

	// Encode sequence number using big-endian byte order
	binary.BigEndian.PutUint32(packet[1:5], seqNum)

	// Payload placed immediately after header
	copy(packet[HeaderSize:], payload)

	// Calculate checksum over flags, sequence number, and payload
	checksum := ComputeChecksum(flags, seqNum, payload)

	// Store checksum in header in big-endian format
	binary.BigEndian.PutUint32(packet[5:9], checksum)

	return packet
}

// BuildACKPacket constructs a byte slice representing an acknowledgment packet
// with header fields and checksum, containing no payload.
func BuildACKPacket(seqNum uint32) []byte {
	flags := FlagACK
	packet := make([]byte, HeaderSize)

	packet[0] = flags
	binary.BigEndian.PutUint32(packet[1:5], seqNum)

	// Checksum computed only over flags and sequence number (no payload)
	checksum := ComputeChecksum(flags, seqNum, nil)
	binary.BigEndian.PutUint32(packet[5:9], checksum)

	return packet
}

// ParsePacket converts a raw byte slice received over UDP into a Packet struct,
// validating that the slice is at least as long as the header.
func ParsePacket(rawPacket []byte) (*Packet, error) {
	if len(rawPacket) < HeaderSize {
		return nil, fmt.Errorf("packet too small: %d bytes, minimum %d", len(rawPacket), HeaderSize)
	}

	flags := rawPacket[0]
	seqNum := binary.BigEndian.Uint32(rawPacket[1:5])
	checksum := binary.BigEndian.Uint32(rawPacket[5:9])
	payload := rawPacket[HeaderSize:]

	return &Packet{
		Flags:    flags,
		SeqNum:   seqNum,
		Checksum: checksum,
		Payload:  payload,
	}, nil
}

// ValidateChecksum recomputes the checksum and compares it to the packet's checksum field,
// returning true if they match and the packet data is intact.
func ValidateChecksum(packet *Packet) bool {
	computedChecksum := ComputeChecksum(packet.Flags, packet.SeqNum, packet.Payload)
	return computedChecksum == packet.Checksum
}

// ComputeChecksum calculates the CRC32 checksum over the packet fields:
// flags, sequence number, and optional payload bytes.
func ComputeChecksum(flags uint8, seqNum uint32, payload []byte) uint32 {
	// Prepare a byte slice to accumulate checksum input data efficiently
	checksumData := make([]byte, 0, 5+len(payload))

	// Append flags byte
	checksumData = append(checksumData, flags)

	// Convert seqNum to 4-byte big-endian and append
	seqNumBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(seqNumBytes, seqNum)
	checksumData = append(checksumData, seqNumBytes...)

	// Append payload if present
	if payload != nil {
		checksumData = append(checksumData, payload...)
	}

	// Compute and return CRC32 checksum using IEEE polynomial
	return crc32.ChecksumIEEE(checksumData)
}

// IsACK returns true if the packet's flags indicate it is an acknowledgment.
func (p *Packet) IsACK() bool {
	return p.Flags&FlagACK != 0
}

// IsDATA returns true if the packet's flags indicate it is a data packet.
func (p *Packet) IsDATA() bool {
	return p.Flags&FlagDATA != 0
}
