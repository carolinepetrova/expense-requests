// Package init wires the expense context together.
//
// It lives in a subpackage rather than beside the aggregate because the store,
// the service and the REST layer all import the aggregate — so the aggregate's
// own package cannot import them back to assemble itself.
package init

import (
	"github.com/labstack/echo/v4"

	"github.com/carolinepetrova/expense-requests/internal/approval"
	"github.com/carolinepetrova/expense-requests/internal/client"
	"github.com/carolinepetrova/expense-requests/internal/expense/model"
	"github.com/carolinepetrova/expense-requests/internal/expense/rest"
	"github.com/carolinepetrova/expense-requests/internal/expense/service"
	"github.com/carolinepetrova/expense-requests/internal/expense/store"
	"github.com/carolinepetrova/expense-requests/internal/user"
)

func Init(
	e *echo.Echo,
	requests []model.Record,
	users user.Directory,
	clients client.Directory,
	spec approval.Spec[model.Subject],
) *service.Service {
	svc := service.New(store.NewRequests(requests), users, clients, spec)
	rest.Register(e, svc, users)

	return svc
}
