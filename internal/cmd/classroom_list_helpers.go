package cmd

import (
	"context"
	"io"

	"github.com/steipete/gogcli/internal/outfmt"
	"github.com/steipete/gogcli/internal/ui"
)

func fetchClassroomPagedList[T any](all bool, page string, fetch func(string) ([]*T, string, error)) ([]*T, string, error) {
	return loadPagedItems(page, all, fetch)
}

func nonNilClassroomItems[T any](items []*T) []*T {
	if items == nil {
		return []*T{}
	}
	return items
}

func writeClassroomPagedList[T any](
	ctx context.Context,
	jsonKey string,
	items []*T,
	nextPageToken string,
	emptyMessage string,
	failEmpty bool,
	hintOnEmpty bool,
	printTable func(io.Writer),
) error {
	items = nonNilClassroomItems(items)
	if outfmt.IsJSON(ctx) {
		if err := outfmt.WriteJSON(ctx, stdoutWriter(ctx), map[string]any{
			jsonKey:         items,
			"nextPageToken": nextPageToken,
		}); err != nil {
			return err
		}
		if len(items) == 0 {
			return failEmptyExit(failEmpty)
		}
		return nil
	}

	u := ui.FromContext(ctx)
	if len(items) == 0 {
		u.Err().Println(emptyMessage)
		if hintOnEmpty {
			printNextPageHint(u, nextPageToken)
		}
		return failEmptyExit(failEmpty)
	}

	w, flush := tableWriter(ctx)
	defer flush()
	printTable(w)
	printNextPageHint(u, nextPageToken)
	return nil
}
