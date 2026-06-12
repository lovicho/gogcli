package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/steipete/gogcli/internal/googleapi"
)

func TestPhotosSearchBuildsReadOnlyRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/mediaItems:search" {
			http.NotFound(w, r)
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["pageSize"].(float64) != 10 {
			t.Fatalf("unexpected body: %#v", body)
		}
		filters := body["filters"].(map[string]any)
		mt := filters["mediaTypeFilter"].(map[string]any)
		if got := mt["mediaTypes"].([]any)[0]; got != "PHOTO" {
			t.Fatalf("media type = %v", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"mediaItems": []map[string]any{{
				"id":         "m1",
				"filename":   "photo.jpg",
				"mimeType":   "image/jpeg",
				"productUrl": "https://photos.example/m1",
				"mediaMetadata": map[string]any{
					"creationTime": "2026-01-01T00:00:00Z",
				},
			}},
		})
	}))
	defer srv.Close()

	orig := newPhotosClient
	t.Cleanup(func() { newPhotosClient = orig })
	newPhotosClient = func(context.Context, string) (*googleapi.PhotosClient, error) {
		return googleapi.NewPhotosClient(srv.Client(), googleapi.WithPhotosBaseURL(srv.URL)), nil
	}

	out := captureStdout(t, func() {
		cmd := &PhotosSearchCmd{MediaType: "PHOTO", Max: 10, From: "2026-01-01", To: "2026-01-02"}
		if err := cmd.Run(newCmdJSONContext(t), &RootFlags{Account: "a@example.com"}); err != nil {
			t.Fatalf("Run: %v", err)
		}
	})
	var parsed struct {
		MediaItemCount int `json:"mediaItemCount"`
		MediaItems     []struct {
			ID string `json:"id"`
		} `json:"mediaItems"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json parse: %v\n%s", err, out)
	}
	if parsed.MediaItemCount != 1 || len(parsed.MediaItems) != 1 || parsed.MediaItems[0].ID != "m1" {
		t.Fatalf("unexpected output: %#v", parsed)
	}
}

func TestPhotosValidationFailsBeforeClient(t *testing.T) {
	orig := newPhotosClient
	t.Cleanup(func() { newPhotosClient = orig })
	newPhotosClient = func(context.Context, string) (*googleapi.PhotosClient, error) {
		t.Fatalf("expected local validation to fail before creating Photos client")
		return nil, context.Canceled
	}

	testCases := []struct {
		name string
		args []string
		want string
	}{
		{name: "list zero max", args: []string{"--account", "a@b.com", "photos", "list", "--max", "0"}, want: "max must be > 0"},
		{name: "list negative max", args: []string{"--account", "a@b.com", "photos", "list", "--max=-1"}, want: "max must be > 0"},
		{name: "list max above api limit", args: []string{"--account", "a@b.com", "photos", "list", "--max", "101"}, want: "max must be <= 100"},
		{name: "search zero max", args: []string{"--account", "a@b.com", "photos", "search", "--max", "0"}, want: "max must be > 0"},
		{name: "search negative max", args: []string{"--account", "a@b.com", "photos", "search", "--max=-1"}, want: "max must be > 0"},
		{name: "search max above api limit", args: []string{"--account", "a@b.com", "photos", "search", "--max", "101"}, want: "max must be <= 100"},
		{name: "search bad from", args: []string{"--account", "a@b.com", "photos", "search", "--from", "nope"}, want: "--from must be YYYY-MM-DD"},
		{name: "search bad to", args: []string{"--account", "a@b.com", "photos", "search", "--to", "nope"}, want: "--to must be YYYY-MM-DD"},
		{name: "get empty id", args: []string{"--account", "a@b.com", "photos", "get", ""}, want: "empty mediaItemId"},
		{name: "download empty id", args: []string{"--account", "a@b.com", "photos", "download", ""}, want: "empty mediaItemId"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_ = captureStderr(t, func() {
				err := Execute(tc.args)
				if err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("unexpected err: %v", err)
				}
			})
		})
	}
}
