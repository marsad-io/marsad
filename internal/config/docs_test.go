package config

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

// collectYAMLTags walks a struct type and returns every yaml tag, so the docs
// test can never drift from the actual file format.
func collectYAMLTags(t *testing.T, typ reflect.Type, into map[string]bool) {
	t.Helper()
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		tag := strings.Split(field.Tag.Get("yaml"), ",")[0]
		if tag != "" && tag != "-" {
			into[tag] = true
		}
		ft := field.Type
		if ft.Kind() == reflect.Slice {
			ft = ft.Elem()
		}
		if ft.Kind() == reflect.Struct {
			collectYAMLTags(t, ft, into)
		}
	}
}

func TestConfigurationDocsCoverEveryField(t *testing.T) {
	docs, err := os.ReadFile("../../docs/configuration.md")
	if err != nil {
		t.Fatalf("docs/configuration.md must exist and document the config format: %v", err)
	}

	tags := map[string]bool{}
	collectYAMLTags(t, reflect.TypeOf(yamlConfig{}), tags)
	if len(tags) < 5 {
		t.Fatalf("collected only %d yaml tags, reflection walk is broken", len(tags))
	}

	for tag := range tags {
		if !strings.Contains(string(docs), "`"+tag+"`") {
			t.Errorf("docs/configuration.md does not document config field `%s`", tag)
		}
	}

	// Env overrides are part of the config surface too.
	for _, env := range []string{"MARSAD_AUDIT_SINK", "MARSAD_GUARDRAILS_MAX_TIME_RANGE"} {
		if !strings.Contains(string(docs), env) {
			t.Errorf("docs/configuration.md does not document env override %s", env)
		}
	}
}
