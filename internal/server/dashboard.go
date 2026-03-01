package server

import (
	"html/template"
	"net/http"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/vector76/beads_server/internal/model"
	"github.com/vector76/beads_server/internal/store"
)

// dashboardProject holds the template data for one project.
type dashboardProject struct {
	Name       string
	InProgress []store.BeadSummary
	Open       []store.BeadSummary
	Closed     []store.BeadSummary
	NotReady   []store.BeadSummary
}

// dashboardData holds the full template data.
type dashboardData struct {
	Projects []dashboardProject
	Theme    string
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	projects := s.provider.Projects()

	var data dashboardData
	for _, p := range projects {
		all := p.Store.List(store.ListFilters{All: true, PerPage: 10000})

		dp := dashboardProject{Name: p.Name}
		for _, b := range all.Beads {
			switch b.Status {
			case model.StatusInProgress:
				dp.InProgress = append(dp.InProgress, b)
			case model.StatusOpen:
				dp.Open = append(dp.Open, b)
			case model.StatusClosed:
				dp.Closed = append(dp.Closed, b)
			case model.StatusNotReady:
				dp.NotReady = append(dp.NotReady, b)
			}
		}
		sortByUpdatedDesc(dp.InProgress)
		sortByUpdatedDesc(dp.Open)
		sortByUpdatedDesc(dp.Closed)
		sortByUpdatedDesc(dp.NotReady)
		data.Projects = append(data.Projects, dp)
	}

	if c, err := r.Cookie("theme"); err == nil && (c.Value == "dark" || c.Value == "light") {
		data.Theme = c.Value
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := dashboardTmpl.Execute(w, data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// beadDetailData holds the template data for the bead detail page.
type beadDetailData struct {
	Project          string
	Bead             model.Bead
	ActiveBlockers   []model.Bead
	ResolvedBlockers []model.Bead
	Theme            string
}

func (s *Server) handleBeadDetail(w http.ResponseWriter, r *http.Request) {
	projectName := chi.URLParam(r, "project")
	beadID := chi.URLParam(r, "id")

	var st *store.Store
	for _, p := range s.provider.Projects() {
		if p.Name == projectName {
			st = p.Store
			break
		}
	}
	if st == nil {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}

	b, err := st.Get(beadID)
	if err != nil {
		http.Error(w, "bead not found", http.StatusNotFound)
		return
	}

	deps, _ := st.Deps(beadID)

	data := beadDetailData{
		Project:          projectName,
		Bead:             b,
		ActiveBlockers:   deps.ActiveBlockers,
		ResolvedBlockers: deps.ResolvedBlockers,
	}

	if c, err := r.Cookie("theme"); err == nil && (c.Value == "dark" || c.Value == "light") {
		data.Theme = c.Value
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := beadDetailTmpl.Execute(w, data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// sortByUpdatedDesc sorts beads by UpdatedAt descending (most recent first).
func sortByUpdatedDesc(beads []store.BeadSummary) {
	sort.Slice(beads, func(i, j int) bool {
		return beads[j].UpdatedAt.Before(beads[i].UpdatedAt)
	})
}

var dashboardTmpl = template.Must(template.New("dashboard").Funcs(template.FuncMap{
	"fmtTime": func(t time.Time) template.HTML {
		utc := t.UTC().Format(time.RFC3339)
		display := t.UTC().Format("2006-01-02 15:04")
		return template.HTML(`<time datetime="` + utc + `">` + display + `</time>`)
	},
}).Parse(`<!DOCTYPE html>
<html{{if .Theme}} data-theme="{{.Theme}}"{{end}}>
<head>
<meta charset="utf-8">
<title>Beads Dashboard</title>
<style>
  :root {
    --color-text: #222;
    --color-bg-page: #fff;
    --color-link: #0366d6;
    --color-border-subtle: #ccc;
    --color-border: #ddd;
    --color-bg-header: #f5f5f5;
    --color-bg-badge: #f0f0f0;
    --color-text-muted: #555;
    --color-bg-badge-yellow: #f0d84a;
    --color-bg-badge-green: #68cc8c;
  }
  [data-theme="dark"] {
    --color-text: #e0e0e0;
    --color-bg-page: #121212;
    --color-link: #58a6ff;
    --color-border-subtle: #555;
    --color-border: #444;
    --color-bg-header: #2a2a2a;
    --color-bg-badge: #333;
    --color-text-muted: #aaa;
    --color-bg-badge-yellow: #4a3c0e;
    --color-bg-badge-green: #1c4530;
  }
  body { font-family: sans-serif; margin: 2em; color: var(--color-text); background: var(--color-bg-page); }
  a { color: var(--color-link); text-decoration: none; }
  a:hover { text-decoration: underline; }
  h1 { margin-bottom: 0.2em; }
  h2 { border-bottom: 1px solid var(--color-border-subtle); padding-bottom: 0.2em; }
  h3 { margin-top: 1.2em; }
  table { border-collapse: collapse; width: 100%; margin-bottom: 1em; }
  th, td { text-align: left; padding: 0.35em 0.7em; border: 1px solid var(--color-border); }
  th { background: var(--color-bg-header); }
  .counts { display: flex; gap: 0.8em; flex-wrap: wrap; }
  .counts div { padding: 0.3em 0.7em; border-radius: 4px; background: var(--color-bg-badge); font-size: 0.9em; }
  .counts div.badge-yellow { background: var(--color-bg-badge-yellow); }
  .counts div.badge-green { background: var(--color-bg-badge-green); }
  .section { margin-bottom: 2em; border: 1px solid var(--color-border); border-radius: 4px; padding: 0.5em 1em; }
  details.section > summary { cursor: pointer; display: flex; align-items: center; gap: 0.8em; list-style: none; padding: 0.2em 0; }
  details.section > summary::-webkit-details-marker { display: none; }
  details.section > summary::before { content: "▶"; font-size: 0.75em; color: var(--color-text-muted); }
  details[open].section > summary::before { content: "▼"; }
  details.section > summary h2 { margin: 0; border-bottom: none; padding-bottom: 0; }
  .theme-toggle { position: fixed; top: 1em; right: 1em; padding: 0.4em 0.8em; border: 1px solid var(--color-border); border-radius: 4px; background: var(--color-bg-badge); color: var(--color-text); cursor: pointer; font-size: 0.9em; }
</style>
</head>
<body>
<button class="theme-toggle" aria-label="Toggle dark mode">{{if eq .Theme "dark"}}☀️{{else}}🌙{{end}}</button>
<h1>Beads Dashboard</h1>
{{range .Projects}}{{$proj := .Name}}
<details class="section" open>
<summary>
  <h2>{{.Name}}</h2>
  <div class="counts">
    <div{{if .NotReady}} class="badge-yellow"{{end}}><strong>Not Ready:</strong> {{len .NotReady}}</div>
    <div{{if .Open}} class="badge-green"{{end}}><strong>Open:</strong> {{len .Open}}</div>
    <div{{if .InProgress}} class="badge-green"{{end}}><strong>In Progress:</strong> {{len .InProgress}}</div>
    <div><strong>Closed:</strong> {{len .Closed}}</div>
  </div>
</summary>

{{if .NotReady}}
<h3>Not Ready</h3>
<table>
<tr><th style="width:1.5em"></th><th>ID</th><th>Title</th><th>Priority</th><th>Type</th><th>Updated</th></tr>
{{range .NotReady}}<tr><td>{{if .Blocked}}🔒{{end}}</td><td><a href="/bead/{{$proj}}/{{.ID}}">{{.ID}}</a></td><td>{{.Title}}</td><td>{{.Priority}}</td><td>{{.Type}}</td><td>{{fmtTime .UpdatedAt}}</td></tr>
{{end}}</table>
{{end}}

{{if .InProgress}}
<h3>In Progress</h3>
<table>
<tr><th>ID</th><th>Title</th><th>Assignee</th><th>Priority</th><th>Updated</th></tr>
{{range .InProgress}}<tr><td><a href="/bead/{{$proj}}/{{.ID}}">{{.ID}}</a></td><td>{{.Title}}</td><td>{{.Assignee}}</td><td>{{.Priority}}</td><td>{{fmtTime .UpdatedAt}}</td></tr>
{{end}}</table>
{{end}}

{{if .Open}}
<h3>Open</h3>
<table>
<tr><th style="width:1.5em"></th><th>ID</th><th>Title</th><th>Priority</th><th>Type</th><th>Updated</th></tr>
{{range .Open}}<tr><td>{{if .Blocked}}🔒{{end}}</td><td><a href="/bead/{{$proj}}/{{.ID}}">{{.ID}}</a></td><td>{{.Title}}</td><td>{{.Priority}}</td><td>{{.Type}}</td><td>{{fmtTime .UpdatedAt}}</td></tr>
{{end}}</table>
{{end}}

{{if .Closed}}
<h3>Closed ({{len .Closed}})</h3>
<table>
<tr><th>ID</th><th>Title</th><th>Priority</th><th>Updated</th></tr>
{{range .Closed}}<tr><td><a href="/bead/{{$proj}}/{{.ID}}">{{.ID}}</a></td><td>{{.Title}}</td><td>{{.Priority}}</td><td>{{fmtTime .UpdatedAt}}</td></tr>
{{end}}</table>
{{end}}

</details>
{{end}}
<script>
document.querySelectorAll("time[datetime]").forEach(function(el) {
  var d = new Date(el.getAttribute("datetime"));
  if (isNaN(d)) return;
  var pad = function(n) { return n < 10 ? "0" + n : "" + n; };
  var formatted = d.getFullYear() + "-" + pad(d.getMonth()+1) + "-" + pad(d.getDate()) +
    " " + pad(d.getHours()) + ":" + pad(d.getMinutes());
  var tz = d.toLocaleTimeString(undefined, {timeZoneName: "short"}).split(" ").pop();
  el.textContent = formatted + " " + tz;
});
document.querySelectorAll("details.section").forEach(function(el) {
  var h2 = el.querySelector("summary h2");
  if (!h2) return;
  var key = "section-open:" + h2.textContent.trim();
  if (localStorage.getItem(key) === "false") { el.removeAttribute("open"); }
  el.addEventListener("toggle", function() {
    localStorage.setItem(key, el.open ? "true" : "false");
  });
});
var html = document.documentElement;
if (!html.hasAttribute("data-theme")) {
  html.setAttribute("data-theme", window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light");
}
var themeBtn = document.querySelector("[aria-label=\"Toggle dark mode\"]");
function syncToggleBtn() {
  if (themeBtn) { themeBtn.textContent = html.getAttribute("data-theme") === "dark" ? "☀️" : "🌙"; }
}
syncToggleBtn();
if (themeBtn) {
  themeBtn.addEventListener("click", function() {
    var next = html.getAttribute("data-theme") === "dark" ? "light" : "dark";
    html.setAttribute("data-theme", next);
    document.cookie = "theme=" + next + "; path=/; max-age=31536000";
    syncToggleBtn();
  });
}
</script>
</body>
</html>
`))

var beadDetailTmpl = template.Must(template.New("bead-detail").Funcs(template.FuncMap{
	"fmtTime": func(t time.Time) template.HTML {
		utc := t.UTC().Format(time.RFC3339)
		display := t.UTC().Format("2006-01-02 15:04")
		return template.HTML(`<time datetime="` + utc + `">` + display + `</time>`)
	},
	"renderMarkdown": renderMarkdown,
}).Parse(`<!DOCTYPE html>
<html{{if .Theme}} data-theme="{{.Theme}}"{{end}}>
<head>
<meta charset="utf-8">
<title>{{.Bead.ID}} — {{.Bead.Title}}</title>
<style>
  :root {
    --color-text: #222;
    --color-bg-page: #fff;
    --color-link: #0366d6;
    --color-bg-badge: #f0f0f0;
    --color-bg-subtle: #fafafa;
    --color-border-light: #eee;
    --color-bg-tag: #e1ecf4;
    --color-border: #ddd;
    --color-bg-header: #f5f5f5;
    --color-text-secondary: #666;
  }
  [data-theme="dark"] {
    --color-text: #e0e0e0;
    --color-bg-page: #121212;
    --color-link: #58a6ff;
    --color-bg-badge: #333;
    --color-bg-subtle: #1a1a1a;
    --color-border-light: #333;
    --color-bg-tag: #1a3a5c;
    --color-border: #444;
    --color-bg-header: #2a2a2a;
    --color-text-secondary: #aaa;
  }
  body { font-family: sans-serif; margin: 2em; color: var(--color-text); background: var(--color-bg-page); }
  a { color: var(--color-link); text-decoration: none; }
  a:hover { text-decoration: underline; }
  h1 { margin-bottom: 0.2em; }
  .back { margin-bottom: 1em; }
  .meta { display: flex; gap: 1.5em; flex-wrap: wrap; margin-bottom: 1em; }
  .meta div { padding: 0.4em 0.8em; border-radius: 4px; background: var(--color-bg-badge); }
  .description { background: var(--color-bg-subtle); border: 1px solid var(--color-border-light); padding: 1em; border-radius: 4px; margin-bottom: 1em; }
  .tags span { display: inline-block; background: var(--color-bg-tag); padding: 0.2em 0.6em; border-radius: 3px; margin-right: 0.4em; font-size: 0.9em; }
  table { border-collapse: collapse; width: 100%; margin-bottom: 1em; }
  th, td { text-align: left; padding: 0.35em 0.7em; border: 1px solid var(--color-border); }
  th { background: var(--color-bg-header); }
  .comment { border: 1px solid var(--color-border-light); padding: 0.8em; margin-bottom: 0.5em; border-radius: 4px; }
  .comment-meta { font-size: 0.85em; color: var(--color-text-secondary); margin-bottom: 0.3em; }
  .comment-text { white-space: pre-wrap; }
  .section { margin-bottom: 1.5em; }
  .theme-toggle { position: fixed; top: 1em; right: 1em; padding: 0.4em 0.8em; border: 1px solid var(--color-border); border-radius: 4px; background: var(--color-bg-badge); color: var(--color-text); cursor: pointer; font-size: 0.9em; }
</style>
</head>
<body>
<button class="theme-toggle" aria-label="Toggle dark mode">{{if eq .Theme "dark"}}☀️{{else}}🌙{{end}}</button>
<div class="back"><a href="/">&#8592; Dashboard</a></div>
<h1>{{.Bead.Title}}</h1>
<p style="color: var(--color-text-secondary); margin-top:0;">{{.Bead.ID}}</p>

<div class="meta">
  <div><strong>Status:</strong> {{.Bead.Status}}</div>
  <div><strong>Priority:</strong> {{.Bead.Priority}}</div>
  <div><strong>Type:</strong> {{.Bead.Type}}</div>
  {{if .Bead.Assignee}}<div><strong>Assignee:</strong> {{.Bead.Assignee}}</div>{{end}}
  <div><strong>Created:</strong> {{fmtTime .Bead.CreatedAt}}</div>
  <div><strong>Updated:</strong> {{fmtTime .Bead.UpdatedAt}}</div>
</div>

{{if .Bead.Tags}}
<div class="section">
<h3>Tags</h3>
<div class="tags">{{range .Bead.Tags}}<span>{{.}}</span>{{end}}</div>
</div>
{{end}}

{{if .Bead.Description}}
<div class="section">
<h3>Description</h3>
<div class="description">{{renderMarkdown .Bead.Description}}</div>
</div>
{{end}}

{{if .ActiveBlockers}}
<div class="section">
<h3>Blocked By (Active)</h3>
<table>
<tr><th>ID</th><th>Title</th><th>Status</th><th>Priority</th></tr>
{{range .ActiveBlockers}}<tr><td><a href="/bead/{{$.Project}}/{{.ID}}">{{.ID}}</a></td><td>{{.Title}}</td><td>{{.Status}}</td><td>{{.Priority}}</td></tr>
{{end}}</table>
</div>
{{end}}

{{if .ResolvedBlockers}}
<div class="section">
<h3>Blocked By (Resolved)</h3>
<table>
<tr><th>ID</th><th>Title</th><th>Status</th><th>Priority</th></tr>
{{range .ResolvedBlockers}}<tr><td><a href="/bead/{{$.Project}}/{{.ID}}">{{.ID}}</a></td><td>{{.Title}}</td><td>{{.Status}}</td><td>{{.Priority}}</td></tr>
{{end}}</table>
</div>
{{end}}

{{if .Bead.Comments}}
<div class="section">
<h3>Comments ({{len .Bead.Comments}})</h3>
{{range .Bead.Comments}}<div class="comment">
<div class="comment-meta"><strong>{{.Author}}</strong> &middot; {{fmtTime .CreatedAt}}</div>
<div class="comment-text">{{.Text}}</div>
</div>
{{end}}</div>
{{end}}

<script>
document.querySelectorAll("time[datetime]").forEach(function(el) {
  var d = new Date(el.getAttribute("datetime"));
  if (isNaN(d)) return;
  var pad = function(n) { return n < 10 ? "0" + n : "" + n; };
  var formatted = d.getFullYear() + "-" + pad(d.getMonth()+1) + "-" + pad(d.getDate()) +
    " " + pad(d.getHours()) + ":" + pad(d.getMinutes());
  var tz = d.toLocaleTimeString(undefined, {timeZoneName: "short"}).split(" ").pop();
  el.textContent = formatted + " " + tz;
});
var html = document.documentElement;
if (!html.hasAttribute("data-theme")) {
  html.setAttribute("data-theme", window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light");
}
var themeBtn = document.querySelector("[aria-label=\"Toggle dark mode\"]");
function syncToggleBtn() {
  if (themeBtn) { themeBtn.textContent = html.getAttribute("data-theme") === "dark" ? "☀️" : "🌙"; }
}
syncToggleBtn();
if (themeBtn) {
  themeBtn.addEventListener("click", function() {
    var next = html.getAttribute("data-theme") === "dark" ? "light" : "dark";
    html.setAttribute("data-theme", next);
    document.cookie = "theme=" + next + "; path=/; max-age=31536000";
    syncToggleBtn();
  });
}
</script>
</body>
</html>
`))
