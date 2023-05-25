package eventrecorder

import (
	"errors"
	"fmt"
	"time"

	"github.com/filecoin-project/lassie/pkg/types"
	"github.com/ipfs/go-cid"
)

var (
	errInvalidPhase     = fmt.Errorf("phase must be one of: [%s %s %s]", types.IndexerPhase, types.QueryPhase, types.RetrievalPhase)
	errInvalidEventCode error
	emptyRetrievalID    types.RetrievalID
	eventCodes          = map[types.EventCode]any{
		types.CandidatesFoundCode:    nil,
		types.CandidatesFilteredCode: nil,
		types.StartedCode:            nil,
		types.ConnectedCode:          nil,
		types.QueryAskedCode:         nil,
		types.QueryAskedFilteredCode: nil,
		types.ProposedCode:           nil,
		types.AcceptedCode:           nil,
		types.FirstByteCode:          nil,
		types.FailedCode:             nil,
		types.SuccessCode:            nil,
	}
)

func init() {
	codes := make([]types.EventCode, 0, len(eventCodes))
	for code := range eventCodes {
		codes = append(codes, code)
	}
	errInvalidEventCode = fmt.Errorf("eventName must be one of: %v", codes)
}

type Event struct {
	RetrievalId       types.RetrievalID `json:"retrievalId"`
	InstanceId        string            `json:"instanceId,omitempty"`
	Cid               string            `json:"cid"`
	StorageProviderId string            `json:"storageProviderId"`
	Phase             types.Phase       `json:"phase"`
	PhaseStartTime    time.Time         `json:"phaseStartTime"`
	EventName         types.EventCode   `json:"eventName"`
	EventTime         time.Time         `json:"eventTime"`
	EventDetails      any               `json:"eventDetails,omitempty"`
}

func (e Event) Validate() error {
	switch {
	case e.RetrievalId == emptyRetrievalID:
		return errors.New("property retrievalId is required")
	case e.InstanceId == "":
		return errors.New("property instanceId is required")
	case e.Cid == "":
		return errors.New("property cid is required")
	case e.Phase == "":
		return errors.New("property phase is required")
	case !validPhase(e.Phase):
		return errInvalidPhase
	case e.PhaseStartTime.IsZero():
		return errors.New("property phaseStartTime is required")
	case e.PhaseStartTime.After(time.Now().Add(24 * time.Hour)):
		return errors.New("property phaseStartTime cannot be in the future")
	case e.EventName == "":
		return errors.New("property eventName is required")
	case !validEventCode(e.EventName):
		return errInvalidEventCode
	case e.EventTime.IsZero():
		return errors.New("property eventTime is required")
	case e.EventTime.After(time.Now().Add(24 * time.Hour)):
		return errors.New("property eventTime cannot be in the future")
	default:
		_, err := cid.Decode(e.Cid)
		if err != nil {
			return fmt.Errorf("cid must be valid: %w", err)
		}
		// a few non rejecting weird cases we want to write a log about to monitor
		switch {
		case e.PhaseStartTime.After(time.Now()):
			logger.Warnf("phaseStartTime (%s) ahead of current time (%s) for event %s, source %s",
				e.PhaseStartTime, time.Now(), e.EventName, e.InstanceId)
		case e.EventTime.After(time.Now()):
			logger.Warnf("eventTime (%s) ahead of current time (%s) for event %s, source %s",
				e.EventTime, time.Now(), e.EventName, e.InstanceId)
		}
		return nil
	}
}

func validPhase(phase types.Phase) bool {
	switch phase {
	case types.IndexerPhase, types.QueryPhase, types.RetrievalPhase:
		return true
	default:
		return false
	}
}

func validEventCode(code types.EventCode) bool {
	_, ok := eventCodes[code]
	return ok
}

type EventBatch struct {
	Events []Event `json:"events"`
}

func (e EventBatch) Validate() error {
	if len(e.Events) == 0 {
		return errors.New("property events is required")
	}
	for _, event := range e.Events {
		if err := event.Validate(); err != nil {
			return err
		}
	}
	return nil
}

type RetrievalAttempt struct {
	Error           string `json:"error,omitempty"`
	TimeToFirstByte string `json:"timeToFirstByte,omitempty"`
}

type AggregateEvent struct {
	InstanceID        string    `json:"instanceId"`                  // The ID of the Lassie instance generating the event
	RetrievalID       string    `json:"retrievalId"`                 // The unique ID of the retrieval
	StorageProviderID string    `json:"storageProviderId,omitempty"` // The ID of the storage provider that served the retrieval content
	TimeToFirstByte   string    `json:"timeToFirstByte,omitempty"`   // The time it took to receive the first byte in milliseconds
	Bandwidth         uint64    `json:"bandwidth,omitempty"`         // The bandwidth of the retrieval in bytes per second
	BytesTransferred  uint64    `json:"bytesTransferred,omitempty"`  // The total transmitted deal size
	Success           bool      `json:"success"`                     // Wether or not the retreival ended with a success event
	StartTime         time.Time `json:"startTime"`                   // The time the retrieval started
	EndTime           time.Time `json:"endTime"`                     // The time the retrieval ended

	TimeToFirstIndexerResult  string                       `json:"timeToFirstIndexerResult,omitempty"` // time it took to receive our first "CandidateFound" event
	IndexerCandidatesReceived int                          `json:"indexerCandidatesReceived"`          // The number of candidates received from the indexer
	IndexerCandidatesFiltered int                          `json:"indexerCandidatesFiltered"`          // The number of candidates that made it through the filtering stage
	ProtocolsAllowed          []string                     `json:"protocolsAllowed,omitempty"`         // The available protocols that could be used for this retrieval
	ProtocolsAttempted        []string                     `json:"protocolsAttempted,omitempty"`       // The protocols that were used to attempt this retrieval
	ProtocolSucceeded         string                       `json:"protocolSucceeded,omitempty"`        // The protocol used for a successful event
	RetrievalAttempts         map[string]*RetrievalAttempt `json:"retrievalAttempts,omitempty"`        // All of the retrieval attempts, indexed by their SP ID
}

func (e AggregateEvent) Validate() error {
	switch {
	case e.RetrievalID == "":
		return errors.New("property retrievalId is required")
	case e.InstanceID == "":
		return errors.New("property instanceId is required")
	case e.StartTime.IsZero():
		return errors.New("property startTime is required")
	case e.EndTime.IsZero():
		return errors.New("property endTime is required")
	case e.EndTime.Before(e.StartTime):
		return errors.New("property endTime cannot be before startTime")
	default:
		if e.TimeToFirstByte != "" {
			_, err := time.ParseDuration(e.TimeToFirstByte)
			if err != nil {
				return err
			}
		}
		if e.TimeToFirstIndexerResult != "" {
			_, err := time.ParseDuration(e.TimeToFirstIndexerResult)
			if err != nil {
				return err
			}
		}
		for _, retrievalAttempt := range e.RetrievalAttempts {
			if retrievalAttempt == nil {
				return errors.New("all retrieval attempts should have values")
			}
			if retrievalAttempt.TimeToFirstByte != "" {
				_, err := time.ParseDuration(retrievalAttempt.TimeToFirstByte)
				if err != nil {
					return err
				}
			}
		}
		return nil
	}
}

type AggregateEventBatch struct {
	Events []AggregateEvent `json:"events"`
}

func (e AggregateEventBatch) Validate() error {
	if len(e.Events) == 0 {
		return errors.New("property events is required")
	}
	for _, event := range e.Events {
		if err := event.Validate(); err != nil {
			return err
		}
	}
	return nil
}
