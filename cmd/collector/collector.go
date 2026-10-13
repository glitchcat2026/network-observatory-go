package main

import (
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"network-observatory-go/internal"
	"os"
	"sort"
	"time"

	_ "modernc.org/sqlite"
)

type Collector struct {
	db       *sql.DB
	token    string
	pageTmpl *template.Template
}

type PingRecord struct {
	ID         int64
	ProbeName  string
	Target     string
	IPVersion  string
	Success    bool
	MinRTT     float64
	MaxRTT     float64
	AvgRTT     float64
	PacketLoss float64
	Datetime   string
	ReceivedAt string
}

type PageData struct {
	Records     []PingRecord
	Total       int
	ProbeCount  int
	TargetCount int
	UpdatedAt   string
	Lang        string
	Labels      map[string]string
}

var uiLanguage string
var maxBodySize int64

func main() {
	configPath, err := internal.ExecutableRelativePath(internal.ConfigFile)
	if err != nil {
		log.Fatalf("resolve Collector config path: %v", err)
	}

	cfg, created, err := internal.LoadOrCreateConfig(configPath)

	if err != nil {
		log.Fatalf("load Collector config: %v", err)
	}

	if created {
		os.Exit(0)
	}

	addr := cfg.Addr
	token := cfg.Token
	maxBodySize = cfg.MaxBodySize
	uiLanguage = cfg.Lang

	dbPath, err := internal.ExecutableRelativePath(cfg.DBPath)
	if err != nil {
		log.Fatalf("resolve database path: %v", err)
	}

	// Initialize the SQLite database.
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("open SQLite database: %v", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		log.Fatalf("connect to SQLite database: %v", err)
	}

	if _, err := db.Exec(`
		PRAGMA journal_mode = WAL;
		PRAGMA busy_timeout = 5000;
		CREATE TABLE IF NOT EXISTS ping_results (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			probe_name  TEXT NOT NULL,
			target      TEXT NOT NULL,
			ip_version  TEXT NOT NULL,
			success     INTEGER NOT NULL,
			min_rtt     REAL NOT NULL,
			max_rtt     REAL NOT NULL,
			avg_rtt     REAL NOT NULL,
			packet_loss REAL NOT NULL,
			datetime    TEXT NOT NULL,
			received_at TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_ping_datetime
		ON ping_results(datetime);
		CREATE INDEX IF NOT EXISTS idx_ping_probe
		ON ping_results(probe_name);
	`); err != nil {
		log.Fatalf("initialize database: %v", err)
	}
	if err := validateDatabaseSchema(db); err != nil {
		log.Fatal(err)
	}

	pageTmpl := template.Must(template.New("dashboard").Parse(dashboardHTML))

	collector := &Collector{
		db:       db,
		token:    token,
		pageTmpl: pageTmpl,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api", collector.handleReport)
	mux.HandleFunc("/", collector.handleDashboard)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := db.Ping(); err != nil {
			http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("Collector started, listening on: %s", addr)
	log.Printf("SQLite database: %s", dbPath)
	log.Print("Report API: POST /api")
	log.Print("Web dashboard: GET /")

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("listen on %s: %v", addr, err)
	}
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP server failed: %v", err)
	}
}

func validateDatabaseSchema(db *sql.DB) error {
	rows, err := db.Query(`
		SELECT id, probe_name, target, ip_version, success,
			min_rtt, max_rtt, avg_rtt, packet_loss,
			datetime, received_at
		FROM ping_results
		LIMIT 0
	`)
	if err != nil {
		return fmt.Errorf("database schema is incompatible; configure a new database file: %w", err)
	}
	return rows.Close()
}

