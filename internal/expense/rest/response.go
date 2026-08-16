package rest

import (
	"github.com/carolinepetrova/expense-requests/internal/user"
)

type UserResponse struct {
	ID        user.ID  `json:"id"`
	Name      string   `json:"name"`
	Role      string   `json:"role"`
	ManagerID *user.ID `json:"managerId"`
}
