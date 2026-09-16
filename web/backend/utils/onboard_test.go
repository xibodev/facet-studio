package utils

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEnsureOnboardedSkipsWhenConfigExists(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, []byte(`{}`), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	origExecCommand := execCommand
	defer func() { execCommand = origExecCommand }()

	called := false
	execCommand = func(name string, args ...string) *exec.Cmd {
		called = true
		if runtime.GOOS == "windows" {
			return exec.Command("cmd", "/c", "exit 1")
		}
		return exec.Command("sh", "-c", "exit 1")
	}

	if err := EnsureOnboarded(configPath); err != nil {
		t.Fatalf("EnsureOnboarded() error = %v", err)
	}
	if called {
		t.Fatal("expected onboard command not to run when config already exists")
	}
}

func TestEnsureOnboardedRunsOnboardWhenConfigMissing(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("EXPECTED_CONFIG_PATH", configPath)

	origExecCommand := execCommand
	defer func() { execCommand = origExecCommand }()

	var gotName string
	var gotArgs []string
	execCommand = func(name string, args ...string) *exec.Cmd {
		gotName = name
		gotArgs = append([]string(nil), args...)
		if runtime.GOOS == "windows" {
			return exec.Command(
				"powershell",
				"-NoProfile",
				"-Command",
				`if ($env:FACET_STUDIO_CONFIG -ne $env:EXPECTED_CONFIG_PATH) { exit 1 }; $dir = Split-Path -Parent $env:FACET_STUDIO_CONFIG; if ($dir) { New-Item -ItemType Directory -Force -Path $dir | Out-Null }; Set-Content -Path $env:FACET_STUDIO_CONFIG -Value '{}'`,
			)
		}
		return exec.Command(
			"sh",
			"-c",
			`test "$FACET_STUDIO_CONFIG" = "$EXPECTED_CONFIG_PATH" &&
mkdir -p "$(dirname "$FACET_STUDIO_CONFIG")" &&
printf '{}' > "$FACET_STUDIO_CONFIG"`,
		)
	}

	if err := EnsureOnboarded(configPath); err != nil {
		t.Fatalf("EnsureOnboarded() error = %v", err)
	}
	if gotName == "" {
		t.Fatal("expected onboard command to run")
	}
	if len(gotArgs) != 1 || gotArgs[0] != "onboard" {
		t.Fatalf("command args = %#v, want []string{\"onboard\"}", gotArgs)
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("expected config to be created: %v", err)
	}
}

func TestEnsureOnboardedFailsWhenOnboardDoesNotCreateConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")

	origExecCommand := execCommand
	defer func() { execCommand = origExecCommand }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		if runtime.GOOS == "windows" {
			return exec.Command("cmd", "/c", "exit 0")
		}
		return exec.Command("sh", "-c", "exit 0")
	}

	if err := EnsureOnboarded(configPath); err == nil {
		t.Fatal("EnsureOnboarded() error = nil, want failure when onboard does not create config")
	}
}

func TestEnsureOnboardedIncludesOnboardOutputOnFailure(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")

	origExecCommand := execCommand
	defer func() { execCommand = origExecCommand }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		if runtime.GOOS == "windows" {
			return exec.Command("cmd", "/c", "echo onboarding failed 1>&2 & exit 2")
		}
		return exec.Command("sh", "-c", "echo onboarding failed >&2; exit 2")
	}

	err := EnsureOnboarded(configPath)
	if err == nil {
		t.Fatal("EnsureOnboarded() error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "onboarding failed") {
		t.Fatalf("error = %q, want onboard output included", err)
	}
}
