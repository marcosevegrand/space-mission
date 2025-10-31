// Package interfaces defines common interfaces and type definitions for the transport layer
package interfaces

// Serializer is a function type that converts a typed data structure into bytes
// Used by clients to serialize data before sending
type Encoder[T any] func(T) ([]byte, error)

// Deserializer is a function type that converts raw bytes into a typed data structure
// Used by servers to deserialize incoming packets
type Decoder[T any] func([]byte) (T, error)

// Handler is a callback function that processes deserialized data
// Used by servers to handle received data after deserialization
// Returns error if the packet should cause connection closure
type TCPHandler[T any] func(data T) error

// Handler is a callback function that processes deserialized data
// Used by servers to handle received data after deserialization
// Returns error if the packet should cause connection closure
type UDPHandler[T any] func(data T, addr string) error
