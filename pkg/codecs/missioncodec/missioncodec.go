package missioncodec

import (
    "fmt"
    "time"

    "space-mission/pkg/codecs"
    "space-mission/pkg/models"
)

// Re-export message type constants for compatibility
const (
    MessageTypeMissionRequest    = codecs.MessageTypeMissionRequest
    MessageTypeMissionAssignment = codecs.MessageTypeMissionAssignment
    MessageTypeProgressUpdate    = codecs.MessageTypeProgressUpdate
)

// Wrapper types expected by legacy rover code
type MissionRequest = models.MissionRequest

type MissionAssignment struct {
    RoverID        uint16
    MissionID      uint16
    GeographicArea models.GeographicArea
    Task           models.Task
    MaxDuration    time.Duration
    UpdateInterval time.Duration
    Timestamp      time.Time
}

type ProgressUpdate struct {
    RoverID         uint16
    MissionID       uint16
    Status          models.MissionStatus
    Progress        float32
    CurrentPosition models.Position
    Timestamp       time.Time
}

// MissionLinkMessage is the legacy envelope used by rover code
type MissionLinkMessage struct {
    MessageType uint8
    Payload     interface{}
}

// MissionCodec is a thin shim around codecs.MissionCodec
type MissionCodec struct {
    inner *codecs.MissionCodec
}

func NewMissionCodec() *MissionCodec {
    return &MissionCodec{inner: codecs.NewMissionCodec()}
}

// Serialize converts the legacy envelope into bytes by mapping to models types
func (c *MissionCodec) Serialize(msg MissionLinkMessage) ([]byte, error) {
    switch p := msg.Payload.(type) {
    case MissionRequest:
        return c.inner.Serialize(models.MissionRequest{RoverID: p.RoverID, Timestamp: p.Timestamp})
    case MissionAssignment:
        // Map to models.MissionAssignment using fields from the legacy struct
        m := models.Mission{
            ID:             p.MissionID,
            Task:           p.Task,
            GeographicArea: p.GeographicArea,
            Status:         models.MissionInProgress,
            Progress:       0,
        }
        ma := models.MissionAssignment{
            RoverID:        p.RoverID,
            MaxDuration:    p.MaxDuration,
            UpdateInterval: p.UpdateInterval,
            Mission:        m,
            Timestamp:      p.Timestamp,
        }
        return c.inner.Serialize(ma)
    case ProgressUpdate:
        mu := models.ProgressUpdate{
            RoverID:   p.RoverID,
            MissionID: p.MissionID,
            Status:    p.Status,
            Progress:  p.Progress,
            Content:   "",
            Timestamp: p.Timestamp,
        }
        return c.inner.Serialize(mu)
    default:
        return nil, fmt.Errorf("missioncodec: unsupported payload type: %T", p)
    }
}

// Deserialize converts bytes into the legacy envelope by mapping from models types
func (c *MissionCodec) Deserialize(data []byte) (MissionLinkMessage, error) {
    mm, err := c.inner.Deserialize(data)
    if err != nil {
        return MissionLinkMessage{}, err
    }

    switch v := mm.(type) {
    case models.MissionRequest:
        return MissionLinkMessage{MessageType: MessageTypeMissionRequest, Payload: MissionRequest(v)}, nil
    case models.MissionAssignment:
        // Map models.MissionAssignment -> legacy MissionAssignment
        pa := MissionAssignment{
            RoverID:        v.RoverID,
            MissionID:      v.Mission.ID,
            GeographicArea: v.Mission.GeographicArea,
            Task:           v.Mission.Task,
            MaxDuration:    v.MaxDuration,
            UpdateInterval: v.UpdateInterval,
            Timestamp:      v.Timestamp,
        }
        return MissionLinkMessage{MessageType: MessageTypeMissionAssignment, Payload: pa}, nil
    case models.ProgressUpdate:
        pu := ProgressUpdate{
            RoverID:   v.RoverID,
            MissionID: v.MissionID,
            Status:    v.Status,
            Progress:  v.Progress,
            // CurrentPosition is not part of models.ProgressUpdate - leave zero value
            Timestamp: v.Timestamp,
        }
        return MissionLinkMessage{MessageType: MessageTypeProgressUpdate, Payload: pu}, nil
    default:
        return MissionLinkMessage{}, fmt.Errorf("missioncodec: unexpected message type: %T", v)
    }
}
