// Package config loads and validates marsad.yaml. Loading is pure: the only
// I/O is reading the file; environment access is injected so tests control it.
package config

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// DefaultMaxTimeRange caps query time ranges when the operator sets none.
const DefaultMaxTimeRange = 24 * time.Hour

// DefaultMaxResultBytes caps tool result sizes when the operator sets none.
// 1 MiB keeps even chatty log searches inside a sane context budget.
const DefaultMaxResultBytes = 1 << 20

// Config is the fully resolved gateway configuration.
type Config struct {
	Connectors []ConnectorConfig
	Guardrails GuardrailsConfig
	Audit      AuditConfig
}

// ConnectorConfig describes one backend connection. Credentials are resolved
// from environment variable references at load time and never stored in YAML.
type ConnectorConfig struct {
	Name              string
	Type              string
	URL               string
	BearerToken       string
	BasicAuthUser     string
	BasicAuthPassword string
}

// GuardrailsConfig holds gateway-wide guardrail settings.
type GuardrailsConfig struct {
	MaxTimeRange   time.Duration
	MaxResultBytes int
}

// AuditConfig controls the audit log sink.
type AuditConfig struct {
	Sink             string
	IncludeArguments bool
}

// yamlConfig mirrors the file format. Strict decoding rejects unknown fields,
// which is also what blocks inline secrets like `password:`.
type yamlConfig struct {
	Connectors []yamlConnector `yaml:"connectors"`
	Guardrails yamlGuardrails  `yaml:"guardrails"`
	Audit      yamlAudit       `yaml:"audit"`
}

type yamlConnector struct {
	Name                 string `yaml:"name"`
	Type                 string `yaml:"type"`
	URL                  string `yaml:"url"`
	BearerTokenEnv       string `yaml:"bearer_token_env"`
	BasicAuthUserEnv     string `yaml:"basic_auth_user_env"`
	BasicAuthPasswordEnv string `yaml:"basic_auth_password_env"`
}

type yamlGuardrails struct {
	MaxTimeRange   string `yaml:"max_time_range"`
	MaxResultBytes *int   `yaml:"max_result_bytes"`
}

type yamlAudit struct {
	Sink             string `yaml:"sink"`
	IncludeArguments bool   `yaml:"include_arguments"`
}

var secretishFields = []string{"password", "token", "secret", "api_key", "apikey", "key"}

// Load reads the YAML file at path, applies MARSAD_ env overrides via getenv,
// resolves credential env references, validates, and returns a typed Config.
func Load(path string, getenv func(string) string) (Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("config %s: %w", path, err)
	}

	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	var yc yamlConfig
	if err := dec.Decode(&yc); err != nil {
		if field, ok := unknownSecretField(err); ok {
			return Config{}, fmt.Errorf("config %s: field %q is not allowed inline; reference an environment variable instead (e.g. %s_env: MY_VAR)", path, field, field)
		}
		return Config{}, fmt.Errorf("config %s: %w", path, err)
	}

	cfg := Config{
		Guardrails: GuardrailsConfig{MaxTimeRange: DefaultMaxTimeRange, MaxResultBytes: DefaultMaxResultBytes},
		Audit:      AuditConfig{Sink: "stderr", IncludeArguments: yc.Audit.IncludeArguments},
	}

	if yc.Guardrails.MaxTimeRange != "" {
		d, err := time.ParseDuration(yc.Guardrails.MaxTimeRange)
		if err != nil {
			return Config{}, fmt.Errorf("config %s: guardrails.max_time_range: %w", path, err)
		}
		cfg.Guardrails.MaxTimeRange = d
	}
	if yc.Guardrails.MaxResultBytes != nil {
		if *yc.Guardrails.MaxResultBytes <= 0 {
			return Config{}, fmt.Errorf("config %s: guardrails.max_result_bytes must be positive, got %d", path, *yc.Guardrails.MaxResultBytes)
		}
		cfg.Guardrails.MaxResultBytes = *yc.Guardrails.MaxResultBytes
	}
	if yc.Audit.Sink != "" {
		cfg.Audit.Sink = yc.Audit.Sink
	}

	seen := map[string]bool{}
	for i, c := range yc.Connectors {
		if c.Name == "" || c.Type == "" || c.URL == "" {
			return Config{}, fmt.Errorf("config %s: connectors[%d]: name, type, and url are required", path, i)
		}
		if seen[c.Name] {
			return Config{}, fmt.Errorf("config %s: duplicate connector name %q", path, c.Name)
		}
		seen[c.Name] = true

		cc := ConnectorConfig{Name: c.Name, Type: c.Type, URL: c.URL}
		if c.BearerTokenEnv != "" {
			cc.BearerToken = getenv(c.BearerTokenEnv)
		}
		if c.BasicAuthUserEnv != "" {
			cc.BasicAuthUser = getenv(c.BasicAuthUserEnv)
		}
		if c.BasicAuthPasswordEnv != "" {
			cc.BasicAuthPassword = getenv(c.BasicAuthPasswordEnv)
		}
		cfg.Connectors = append(cfg.Connectors, cc)
	}

	if err := applyEnvOverrides(&cfg, getenv); err != nil {
		return Config{}, fmt.Errorf("config %s: %w", path, err)
	}
	return cfg, nil
}

// applyEnvOverrides applies the documented MARSAD_ overrides. The set is a
// deliberate allowlist, not reflection magic, so docs and behavior stay in sync.
func applyEnvOverrides(cfg *Config, getenv func(string) string) error {
	if v := getenv("MARSAD_AUDIT_SINK"); v != "" {
		cfg.Audit.Sink = v
	}
	if v := getenv("MARSAD_GUARDRAILS_MAX_TIME_RANGE"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("MARSAD_GUARDRAILS_MAX_TIME_RANGE: %w", err)
		}
		cfg.Guardrails.MaxTimeRange = d
	}
	if v := getenv("MARSAD_GUARDRAILS_MAX_RESULT_BYTES"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return fmt.Errorf("MARSAD_GUARDRAILS_MAX_RESULT_BYTES: %q is not a positive integer", v)
		}
		cfg.Guardrails.MaxResultBytes = n
	}
	return nil
}

// unknownSecretField reports whether a strict-decode error was caused by an
// unknown field whose name suggests an inline credential.
func unknownSecretField(err error) (string, bool) {
	msg := err.Error()
	if !strings.Contains(msg, "not found in type") {
		return "", false
	}
	for _, f := range secretishFields {
		if strings.Contains(msg, "field "+f) {
			return f, true
		}
	}
	return "", false
}
