package eventrecorder

import (
	"errors"
	"fmt"
	"time"

	"github.com/filecoin-project/lassie/pkg/types"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
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
	case e.PhaseStartTime.After(time.Now()):
		return errors.New("property phaseStartTime cannot be in the future")
	case e.EventName == "":
		return errors.New("property eventName is required")
	case !validEventCode(e.EventName):
		return errInvalidEventCode
	case e.EventTime.IsZero():
		return errors.New("property eventTime is required")
	case e.EventTime.After(time.Now()):
		return errors.New("property eventTime cannot be in the future")
	default:
		_, err := cid.Decode(e.Cid)
		if err != nil {
			return fmt.Errorf("cid must be valid: %w", err)
		}
		if e.StorageProviderId != "" {
			if _, err := peer.Decode(e.StorageProviderId); err != nil {
				return fmt.Errorf("storageProviderId must be valid: %w", err)
			}
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
