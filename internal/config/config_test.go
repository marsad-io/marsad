package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "marsad.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func noEnv(string) string { return "" }

const validYAML = `
connectors:
  - name: prod-prometheus
    type: prometheus
    url: http://localhost:9090
guardrails:
  max_time_range: 6h
audit:
  sink: stderr
`

func TestLoadValidFile(t *testing.T) {
	cfg, err := Load(writeFile(t, validYAML), noEnv)
	if err != nil {
		t.Fatalf("Load(valid) = %v", err)
	}

	if len(cfg.Connectors) != 1 {
		t.Fatalf("got %d connectors, want 1", len(cfg.Connectors))
	}
	c := cfg.Connectors[0]
	if c.Name != "prod-prometheus" || c.Type != "prometheus" || c.URL != "http://localhost:9090" {
		t.Errorf("connector = %+v", c)
	}
	if cfg.Guardrails.MaxTimeRange != 6*time.Hour {
		t.Errorf("MaxTimeRange = %v, want 6h", cfg.Guardrails.MaxTimeRange)
	}
	if cfg.Audit.Sink != "stderr" {
		t.Errorf("Audit.Sink = %q, want stderr", cfg.Audit.Sink)
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	cfg, err := Load(writeFile(t, `
connectors:
  - name: p
    type: prometheus
    url: http://localhost:9090
`), noEnv)
	if err != nil {
		t.Fatalf("Load = %v", err)
	}
	if cfg.Guardrails.MaxTimeRange != DefaultMaxTimeRange {
		t.Errorf("MaxTimeRange default = %v, want %v", cfg.Guardrails.MaxTimeRange, DefaultMaxTimeRange)
	}
	if cfg.Audit.Sink != "stderr" {
		t.Errorf("Audit.Sink default = %q, want stderr", cfg.Audit.Sink)
	}
}

func TestLoadMissingFileNamesFile(t *testing.T) {
	_, err := Load("/nonexistent/marsad.yaml", noEnv)
	if err == nil {
		t.Fatal("Load(missing) = nil error")
	}
	if !strings.Contains(err.Error(), "/nonexistent/marsad.yaml") {
		t.Errorf("error %q does not name the file", err)
	}
}

func TestLoadMalformedYAMLNamesFileAndProblem(t *testing.T) {
	path := writeFile(t, "connectors: [")
	_, err := Load(path, noEnv)
	if err == nil {
		t.Fatal("Load(malformed) = nil error")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error %q does not name the file", err)
	}
}

func TestLoadRejectsInlineSecret(t *testing.T) {
	path := writeFile(t, `
connectors:
  - name: p
    type: prometheus
    url: http://localhost:9090
    password: hunter2
`)
	_, err := Load(path, noEnv)
	if err == nil {
		t.Fatal("Load(inline secret) = nil error")
	}
	if !strings.Contains(err.Error(), "environment variable") {
		t.Errorf("error %q does not instruct operator to use an env reference", err)
	}
}

func TestLoadRejectsConnectorMissingFields(t *testing.T) {
	path := writeFile(t, `
connectors:
  - name: p
    type: prometheus
`)
	_, err := Load(path, noEnv)
	if err == nil {
		t.Fatal("Load(connector without url) = nil error")
	}
	if !strings.Contains(err.Error(), "url") {
		t.Errorf("error %q does not name the missing field", err)
	}
}

func TestLoadRejectsDuplicateConnectorNames(t *testing.T) {
	path := writeFile(t, `
connectors:
  - name: p
    type: prometheus
    url: http://a:9090
  - name: p
    type: prometheus
    url: http://b:9090
`)
	_, err := Load(path, noEnv)
	if err == nil {
		t.Fatal("Load(duplicate names) = nil error")
	}
	if !strings.Contains(err.Error(), `"p"`) {
		t.Errorf("error %q does not name the duplicate", err)
	}
}

func TestEnvOverridesWin(t *testing.T) {
	getenv := func(key string) string {
		switch key {
		case "MARSAD_AUDIT_SINK":
			return "/var/log/marsad-audit.log"
		case "MARSAD_GUARDRAILS_MAX_TIME_RANGE":
			return "30m"
		}
		return ""
	}

	cfg, err := Load(writeFile(t, validYAML), getenv)
	if err != nil {
		t.Fatalf("Load = %v", err)
	}

	if cfg.Audit.Sink != "/var/log/marsad-audit.log" {
		t.Errorf("Audit.Sink = %q, env override did not win", cfg.Audit.Sink)
	}
	if cfg.Guardrails.MaxTimeRange != 30*time.Minute {
		t.Errorf("MaxTimeRange = %v, env override did not win", cfg.Guardrails.MaxTimeRange)
	}
}

func TestBearerTokenComesFromEnvReference(t *testing.T) {
	path := writeFile(t, `
connectors:
  - name: p
    type: prometheus
    url: http://localhost:9090
    bearer_token_env: PROM_TOKEN
`)
	getenv := func(key string) string {
		if key == "PROM_TOKEN" {
			return "s3cret"
		}
		return ""
	}

	cfg, err := Load(path, getenv)
	if err != nil {
		t.Fatalf("Load = %v", err)
	}
	if cfg.Connectors[0].BearerToken != "s3cret" {
		t.Errorf("BearerToken = %q, want value resolved from PROM_TOKEN", cfg.Connectors[0].BearerToken)
	}
}

func TestMaxResultBytesDefaultsAndLoadsFromYAML(t *testing.T) {
	cfg, err := Load(writeFile(t, validYAML), noEnv)
	if err != nil {
		t.Fatalf("Load = %v", err)
	}
	if cfg.Guardrails.MaxResultBytes != DefaultMaxResultBytes {
		t.Errorf("MaxResultBytes default = %d, want %d", cfg.Guardrails.MaxResultBytes, DefaultMaxResultBytes)
	}

	cfg, err = Load(writeFile(t, `
connectors:
  - name: p
    type: prometheus
    url: http://localhost:9090
guardrails:
  max_result_bytes: 262144
`), noEnv)
	if err != nil {
		t.Fatalf("Load = %v", err)
	}
	if cfg.Guardrails.MaxResultBytes != 262144 {
		t.Errorf("MaxResultBytes = %d, want 262144", cfg.Guardrails.MaxResultBytes)
	}
}

func TestMaxResultBytesRejectsNonPositive(t *testing.T) {
	_, err := Load(writeFile(t, `
connectors:
  - name: p
    type: prometheus
    url: http://localhost:9090
guardrails:
  max_result_bytes: 0
`), noEnv)
	if err == nil {
		t.Fatal("Load(max_result_bytes: 0) = nil error")
	}
	if !strings.Contains(err.Error(), "max_result_bytes") {
		t.Errorf("error %q does not name the field", err)
	}
}

func TestMaxResultBytesEnvOverrideWins(t *testing.T) {
	getenv := func(key string) string {
		if key == "MARSAD_GUARDRAILS_MAX_RESULT_BYTES" {
			return "4096"
		}
		return ""
	}
	cfg, err := Load(writeFile(t, validYAML), getenv)
	if err != nil {
		t.Fatalf("Load = %v", err)
	}
	if cfg.Guardrails.MaxResultBytes != 4096 {
		t.Errorf("MaxResultBytes = %d, env override did not win", cfg.Guardrails.MaxResultBytes)
	}
}

func TestMaxResultBytesEnvOverrideRejectsGarbage(t *testing.T) {
	getenv := func(key string) string {
		if key == "MARSAD_GUARDRAILS_MAX_RESULT_BYTES" {
			return "lots"
		}
		return ""
	}
	_, err := Load(writeFile(t, validYAML), getenv)
	if err == nil {
		t.Fatal("Load(env override garbage) = nil error")
	}
	if !strings.Contains(err.Error(), "MARSAD_GUARDRAILS_MAX_RESULT_BYTES") {
		t.Errorf("error %q does not name the env var", err)
	}
}
