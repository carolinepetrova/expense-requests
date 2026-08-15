package model

//go:generate go-enum --marshal --values -f=$GOFILE

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

const ThresholdCents int64 = 100_000

var (
	ErrNotFound          = errors.New("request not found")
	ErrForbidden         = errors.New("forbidden")
	ErrInvalidTransition = errors.New("invalid transition")
)

type ID string

// NewID generates an identifier for a new request. The seed data uses REQ-001
// and similar; generated ones keep the prefix.
func NewID() ID {
	var b [6]byte
	_, _ = rand.Read(b[:])

	return ID("REQ-" + strings.ToUpper(hex.EncodeToString(b[:])))
}

// ENUM(Draft, Submitted, Approved, Rejected).
type Status string

// Terminal reports whether a request can never change again. Rejection is
// final: there is no fix-and-resubmit flow, so the state machine has no cycles.
func (x Status) Terminal() bool {
	return x == StatusApproved || x == StatusRejected
}

// ENUM(Travel, Software, Equipment, Meal, Other).
type Type string

// Values is the form payload: everything the requester controls, and the only
// part of a request a client is ever allowed to send.
//
// The three conditional fields are pointers so that "not filled in" and
// "filled in as empty" stay distinguishable on the wire.
type Values struct {
	Type                    Type    `json:"expenseType"`
	AmountCents             int64   `json:"amountCents"`
	Description             string  `json:"description"`
	Billable                bool    `json:"billable"`
	Client                  *string `json:"client,omitempty"`
	AdditionalJustification *string `json:"additionalJustification,omitempty"`
	OtherReason             *string `json:"otherReason,omitempty"`
}

// FieldError names a broken form rule. Code is a stable identifier the UI keys
// off; Message is for people.
type FieldError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// FieldErrors is returned as an error so validation travels the ordinary error
// path, and is unwrapped into a 422 body at the HTTP boundary.
type FieldErrors []FieldError

func (fe FieldErrors) Error() string {
	return fmt.Sprintf("validation failed on %d field(s)", len(fe))
}

// OrNil collapses an empty slice to a nil error.
func (fe FieldErrors) OrNil() error {
	if len(fe) == 0 {
		return nil
	}
	return fe
}
