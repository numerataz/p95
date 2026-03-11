package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCompositeGetTeamsAndRouting(t *testing.T) {
	remoteRunID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	localRunID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	var remoteHits, localHits []string

	remoteSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remoteHits = append(remoteHits, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/v1/teams":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id":          "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
				"name":        "Remote Team",
				"slug":        "acme",
				"description": "",
				"role":        "owner",
			}})
		case r.URL.Path == "/api/v1/teams/acme/apps":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id":          "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
				"name":        "Remote App",
				"slug":        "remote-app",
				"description": "",
				"run_count":   1,
			}})
		case r.URL.Path == "/api/v1/teams/acme/apps/remote-app/runs":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id":         remoteRunID,
				"name":       "remote-run",
				"status":     "completed",
				"started_at": time.Now().UTC().Format(time.RFC3339),
			}})
		case r.URL.Path == "/api/v1/runs/"+remoteRunID.String():
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":         remoteRunID,
				"name":       "remote-run",
				"status":     "completed",
				"started_at": time.Now().UTC().Format(time.RFC3339),
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer remoteSrv.Close()

	localSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		localHits = append(localHits, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/v1/teams/local/apps":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id":          "cccccccc-cccc-cccc-cccc-cccccccccccc",
				"name":        "Local App",
				"slug":        "local-app",
				"description": "",
				"run_count":   1,
			}})
		case r.URL.Path == "/api/v1/teams/local/apps/local-app/runs":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id":         localRunID,
				"name":       "local-run",
				"status":     "completed",
				"started_at": time.Now().UTC().Format(time.RFC3339),
			}})
		case r.URL.Path == "/api/v1/runs/"+localRunID.String():
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":         localRunID,
				"name":       "local-run",
				"status":     "completed",
				"started_at": time.Now().UTC().Format(time.RFC3339),
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer localSrv.Close()

	cc := NewComposite(New(localSrv.URL), New(remoteSrv.URL))

	teams, err := cc.GetTeams()
	if err != nil {
		t.Fatalf("GetTeams() error = %v", err)
	}
	if len(teams) != 2 {
		t.Fatalf("GetTeams() len = %d, want 2", len(teams))
	}
	if teams[0].Slug != "remote:acme" {
		t.Fatalf("remote team slug = %q, want %q", teams[0].Slug, "remote:acme")
	}
	if teams[1].Slug != "__local__" {
		t.Fatalf("local team slug = %q, want %q", teams[1].Slug, "__local__")
	}

	if _, err := cc.GetApps("remote:acme"); err != nil {
		t.Fatalf("GetApps(remote) error = %v", err)
	}
	if _, err := cc.GetApps("__local__"); err != nil {
		t.Fatalf("GetApps(local) error = %v", err)
	}
	if _, err := cc.GetRuns("remote:acme", "remote-app"); err != nil {
		t.Fatalf("GetRuns(remote) error = %v", err)
	}
	if _, err := cc.GetRuns("__local__", "local-app"); err != nil {
		t.Fatalf("GetRuns(local) error = %v", err)
	}

	if _, err := cc.GetRun(remoteRunID, true); err != nil {
		t.Fatalf("GetRun(remote) error = %v", err)
	}
	if _, err := cc.GetRun(localRunID, true); err != nil {
		t.Fatalf("GetRun(local) error = %v", err)
	}

	if !containsPath(remoteHits, "/api/v1/teams/acme/apps") {
		t.Fatalf("remote server did not receive remote apps request, hits=%v", remoteHits)
	}
	if !containsPath(localHits, "/api/v1/teams/local/apps") {
		t.Fatalf("local server did not receive local apps request, hits=%v", localHits)
	}
	if !containsPath(remoteHits, "/api/v1/runs/"+remoteRunID.String()) {
		t.Fatalf("remote server did not receive remote run detail request, hits=%v", remoteHits)
	}
	if !containsPath(localHits, "/api/v1/runs/"+localRunID.String()) {
		t.Fatalf("local server did not receive local run detail request, hits=%v", localHits)
	}
}

func TestCompositeGetTeamsFallsBackToLocalWhenRemoteUnavailable(t *testing.T) {
	remoteSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "remote down", http.StatusInternalServerError)
	}))
	defer remoteSrv.Close()

	localSrv := httptest.NewServer(http.NotFoundHandler())
	defer localSrv.Close()

	cc := NewComposite(New(localSrv.URL), New(remoteSrv.URL))
	teams, err := cc.GetTeams()
	if err != nil {
		t.Fatalf("GetTeams() error = %v", err)
	}
	if len(teams) != 1 {
		t.Fatalf("GetTeams() len = %d, want 1", len(teams))
	}
	if teams[0].Slug != "__local__" {
		t.Fatalf("local team slug = %q, want %q", teams[0].Slug, "__local__")
	}
}

func containsPath(hits []string, want string) bool {
	for _, h := range hits {
		if strings.SplitN(h, "?", 2)[0] == want {
			return true
		}
	}
	return false
}
