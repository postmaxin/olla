package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/thushan/olla/internal/adapter/discovery"
	"github.com/thushan/olla/internal/adapter/inspector"
	"github.com/thushan/olla/internal/config"
	"github.com/thushan/olla/internal/router"
)

// TestDrainRoutes_DrainThenUndrain drives the registered drain and undrain
// routes against a real static repository. The undrain route was registered
// with drainHandler, so "undrain" re-drained the endpoint and it never
// returned to rotation.
func TestDrainRoutes_DrainThenUndrain(t *testing.T) {
	t.Parallel()

	priority := 100
	repo := discovery.NewStaticEndpointRepository()
	require.NoError(t, repo.LoadFromConfig(context.Background(), []config.EndpointConfig{{
		Name:          "gpu-1",
		URL:           "http://localhost:11434",
		Type:          "ollama",
		Priority:      &priority,
		CheckInterval: 5 * time.Second,
		CheckTimeout:  2 * time.Second,
	}}))

	reg := router.NewRouteRegistry(&mockStyledLogger{})
	app := &Application{
		Config:           &config.Config{},
		inspectorChain:   inspector.NewChain(&mockStyledLogger{}),
		discoveryService: &mockDiscoveryService{},
		logger:           &mockStyledLogger{},
		routeRegistry:    reg,
		repository:       repo,
	}
	app.registerRoutes()

	call := func(action string) {
		t.Helper()
		route, ok := reg.GetRoutes()["/internal/endpoints/{name}/"+action]
		require.True(t, ok, "route for %q not registered", action)
		rec := httptest.NewRecorder()
		route.Handler(rec, httptest.NewRequest(http.MethodPost, "/internal/endpoints/gpu-1/"+action, nil))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	}
	drained := func() bool {
		t.Helper()
		eps, err := repo.GetAll(context.Background())
		require.NoError(t, err)
		return eps[0].Drained
	}

	call("drain")
	require.True(t, drained(), "endpoint should be drained after POST .../drain")

	call("undrain")
	require.False(t, drained(), "endpoint should be back in rotation after POST .../undrain")
}
