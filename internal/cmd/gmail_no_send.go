package cmd

import (
	"context"
	"strings"

	"github.com/alecthomas/kong"

	"github.com/steipete/gogcli/internal/app"
)

var gmailSendCommandPaths = map[string]struct{}{
	"send":              {},
	"gmail.send":        {},
	"gmail.autoreply":   {},
	"gmail.forward":     {},
	"gmail.fwd":         {},
	"gmail.drafts.send": {},
}

func enforceGmailNoSend(kctx *kong.Context, flags *RootFlags, runtime *app.Runtime) error {
	if !isGmailSendPath(commandPath(kctx.Command())) {
		return nil
	}
	homeOverride := ""
	if flags != nil {
		if flags.GmailNoSend {
			return usage("Gmail sending is blocked by --gmail-no-send")
		}
		homeOverride = flags.Home
	}
	if err := configureRuntimeConfig(runtime, homeOverride); err != nil {
		return err
	}
	cfg, err := runtime.Config.Read()
	if err != nil {
		return err
	}
	if cfg.GmailNoSend {
		return usage("Gmail sending is blocked by config gmail_no_send")
	}
	return nil
}

func checkAccountNoSend(ctx context.Context, account string) error {
	store, err := commandConfigStore(ctx)
	if err != nil {
		return err
	}
	disabled, err := store.IsNoSendAccount(account)
	if err != nil {
		return err
	}
	if disabled {
		return usagef("Gmail sending is blocked for %s (config no-send)", strings.TrimSpace(account))
	}
	return nil
}

func isGmailSendPath(path []string) bool {
	if len(path) == 0 {
		return false
	}
	_, ok := gmailSendCommandPaths[strings.Join(path, ".")]
	return ok
}
