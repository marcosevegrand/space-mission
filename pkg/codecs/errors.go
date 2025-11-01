package codecs

import "errors"

var (
	ErrUnsupportedMessageType = errors.New("unsupported message type")
	ErrInvalidMessageType     = errors.New("invalid message type")
	ErrPacketTooShort         = errors.New("packet too short")
	ErrInvalidShapeType       = errors.New("invalid shape type")
	ErrInvalidPacketSize      = errors.New("invalid packet size in length prefix")
)
