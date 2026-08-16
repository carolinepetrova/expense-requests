// Package seed loads the starting data from disk.
//
// The files in data/ are the ones the exercise supplies, read as they are
// rather than rewritten into some other shape. That works because the domain
// types carry the same JSON names the sample data uses, so decoding is direct
// and the enums validate themselves on the way in — an unknown role or expense
// type fails here, at startup, rather than surfacing as a strange status later.
package seed

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/carolinepetrova/expense-requests/internal/client"
	"github.com/carolinepetrova/expense-requests/internal/expense/model"
	"github.com/carolinepetrova/expense-requests/internal/user"
)

type Data struct {
	Users    []user.User
	Clients  []client.Client
	Requests []model.Record
}

// userFile mirrors data/users.json. It exists so the domain's User does not
// have to carry JSON tags for a format only this package reads.
type userFile struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Role      user.Role `json:"role"`
	ManagerID *user.ID  `json:"managerId"`
}

// requestFile mirrors data/requests.json. Values and Events decode straight
// into the domain types.
type requestFile struct {
	ID          model.ID      `json:"id"`
	RequesterID user.ID       `json:"requesterId"`
	Values      model.Values  `json:"values"`
	Events      []model.Event `json:"events"`
}

// Load reads users, clients and requests from dir.
//
// Any problem is fatal to startup by design: a half-seeded server is worse
// than one that refuses to start.
func Load(dir string) (Data, error) {
	var (
		data     Data
		users    []userFile
		clients  []client.Client
		requests []requestFile
	)

	if err := readJSON(filepath.Join(dir, "users.json"), &users); err != nil {
		return data, err
	}
	if err := readJSON(filepath.Join(dir, "clients.json"), &clients); err != nil {
		return data, err
	}
	if err := readJSON(filepath.Join(dir, "requests.json"), &requests); err != nil {
		return data, err
	}

	data.Clients = clients

	seen := make(map[user.ID]struct{}, len(users))
	for _, u := range users {
		id := user.ID(u.ID)
		if _, duplicate := seen[id]; duplicate {
			return data, fmt.Errorf("users.json: duplicate user %s", id)
		}
		seen[id] = struct{}{}

		data.Users = append(data.Users, user.User{
			ID: id, Name: u.Name, Role: u.Role, ManagerID: u.ManagerID,
		})
	}

	for _, r := range requests {
		if _, known := seen[r.RequesterID]; !known {
			return data, fmt.Errorf("requests.json: %s is requested by unknown user %s",
				r.ID, r.RequesterID)
		}

		data.Requests = append(data.Requests, model.Record{
			ID:          r.ID,
			RequesterID: r.RequesterID,
			Values:      r.Values,
			Events:      r.Events,
		})
	}

	return data, nil
}

// readJSON decodes strictly, so a typo in a seed file is an error rather than
// a silently ignored field.
func readJSON(path string, dst any) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open seed file: %w", err)
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return nil
}
