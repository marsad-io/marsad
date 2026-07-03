// Package guardrails is the request pipeline every tool call flows through:
// guard checks, execution, and an audit line for every outcome. No call
// reaches a connector without passing here, and no call escapes auditing.
package guardrails

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"sync"
	"time"

	"github.com/marsad-io/marsad/internal/connector"
	"github.com/marsad-io/marsad/internal/schema"
)

// Executor performs the routed call and reports which connector handled it.
type Executor func(ctx context.Context, call connector.ToolCall) (connector.ToolResult, string, error)

// Options configures the pipeline.
type Options struct {
	MaxTimeRange     time.Duration
	AuditSink        io.Writer
	IncludeArguments bool
}

// Pipeline applies guards, executes, and audits.
type Pipeline struct {
	opts Options
	exec Executor
	mu   sync.Mutex // serializes audit writes
	now  func() time.Time
}

// NewPipeline builds a pipeline around an executor.
func NewPipeline(opts Options, exec Executor) *Pipeline {
	return &Pipeline{opts: opts, exec: exec, now: time.Now}
}

// Execute runs one tool call through guards, executor, and audit.
func (p *Pipeline) Execute(ctx context.Context, call connector.ToolCall) (connector.ToolResult, error) {
	start := p.now()

	if err := p.check(call); err != nil {
		p.audit(call, "", "rejected", err, 0, p.now().Sub(start))
		return connector.ToolResult{}, err
	}

	res, connectorName, err := p.exec(ctx, call)
	if err != nil {
		p.audit(call, connectorName, "error", err, 0, p.now().Sub(start))
		return connector.ToolResult{}, err
	}
	p.audit(call, connectorName, "ok", nil, resultSize(res), p.now().Sub(start))
	return res, nil
}

// check runs the guard chain. Adding a guardrail means adding a function here.
func (p *Pipeline) check(call connector.ToolCall) error {
	if err := p.checkReadOnly(call); err != nil {
		return err
	}
	return p.checkTimeRange(call)
}

func (p *Pipeline) checkReadOnly(call connector.ToolCall) error {
	spec, ok := schema.Lookup(call.Tool)
	if !ok || !spec.ReadOnly {
		return fmt.Errorf("tool %q rejected: marsad is read-only and executes only read-only unified tools", call.Tool)
	}
	return nil
}

func (p *Pipeline) checkTimeRange(call connector.ToolCall) error {
	startArg, _ := call.Args["start"].(string)
	endArg, _ := call.Args["end"].(string)
	if startArg == "" || endArg == "" {
		return nil
	}
	startT, err := parseTime(startArg)
	if err != nil {
		return fmt.Errorf("invalid start: %w", err)
	}
	endT, err := parseTime(endArg)
	if err != nil {
		return fmt.Errorf("invalid end: %w", err)
	}
	if span := endT.Sub(startT); span > p.opts.MaxTimeRange {
		return fmt.Errorf("time range %s exceeds the configured maximum of %s", span, p.opts.MaxTimeRange)
	}
	return nil
}

// parseTime accepts RFC 3339 or Unix seconds, matching the Prometheus API.
func parseTime(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if secs, err := strconv.ParseFloat(s, 64); err == nil {
		return time.Unix(int64(secs), 0), nil
	}
	return time.Time{}, fmt.Errorf("%q is neither RFC 3339 nor Unix seconds", s)
}

type auditLine struct {
	TS         string         `json:"ts"`
	Tool       string         `json:"tool"`
	Connector  string         `json:"connector"`
	ArgsHash   string         `json:"args_hash"`
	DurationMS int64          `json:"duration_ms"`
	Outcome    string         `json:"outcome"`
	Bytes      int            `json:"bytes"`
	Error      string         `json:"error,omitempty"`
	Args       map[string]any `json:"args,omitempty"`
}

func (p *Pipeline) audit(call connector.ToolCall, connectorName, outcome string, callErr error, size int, d time.Duration) {
	if p.opts.AuditSink == nil {
		return
	}
	line := auditLine{
		TS:         p.now().UTC().Format(time.RFC3339Nano),
		Tool:       call.Tool,
		Connector:  connectorName,
		ArgsHash:   hashArgs(call.Args),
		DurationMS: d.Milliseconds(),
		Outcome:    outcome,
		Bytes:      size,
	}
	if callErr != nil {
		line.Error = callErr.Error()
	}
	if p.opts.IncludeArguments {
		line.Args = call.Args
	}
	b, err := json.Marshal(line)
	if err != nil {
		return // an unmarshalable audit line must never break the call path
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.opts.AuditSink.Write(append(b, '\n'))
}

// hashArgs fingerprints arguments without recording their values.
// encoding/json sorts map keys, so the hash is canonical.
func hashArgs(args map[string]any) string {
	b, err := json.Marshal(args)
	if err != nil {
		return "unhashable"
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:8])
}

func resultSize(res connector.ToolResult) int {
	b, err := json.Marshal(res.Content)
	if err != nil {
		return 0
	}
	return len(b)
}
