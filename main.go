package main

import (
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/dmage/team-gates/pkg/bugzilla"
	"github.com/dmage/team-gates/pkg/config"
)

const (
	bugzillaEndpoint     = "https://bugzilla.redhat.com/"
	bugzillaRestEndpoint = "https://bugzilla.redhat.com/rest/"
)

var (
	tmpl     = template.Must(template.ParseGlob("templates/*.html"))
	releases = []string{"4.6.0", "4.7.0"}
)

type Counts struct {
	MediumPlus     int
	RecentBlockers int
	AgedBlockers   int
}

func (c Counts) LessEqual(other Counts) bool {
	if c.MediumPlus > other.MediumPlus {
		return false
	}
	if c.RecentBlockers > other.RecentBlockers {
		return false
	}
	if c.AgedBlockers > other.AgedBlockers {
		return false
	}
	return true
}

type ReleaseInfo struct {
	Version    string
	BugCounts  Counts
	Blockers   int
	GateIsOpen bool
}

type bugzillaCachedResult struct {
	bugs       []bugzilla.Bug
	validUntil time.Time
}

type bugzillaCachedClient struct {
	mu      sync.RWMutex
	client  *bugzilla.Client
	results map[string]bugzillaCachedResult
}

func (c *bugzillaCachedClient) refresh(query string) ([]bugzilla.Bug, error) {
	log.Printf("refresh: %s", query)

	bugs, err := c.client.SearchBugs(query)
	if err != nil {
		return bugs, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.results[query] = bugzillaCachedResult{
		bugs:       bugs,
		validUntil: time.Now().Add(1 * time.Minute),
	}

	return bugs, nil
}

func (c *bugzillaCachedClient) SearchBugs(query url.Values) ([]bugzilla.Bug, error) {
	q := query.Encode()

	c.mu.RLock()
	r, ok := c.results[q]
	c.mu.RUnlock()

	if !ok {
		return c.refresh(q)
	}

	if time.Now().After(r.validUntil) {
		return c.refresh(q)
	}

	return r.bugs, nil
}

type server struct {
	cfg            *config.Config
	bugzillaClient *bugzillaCachedClient
	next           http.Handler
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if strings.HasPrefix(path, "/") {
		path = path[1:]
	}

	if path == "" {
		params := map[string]interface{}{
			"Teams": s.cfg.Teams,
		}
		w.Header().Add("Content-Type", "text/html; charset=utf-8")
		tmpl.ExecuteTemplate(w, "list.html", params)
		return
	}

	team, ok := s.cfg.Team(path)
	if !ok {
		s.next.ServeHTTP(w, r)
		return
	}

	if len(team.Components) == 0 {
		http.Error(w, "no components", 500)
		return
	}

	query := make(url.Values)
	query.Add("bug_status", "NEW")
	query.Add("bug_status", "ASSIGNED")
	query.Add("bug_status", "POST")
	query.Add("bug_status", "MODIFIED")
	query.Add("product", "OpenShift Container Platform")
	for _, c := range team.Components {
		query.Add("component", c)
	}

	now := time.Now()
	weekAgo := now.Add(-7 * 24 * time.Hour)

	bugs, err := s.bugzillaClient.SearchBugs(query)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	threshold := Counts{
		MediumPlus:     5 * team.Peeps,
		RecentBlockers: 1 * team.Peeps,
		AgedBlockers:   0,
	}

	var releaseInfos []ReleaseInfo
	for _, release := range releases {
		rel := ReleaseInfo{
			Version: release,
		}

		var counts Counts
		for _, bug := range bugs {
			if bug.Severity == "low" {
				continue
			}
			counts.MediumPlus++
			if len(bug.TargetRelease) == 0 || bug.TargetRelease[0] == release {
				if bug.CreationTime.After(weekAgo) {
					counts.RecentBlockers++
				} else {
					counts.AgedBlockers++
				}
			}
		}

		rel.BugCounts = counts
		rel.Blockers = counts.RecentBlockers + counts.AgedBlockers

		rel.GateIsOpen = counts.LessEqual(threshold)

		releaseInfos = append(releaseInfos, rel)
	}

	params := map[string]interface{}{
		"TeamName":         team.Name,
		"Releases":         releaseInfos,
		"Threshold":        threshold,
		"BugzillaEndpoint": bugzillaEndpoint,
		"BugzillaQuery":    template.URL(query.Encode()),
	}
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	err = tmpl.ExecuteTemplate(w, "index.html", params)
	if err != nil {
		log.Println(err)
	}
}

func main() {
	cfg, err := config.FromFile("./config.yaml")
	if err != nil {
		log.Fatal(err)
	}

	bugzillaClient := &bugzillaCachedClient{
		client:  bugzilla.NewClient(bugzillaRestEndpoint),
		results: make(map[string]bugzillaCachedResult),
	}

	s := &server{
		cfg:            cfg,
		bugzillaClient: bugzillaClient,
		next:           http.NotFoundHandler(),
	}

	http.Handle("/", s)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
