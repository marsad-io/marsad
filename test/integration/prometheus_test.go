//go:build integration

// Integration tests run against real backend images via testcontainers.
// They skip when no Docker daemon is available locally, but fail hard in CI.
package integration

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/marsad-io/marsad/internal/connector"
	"github.com/marsad-io/marsad/internal/connector/prometheus"
)

func startPrometheus(t *testing.T) string {
	t.Helper()
	ctx := context.Background()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "prom/prometheus:v2.53.0",
			ExposedPorts: []string{"9090/tcp"},
			WaitingFor:   wait.ForHTTP("/-/ready").WithPort("9090/tcp").WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("starting prometheus container in CI: %v", err)
		}
		t.Skipf("no usable Docker daemon, skipping integration test: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })

	endpoint, err := container.PortEndpoint(ctx, "9090/tcp", "http")
	if err != nil {
		t.Fatal(err)
	}
	return endpoint
}

// End-to-end: real Prometheus image, real scrape data (Prometheus scrapes
// itself by default), queried through the connector's unified tools.
func TestPrometheusConnectorAgainstRealBackend(t *testing.T) {
	url := startPrometheus(t)

	c, err := prometheus.New("prom-int", url, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	if err := c.Health(ctx); err != nil {
		t.Fatalf("Health = %v", err)
	}

	// Prometheus scrapes itself; wait until the first sample lands.
	deadline := time.Now().Add(30 * time.Second)
	var lastResult string
	for {
		res, err := c.Execute(ctx, connector.ToolCall{
			Tool: "query_metrics",
			Args: map[string]any{"query": "up"},
		})
		if err != nil {
			t.Fatalf("Execute(query_metrics) = %v", err)
		}
		b, err := json.Marshal(res.Content)
		if err != nil {
			t.Fatal(err)
		}
		lastResult = string(b)
		if strings.Contains(lastResult, `"__name__":"up"`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no 'up' samples after 30s; last result: %s", lastResult)
		}
		time.Sleep(2 * time.Second)
	}

	res, err := c.Execute(ctx, connector.ToolCall{Tool: "list_metric_names"})
	if err != nil {
		t.Fatalf("Execute(list_metric_names) = %v", err)
	}
	b, _ := json.Marshal(res.Content)
	if !strings.Contains(string(b), `"up"`) {
		t.Errorf("list_metric_names result %s does not contain 'up'", b)
	}
}
