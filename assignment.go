package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

func registerAssignmentRoutes(mux *http.ServeMux) error {
	mux.HandleFunc("/assignment/{id}", authenticatedPage(func(w http.ResponseWriter, r *http.Request, u user) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			httpErrorLog(w, "invalid assignment id", err)
		}
		a := assignment{Id: id}

		if u.Admin {
			switch r.Method {
			case "POST":
				if err := (&a).fromForm(r); err != nil {
					httpErrorLog(w, "could not parse form", err)
					return
				}
				if id == -1 {
					if err := a.create(); err != nil {
						httpErrorLog(w, "could not create assignment", err)
						return
					}
				} else {
					if err := a.update(); err != nil {
						httpErrorLog(w, "could not update assignment", err)
						return
					}
				}
				http.Redirect(w, r, "/home", http.StatusFound)
			case "GET":
				tmpl, err := getTemplate("edit-assignment")
				if err != nil {
					httpErrorLog(w, "could not get assignment template", err)
					return
				}

				err = a.fetch()

				amap := map[string]any{"id": id}

				if err == nil {
					amap = a.formValues()
					// amap["name"] = a.Name
					// amap["start"] = a.Start.Format("2006-01-02T15:04")
					// amap["end"] = a.End.Format("2006-01-02T15:04")
				} else {
					fmt.Println(err.Error())
				}

				tmpl.Execute(w, map[string]any{
					"new":        err != nil,
					"assignment": amap,
				})
			}
		} else {
			// Just show the assignment
		}
	}))
	return nil
}

func createAssignmentTable() error {
	db, err := openDB()
	if err != nil {
		return err
	}
	_, err = db.Exec(
		`
CREATE TABLE IF NOT EXISTS assignments (
    id    INTEGER PRIMARY KEY,
    name  TEXT UNIQUE NOT NULL,
    start INT NOT NULL,
    end   INT NOT NULL
)
		`,
	)
	return err
}

type assignment struct {
	Id    int
	Name  string
	Start time.Time
	End   time.Time
}

func (a *assignment) fromForm(r *http.Request) error {
	a.Name = r.FormValue("name")

	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return err
	}

	if t, err := time.ParseInLocation("2006-01-02T15:04", r.FormValue("start"), loc); err != nil {
		return err
	} else {
		a.Start = t
	}

	if t, err := time.ParseInLocation("2006-01-02T15:04", r.FormValue("end"), loc); err != nil {
		return err
	} else {
		a.End = t
	}

	return nil

}

func (a assignment) create() error {
	db, err := openDB()
	if err != nil {
		return err
	}

	_, err = db.Exec("INSERT INTO assignments (name, start, end) VALUES ($1, $2, $3)", a.Name, a.Start.Unix(), a.End.Unix())
	return err
}

func (a assignment) update() error {
	db, err := openDB()
	if err != nil {
		return err
	}

	affected, err := db.Exec(`
UPDATE assignments
SET name = $1, start = $2, end = $3
WHERE id = $4
`,
		a.Name, a.Start.Unix(), a.End.Unix(), a.Id,
	)
	if err != nil {
		return err
	} else if count, err := affected.RowsAffected(); err != nil {
		return err
	} else if count != 1 {
		return errors.New("did not update a row")
	}
	return nil
}

func (a *assignment) fetch() error {
	db, err := openDB()
	if err != nil {
		return err
	}

	var (
		start int64
		end   int64
	)
	err = db.QueryRow("SELECT name, start, end FROM assignments WHERE id = $1", a.Id).Scan(&a.Name, &start, &end)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("no such assignment")
		}
		return err
	}
	a.Start = time.Unix(start, 0)
	a.End = time.Unix(end, 0)

	return nil
}

func (a assignment) formValues() map[string]any {
	return map[string]any{
		"id":    a.Id,
		"name":  a.Name,
		"start": a.Start.Format("2006-01-02T15:04"),
		"end":   a.End.Format("2006-01-02T15:04"),
	}
}

func getAssignments() ([]assignment, error) {
	db, err := openDB()
	if err != nil {
		return nil, err
	}

	rows, err := db.Query("SELECT id, name, start, end FROM assignments")
	if err != nil {
		return nil, err
	}

	var assignments []assignment

	for rows.Next() {
		var (
			a     assignment
			start int64
			end   int64
		)
		err := rows.Scan(&a.Id, &a.Name, &start, &end)
		if err != nil {
			return nil, err
		}
		a.Start = time.Unix(start, 0)
		a.End = time.Unix(end, 0)
		assignments = append(assignments, a)
	}

	return assignments, nil
}
