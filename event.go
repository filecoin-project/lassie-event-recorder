package recorder

import "time"

type (
	PostRetrievalEventsRequest struct {
		Events []Event `json:"events"`
	}
	Event struct {
		RetrievalId       string    `json:"retrievalId"`
		InstanceId        string    `json:"instanceId"`
		Cid               string    `json:"cid"`
		StorageProviderId string    `json:"storageProviderId"`
		Phase             string    `json:"phase"`
		PhaseStartTime    time.Time `json:"phaseStartTime"`
		EventName         string    `json:"eventName"`
		EventTime         time.Time `json:"eventTime"`
	}
)
