# config Specification (delta)

## ADDED Requirements

### Requirement: YAML configuration with env overrides

The gateway SHALL load configuration from a single YAML file (default `marsad.yaml`, overridable via `--config`), with `MARSAD_`-prefixed environment variables taking precedence over file values.

#### Scenario: Valid file loads

- **WHEN** `marsad.yaml` defines a prometheus connector with a URL and guardrail settings
- **THEN** the typed Config contains that connector and those settings

#### Scenario: Clear failure on bad input

- **WHEN** the file is missing or contains malformed YAML
- **THEN** startup fails with an error naming the file and the problem, not a stack trace

#### Scenario: Env override wins

- **WHEN** the file sets a value and a corresponding `MARSAD_` env var sets a different value
- **THEN** the env var value is used

### Requirement: Secrets never inline

Connector credentials SHALL be referenced via environment variable names in config, never as inline literal values.

#### Scenario: Inline secret rejected

- **WHEN** config contains an inline credential field (e.g. `password: hunter2`)
- **THEN** config validation fails, instructing the operator to use an env reference
