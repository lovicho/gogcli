package cmd

import (
	"context"
	"errors"
	"io"
	"testing"

	"google.golang.org/api/gmail/v1"

	"github.com/steipete/gogcli/internal/ui"
)

func TestGmailSendAsCmd_ValidationErrors(t *testing.T) {
	u, uiErr := ui.New(ui.Options{Stdout: io.Discard, Stderr: io.Discard, Color: "never"})
	if uiErr != nil {
		t.Fatalf("ui.New: %v", uiErr)
	}
	ctx := ui.WithUI(context.Background(), u)

	if err := (&GmailSendAsListCmd{}).Run(ctx, &RootFlags{}); err == nil {
		t.Fatalf("expected missing account error")
	}
	tests := []struct {
		name string
		run  func() error
	}{
		{name: "get", run: func() error { return (&GmailSendAsGetCmd{}).Run(ctx, &RootFlags{Account: "a@b.com"}) }},
		{name: "create", run: func() error { return (&GmailSendAsCreateCmd{}).Run(ctx, &RootFlags{Account: "a@b.com"}) }},
		{name: "verify", run: func() error { return (&GmailSendAsVerifyCmd{}).Run(ctx, &RootFlags{Account: "a@b.com"}) }},
		{name: "delete", run: func() error { return (&GmailSendAsDeleteCmd{}).Run(ctx, &RootFlags{Account: "a@b.com"}) }},
		{name: "update", run: func() error { return (&GmailSendAsUpdateCmd{}).Run(ctx, nil, &RootFlags{Account: "a@b.com"}) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run()
			if err == nil {
				t.Fatal("expected missing email error")
			}
			if got := ExitCode(err); got != 2 {
				t.Fatalf("ExitCode = %d, want 2 (err=%v)", got, err)
			}
		})
	}
}

func TestGmailSendAsListCmd_ServiceError(t *testing.T) {
	origNew := newGmailService
	t.Cleanup(func() { newGmailService = origNew })

	newGmailService = func(context.Context, string) (*gmail.Service, error) {
		return nil, errors.New("service down")
	}

	u, uiErr := ui.New(ui.Options{Stdout: io.Discard, Stderr: io.Discard, Color: "never"})
	if uiErr != nil {
		t.Fatalf("ui.New: %v", uiErr)
	}
	ctx := ui.WithUI(context.Background(), u)

	if err := (&GmailSendAsListCmd{}).Run(ctx, &RootFlags{Account: "a@b.com"}); err == nil {
		t.Fatalf("expected service error")
	}
}
