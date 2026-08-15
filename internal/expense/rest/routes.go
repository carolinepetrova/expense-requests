package rest

import (
	"github.com/labstack/echo/v4"

	"github.com/carolinepetrova/expense-requests/internal/expense/service"
	"github.com/carolinepetrova/expense-requests/internal/httpctrl"
	"github.com/carolinepetrova/expense-requests/internal/user"
)

func Register(e *echo.Echo, svc *service.Service, users user.Directory) {
	api := e.Group("/api")

	api.GET("/users", NewListUsersHandler(svc))
	api.GET("/clients", NewListClientsHandler(svc))

	requests := api.Group("/requests", httpctrl.ActorMiddleware(users))
	requests.GET("", NewListRequestsHandler(svc))
	requests.POST("", NewCreateRequestHandler(svc))
	requests.GET("/:id", NewGetRequestHandler(svc))
	requests.PATCH("/:id", NewUpdateValuesHandler(svc))
	requests.POST("/:id/submit", NewSubmitRequestHandler(svc))
	requests.POST("/:id/approve", NewApproveRequestHandler(svc))
	requests.POST("/:id/reject", NewRejectRequestHandler(svc))
}