// handleReport receives a JSON report from a Probe and stores its results.
func (c *Collector) handleReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	if r.Header.Get("Content-Type") != "application/json" &&
		len(r.Header.Values("Content-Type")) == 0 {
		http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return
	}

	// Limit the request body size to prevent oversized requests.
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	defer r.Body.Close()

	var report internal.Report
	decoder := json.NewDecoder(r.Body)

	if err := decoder.Decode(&report); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Reject requests containing more than one JSON value.
	var extra any
	if err := decoder.Decode(&extra); err != nil && err.Error() != "EOF" {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	} else if err == nil {
		http.Error(w, "Multiple JSON values are not allowed", http.StatusBadRequest)
		return
	}

	// Validate the token.
	if subtle.ConstantTimeCompare(
		[]byte(report.Token),
		[]byte(c.token),
	) != 1 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if report.Probe_name == "" || len(report.Results) == 0 {
		http.Error(w, "Missing probe name or results", http.StatusBadRequest)
		return
	}
	ipVersion, err := internal.NormalizeIPVersion(report.IPVersion)
	if err != nil {
		http.Error(w, "Invalid probe result", http.StatusBadRequest)
		return
	}
	report.IPVersion = ipVersion

	// Sort targets to keep insertion order stable within each batch.
	targets := make([]string, 0, len(report.Results))
	for target := range report.Results {
		targets = append(targets, target)
	}
	sort.Strings(targets)

	tx, err := c.db.Begin()
	if err != nil {
		log.Printf("Begin database transaction: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO ping_results (
			probe_name, target, ip_version, success,
			min_rtt, max_rtt, avg_rtt,
			packet_loss, datetime, received_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		log.Printf("Prepare SQL statement: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	receivedAt := time.Now().UTC().Format(time.RFC3339Nano)

	for _, target := range targets {
		result := report.Results[target]

		// Validate numeric values to prevent invalid data from polluting the database.
		if target == "" ||
			result.MinRTT < 0 ||
			result.MaxRTT < 0 ||
			result.AvgRTT < 0 ||
			result.PacketLoss < 0 ||
			result.PacketLoss > 100 {
			http.Error(w, "Invalid probe result", http.StatusBadRequest)
			return
		}

		_, err := stmt.Exec(
			report.Probe_name,
			target,
			report.IPVersion,
			result.Success,
			result.MinRTT,
			result.MaxRTT,
			result.AvgRTT,
			result.PacketLoss,
			result.Datetime,
			receivedAt,
		)
		if err != nil {
			log.Printf("Insert database record: %v", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("Commit database transaction: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	log.Printf("Received report from Probe=%s, saved %d records", report.Probe_name, len(targets))

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":  "ok",
		"saved":   len(targets),
		"message": "Report stored",
	})
}

// handleDashboard queries SQLite and renders the web dashboard.
func (c *Collector) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	rows, err := c.db.Query(`
		SELECT id, probe_name, target, ip_version, success,
			min_rtt, max_rtt, avg_rtt, packet_loss,
			datetime, received_at
		FROM ping_results
		ORDER BY id DESC
		LIMIT 500
	`)
	if err != nil {
		log.Printf("Query database: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	data := PageData{
		Records: make([]PingRecord, 0),
	}

	probes := make(map[string]struct{})
	targets := make(map[string]struct{})

	for rows.Next() {
		var record PingRecord
		if err := rows.Scan(
			&record.ID,
			&record.ProbeName,
			&record.Target,
			&record.IPVersion,
			&record.Success,
			&record.MinRTT,
			&record.MaxRTT,
			&record.AvgRTT,
			&record.PacketLoss,
			&record.Datetime,
			&record.ReceivedAt,
		); err != nil {
			log.Printf("Scan database row: %v", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		data.Records = append(data.Records, record)
		probes[record.ProbeName] = struct{}{}
		targets[record.Target] = struct{}{}
	}

	if err := rows.Err(); err != nil {
		log.Printf("Iterate database rows: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	data.Total = len(data.Records)
	data.ProbeCount = len(probes)
	data.TargetCount = len(targets)
	data.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
	data.Lang = uiLanguage
	data.Labels = DashboardTexts(uiLanguage)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := c.pageTmpl.Execute(w, data); err != nil {
		log.Printf("Render dashboard: %v", err)
	}
}

const dashboardHTML = `<!DOCTYPE html>
<html lang="{{.Lang}}">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta http-equiv="refresh" content="30">
<title>{{.Labels.title}}</title>
<style>
:root { color-scheme: light dark; }
body {
  margin: 0; padding: 28px;
  font-family: system-ui, -apple-system, "Segoe UI", sans-serif;
  background: #f4f6fa; color: #1f2937;
}
main { max-width: 1400px; margin: auto; }
h1 { margin-bottom: 6px; }
.sub { color: #64748b; margin-top: 0; }
.cards { display: grid; grid-template-columns: repeat(3, 1fr); gap: 16px; margin: 24px 0; }
.card, .panel { background: white; border: 1px solid #e2e8f0; border-radius: 12px; padding: 18px; }
.label { color: #64748b; font-size: 14px; }
.value { font-size: 28px; font-weight: 700; margin-top: 6px; }
.panel { padding: 0; overflow: hidden; }
.panel-head { padding: 18px; border-bottom: 1px solid #e2e8f0; }
.table-wrap { overflow-x: auto; }
table { width: 100%; border-collapse: collapse; white-space: nowrap; font-size: 14px; }
th, td { padding: 12px 14px; text-align: left; border-bottom: 1px solid #edf0f5; }
th { background: #f8fafc; color: #475569; font-weight: 600; }
tr:last-child td { border-bottom: 0; }
.status { display: inline-block; padding: 3px 9px; border-radius: 99px; font-size: 12px; }
.ok { background: #dcfce7; color: #166534; }
.bad { background: #fee2e2; color: #991b1b; }
footer { color: #64748b; font-size: 12px; margin-top: 16px; }
@media (max-width: 700px) {
  body { padding: 16px; }
  .cards { grid-template-columns: 1fr; }
}
@media (prefers-color-scheme: dark) {
  body { background: #0f172a; color: #e2e8f0; }
  .card, .panel { background: #1e293b; border-color: #334155; }
  th { background: #273449; color: #cbd5e1; }
  th, td, .panel-head { border-color: #334155; }
  .sub, .label, footer { color: #94a3b8; }
  .ok { background: #14532d; color: #bbf7d0; }
  .bad { background: #7f1d1d; color: #fecaca; }
}
</style>
</head>
<body>
<main>
  <h1>{{.Labels.title}}</h1>
  <p class="sub">{{.Labels.subtitle}}</p>

  <section class="cards">
    <div class="card">
      <div class="label">{{.Labels.records}}</div>
      <div class="value">{{.Total}}</div>
    </div>
    <div class="card">
      <div class="label">{{.Labels.probe_count}}</div>
      <div class="value">{{.ProbeCount}}</div>
    </div>
    <div class="card">
      <div class="label">{{.Labels.target_count}}</div>
      <div class="value">{{.TargetCount}}</div>
    </div>
  </section>

  <section class="panel">
    <div class="panel-head">
      <strong>{{.Labels.history}}</strong>
      <div class="sub">{{.Labels.updated}}: {{.UpdatedAt}}</div>
    </div>
    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th>{{.Labels.time}}</th><th>{{.Labels.probe}}</th><th>{{.Labels.target}}</th>
            <th>{{.Labels.ip_version}}</th>
            <th>{{.Labels.status}}</th><th>{{.Labels.min_rtt}}</th>
            <th>{{.Labels.avg_rtt}}</th><th>{{.Labels.max_rtt}}</th>
            <th>{{.Labels.packet_loss}}</th>
          </tr>
        </thead>
        <tbody>
        {{range .Records}}
          <tr>
            <td>{{.Datetime}}</td>
            <td>{{.ProbeName}}</td>
            <td>{{.Target}}</td>
            <td>{{.IPVersion}}</td>
            <td>{{if .Success}}<span class="status ok">{{$.Labels.normal}}</span>{{else}}<span class="status bad">{{$.Labels.failed}}</span>{{end}}</td>
            <td>{{printf "%.3f" .MinRTT}}</td>
            <td>{{printf "%.3f" .AvgRTT}}</td>
            <td>{{printf "%.3f" .MaxRTT}}</td>
            <td>{{printf "%.2f" .PacketLoss}}%</td>
          </tr>
        {{else}}
          <tr><td colspan="9">{{$.Labels.empty}}</td></tr>
        {{end}}
        </tbody>
      </table>
    </div>
  </section>

  <footer>{{.Labels.footer}}</footer>
</main>
</body>
</html>`

func DashboardTexts(language string) map[string]string {
	return map[string]string{
		"title":        internal.T(language, "title"),
		"subtitle":     internal.T(language, "subtitle"),
		"history":      internal.T(language, "history"),
		"records":      internal.T(language, "records"),
		"probe_count":  internal.T(language, "probe_count"),
		"target_count": internal.T(language, "target_count"),
		"probe":        internal.T(language, "probe"),
		"target":       internal.T(language, "target"),
		"ip_version":   internal.T(language, "ip_version"),
		"time":         internal.T(language, "time"),
		"status":       internal.T(language, "status"),
		"min_rtt":      internal.T(language, "min_rtt"),
		"avg_rtt":      internal.T(language, "avg_rtt"),
		"max_rtt":      internal.T(language, "max_rtt"),
		"packet_loss":  internal.T(language, "packet_loss"),
		"updated":      internal.T(language, "updated"),
		"normal":       internal.T(language, "normal"),
		"failed":       internal.T(language, "failed"),
		"empty":        internal.T(language, "empty"),
		"footer":       internal.T(language, "footer"),
	}
}
