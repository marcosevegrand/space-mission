package udp

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
)

const (
	// Packet header constants
	HeaderSize int = 9 // 1 byte flags + 4 bytes sequence number + 4 bytes checksum

	// Flag bit masks
	FlagACK  uint8 = 0x01 // Acknowledgment packet
	FlagDATA uint8 = 0x02 // Data packet
)

// Packet represents a UDP reliability protocol packet
type Packet struct {
	Flags    uint8
	SeqNum   uint32
	Checksum uint32
	Payload  []byte
}

// BuildDataPacket builds a data packet with header and checksum
func BuildDataPacket(seqNum uint32, payload []byte) []byte {
	flags := FlagDATA
	packet := make([]byte, HeaderSize+len(payload))
	packet[0] = flags
	binary.BigEndian.PutUint32(packet[1:5], seqNum)
	copy(packet[9:], payload)

	// Compute checksum
	checksum := ComputeChecksum(flags, seqNum, payload)
	binary.BigEndian.PutUint32(packet[5:9], checksum)

	return packet
}

// BuildACKPacket builds an acknowledgment packet
func BuildACKPacket(seqNum uint32) []byte {
	flags := FlagACK
	packet := make([]byte, HeaderSize)
	packet[0] = flags
	binary.BigEndian.PutUint32(packet[1:5], seqNum)

	// Compute checksum (over flags + seqNum, no payload for ACK)
	checksum := ComputeChecksum(flags, seqNum, nil)
	binary.BigEndian.PutUint32(packet[5:9], checksum)

	return packet
}

// ParsePacket parses a raw UDP packet into a Packet struct
func ParsePacket(rawPacket []byte) (*Packet, error) {
	if len(rawPacket) < HeaderSize {
		return nil, fmt.Errorf("packet too small: %d bytes, minimum %d", len(rawPacket), HeaderSize)
	}

	flags := rawPacket[0]
	seqNum := binary.BigEndian.Uint32(rawPacket[1:5])
	checksum := binary.BigEndian.Uint32(rawPacket[5:9])
	payload := rawPacket[9:]

	return &Packet{
		Flags:    flags,
		SeqNum:   seqNum,
		Checksum: checksum,
		Payload:  payload,
	}, nil
}

// ValidateChecksum validates the checksum of a packet
func ValidateChecksum(packet *Packet) bool {
	computedChecksum := ComputeChecksum(packet.Flags, packet.SeqNum, packet.Payload)
	return computedChecksum == packet.Checksum
}

// ComputeChecksum computes CRC32 checksum over flags + seqNum + payload
func ComputeChecksum(flags uint8, seqNum uint32, payload []byte) uint32 {
	checksumData := make([]byte, 0, 5+len(payload))
	checksumData = append(checksumData, flags)

	seqNumBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(seqNumBytes, seqNum)
	checksumData = append(checksumData, seqNumBytes...)

	if payload != nil {
		checksumData = append(checksumData, payload...)
	}

	return crc32.ChecksumIEEE(checksumData)
}

// IsACK checks if packet is an acknowledgment
func (p *Packet) IsACK() bool {
	return p.Flags&FlagACK != 0
}

// IsDATA checks if packet is a data packet
func (p *Packet) IsDATA() bool {
	return p.Flags&FlagDATA != 0
}
