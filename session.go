package codeharvest

import (
	"encoding/json"
)

// Session is the raw representation of a coding session. These
// sessions are stored temporarily on disk, and are later merged
// by the day they occurred, and moved to a database.
type Session struct {
	StartedAt  int64  `json:"started_at"`
	EndedAt    int64  `json:"ended_at"`
	DurationMs int64  `json:"duration_ms"`
	OS         string `json:"os"`
	Editor     string `json:"editor"`
	Files      Files  `json:"files"`
}

func (session Session) TotalFileDuration() int64 {
	totalDuration := int64(0)
	for _, file := range session.Files {
		totalDuration += file.DurationMs
	}
	return totalDuration
}

func (session Session) Serialize() ([]byte, error) {
	return json.MarshalIndent(session, "", "  ")
}
