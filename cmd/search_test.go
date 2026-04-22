package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/kalistat-data/cli/internal/api"
)

const searchResponseJSON = `{
  "data": [
    {
      "code": "EU.LAMA.014",
      "name": "Employment rate",
      "type": "dataset",
      "source": "eurostat",
      "category": {"name": "Employment and activity - LFS", "key": "EU.5.2.1", "source": "eurostat"},
      "links": {"self": "/api/v1/datasets/EU.LAMA.014"}
    },
    {
      "code": "IT.LAMA.132",
      "name": "Labour market — monthly",
      "type": "dataset",
      "source": "istat",
      "category": {"name": "Labour market", "key": "IT.5.2", "source": "istat"},
      "links": {"self": "/api/v1/datasets/IT.LAMA.132"}
    }
  ],
  "meta": {
    "query": "employment",
    "pagination": {"total": 88, "page": 1, "page_size": 2, "has_more": true},
    "generated_at": "2026-04-22T18:00:00Z"
  }
}`

func TestSearch_PassesQueryParamsToAPI(t *testing.T) {
	loggedIn(t)
	var gotPath, gotQuery string
	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(searchResponseJSON))
	})

	err := runCLI(t, "search", "employment",
		"--source", "istat",
		"--category-key", "IT.5",
		"--page", "2",
		"--page-size", "10")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasSuffix(gotPath, "/search") {
		t.Errorf("path = %q, want to end with /search", gotPath)
	}
	for _, want := range []string{"q=employment", "source=istat", "category_key=IT.5", "page=2", "page_size=10"} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("query %q missing %q", gotQuery, want)
		}
	}
}

func TestSearch_PrettyOutputIncludesPagination(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, searchResponseJSON, 0)

	if err := runCLI(t, "search", "employment"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"Showing 1-2 of 88 matches",
		"EU.LAMA.014", "Employment rate",
		"IT.LAMA.132",
		"CATEGORY KEY",
		"EU.5.2.1", "IT.5.2", // category keys must appear so users can filter by them
		"More results", "--page 2",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n%s", want, out)
		}
	}
}

func TestSearch_EmptyResult(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, `{"data":[],"meta":{"pagination":{"total":0,"page":1,"page_size":50,"has_more":false}}}`, 0)

	if err := runCLI(t, "search", "nothing"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "No matches") {
		t.Errorf("output should say no matches, got %q", buf.String())
	}
}

func TestSearch_JSONPassthrough(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, searchResponseJSON, 0)

	if err := runCLI(t, "search", "employment", "--json"); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, buf.String())
	}
	if _, ok := got["data"]; !ok {
		t.Errorf("missing 'data'")
	}
}

func TestSearch_APIErrorSurfacesCleanly(t *testing.T) {
	loggedIn(t)
	mockJSONAPI(t,
		`{"error":{"code":"bad_request","message":"q is required"}}`,
		http.StatusBadRequest,
	)

	err := runCLI(t, "search", "x")
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %T %v, want *APIError", err, err)
	}
	if apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d", apiErr.StatusCode)
	}
}

func TestSearch_RequiresQueryArg(t *testing.T) {
	resetCmd(t)

	if err := runCLI(t, "search"); err == nil {
		t.Fatal("expected error when query arg is missing")
	}
}

func TestSearch_EmptyQueryRejectedLocally(t *testing.T) {
	loggedIn(t)
	// Server should never be hit; fail the test if it is.
	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("API should not be called for empty query; got %s %s", r.Method, r.URL)
	})

	for _, empty := range []string{"", "   ", "\t\n"} {
		err := runCLI(t, "search", empty)
		if err == nil {
			t.Errorf("search %q should fail locally", empty)
			continue
		}
		if !strings.Contains(err.Error(), "cannot be empty") {
			t.Errorf("error = %q, want to mention empty query", err)
		}
		if strings.Contains(err.Error(), "'q'") {
			t.Errorf("error must not leak internal param name %q: %q", "q", err)
		}
	}
}
