package schema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The golden files in testdata/tools are the public contract of the unified
// tool schema. Any change to them is a breaking API change and must be
// deliberate.
func TestToolSpecsMatchGoldenFiles(t *testing.T) {
	specs := All()
	if len(specs) == 0 {
		t.Fatal("All() returned no tool specs")
	}

	seen := map[string]bool{}
	for _, spec := range specs {
		seen[spec.Name] = true
		t.Run(spec.Name, func(t *testing.T) {
			got, err := json.MarshalIndent(spec, "", "  ")
			if err != nil {
				t.Fatalf("marshal %s: %v", spec.Name, err)
			}
			got = append(got, '\n')

			goldenPath := filepath.Join("testdata", "tools", spec.Name+".json")
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden file: %v", err)
			}
			if string(got) != string(want) {
				t.Errorf("spec %s does not match golden file %s\ngot:\n%s\nwant:\n%s",
					spec.Name, goldenPath, got, want)
			}
		})
	}

	// Every golden file must be backed by a spec - no orphans.
	entries, err := os.ReadDir(filepath.Join("testdata", "tools"))
	if err != nil {
		t.Fatalf("read testdata/tools: %v", err)
	}
	for _, e := range entries {
		name := e.Name()[:len(e.Name())-len(".json")]
		if !seen[name] {
			t.Errorf("golden file %s has no corresponding ToolSpec", e.Name())
		}
	}
}

func TestLookupFindsSpecByName(t *testing.T) {
	spec, ok := Lookup("query_metrics")
	if !ok {
		t.Fatal("Lookup(query_metrics) not found")
	}
	if spec.Name != "query_metrics" {
		t.Errorf("Lookup returned spec named %q", spec.Name)
	}
	if _, ok := Lookup("no_such_tool"); ok {
		t.Error("Lookup(no_such_tool) unexpectedly found")
	}
}

// The search_logs contract must stay backend-neutral so Elasticsearch,
// OpenSearch, and ClickHouse can implement the identical tool later.
func TestSearchLogsRequiredArgumentsAreBackendNeutral(t *testing.T) {
	spec, ok := Lookup("search_logs")
	if !ok {
		t.Fatal("Lookup(search_logs) not found")
	}
	if !spec.ReadOnly {
		t.Error("search_logs must be read-only")
	}

	want := []string{"query", "start", "end"}
	if len(spec.Input.Required) != len(want) {
		t.Fatalf("required = %v, want exactly %v", spec.Input.Required, want)
	}
	for i, name := range want {
		if spec.Input.Required[i] != name {
			t.Errorf("required[%d] = %q, want %q", i, spec.Input.Required[i], name)
		}
	}

	// Backend-specific capabilities ride along as optional arguments only.
	for _, optional := range []string{"limit", "direction", "connector"} {
		if _, ok := spec.Input.Properties[optional]; !ok {
			t.Errorf("search_logs is missing optional argument %q", optional)
		}
	}
}

func TestListLogLabelsHasNoRequiredArguments(t *testing.T) {
	spec, ok := Lookup("list_log_labels")
	if !ok {
		t.Fatal("Lookup(list_log_labels) not found")
	}
	if len(spec.Input.Required) != 0 {
		t.Errorf("required = %v, want none", spec.Input.Required)
	}
	if _, ok := spec.Input.Properties["label"]; !ok {
		t.Error("list_log_labels is missing the optional label argument")
	}
}
