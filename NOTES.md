# Expense Requests — notes

## Technologies
- Go backend, echo framework for REST, ginkgo for testing
- React + TypeScript frontend
- no database, everything is in memory
- Docker, so it runs without a Go or Node toolchain

## Running it

### With Docker — nothing else needed

```bash
make docker
```

Then open **http://localhost:8080**. The API and the UI are served from the same
port, so there is nothing else to start. `make docker-multi-step` does the same
with the two-stage approval chain described under *Stretch tasks* below.

### With Go and Node

```bash
make run                               # API on :8080, seeded from ./data
cd web && npm install && npm run dev   # UI on :5173
```

Two ports in development, because Vite reloads the UI as it changes.
`make run-multi-step` starts the API with the two-stage chain.

```bash
make test          # go test ./...
make install-tools # go-enum, ginkgo, golangci-lint
make lint          # golangci-lint run
```

### Additional notes on running
- As authetication was out of scope for this task, the UI has a person picker and every API call carries an
`X-User-Id` header. 
- Data is seeded to the app on every run. The data which I am using are from the example users.json and requests.json

## High-level Architecture

**I used an event log and a CQRS split, because the status and the history are the same thing. If I store both, they will disagree sooner or later.**
The problem is suitable for such style of architecture because at the bottom it is an append-only list of events — created, submitted, stepApproved, approved, rejected.
This way we have the history view for free, and the status cannot be faked
because there is no field to set.

On the write side, the aggregate replays its events, decides whether the command is allowed, and appends one more. `when` is the only fold, and it runs
the same way on a new event as on replay — so a request rebuilt from storage cannot end up somewhere a live one could not.

On the read side, every write also produces a projection, and the store keeps it next to the events. Reads never fold anything: a list request filters rows
that are already built. Replaying every request on every list call is fine for four rows and hopeless later.

Both are written in the same locked section, so we get the shape of CQRS without the cost of it - one process, one store, no bus, no eventual consistency. The read side is a cache that cannot go stale, because the call
that writes the event writes it too.

**Why is the architecture so complex?**
An event log and a projection are more machinery than a CRUD handler over a map, and for seven endpoints that is arguably more structure than the problem
demands.

I built it this way as I imagined this problem is not just exercise, it's a problem I would be facing in my daily work.  
Such system would not stay on seven endpoints. An expense system grows approval rules, then reporting, then somebody asks who approved
what last quarter and why. Every one of those is cheap here and expensive in the simple version. New rule: one row. Reporting: another projection over the
same events, and the write side does not move. "Who approved what": already there, because the log was never thrown away.

The cost is paid up front, and it is real: more indirection than the exercise needs, and a reviewer has to follow a command through three files instead of
one. I think that is the right trade when the subject is correctness — every rule has exactly one place it could be enforced, which is what makes it
arguable that it *is*.

### Approval routing

**I made routing a rule table rather than a chain of ifs, because approval rules only ever get added.** Today there are two. Next comes a department
budget, a category that always needs finance, a limit per client. With ifs, every one of those is a rewrite of the same function and a new way to break the
old cases. With a table, it is one more row.

`internal/approval` is the engine. It knows nothing about expenses, users or HTTP - you give it rules that say *when this, ask them*, and it builds the
chain. For each rule it finds the approver, falls back to finance if there is nobody or if it picked the requester, drops the rule if finance is the
requester too, and skips a person who is already on the step before. If no rule survives, only the requester could approve it, so it cannot be submitted at
all.

Then the policy from the task is written in thirty lines in `model/routing.go`: under $1,000 → manager, falling back to finance; $1,000 and over → finance.
Bob's manager is Mallory, so his small expenses go to her. Peggy has no manager, so hers go to Trent. Trent's own large expenses have nowhere to go, and
the server refuses them with `cannot_route_to_self`.

## Low-level design

```
internal/
  approval/          the approval engine — Spec, Rule, Chain
  user/, client/     directories, behind interfaces
  expense/
    model/           values, events, validation, routing rules
    aggregate.go     the write model — decides, emits, folds
    policy.go        can this person do this, right now
    views/           read models: RequestView, Summary, Filter
    store/           event log + projections, in memory
    service/         load → authorize → validate → route → act → persist
    rest/            echo handlers and the DTOs they bind
    init/            wiring, so main.go stays short
  httpctrl/          handler wrapper, actor middleware, error mapping
  seed/              reads data/*.json at startup
```

