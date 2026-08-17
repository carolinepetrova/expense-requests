package main

import (
	"flag"
	"log"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/carolinepetrova/expense-requests/internal/client"
	expenseinit "github.com/carolinepetrova/expense-requests/internal/expense/init"
	"github.com/carolinepetrova/expense-requests/internal/expense/model"
	"github.com/carolinepetrova/expense-requests/internal/httpctrl"
	"github.com/carolinepetrova/expense-requests/internal/seed"
	"github.com/carolinepetrova/expense-requests/internal/user"
)

func main() {
	dataDir := flag.String("data", "./data", "directory holding the seed JSON")
	addr := flag.String("addr", ":8080", "listen address")
	webOrigin := flag.String("web-origin", "http://localhost:5173", "origin allowed to call the API")
	webDir := flag.String("web-dir", "",
		"directory of the built frontend to serve at /; empty means API only, "+
			"which is what you want when Vite is serving the UI")
	multiStep := flag.Bool("multi-step", false,
		"route expenses of $1,000 or more through the manager and then finance, "+
			"instead of straight to finance")
	flag.Parse()

	// The exercise asks for one approver; multi-step is the optional extension,
	// and switching between them is a different rule table and nothing else.
	spec := model.SingleStepSpec()
	if *multiStep {
		spec = model.MultiStepSpec()
	}

	// Seed data is read from disk rather than embedded so it can be edited and
	// the server restarted, without a rebuild.
	seeded, err := seed.Load(*dataDir)
	if err != nil {
		log.Fatalf("load seed data: %v", err)
	}

	e := newServer(*webOrigin)

	expenseinit.Init(
		e,
		seeded.Requests,
		user.NewMemory(seeded.Users),
		client.NewMemory(seeded.Clients),
		spec,
	)

	if *webDir != "" {
		serveUI(e, *webDir)
	}

	approvals := "single-step"
	if *multiStep {
		approvals = "multi-step"
	}

	log.Printf("listening on %s — %d users, %d clients, %d requests, %s approvals",
		*addr, len(seeded.Users), len(seeded.Clients), len(seeded.Requests), approvals)

	if err := e.Start(*addr); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}

// newServer builds the echo instance and everything that is true of every
// route: how errors become responses, what happens on a panic, what gets
// logged, and who is allowed to call the API.
func newServer(webOrigin string) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.HTTPErrorHandler = httpctrl.ErrorHandler

	e.Use(
		middleware.Recover(),

		// One line per request, so the routing and authorization decisions can
		// be followed while clicking through the UI.
		middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
			LogMethod: true,
			LogURI:    true,
			LogStatus: true,
			LogError:  true,
			LogValuesFunc: func(_ echo.Context, v middleware.RequestLoggerValues) error {
				log.Printf("%s %s -> %d%s", v.Method, v.URI, v.Status, because(v.Error))
				return nil
			},
		}),

		// The UI is served by Vite on its own port during development, and it
		// has to be able to send the header that identifies the caller.
		middleware.CORSWithConfig(middleware.CORSConfig{
			AllowOrigins: []string{webOrigin},
			AllowHeaders: []string{echo.HeaderContentType, httpctrl.HeaderUserID},
			AllowMethods: []string{"GET", "POST", "PATCH", "OPTIONS"},
		}),
	)

	return e
}

// serveUI serves the built frontend from the same origin as the API, so the
// whole thing runs on one port with no CORS involved. Only the container does
// this — in development Vite serves the UI and proxies nothing.
//
// HTML5 mode falls back to index.html for paths that are not files, so a
// refresh on a deep link loads the app instead of a 404.
func serveUI(e *echo.Echo, dir string) {
	e.Use(middleware.StaticWithConfig(middleware.StaticConfig{
		Root:  dir,
		Index: "index.html",
		HTML5: true,
		Skipper: func(c echo.Context) bool {
			return strings.HasPrefix(c.Request().URL.Path, "/api")
		},
	}))
}

// because renders the reason a request failed, and nothing at all when it did
// not. The logger has to return nil either way — a log line is not a place to
// handle the error, only to record it.
func because(err error) string {
	if err == nil {
		return ""
	}
	return ": " + err.Error()
}
