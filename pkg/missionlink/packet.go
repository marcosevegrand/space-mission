package missionlink

// Functions:
- SerializePacket(packet *MLPacket) ([]byte, error)
- DeserializePacket(data []byte) (*MLPacket, error)
- EncodePayload(payload interface{}) ([]byte, error)
- DecodePayload(data []byte, v interface{}) error

// // Implements:
// - Binary packet encoding (header + JSON payload)
// - Packet validation
// - Efficient byte packing
