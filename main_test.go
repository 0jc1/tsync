package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIInformationalCommands(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{args: []string{"help"}, want: "tsync keeps configured files synchronized"},
		{args: []string{"--help"}, want: "Usage:"},
		{args: []string{"version"}, want: "tsync dev"},
		{args: []string{"--version"}, want: "tsync dev"},
	}
	for _, test := range tests {
		var stdout, stderr bytes.Buffer
		if err := runCLI(test.args, &stdout, &stderr); err != nil {
			t.Fatalf("runCLI(%v) error = %v", test.args, err)
		}
		if !strings.Contains(stdout.String(), test.want) {
			t.Fatalf("runCLI(%v) output = %q, want substring %q", test.args, stdout.String(), test.want)
		}
	}
}

func TestCLIValidateAndStatus(t *testing.T) {
	dir := t.TempDir()
	configPath := writeConfig(t, dir, `{
		"source_path": `+quote(filepath.Join(dir, "source.txt"))+`,
		"paths": [`+quote(filepath.Join(dir, "copy.txt"))+`]
	}`)

	var stdout, stderr bytes.Buffer
	if err := runCLI([]string{"--config", configPath, "validate"}, &stdout, &stderr); err != nil {
		t.Fatalf("validate error = %v", err)
	}
	if got := stdout.String(); got != "valid: 1 sync set(s)\n" {
		t.Fatalf("validate output = %q", got)
	}

	stdout.Reset()
	if err := runCLI([]string{"status", "--config", configPath}, &stdout, &stderr); err != nil {
		t.Fatalf("status error = %v", err)
	}
	if got := stdout.String(); got != "stopped\n" {
		t.Fatalf("status output = %q", got)
	}
}

func TestCLIRejectsUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := runCLI([]string{"install"}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("runCLI() error = %v, want unknown command", err)
	}
}
