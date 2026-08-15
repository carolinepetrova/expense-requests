package rest

import (
	"context"

	"github.com/carolinepetrova/expense-requests/internal/client"
	"github.com/carolinepetrova/expense-requests/internal/expense"
	"github.com/carolinepetrova/expense-requests/internal/expense/model"
	"github.com/carolinepetrova/expense-requests/internal/expense/service"
	"github.com/carolinepetrova/expense-requests/internal/httpctrl"
	"github.com/carolinepetrova/expense-requests/internal/user"
	"github.com/labstack/echo/v4"
)

func NewListUsersHandler(svc *service.Service) echo.HandlerFunc {
	return httpctrl.Response(func(ctx context.Context) ([]model.UserResponse, error) {
		users, err := svc.Users(ctx)
		if err != nil {
			return nil, err
		}

		out := make([]model.UserResponse, 0, len(users))
		for _, u := range users {
			out = append(out, model.UserResponse{
				ID: u.ID, Name: u.Name, Role: string(u.Role), ManagerID: u.ManagerID,
			})
		}
		return out, nil
	})
}

func NewListClientsHandler(svc *service.Service) echo.HandlerFunc {
	return httpctrl.Response(func(ctx context.Context) ([]client.Client, error) {
		return svc.Clients(ctx)
	})
}

func NewListRequestsHandler(svc *service.Service) echo.HandlerFunc {
	return httpctrl.RequestResponse(func(
		ctx context.Context, me user.User, in *model.ListRequestsInput,
	) ([]expense.RequestSummary, error) {
		var filter expense.Filter

		if in.Status != "" {
			status, err := model.ParseStatus(in.Status)
			if err != nil {
				return nil, httpctrl.BadRequest("Unknown status.")
			}
			filter.Status = &status
		}

		switch in.Scope {
		case "mine":
			id := me.ID
			filter.RequesterID = &id
		case "assigned":
			id := me.ID
			filter.ApproverID = &id
		case "", "all":
		default:
			return nil, httpctrl.BadRequest("Unknown scope.")
		}

		filter.Query = in.Query

		return svc.Requests(ctx, filter)
	})
}

func NewGetRequestHandler(svc *service.Service) echo.HandlerFunc {
	return httpctrl.RequestResponse(func(
		ctx context.Context, _ user.User, in *model.GetRequestInput,
	) (expense.RequestView, error) {
		return svc.Request(ctx, in.ID)
	})
}

func NewCreateRequestHandler(svc *service.Service) echo.HandlerFunc {
	return httpctrl.RequestCreated(func(
		ctx context.Context, me user.User, in *model.CreateRequestInput,
	) (expense.RequestView, error) {
		return svc.CreateRequest(ctx, &model.CreateRequest{
			Command: model.Command{Actor: me},
			Values:  in.Values.ToModel(),
		})
	})
}

func NewUpdateValuesHandler(svc *service.Service) echo.HandlerFunc {
	return httpctrl.RequestResponse(func(
		ctx context.Context, me user.User, in *model.UpdateValuesInput,
	) (expense.RequestView, error) {
		return svc.UpdateValues(ctx, &model.UpdateValues{
			Command: model.Command{Actor: me},
			ID:      in.ID,
			Values:  in.Values.ToModel(),
		})
	})
}

func NewSubmitRequestHandler(svc *service.Service) echo.HandlerFunc {
	return httpctrl.RequestResponse(func(
		ctx context.Context, me user.User, in *model.SubmitRequestInput,
	) (expense.RequestView, error) {
		return svc.SubmitRequest(ctx, &model.SubmitRequest{
			Command: model.Command{Actor: me},
			ID:      in.ID,
		})
	})
}

func NewApproveRequestHandler(svc *service.Service) echo.HandlerFunc {
	return httpctrl.RequestResponse(func(
		ctx context.Context, me user.User, in *model.DecisionInput,
	) (expense.RequestView, error) {
		return svc.ApproveRequest(ctx, &model.ApproveRequest{
			Command: model.Command{Actor: me},
			ID:      in.ID,
			Comment: in.Comment,
		})
	})
}

func NewRejectRequestHandler(svc *service.Service) echo.HandlerFunc {
	return httpctrl.RequestResponse(func(
		ctx context.Context, me user.User, in *model.DecisionInput,
	) (expense.RequestView, error) {
		return svc.RejectRequest(ctx, &model.RejectRequest{
			Command: model.Command{Actor: me},
			ID:      in.ID,
			Comment: in.Comment,
		})
	})
}
