package model

//go:generate go-enum --marshal --values -f=$GOFILE

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/carolinepetrova/expense-requests/internal/user"
)

const ThresholdCents int64 = 100_000

var (
	ErrNotFound          = errors.New("request not found")
	ErrForbidden         = errors.New("forbidden")
	ErrInvalidTransition = errors.New("invalid transition")
	ErrConflict          = errors.New("modified concurrently")
)

type ID string

func NewID() ID {
	var b [6]byte
	_, _ = rand.Read(b[:])

	return ID("REQ-" + strings.ToUpper(hex.EncodeToString(b[:])))
}

// ENUM(Draft, Submitted, Approved, Rejected).
type Status string

func (x Status) Terminal() bool {
	return x == StatusApproved || x == StatusRejected
}

// ENUM(Travel, Software, Equipment, Meal, Other).
type Type string

type Values struct {
	Type                    Type    `json:"expenseType"`
	AmountCents             int64   `json:"amountCents"`
	Description             string  `json:"description"`
	Billable                bool    `json:"billable"`
	Client                  *string `json:"client,omitempty"`
	AdditionalJustification *string `json:"additionalJustification,omitempty"`
	OtherReason             *string `json:"otherReason,omitempty"`
}

type FieldError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type FieldErrors []FieldError

func (fe FieldErrors) Error() string {
	return fmt.Sprintf("validation failed on %d field(s)", len(fe))
}

func (fe FieldErrors) OrNil() error {
	if len(fe) == 0 {
		return nil
	}
	return fe
}

type Record struct {
	ID          ID
	RequesterID user.ID
	Values      Values
	Events      []Event

	Version int
}
