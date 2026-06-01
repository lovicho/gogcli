package cmd

import (
	"context"
	"errors"
	"strings"
	"testing"

	"google.golang.org/api/tasks/v1"
)

func TestExecute_TasksAdd_RequiresTitle(t *testing.T) {
	origNew := newTasksService
	t.Cleanup(func() { newTasksService = origNew })
	newTasksService = func(context.Context, string) (*tasks.Service, error) {
		t.Fatalf("expected validation to fail before creating service")
		return nil, errors.New("unexpected tasks service call")
	}

	_ = captureStderr(t, func() {
		err := Execute([]string{"--account", "a@b.com", "tasks", "add", "l1"})
		if err == nil || !strings.Contains(err.Error(), "required: --title") {
			t.Fatalf("unexpected err: %v", err)
		}
	})
}

func TestExecute_TasksListInvalidMaxFailsBeforeService(t *testing.T) {
	origNew := newTasksService
	t.Cleanup(func() { newTasksService = origNew })
	newTasksService = func(context.Context, string) (*tasks.Service, error) {
		t.Fatalf("expected max validation to fail before creating service")
		return nil, errors.New("unexpected tasks service call")
	}

	cases := [][]string{
		{"--account", "a@b.com", "tasks", "lists", "--max", "0"},
		{"--account", "a@b.com", "tasks", "lists", "--max=-1"},
		{"--account", "a@b.com", "tasks", "list", "l1", "--max", "0"},
		{"--account", "a@b.com", "tasks", "list", "l1", "--max=-1"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			_ = captureStderr(t, func() {
				err := Execute(args)
				if ExitCode(err) != 2 || !strings.Contains(err.Error(), "max must be > 0") {
					t.Fatalf("unexpected err: %v", err)
				}
			})
		})
	}
}

func TestExecute_TasksAdd_RejectsInvalidDueBeforeDryRun(t *testing.T) {
	origNew := newTasksService
	t.Cleanup(func() { newTasksService = origNew })
	newTasksService = func(context.Context, string) (*tasks.Service, error) {
		t.Fatalf("expected validation to fail before creating service")
		return nil, errors.New("unexpected tasks service call")
	}

	_ = captureStderr(t, func() {
		err := Execute([]string{"--account", "a@b.com", "tasks", "add", "l1", "--title", "Task", "--due", "nope", "--dry-run"})
		var exitErr *ExitError
		if !errors.As(err, &exitErr) || exitErr.Code != 2 || !strings.Contains(err.Error(), "invalid date/time") {
			t.Fatalf("unexpected err: %v", err)
		}
	})
}

func TestExecute_TasksAdd_RejectsInvalidRepeatDatesBeforeDryRun(t *testing.T) {
	origNew := newTasksService
	t.Cleanup(func() { newTasksService = origNew })
	newTasksService = func(context.Context, string) (*tasks.Service, error) {
		t.Fatalf("expected validation to fail before creating service")
		return nil, errors.New("unexpected tasks service call")
	}

	_ = captureStderr(t, func() {
		err := Execute([]string{"--account", "a@b.com", "tasks", "add", "l1", "--title", "Task", "--due", "2026-01-01", "--repeat", "daily", "--repeat-until", "nope", "--dry-run"})
		var exitErr *ExitError
		if !errors.As(err, &exitErr) || exitErr.Code != 2 || !strings.Contains(err.Error(), "invalid date/time") {
			t.Fatalf("unexpected err: %v", err)
		}
	})

	_ = captureStderr(t, func() {
		err := Execute([]string{"--account", "a@b.com", "tasks", "add", "l1", "--title", "Task", "--due", "2026-01-02", "--repeat", "daily", "--repeat-until", "2026-01-01", "--dry-run"})
		var exitErr *ExitError
		if !errors.As(err, &exitErr) || exitErr.Code != 2 || !strings.Contains(err.Error(), "repeat produced no occurrences") {
			t.Fatalf("unexpected err: %v", err)
		}
	})
}

func TestExecute_TasksAdd_DryRunDoesNotExpandRepeatSchedule(t *testing.T) {
	origNew := newTasksService
	t.Cleanup(func() { newTasksService = origNew })
	newTasksService = func(context.Context, string) (*tasks.Service, error) {
		t.Fatalf("dry-run should exit before creating service")
		return nil, errors.New("unexpected tasks service call")
	}

	out := captureStdout(t, func() {
		err := Execute([]string{"--account", "a@b.com", "tasks", "add", "l1", "--title", "Task", "--due", "2026-01-01", "--repeat", "daily", "--repeat-count", "1000000000", "--dry-run", "--json"})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
	})
	if !strings.Contains(out, `"repeat_count": 1000000000`) {
		t.Fatalf("expected dry-run payload, got %s", out)
	}
}

func TestExecute_TasksUpdate_RequiresFields(t *testing.T) {
	origNew := newTasksService
	t.Cleanup(func() { newTasksService = origNew })
	newTasksService = func(context.Context, string) (*tasks.Service, error) {
		t.Fatalf("expected validation to fail before creating service")
		return nil, errors.New("unexpected tasks service call")
	}

	_ = captureStderr(t, func() {
		err := Execute([]string{"--account", "a@b.com", "tasks", "update", "l1", "t1"})
		if err == nil || !strings.Contains(err.Error(), "no fields to update") {
			t.Fatalf("unexpected err: %v", err)
		}
	})
}

func TestExecute_TasksUpdate_RejectsInvalidStatus(t *testing.T) {
	origNew := newTasksService
	t.Cleanup(func() { newTasksService = origNew })
	newTasksService = func(context.Context, string) (*tasks.Service, error) {
		t.Fatalf("expected validation to fail before creating service")
		return nil, errors.New("unexpected tasks service call")
	}

	_ = captureStderr(t, func() {
		err := Execute([]string{"--account", "a@b.com", "tasks", "update", "l1", "t1", "--status", "nope"})
		if err == nil || !strings.Contains(err.Error(), "invalid --status") {
			t.Fatalf("unexpected err: %v", err)
		}
	})
}

func TestExecute_TasksUpdate_RejectsInvalidDueAsUsage(t *testing.T) {
	origNew := newTasksService
	t.Cleanup(func() { newTasksService = origNew })
	newTasksService = func(context.Context, string) (*tasks.Service, error) {
		t.Fatalf("expected validation to fail before creating service")
		return nil, errors.New("unexpected tasks service call")
	}

	_ = captureStderr(t, func() {
		err := Execute([]string{"--account", "a@b.com", "tasks", "update", "l1", "t1", "--due", "nope", "--dry-run"})
		var exitErr *ExitError
		if !errors.As(err, &exitErr) || exitErr.Code != 2 || !strings.Contains(err.Error(), "invalid date/time") {
			t.Fatalf("unexpected err: %v", err)
		}
	})
}
