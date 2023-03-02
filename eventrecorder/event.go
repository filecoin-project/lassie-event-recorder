package eventrecorder

import (
	"errors"
	"fmt"
	"time"

	types "github.com/filecoin-project/lassie/pkg/types"
	"github.com/google/uuid"
)

var (
	validPhases     = []string{"indexer", "query", "retrieval"}
	validEventNames = []string{
		"accepted",
		"candidates-filtered",
		"candidates-found",
		"connected",
		"failure",
		"first-byte-received",
		"proposed",
		"query-asked",
		"query-asked-filtered",
		"started",
		"success",
	}
)

type event struct {
	RetrievalId       *types.RetrievalID `json:"retrievalId"`
	InstanceId        *string            `json:"instanceId,omitempty"`
	Cid               *string            `json:"cid"`
	StorageProviderId *string            `json:"storageProviderId"`
	Phase             *types.Phase       `json:"phase"`
	PhaseStartTime    *time.Time         `json:"phaseStartTime"`
	EventName         *types.EventCode   `json:"eventName"`
	EventTime         *time.Time         `json:"eventTime"`
	EventDetails      interface{}        `json:"eventDetails,omitempty"`
}

func (e event) validate() error {
	// RetrievalId
	if e.RetrievalId == nil {
		return errors.New("Property retrievalId is required")
	}
	if _, err := uuid.Parse(e.RetrievalId.String()); err != nil {
		return errors.New("Property retrievalId should be a valud v4 uuid")
	}

	// InstanceId
	if e.InstanceId == nil {
		return errors.New("Property instanceId is required")
	}

	// Cid
	if e.Cid == nil {
		return errors.New("Property cid is required")
	}

	// StorageProviderId
	if e.StorageProviderId == nil {
		return errors.New("Property storageProviderId is required")
	}

	// Phase
	if e.Phase == nil {
		return errors.New("Property phase is required")
	}
	isValidPhase := false
	for _, phase := range validPhases {
		if string(*e.Phase) == phase {
			isValidPhase = true
			break
		}
	}
	if !isValidPhase {
		return errors.New(fmt.Sprintf("Property phase failed validation. Phase must be created with one of the following values: %v", validPhases))
	}

	// PhaseStartTime
	if e.PhaseStartTime == nil {
		return errors.New("Property phaseStartTime is required")
	}

	// EventName
	if e.EventName == nil {
		return errors.New("Property eventName is required")
	}
	isValidEventName := false
	for _, phase := range validEventNames {
		if string(*e.EventName) == phase {
			isValidEventName = true
			break
		}
	}
	if !isValidEventName {
		return errors.New(fmt.Sprintf("Property eventName failed validation. Event name must be created with one of the following values: %v", validEventNames))
	}

	// EventTime
	if e.EventTime == nil {
		return errors.New("Property eventTime is required")
	}

	return nil
}

type eventBatch struct {
	Events []event `json:"events"`
}

func (e eventBatch) validate() error {
	if e.Events == nil {
		return errors.New("Property events is required")
	}

	for _, event := range e.Events {
		if err := event.validate(); err != nil {
			return err
		}
	}

	return nil
}
