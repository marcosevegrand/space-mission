package telemetrycodec

import (
    "space-mission/pkg/codecs"
)

// TelemetryCodec is a compatibility alias to the newer codecs.TelemetryCodec
type TelemetryCodec = codecs.TelemetryCodec

// NewTelemetryCodec constructs a TelemetryCodec
func NewTelemetryCodec() *TelemetryCodec {
    return codecs.NewTelemetryCodec()
}
