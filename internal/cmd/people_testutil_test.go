package cmd

import (
	"context"
	"net/http/httptest"
	"testing"

	"google.golang.org/api/people/v1"

	"github.com/steipete/gogcli/internal/app"
)

func newPeopleServiceFromServer(t *testing.T, srv *httptest.Server) *people.Service {
	t.Helper()
	return newGoogleTestServiceWithEndpoint(t, srv.Client(), srv.URL+"/", people.NewService)
}

func withPeopleContactsTestService(ctx context.Context, svc *people.Service) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	runtime := &app.Runtime{}
	if existing, ok := app.FromContext(ctx); ok {
		*runtime = *existing
	}
	runtime.Services.PeopleContacts = func(context.Context, string) (*people.Service, error) {
		return svc, nil
	}
	return app.WithRuntime(ctx, runtime)
}
