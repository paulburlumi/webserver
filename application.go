package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"text/template"

	"github.com/delaneyj/toolbelt"
	"github.com/gorilla/sessions"
	"github.com/starfederation/datastar-go/datastar"
)

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>MastersCalc</title>
	<link rel="stylesheet" type="text/css" href="/static/css/styles.css">
	<script type="module" src="https://cdn.jsdelivr.net/gh/starfederation/datastar@1.0.0-RC.6/bundles/datastar.js"></script>
</head>
<body>
<h1>MastersCalc</h1>
<div class="form-container">
<form>
	<div class="form-group">
		<div class="form-text">Enter each crew member's details.</div>
		<label for="inputName" class="form-label">Name</label>
		<input id="inputName" class="form-control" placeholder="e.g. Bob" value="" data-bind:name>
	</div>
	<div class="form-group">
		<label for="inputYear" class="form-label">Year of Birth / Age on their Birthday this year</label>
		<input id="inputYear" class="form-control" data-attr:placeholder="$example" type="number" min="1900" max="3000" data-bind:birth-year-or-age>
	</div>
	<div class="form-group">
		<button type="button" class="btn btn-secondary" data-attr:disabled="$name.length === 0 || !$birthYearOrAge" data-on:click="@post('/masterscalc/rowers')">Add</button>
	</div>
</form>
</div>
<div class="table-container">
<table>
	<thead>
		<tr>
			<th>Name</th>
			<th>Born</th>
			<th>Age</th>
			<th>Masters Category</th>
			<th>Actions</th>
		</tr>
	</thead>
	<tbody id="rower-table-body" data-init="@get('/masterscalc/rowers')"/>
</table>
</div>
<div class="card">
	<div class="card-body">
	<p class="lead">
		Average age: <span class="badge" data-text="$averageAge" />
	</p>
	<p class="lead">
		Crew Masters Category: <span class="badge" data-text="$averageBand" />
	</p>
	</div>
</div>
</body>
</html>`

const rowerTableTemplate = `<tbody id="rower-table-body">
	{{range $i, $rower := .}}
	<tr>
		<td>
			{{.Name}}
		</td>
		<td>
			{{.BirthYear}}
		</td>
		<td>
			{{.Age}}
		</td>
		<td>
			{{.Band}}
		</td>
		<td>
			<button class="remove-btn" data-on:click="@delete('/masterscalc/rowers/{{$i}}')">Remove</button>
		</td>
	</tr>
	{{end}}
</tbody>`

type application struct {
	table        *template.Template
	sessionStore *sessions.CookieStore
	bus          *business
}

func newApplication(sessionStore *sessions.CookieStore, bus *business) (*application, error) {
	table, err := template.New("rowerTable").Parse(rowerTableTemplate)
	if err != nil {
		return nil, fmt.Errorf("could not parse template: %w", err)
	}

	return &application{
		table:        table,
		sessionStore: sessionStore,
		bus:          bus,
	}, nil
}

func (app *application) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /masterscalc", app.showMainPage)
	mux.HandleFunc("GET /masterscalc/rowers", app.watch)
	mux.HandleFunc("POST /masterscalc/rowers", app.createRower)
	mux.HandleFunc("DELETE /masterscalc/rowers/{idx}", app.deleteRower)
}

func (app *application) showMainPage(w http.ResponseWriter, r *http.Request) {
	slog.Info("Showing main page")

	_, err := app.upsertSessionID(r, w)
	if err != nil {
		http.Error(w, "Error managing session: "+err.Error(), http.StatusInternalServerError)
		return
	}

	tmpl, err := template.New("main").Parse(htmlTemplate)
	if err != nil {
		http.Error(w, "Error parsing template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, nil)
	if err != nil {
		http.Error(w, "Error executing template: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

func (app *application) watch(w http.ResponseWriter, r *http.Request) {
	slog.Info("Watching rowers")

	sessionID, err := app.upsertSessionID(r, w)
	if err != nil {
		http.Error(w, "Error managing session: "+err.Error(), http.StatusInternalServerError)
		return
	}

	sse := datastar.NewSSE(w, r)

	callback := func(s *state) error {
		tableBuffer := new(strings.Builder)
		if err := app.table.Execute(tableBuffer, s.Rowers); err != nil {
			return fmt.Errorf("could not write table template: %w", err)
		}

		if err := sse.PatchElements(tableBuffer.String()); err != nil {
			return fmt.Errorf("could not patch elements: %w", err)
		}

		if err := sse.MarshalAndPatchSignals(&s.Signals); err != nil {
			return fmt.Errorf("could not patch signals: %w", err)
		}
		return nil
	}

	if err := app.bus.Watch(r.Context(), sessionID, callback); err != nil {
		http.Error(w, "Error while watching: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

func (app *application) createRower(w http.ResponseWriter, r *http.Request) {
	signals := struct {
		Name           string `json:"name"`
		BirthYearOrAge string `json:"birthYearOrAge"`
	}{}

	if err := datastar.ReadSignals(r, &signals); err != nil {
		slog.Error("Error reading signals", "error", err)
		http.Error(w, "Error reading signals: "+err.Error(), http.StatusBadRequest)
		return
	}

	sessionID, err := app.upsertSessionID(r, w)
	if err != nil {
		http.Error(w, "Error managing session: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := app.bus.Create(r.Context(), sessionID, signals.Name, signals.BirthYearOrAge); err != nil {
		http.Error(w, "Error creating rower: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

func (app *application) deleteRower(w http.ResponseWriter, r *http.Request) {
	idx := r.PathValue("idx")
	if idx == "" {
		http.Error(w, "Missing rower index", http.StatusBadRequest)
		return
	}

	i, err := strconv.Atoi(idx)
	if err != nil {
		http.Error(w, "Invalid rower index: "+err.Error(), http.StatusBadRequest)
		return
	}

	sessionID, err := app.upsertSessionID(r, w)
	if err != nil {
		http.Error(w, "Error managing session: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := app.bus.Delete(r.Context(), sessionID, i); err != nil {
		http.Error(w, "Error deleting rower: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

func (app *application) upsertSessionID(r *http.Request, w http.ResponseWriter) (string, error) {
	sess, err := app.sessionStore.Get(r, "connections")
	if err != nil {
		return "", fmt.Errorf("could not get session: %w", err)
	}

	id, ok := sess.Values["id"].(string)

	if !ok {
		id = toolbelt.NextEncodedID()
		sess.Values["id"] = id
		if err := sess.Save(r, w); err != nil {
			return "", fmt.Errorf("could not save session: %w", err)
		}
	}

	return id, nil
}
