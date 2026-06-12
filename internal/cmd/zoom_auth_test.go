package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/steipete/gogcli/internal/outfmt"
	"github.com/steipete/gogcli/internal/zoom"
)

func withTempZoomAuthStore(t *testing.T) string {
	t.Helper()
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("GOG_KEYRING_BACKEND", "file")
	t.Setenv("GOG_KEYRING_PASSWORD", "test-pass")
	t.Setenv("GOG_ZOOM_ACCOUNT_ID", "")
	t.Setenv("GOG_ZOOM_CLIENT_ID", "")
	t.Setenv("GOG_ZOOM_CLIENT_SECRET", "")
	return xdg
}

func TestZoomAuthSetupCmd_StoresCredentialsWithoutValidation(t *testing.T) {
	withTempZoomAuthStore(t)
	cmd := &ZoomAuthSetupCmd{
		Alias:        "work",
		AccountID:    "acct",
		ClientID:     "client",
		ClientSecret: "secret",
		SkipValidate: true,
	}
	var output bytes.Buffer
	if err := cmd.Run(newCmdRuntimeJSONOutputContext(t, &output, io.Discard), &RootFlags{}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(output.String(), `"saved": true`) {
		t.Fatalf("unexpected output: %s", output.String())
	}
	creds, err := zoom.LoadCredentials("work")
	if err != nil {
		t.Fatalf("LoadCredentials: %v", err)
	}
	if creds.AccountID != "acct" || creds.ClientID != "client" || creds.ClientSecret != "secret" {
		t.Fatalf("unexpected creds: %#v", creds)
	}
}

func TestZoomAuthDoctorCmd_NoCredentials(t *testing.T) {
	withTempZoomAuthStore(t)
	var output bytes.Buffer
	if err := (&ZoomAuthDoctorCmd{}).Run(newCmdRuntimeJSONOutputContext(t, &output, io.Discard), &RootFlags{}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(output.String(), `"status": "error"`) {
		t.Fatalf("unexpected output: %s", output.String())
	}
}

func TestZoomAuthSetupCmd_NoInputRequiresFlags(t *testing.T) {
	withTempZoomAuthStore(t)
	err := (&ZoomAuthSetupCmd{SkipValidate: true}).Run(context.Background(), &RootFlags{NoInput: true})
	if err == nil {
		t.Fatalf("expected usage error")
	}
}

func TestZoomAuthSetupCmd_DryRunDoesNotStoreCredentials(t *testing.T) {
	xdg := withTempZoomAuthStore(t)
	cmd := &ZoomAuthSetupCmd{
		Alias:        "dry",
		SkipValidate: true,
	}
	var output bytes.Buffer
	err := cmd.Run(newCmdRuntimeJSONOutputContext(t, &output, io.Discard), &RootFlags{DryRun: true, NoInput: true})
	if ExitCode(err) != 0 {
		t.Fatalf("expected dry-run exit 0, got %v", err)
	}
	out := output.String()
	if strings.Contains(out, "topsecretvalue") {
		t.Fatalf("dry-run output leaked client secret: %s", out)
	}
	var parsed struct {
		DryRun  bool `json:"dry_run"`
		Request struct {
			Alias           string `json:"alias"`
			ClientSecretSet bool   `json:"client_secret_set"`
		} `json:"request"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("parse dry-run JSON: %v\n%s", err, out)
	}
	if !parsed.DryRun || parsed.Request.Alias != "dry" || parsed.Request.ClientSecretSet {
		t.Fatalf("unexpected dry-run payload: %#v", parsed)
	}
	if _, err := os.Stat(filepath.Join(xdg, "gogcli")); err == nil {
		t.Fatalf("dry-run created config directory")
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat config directory: %v", err)
	}
}

func TestZoomAuthSetupCmd_ReadsPromptsFromRuntime(t *testing.T) {
	withTempZoomAuthStore(t)
	var output bytes.Buffer
	var diagnostics bytes.Buffer
	ctx := newCmdRuntimeIOContext(t, strings.NewReader("acct\nclient\nsecret\n"), &output, &diagnostics)
	ctx = outfmt.WithMode(ctx, outfmt.Mode{JSON: true})

	err := (&ZoomAuthSetupCmd{
		Alias:        "prompted",
		SkipValidate: true,
	}).Run(ctx, &RootFlags{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	creds, err := zoom.LoadCredentials("prompted")
	if err != nil {
		t.Fatalf("LoadCredentials: %v", err)
	}
	if creds.AccountID != "acct" || creds.ClientID != "client" || creds.ClientSecret != "secret" {
		t.Fatalf("unexpected creds: %#v", creds)
	}
	if got := diagnostics.String(); !strings.Contains(got, "Zoom account ID: ") || !strings.Contains(got, "Zoom client secret: ") {
		t.Fatalf("unexpected prompts: %q", got)
	}
}
