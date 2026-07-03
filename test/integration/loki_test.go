//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/marsad-io/marsad/internal/connector"
	"github.com/marsad-io/marsad/internal/connector/loki"
)

func startLoki(t *testing.T) string {
	t.Helper()
	ctx := context.Background()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "grafana/loki:3.1.0",
			ExposedPorts: []string{"3100/tcp"},
			WaitingFor:   wait.ForHTTP("/ready").WithPort("3100/tcp").WithStartupTimeout(120 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("starting loki container in CI: %v", err)
		}
		t.Skipf("no usable Docker daemon, skipping integration test: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })

	endpoint, err := container.PortEndpoint(ctx, "3100/tcp", "http")
	if err != nil {
		t.Fatal(err)
	}
	return endpoint
}

// seedLokiLogs pushes log lines the way an agent's workload would, via the
// Loki push API, all under the given stream labels.
func seedLokiLogs(t *testing.T, baseURL string, labels map[string]string, lines []string) {
	t.Helper()
	now := time.Now()
	values := make([][2]string, len(lines))
	for i, line := range lines {
		ts := now.Add(time.Duration(i) * time.Second)
		values[i] = [2]string{fmt.Sprintf("%d", ts.UnixNano()), line}
	}
	payload, err := json.Marshal(map[string]any{
		"streams": []map[string]any{{"stream": labels, "values": values}},
	})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.Post(baseURL+"/loki/api/v1/push", "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("pushing logs: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		t.Fatalf("push returned HTTP %d", resp.StatusCode)
	}
}

// End-to-end against a real Loki image: seed logs through the push API, then
// search and discover labels through the connector's unified tools.
func TestLokiConnectorAgainstRealBackend(t *testing.T) {
	url := startLoki(t)

	c, err := loki.New("loki-int", url, loki.Auth{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	if err := c.Health(ctx); err != nil {
		t.Fatalf("Health = %v", err)
	}

	seedLokiLogs(t, url,
		map[string]string{"app": "marsad-test", "level": "info"},
		[]string{"gateway starting", "connector registered", "gateway ready"})

	searchArgs := map[string]any{
		"query":     `{app="marsad-test"}`,
		"start":     time.Now().Add(-5 * time.Minute).UTC().Format(time.RFC3339),
		"end":       time.Now().Add(5 * time.Minute).UTC().Format(time.RFC3339),
		"direction": "forward",
	}

	// Ingested entries become queryable shortly after the push; poll briefly.
	deadline := time.Now().Add(30 * time.Second)
	var lastResult string
	for {
		res, err := c.Execute(ctx, connector.ToolCall{Tool: "search_logs", Args: searchArgs})
		if err != nil {
			t.Fatalf("Execute(search_logs) = %v", err)
		}
		b, err := json.Marshal(res.Content)
		if err != nil {
			t.Fatal(err)
		}
		lastResult = string(b)
		if strings.Contains(lastResult, `"count":3`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("seeded entries not searchable after 30s; last result: %s", lastResult)
		}
		time.Sleep(2 * time.Second)
	}

	for _, want := range []string{"gateway starting", "connector registered", "gateway ready", `"app":"marsad-test"`} {
		if !strings.Contains(lastResult, want) {
			t.Errorf("search result %s missing %s", lastResult, want)
		}
	}
	// forward direction: oldest entry first
	if idx := strings.Index(lastResult, "gateway starting"); idx == -1 || idx > strings.Index(lastResult, "gateway ready") {
		t.Errorf("entries are not in forward order: %s", lastResult)
	}

	res, err := c.Execute(ctx, connector.ToolCall{Tool: "list_log_labels"})
	if err != nil {
		t.Fatalf("Execute(list_log_labels) = %v", err)
	}
	b, _ := json.Marshal(res.Content)
	if !strings.Contains(string(b), `"app"`) {
		t.Errorf("list_log_labels result %s does not contain the seeded label name", b)
	}

	res, err = c.Execute(ctx, connector.ToolCall{
		Tool: "list_log_labels",
		Args: map[string]any{"label": "app"},
	})
	if err != nil {
		t.Fatalf("Execute(list_log_labels app values) = %v", err)
	}
	b, _ = json.Marshal(res.Content)
	if !strings.Contains(string(b), "marsad-test") {
		t.Errorf("label values result %s does not contain the seeded value", b)
	}
}
