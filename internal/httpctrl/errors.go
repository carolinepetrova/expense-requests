package httpctrl

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/carolinepetrova/expense-requests/internal/approval"
	"github.com/carolinepetrova/expense-requests/internal/expense/model"
)

// ErrorResponse is the shape of every failure.
type ErrorResponse struct {
	Error   string             `json:"error"`
	Message string             `json:"message,omitempty"`
	Fields  []model.FieldError `json:"fieldErrors,omitempty"`
}

func BadRequest(message string) error {
	return echo.NewHTTPError(http.StatusBadRequest, body("bad_request", message))
}

func Unauthenticated(message string) error {
	return echo.NewHTTPError(http.StatusUnauthorized, body("unauthenticated", message))
}

// ErrorHandler maps domain errors onto status codes.
//
// The distinction worth noticing is 403 against 409: the first means the wrong
// person is asking, the second means the right person is asking at the wrong
// time. Collapsing them, as most APIs do, tells an owner they lack permission
// to edit their own request when the truth is that they already submitted it.
func ErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	status, payload := Translate(err)
	if writeErr := c.JSON(status, payload); writeErr != nil {
		c.Logger().Error(writeErr)
	}
}

func Translate(err error) (int, ErrorResponse) {
	// Validation failures carry the field list the form needs in order to mark
	// every bad field at once.
	var fields model.FieldErrors
	if errors.As(err, &fields) {
		return http.StatusUnprocessableEntity, ErrorResponse{
			Error:   "validation_failed",
			Message: "Some fields need attention.",
			Fields:  fields,
		}
	}

	switch {
	case errors.Is(err, model.ErrNotFound):
		return http.StatusNotFound, body("not_found", err.Error())

	case errors.Is(err, model.ErrForbidden),
		errors.Is(err, approval.ErrNotTheApprover),
		errors.Is(err, approval.ErrNotCurrentStep):
		return http.StatusForbidden, body("forbidden", err.Error())

	case errors.Is(err, model.ErrInvalidTransition):
		return http.StatusConflict, body("invalid_transition", err.Error())
	case errors.Is(err, model.ErrConflict):
		return http.StatusConflict, body("conflict",
			"Somebody else changed this request. Reload and try again.")

	// Nobody but the requester could approve this, so it cannot be submitted
	// at all. A conflict rather than a validation error, because no edit to
	// the form would fix it.
	case errors.Is(err, approval.ErrNoEligibleApprover):
		return http.StatusConflict, body("cannot_route_to_self",
			"This request has no approver other than you.")
	}

	var httpErr *echo.HTTPError
	if errors.As(err, &httpErr) {
		if payload, ok := httpErr.Message.(ErrorResponse); ok {
			return httpErr.Code, payload
		}
		return httpErr.Code, body("error", http.StatusText(httpErr.Code))
	}

	return http.StatusInternalServerError, body("internal_error", "Something went wrong.")
}

func body(code, message string) ErrorResponse {
	return ErrorResponse{Error: code, Message: message}
}
