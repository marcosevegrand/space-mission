// Package interfaces defines common interfaces and type definitions for the transport layer
package interfaces

// Source is a function type that generates a value of type T
// Used by clients or peers to provide data for transmission
type Source[T any] func() (T, error)

// Encoder is a function type that converts a typed data structure into bytes
// Used by clients or peersto serialize data before sending
type Encoder[T any] func(T) ([]byte, error)

// Decoder is a function type that converts raw bytes into a typed data structure
// Used by servers or peers to deserialize incoming packets
type Decoder[T any] func([]byte) (T, error)

// Handler is a callback function that processes deserialized data
type Handler[T any] func(data T, senderAddr string) error