Packages are domain oriented and are named after what they are about, not after what kind of code they hold. 
There is no `handlers/`, `services/`, `models/` triple to edit three files in every time a rule moves.

Dependencies point inwards. `model` imports only `approval` and `user`, and nothing in the domain knows that HTTP or JSON exist, so the rules can be read
without knowing how the app is wired - and tested without starting a server.

Interfaces sit at the boundaries, and they are declared by whoever uses them. `Store` lives in `service`, not in `store`: the service says what it needs, and
the in-memory store happens to satisfy it. Same for `user.Directory` and `client.Directory`. Postgres would be a new implementation, not a new
abstraction.

## Endpoints and status codes

| Endpoint | |
|---|---|
| `GET /api/users`, `GET /api/clients` | pickers; the only unauthenticated routes |
| `GET /api/requests?status=&scope=&q=` | `scope` is `mine`, `assigned` or `all` |
| `POST /api/requests` | create a draft |
| `GET /api/requests/:id` | detail, including the chain and the history |
| `PATCH /api/requests/:id` | edit values; owner, draft only |
| `POST /api/requests/:id/submit` | validate, route, assign |
| `POST /api/requests/:id/approve`, `/reject` | current approver only |

Status codes carry meaning:

- **422** with a `fieldErrors` array - the form is wrong. Every broken rule is reported at once, each with a stable `code` (`required_when_billable`,`required_when_large`, …) that the UI keys off rather than parsing messages.
- **403** - the wrong person is asking.
- **409 `invalid_transition`** - the right person, the wrong moment (editing a request they already submitted).
- **409 `conflict`** - somebody else changed it first; see *Concurrency*.
- **409 `cannot_route_to_self`** - nobody but the requester could approve it.


Authorization is checked before validation. Submitting somebody else's incomplete draft is a 403, not a 422 - you should not learn which of a stranger's fields are blank.

## Concurrency

**I added a version check rather than letting the last write win, because two approvers acting at once is the one race this app can still lose.** Silently dropping somebody's approval is the worst failure available here, and
optimistic concurrency cost ten lines and one error code.

Two approvers can load the same request, both decide, and both write. Neither aggregate can tell the world moved underneath it, so the store is the only
place the clash can be noticed: every record carries a `version`, and`SaveRequest` compares it under the write lock. The loser gets
409 `conflict` and the UI reloads rather than retrying — the decision was made against a screen that is now wrong, so re-sending it would apply an approval
the person never really gave.

## Stretch tasks implemented 

**Multi-step approval.** `make run-multi-step` swaps `SingleStepSpec` for `MultiStepSpec`. That is the whole change: one condition in one rule row
(`always` instead of `amountUnder`). Nothing in the aggregate, the store, the API or the frontend branches on how many steps there are — a chain of one and a
chain of two go down the same path. Large expenses then need Manager *and* Finance, in order, and a rejection anywhere ends the request with the steps it
never reached still marked pending.

**Filter and search.** `?status=`, `?scope=mine|assigned|all` and `?q=` on the  list endpoint, applied to the projections.

## What was tested

**Unit tests** covering crucial parts of the logic:

- **Validation** - every rule, and the code it returns. Billable needs a client, $1,000 needs a justification, Other needs a reason.
- **Routing** - that each request lands on the right person. Small ones go to the manager, large ones to finance. If the requester has no manager, or is
  their own manager, it goes to finance instead. If finance is the requester, the request is refused. Checked with single-step and multi-step routing.
- **The chain** - approving in order, approving out of turn, rejecting part way along.
- **Concurrency** - two people approve at the same time. The first write wins, the second is refused, and the losing comment never reaches the history.

**Story tests** follow one request from start to finish. One gets approved, one is rejected in the middle, one is loaded from the sample data. Each replays its
events at the end and checks the rebuilt request matches the live one — so if the status and the history ever stop agreeing, the build fails.

## AI assistance

I used Claude for code generation. The architecture, all design decisions and low level-design are mine with planning help from Claude.
Code review by me was done on every step + I did one code review of the finished code with Claude. The whole process was like a pair programming session, 
I haven't written any big prompts I could share. If any questions occur about my process of AI coding, I am happy to answer them in the follow-up session.