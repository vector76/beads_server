package server

import (
	"html/template"
	"net/http"

	"github.com/yourorg/beads_server/internal/model"
	"github.com/yourorg/beads_server/internal/store"
)

// dashboardProject holds the template data for one project.
type dashboardProject struct {
	Name       string
	InProgress []store.BeadSummary
	Open       []store.BeadSummary
	Closed     []store.BeadSummary
}

// dashboardData holds the full template data.
type dashboardData struct {
	Projects []dashboardProject
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
			}
		}
		data.Projects = append(data.Projects, dp)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := dashboardTmpl.Execute(w, data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

var dashboardTmpl = template.Must(template.New("dashboard").Parse(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>Beads Dashboard</title>
<style>
  body { font-family: sans-serif; margin: 2em; color: #222; }
  h1 { margin-bottom: 0.2em; }
  h2 { border-bottom: 1px solid #ccc; padding-bottom: 0.2em; }
  h3 { margin-top: 1.2em; }
  table { border-collapse: collapse; width: 100%; margin-bottom: 1em; }
  th, td { text-align: left; padding: 0.35em 0.7em; border: 1px solid #ddd; }
  th { background: #f5f5f5; }
  .counts { display: flex; gap: 1.5em; margin-bottom: 1em; flex-wrap: wrap; }
  .counts div { padding: 0.4em 0.8em; border-radius: 4px; background: #f0f0f0; }
  .section { margin-bottom: 2em; }
</style>
</head>
<body>
<h1>Beads Dashboard</h1>
{{range .Projects}}
<div class="section">
<h2>{{.Name}}</h2>
<div class="counts">
  <div><strong>In Progress:</strong> {{len .InProgress}}</div>
  <div><strong>Open:</strong> {{len .Open}}</div>
  <div><strong>Closed:</strong> {{len .Closed}}</div>
</div>

{{if .InProgress}}
<h3>In Progress</h3>
<table>
<tr><th>ID</th><th>Title</th><th>Assignee</th><th>Priority</th></tr>
{{range .InProgress}}<tr><td>{{.ID}}</td><td>{{.Title}}</td><td>{{.Assignee}}</td><td>{{.Priority}}</td></tr>
{{end}}</table>
{{end}}

{{if .Open}}
<h3>Open</h3>
<table>
<tr><th>ID</th><th>Title</th><th>Priority</th><th>Type</th></tr>
{{range .Open}}<tr><td>{{.ID}}</td><td>{{.Title}}</td><td>{{.Priority}}</td><td>{{.Type}}</td></tr>
{{end}}</table>
{{end}}

{{if .Closed}}
<h3>Closed ({{len .Closed}})</h3>
<table>
<tr><th>ID</th><th>Title</th><th>Priority</th></tr>
{{range .Closed}}<tr><td>{{.ID}}</td><td>{{.Title}}</td><td>{{.Priority}}</td></tr>
{{end}}</table>
{{end}}

</div>
{{end}}
</body>
</html>
`))
