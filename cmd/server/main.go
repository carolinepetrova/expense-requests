package main

import (
	"flag"
	"log"

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
	multiStep := flag.Bool("multi-step", false,
		"route expenses of $1,000 or more through the manager and then finance, "+
			"instead of straight to finance")
	flag.Parse()

	// The exercise asks for one approver; multi-step is the optional extension,
	// and switching between them is a different rule table and nothing else.
	spec := model.SingleStepSpec
	if *multiStep {
		spec = model.MultiStepSpec
	}

	// Seed data is read from disk rather than embedded so it can be edited and
	// the server restarted, without a rebuild.
	seeded, err := seed.Load(*dataDir)
	if err != nil {
		log.Fatalf("load seed data: %v", err)
	}

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.HTTPErrorHandler = httpctrl.ErrorHandler

	e.Use(
		middleware.Recover(),
		middleware.Logger(),

		// The UI is served by Vite on its own port during development, and it
		// has to be able to send the header that identifies the caller.
		middleware.CORSWithConfig(middleware.CORSConfig{
			AllowOrigins: []string{*webOrigin},
			AllowHeaders: []string{echo.HeaderContentType, httpctrl.HeaderUserID},
			AllowMethods: []string{"GET", "POST", "PATCH", "OPTIONS"},
		}),
	)

	expenseinit.Init(
		e,
		seeded.Requests,
		user.NewMemory(seeded.Users),
		client.NewMemory(seeded.Clients),
		spec,
	)

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
