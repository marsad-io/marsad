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

func TestServeMissingConfigFailsWithClearError(t *testing.T) {
	var out bytes.Buffer

	code := run([]string{"serve", "--config", "/nonexistent/marsad.yaml"}, &out)

	if code == 0 {
		t.Fatal("run(serve, missing config) exit code = 0, want non-zero")
	}
	if !bytes.Contains(out.Bytes(), []byte("/nonexistent/marsad.yaml")) {
		t.Errorf("output %q does not name the config file", out.String())
	}
}

func TestServeUnknownTransportFails(t *testing.T) {
	var out bytes.Buffer

	code := run([]string{"serve", "--transport", "carrier-pigeon"}, &out)

	if code == 0 {
		t.Fatal("run(serve, bad transport) exit code = 0, want non-zero")
	}
	if !bytes.Contains(out.Bytes(), []byte("carrier-pigeon")) {
		t.Errorf("output %q does not name the bad transport", out.String())
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
