package main

import (
	"bytes"
	"testing"
)

func TestVersionCommandPrintsVersion(t *testing.T) {
	var out bytes.Buffer

	code := run([]string{"version"}, &out)

	if code != 0 {
		t.Fatalf("run(version) exit code = %d, want 0", code)
	}
	got := out.String()
	want := "marsad " + version + "\n"
	if got != want {
		t.Errorf("run(version) output = %q, want %q", got, want)
	}
}

func TestUnknownCommandFailsWithUsage(t *testing.T) {
	var out bytes.Buffer

	code := run([]string{"bogus"}, &out)

	if code == 0 {
		t.Fatal("run(bogus) exit code = 0, want non-zero")
	}
	if !bytes.Contains(out.Bytes(), []byte("usage")) {
		t.Errorf("run(bogus) output %q does not mention usage", out.String())
	}
}
