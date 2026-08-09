package cli

import (
	"net/http"
	"strings"
	"testing"
)

func TestDeleteCmd_Confirm(t *testing.T) {
	var deleted bool
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/request/5":
			w.Write([]byte(`{"id": 5, "status": 2, "media": {"id": 10, "tmdbId": 1234, "status": 4}, "createdAt": "2026-08-01T10:00:00.000Z"}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/request/5":
			deleted = true
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})

	for _, input := range []string{"y\n", "Y\n", "yes\n", "YES\n"} {
		deleted = false
		out, err := execute(t, input, "delete", "5")
		if err != nil {
			t.Fatalf("execute(%q) returned error: %v", input, err)
		}
		if !deleted {
			t.Errorf("execute(%q): expected a DELETE, got none", input)
		}
		for _, want := range []string{"1234", "Approved", "Partially available", "Deleted request #5."} {
			if !strings.Contains(out, want) {
				t.Errorf("execute(%q) output missing %q: %s", input, want, out)
			}
		}
	}
}

func TestDeleteCmd_Cancel(t *testing.T) {
	var deleted bool
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet:
			w.Write([]byte(`{"id": 5, "status": 2, "media": {"id": 10, "tmdbId": 1234, "status": 4}}`))
		case r.Method == http.MethodDelete:
			deleted = true
		}
	})

	for _, input := range []string{"n\n", "\n", "nope\n"} {
		deleted = false
		out, err := execute(t, input, "delete", "5")
		if err != nil {
			t.Fatalf("execute(%q) returned error: %v", input, err)
		}
		if deleted {
			t.Errorf("execute(%q): should not have deleted", input)
		}
		if !strings.Contains(out, "Cancelled.") {
			t.Errorf("execute(%q) output = %q, want Cancelled.", input, out)
		}
	}
}

func TestDeleteCmd_InvalidID(t *testing.T) {
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request should have been made for a non-numeric id")
	})

	_, err := execute(t, "", "delete", "abc")
	if err == nil {
		t.Fatal("execute() returned nil error, want an error for a non-numeric id")
	}
	if !strings.Contains(err.Error(), "invalid request id") {
		t.Errorf("err = %q, want it to mention invalid request id", err)
	}
}

func TestDeleteCmd_NotFound(t *testing.T) {
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"not found"}`))
	})

	_, err := execute(t, "", "delete", "999")
	if err == nil {
		t.Fatal("execute() returned nil error, want a not-found error")
	}
	if !strings.Contains(FormatError(err), "404") {
		t.Errorf("FormatError(err) = %q, want it to mention 404", FormatError(err))
	}
}

func TestDeleteCmd_Forbidden(t *testing.T) {
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Write([]byte(`{"id": 5, "status": 2, "media": {"id": 10, "tmdbId": 1234, "status": 5}}`))
		case http.MethodDelete:
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"message":"forbidden"}`))
		}
	})

	_, err := execute(t, "y\n", "delete", "5")
	if err == nil {
		t.Fatal("execute() returned nil error, want a forbidden error")
	}
	if !strings.Contains(FormatError(err), "MANAGE_REQUESTS") {
		t.Errorf("FormatError(err) = %q, want it to mention MANAGE_REQUESTS", FormatError(err))
	}
}
