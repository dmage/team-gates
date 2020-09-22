package bugzilla

import (
	"encoding/json"
	"time"
)

type Time struct {
	time.Time
}

func (t *Time) UnmarshalJSON(data []byte) error {
	var s string
	err := json.Unmarshal(data, &s)
	if err != nil {
		return err
	}

	t.Time, err = time.Parse(time.RFC3339, s)
	return err
}

type Bug struct {
	CreationTime  Time     `json:"creation_time"`
	Severity      string   `json:"severity"`
	Status        string   `json:"status"`
	TargetRelease []string `json:"target_release"`
}
