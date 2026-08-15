package model

import (
	"time"

	"github.com/carolinepetrova/expense-requests/internal/user"
)

type Command struct {
	Actor user.User
	At    time.Time
}

type CreateRequest struct {
	Command
	Values Values
}

type UpdateValues struct {
	Command
	ID     ID
	Values Values
}

type SubmitRequest struct {
	Command
	ID ID
}

type ApproveRequest struct {
	Command
	ID      ID
	Comment string
}

type RejectRequest struct {
	Command
	ID      ID
	Comment string
}
