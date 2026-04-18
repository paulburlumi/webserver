package main

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/starfederation/datastar-go/datastar"
)

func main() {
	if err := run(os.Getenv, os.Stdout); err != nil {
		slog.Error("Error running server", "error", err)
		os.Exit(1)
	}
}

func run(getenv func(string) string, stdout io.Writer) error {

	slog.SetDefault(slog.New(slog.NewTextHandler(stdout, &slog.HandlerOptions{Level: slog.LevelDebug})))

	port := getenv("PORT")
	if port == "" {
		port = "8080"
	}

	table, err := template.New("rowerTable").Parse(rowerTableTemplate)
	if err != nil {
		return fmt.Errorf("error parsing template: %w", err)
	}

	rc := rowerCalc{table: table}

	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("GET /masterscalc", rc.showMainPage)
	http.HandleFunc("GET /masterscalc/rowers", rc.listRowers)
	http.HandleFunc("POST /masterscalc/rowers", rc.createRower)
	http.HandleFunc("DELETE /masterscalc/rowers/{idx}", rc.deleteRower)

	slog.Info("Server starting", "url", "http://localhost:"+port+"/masterscalc")
	if err := http.ListenAndServe(":"+port, nil); err != http.ErrServerClosed {
		return fmt.Errorf("error starting server: %w", err)
	}

	return nil
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, "OK")
}

var ageBands = []struct {
	Band   string
	MinAge float64
}{
	{"A", 27},
	{"B", 36},
	{"C", 43},
	{"D", 50},
	{"E", 55},
	{"F", 60},
	{"G", 65},
	{"H", 70},
	{"I", 75},
	{"J", 80},
	{"K", 85},
}

// Cookie helper functions for storing rowers
func getRowersFromCookie(r *http.Request) []rower {
	cookie, err := r.Cookie("rowers")
	if err != nil {
		return []rower{}
	}

	// Base64 decode the cookie value
	decodedBytes, err := base64.URLEncoding.DecodeString(cookie.Value)
	if err != nil {
		slog.Warn("Error decoding cookie", "error", err)
		return []rower{}
	}

	var rowers []rower
	gobBuf := bytes.NewBuffer(decodedBytes)
	gobDecoder := gob.NewDecoder(gobBuf)
	if err := gobDecoder.Decode(&rowers); err != nil {
		slog.Warn("Error decoding GOB data", "error", err)
		return []rower{}
	}

	return rowers
}

func setRowersCookie(w http.ResponseWriter, rowers []rower) error {
	var gobBuf bytes.Buffer
	gobEncoder := gob.NewEncoder(&gobBuf)
	if err := gobEncoder.Encode(rowers); err != nil {
		return fmt.Errorf("error encoding rowers: %w", err)
	}

	// Base64 encode the GOB data for cookie storage
	encodedValue := base64.URLEncoding.EncodeToString(gobBuf.Bytes())

	// Check cookie size limit (browsers typically limit to ~4KB)
	if len(encodedValue) > 4000 {
		return fmt.Errorf("rower data too large for cookie storage")
	}

	cookie := &http.Cookie{
		Name:     "rowers",
		Value:    encodedValue,
		Path:     "/",
		MaxAge:   86400 * 30, // 30 days
		HttpOnly: false,      // Allow client-side access if needed
		SameSite: http.SameSiteLaxMode,
	}

	http.SetCookie(w, cookie)
	return nil
}

type rower struct {
	Name      string
	BirthYear int
	Age       int
	Band      string
}

func newRower(name string, birthYearOrAge int) (rower, error) {
	birthYear := birthYearOrAge
	thisYear := time.Now().Year()
	if birthYearOrAge < 200 {
		birthYear = thisYear - birthYearOrAge
	}
	age := thisYear - birthYear
	if age < 1 {
		return rower{}, fmt.Errorf("invalid birth year or age: %d", birthYearOrAge)
	}
	band := calculateBand(float64(age))
	if band == "" {
		return rower{}, fmt.Errorf("%s aged %d is too young for a masters category", name, age)
	}
	return rower{
		Name:      name,
		BirthYear: birthYear,
		Age:       age,
		Band:      band,
	}, nil
}

func calculateAverageAge(rowers []rower) float64 {
	if len(rowers) == 0 {
		return 0.0
	}
	totalAge := 0
	for _, r := range rowers {
		totalAge += r.Age
	}
	return float64(totalAge) / float64(len(rowers))
}

