package tcpstream

import (
    "time"

    "space-mission/pkg/interfaces"
    tcp "space-mission/pkg/transport/tcp"
)

// Re-export Client type
type Client[T any] = tcp.Client[T]

// NewClient creates a tcp.Client using the old import path/signature expected by rover
func NewClient[T any](addr string, dialTimeout, writeTimeout, callInterval time.Duration, serialize func(T) ([]byte, error)) *Client[T] {
    // serialize already matches interfaces.Encoder[T]
    var enc interfaces.Encoder[T] = serialize
    return tcp.NewClient[T](addr, dialTimeout, writeTimeout, callInterval, enc)
}
