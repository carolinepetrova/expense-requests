package httpctrl

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/carolinepetrova/expense-requests/internal/user"
)

// HeaderUserID identifies the caller.
//
// This stands in for authentication, which the exercise does not require. The
// important property is that it is the only way the server learns who is
// acting: no request body carries a requester or an approver, so a client
// cannot claim to be somebody else by putting it in the payload.
const HeaderUserID = "X-User-Id"

const actorKey = "actor"

// ActorMiddleware resolves the header to a real person, or refuses the request.
func ActorMiddleware(users user.Directory) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			id := c.Request().Header.Get(HeaderUserID)
			if id == "" {
				return Unauthenticated("Send an " + HeaderUserID + " header.")
			}

			actor, err := users.Get(c.Request().Context(), user.ID(id))
			if err != nil {
				return Unauthenticated("Unknown user.")
			}

			c.Set(actorKey, actor)
			return next(c)
		}
	}
}

// Actor returns the caller resolved by the middleware.
func Actor(c echo.Context) user.User {
	actor, _ := c.Get(actorKey).(user.User)
	return actor
}

// RequestResponse builds a handler that binds the request, hands it to fn along
// with the caller, and writes 200 with the result as JSON.
//
// Every authenticated endpoint is written this way, so no handler repeats the
// bind-call-marshal dance or forgets a step of it.
func RequestResponse[Req any, Res any](
	fn func(context.Context, user.User, *Req) (Res, error),
) echo.HandlerFunc {
	return handle(http.StatusOK, fn)
}

// RequestCreated is RequestResponse with a 201.
func RequestCreated[Req any, Res any](
	fn func(context.Context, user.User, *Req) (Res, error),
) echo.HandlerFunc {
	return handle(http.StatusCreated, fn)
}

// Response builds a handler for an endpoint with no caller and no request
// body — the user picker and the client list, which the UI needs before
// anybody has chosen who to be.
func Response[Res any](fn func(context.Context) (Res, error)) echo.HandlerFunc {
	return func(c echo.Context) error {
		res, err := fn(c.Request().Context())
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, res)
	}
}

func handle[Req any, Res any](
	status int, fn func(context.Context, user.User, *Req) (Res, error),
) echo.HandlerFunc {
	return func(c echo.Context) error {
		req, err := Bind[Req](c)
		if err != nil {
			return err
		}

		res, err := fn(c.Request().Context(), Actor(c), req)
		if err != nil {
			return err
		}
		return c.JSON(status, res)
	}
}

// Bind fills a request struct from the path, the query string and the body.
//
// The body is decoded strictly: unknown fields are an error rather than being
// ignored. A client that tries to set status, requesterId or approverId gets a
// loud 400 instead of a silent no-op, which makes the boundary obvious both to
// callers and in the tests that probe it.
//
// An absent body is fine — submit takes none, and a decision's comment is
// optional.
func Bind[Req any](c echo.Context) (*Req, error) {
	var req Req

	binder := new(echo.DefaultBinder)
	if err := binder.BindPathParams(c, &req); err != nil {
		return nil, BadRequest(err.Error())
	}
	if err := binder.BindQueryParams(c, &req); err != nil {
		return nil, BadRequest(err.Error())
	}

	r := c.Request()
	if r.Body == nil || r.ContentLength == 0 {
		return &req, nil
	}

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		return nil, BadRequest(err.Error())
	}
	return &req, nil
}
