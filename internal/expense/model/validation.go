package model

import "strings"

// Validation codes. The UI keys off these rather than off message text.
const (
	CodeRequired             = "required"
	CodeInvalid              = "invalid"
	CodeNegativeAmount       = "negative_amount"
	CodeRequiredWhenBillable = "required_when_billable"
	CodeRequiredWhenLarge    = "required_when_large"
	CodeRequiredWhenOther    = "required_when_other"

	// CodeUnknownClient is raised by the service, not here: whether a client
	// exists is a question for whoever owns customers, not for this domain.
	CodeUnknownClient = "unknown_client"
)

// Validate checks the form rules.
//
// It runs on submit only — a draft is allowed to be incomplete, which is the
// entire point of a draft. Every broken rule is reported rather than just the
// first, so the form can mark all of them in one pass.
//
// The last three rules are the conditional ones: a field is required only
// because of the value of another field.
func Validate(v Values) FieldErrors {
	var errs FieldErrors

	add := func(field, code, message string) {
		errs = append(errs, FieldError{Field: field, Code: code, Message: message})
	}

	if v.Type == "" {
		add("expenseType", CodeRequired, "Expense type is required.")
	} else if _, err := ParseType(string(v.Type)); err != nil {
		add("expenseType", CodeInvalid, "Unknown expense type.")
	}

	// Amount is whole cents. Negative is rejected outright; zero is treated as
	// not filled in, since a claim for nothing is an abandoned form.
	switch {
	case v.AmountCents < 0:
		add("amountCents", CodeNegativeAmount, "Amount cannot be negative.")
	case v.AmountCents == 0:
		add("amountCents", CodeRequired, "Amount is required.")
	}

	if blank(&v.Description) {
		add("description", CodeRequired, "Description is required.")
	}

	if v.Billable && blank(v.Client) {
		add("client", CodeRequiredWhenBillable,
			"A client is required when the expense is billable.")
	}

	if v.AmountCents >= ThresholdCents && blank(v.AdditionalJustification) {
		add("additionalJustification", CodeRequiredWhenLarge,
			"Expenses of $1,000 or more need additional justification.")
	}

	if v.Type == TypeOther && blank(v.OtherReason) {
		add("otherReason", CodeRequiredWhenOther,
			"Please say what this expense is for.")
	}

	return errs
}

// blank treats an absent value and a whitespace-only one the same way: the
// distinction matters on the wire, not to the rules.
func blank(s *string) bool {
	return s == nil || strings.TrimSpace(*s) == ""
}
