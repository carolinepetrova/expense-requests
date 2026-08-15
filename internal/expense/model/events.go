package model

//go:generate go-enum --marshal --values -f=$GOFILE

import (
	"slices"
	"time"

	"github.com/carolinepetrova/expense-requests/internal/approval"
	"github.com/carolinepetrova/expense-requests/internal/user"
)

// ENUM(created, submitted, stepApproved, approved, rejected).
type EventType string

type Event struct {
	Type       EventType      `json:"type"`
	At         time.Time      `json:"at"`
	ActorID    user.ID        `json:"actorId"`
	ApproverID *user.ID       `json:"approverId,omitempty"`
	Steps      approval.Chain `json:"steps,omitempty"`
	StepIndex  *int           `json:"stepIndex,omitempty"`
	Comment    string         `json:"comment,omitempty"`
}

func ChainFrom(e Event) approval.Chain {
	if len(e.Steps) > 0 {
		return slices.Clone(e.Steps)
	}
	if e.ApproverID == nil {
		return nil
	}
	return approval.Chain{{
		Name:       RoleApprover,
		ApproverID: approval.Approver(*e.ApproverID),
		Status:     approval.StepStatusPending,
	}}
}
