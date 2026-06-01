package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestClassroomListJSONEmptyArray(t *testing.T) {
	withClassroomTestService(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/topics") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"nextPageToken":""}`))
	}, func() {
		out := captureStdout(t, func() {
			_ = captureStderr(t, func() {
				if err := Execute([]string{"--json", "--account", "a@b.com", "classroom", "topics", "c1"}); err != nil {
					t.Fatalf("execute: %v", err)
				}
			})
		})

		var payload struct {
			Topics []json.RawMessage `json:"topics"`
		}
		if err := json.Unmarshal([]byte(out), &payload); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if payload.Topics == nil {
			t.Fatalf("topics should be [], got null in %s", out)
		}
		if len(payload.Topics) != 0 {
			t.Fatalf("expected no topics, got %d", len(payload.Topics))
		}
	})
}

func TestClassroomDirectListJSONEmptyArray(t *testing.T) {
	tests := []struct {
		name string
		path string
		args []string
		key  string
	}{
		{
			name: "students",
			path: "/courses/c1/students",
			args: []string{"--json", "--account", "a@b.com", "classroom", "students", "c1"},
			key:  "students",
		},
		{
			name: "invitations",
			path: "/invitations",
			args: []string{"--json", "--account", "a@b.com", "classroom", "invitations", "list", "--course", "c1"},
			key:  "invitations",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withClassroomTestService(t, func(w http.ResponseWriter, r *http.Request) {
				if !strings.Contains(r.URL.Path, tt.path) {
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"nextPageToken":""}`))
			}, func() {
				out := captureStdout(t, func() {
					_ = captureStderr(t, func() {
						if err := Execute(tt.args); err != nil {
							t.Fatalf("execute: %v", err)
						}
					})
				})

				var payload map[string]json.RawMessage
				if err := json.Unmarshal([]byte(out), &payload); err != nil {
					t.Fatalf("unmarshal: %v", err)
				}
				if string(payload[tt.key]) != "[]" {
					t.Fatalf("%s should be [], got %s in %s", tt.key, payload[tt.key], out)
				}
			})
		})
	}
}
