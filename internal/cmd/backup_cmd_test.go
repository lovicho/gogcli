package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestBackupInitDryRunDoesNotWriteConfigOrRepo(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "backup.json")
	repoPath := filepath.Join(dir, "repo")

	var stdout bytes.Buffer
	origStdout := os.Stdout
	readPipe, writePipe, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("pipe: %v", pipeErr)
	}
	os.Stdout = writePipe
	t.Cleanup(func() {
		os.Stdout = origStdout
	})
	err := (&BackupInitCmd{
		backupFlags: backupFlags{
			Config: configPath,
			Repo:   repoPath,
			NoPush: true,
		},
	}).Run(newCmdJSONOutputContext(t, &stdout, nil), &RootFlags{DryRun: true, NoInput: true})
	_ = writePipe.Close()
	os.Stdout = origStdout
	if _, copyErr := io.Copy(&stdout, readPipe); copyErr != nil {
		t.Fatalf("read stdout: %v", copyErr)
	}

	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 0 {
		t.Fatalf("expected dry-run exit 0, got %#v", err)
	}
	if _, statErr := os.Stat(configPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("dry-run wrote config: %v", statErr)
	}
	if _, statErr := os.Stat(repoPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("dry-run created repo: %v", statErr)
	}

	var payload struct {
		DryRun  bool           `json:"dry_run"`
		Op      string         `json:"op"`
		Request map[string]any `json:"request"`
	}
	if decodeErr := json.Unmarshal(stdout.Bytes(), &payload); decodeErr != nil {
		t.Fatalf("decode dry-run output: %v\n%s", decodeErr, stdout.String())
	}
	if !payload.DryRun || payload.Op != "backup.init" {
		t.Fatalf("unexpected dry-run payload: %#v", payload)
	}
	if payload.Request["repo"] != repoPath || payload.Request["push"] != false {
		t.Fatalf("unexpected request: %#v", payload.Request)
	}
}
