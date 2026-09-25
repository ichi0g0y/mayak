package hideoutlog

import (
	"crypto/sha256"
	"fmt"
	"time"
)

type Identity struct {
	AccountID string `json:"accountId"`
	ProfileID string `json:"profileId"`
	Mode      string `json:"mode"`
}

// Event contains only allowlisted diagnostics. Never retain surrounding log text.
type Event struct {
	Identity
	Hidden          bool      `json:"hidden,omitempty"`
	StationName     string    `json:"stationName"`
	Kind            string    `json:"kind"`
	Action          string    `json:"action"`
	AreaType        int       `json:"areaType"`
	ActionTimestamp int64     `json:"actionTimestamp"`
	Source          string    `json:"source"`
	OccurredAt      time.Time `json:"occurredAt"`
	Status          string    `json:"status"`
	Confidence      string    `json:"confidence"`
	Historical      bool      `json:"historical"`
}

func (e Event) Fingerprint() string {
	stamp := fmt.Sprint(e.ActionTimestamp)
	if e.ActionTimestamp == 0 {
		stamp = e.OccurredAt.Format(time.RFC3339Nano)
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%d\x00%s\x00%s", e.AccountID, e.ProfileID, e.Mode, e.Action, e.AreaType, stamp, e.Kind))))
}

func (e Event) Actionable() bool { return e.Status == "failed" }