func calculateBand(age float64) string {
	band := ""
	for _, ageBand := range ageBands {
		if ageBand.MinAge > age {
			break
		}
		band = ageBand.Band
	}
	return band
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>Rowing Calculator</title>
	<script type="module" src="https://cdn.jsdelivr.net/gh/starfederation/datastar@1.0.0-RC.5/bundles/datastar.js"></script>
</head>
<body>
<div>
<form>
	<div class="mb-3">
		<div id="formHelp" class="form-text">Enter each crew member's details.</div>
		<label for="inputName" class="form-label">Name</label>
		<input id="inputName" class="form-control" placeholder="e.g. Bob" value="" data-bind-name>
	</div>
	<div class="mb-3">
		<label for="inputYear" class="form-label">Year of Birth / Age on their Birthday this year</label>
		<input id="inputYear" class="form-control" placeholder="e.g. 1988 or 37" type="number" min="1900" max="3000" value="" data-bind-birth-year-or-age>
	</div>
	<div class="mb-3">
		<button type="button" class="btn btn-secondary" data-attr-disabled="$name.length === 0 || !$birthYearOrAge" data-on-click="@post('/masterscalc/rowers')">Add</button>
	</div>
</form>
</div>
<div>
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
	<tbody id="rower-table-body" data-on-load="@get('/masterscalc/rowers')"/>
</table>
</div>
<div className='card'>
	<div className='card-body'>
	<p className='lead'>
		Average age: <span className='badge rounded-pill bg-light' data-text="$averageAge" />
	</p>
	<p className='lead'>
		Crew Masters Category: <span className='badge rounded-pill bg-light' data-text="$averageBand" />
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
			<button data-on-click="@delete('/masterscalc/rowers/{{$i}}')">Remove</button>
		</td>
	</tr>
	{{end}}
</tbody>`

type rowerSignals struct {
	Name           string `json:"name"`
	BirthYearOrAge int    `json:"birthYearOrAge"`
	AverageAge     string `json:"averageAge"`
	AverageBand    string `json:"averageBand"`
}

type rowerCalc struct {
	table *template.Template
}

func (rc *rowerCalc) showMainPage(w http.ResponseWriter, r *http.Request) {
	slog.Info("Showing main page")

	tmpl, err := template.New("main").Parse(htmlTemplate)
	if err != nil {
		http.Error(w, "Error parsing template", http.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, nil)
	if err != nil {
		http.Error(w, "Error executing template", http.StatusInternalServerError)
		return
	}
}

func (rc *rowerCalc) listRowers(w http.ResponseWriter, r *http.Request) {
	slog.Info("Listing rowers")

	// Get rowers from cookie
	rowers := getRowersFromCookie(r)

	slog.Info("Using cookie storage", "rowerCount", len(rowers))

	sse := datastar.NewSSE(w, r)
	rc.PatchTable(sse, rowers)
}

func (rc *rowerCalc) createRower(w http.ResponseWriter, r *http.Request) {
	signals := rowerSignals{}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		sse := datastar.NewSSE(w, r)
		handleError(sse, err, "Error reading signals")
		return
	}

	rower, err := newRower(signals.Name, signals.BirthYearOrAge)
	if err != nil {
		sse := datastar.NewSSE(w, r)
		handleError(sse, err, "Error creating rower")
		return
	}

	// Get rowers from cookie
	rowers := getRowersFromCookie(r)

	// Add new rower
	rowers = append(rowers, rower)

	// Save back to cookie
	if err := setRowersCookie(w, rowers); err != nil {
		sse := datastar.NewSSE(w, r)
		handleError(sse, err, "Error setting rowers cookie")
		return
	}

	slog.Info("Created rower", "rower", rower)

	sse := datastar.NewSSE(w, r)
	rc.PatchTable(sse, rowers)
}

func (rc *rowerCalc) deleteRower(w http.ResponseWriter, r *http.Request) {
	idx := r.PathValue("idx")
	if idx == "" {
		sse := datastar.NewSSE(w, r)
		handleError(sse, nil, "Missing rower index")
		return
	}

	i, err := strconv.Atoi(idx)
	if err != nil {
		sse := datastar.NewSSE(w, r)
		handleError(sse, err, "Invalid rower index", "index", idx)
		return
	}

	// Get rowers from cookie
	rowers := getRowersFromCookie(r)

	if i < 0 || i >= len(rowers) {
		sse := datastar.NewSSE(w, r)
		handleError(sse, nil, "Row not found", "index", i)
		return
	}

	slog.Info("Deleted rower", "rower", rowers[i])
	rowers = slices.Delete(rowers, i, i+1)

	// Save back to cookie
	if err := setRowersCookie(w, rowers); err != nil {
		sse := datastar.NewSSE(w, r)
		handleError(sse, err, "Error setting rowers cookie")
		return
	}

	sse := datastar.NewSSE(w, r)
	rc.PatchTable(sse, rowers)
}

func (rc *rowerCalc) PatchTable(
	sse *datastar.ServerSentEventGenerator,
	rowers []rower) {

	tableBuffer := new(strings.Builder)
	if err := rc.table.Execute(tableBuffer, rowers); err != nil {
		handleError(sse, err, "Error executing table template")
		return
	}

	if err := sse.PatchElements(tableBuffer.String()); err != nil {
		handleError(sse, err, "Error sending element patch")
		return
	}

	averageAge := calculateAverageAge(rowers)
	averageBand := calculateBand(averageAge)

	slog.Info("Updated averages", "averageAge", averageAge, "averageBand", averageBand)
	if err := sse.MarshalAndPatchSignals(&rowerSignals{
		AverageAge:  fmt.Sprintf("%.1f", averageAge),
		AverageBand: averageBand,
	}); err != nil {
		handleError(sse, err, "Error sending signal patch")
		return
	}
}

func handleError(sse *datastar.ServerSentEventGenerator, err error, msg string, args ...any) {
	if err != nil {
		args = append(args, "error", err)
	}

	slog.Error(msg, args...)
	if len(args) > 0 {
		msg += fmt.Sprintf(" %v", args)
	}
	_ = sse.ExecuteScript("alert('" + msg + "');")
}
