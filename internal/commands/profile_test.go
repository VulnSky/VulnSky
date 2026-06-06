package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProfileListShowsActiveProfile(t *testing.T) {
	root := writeProfiles(t)
	buf := new(bytes.Buffer)
	cmd := NewRootCommandWithOptions(RootOptions{RootDir: root})
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"profile", "ls"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := buf.String()
	for _, want := range []string{"* default", "  class-a"} {
		if !strings.Contains(out, want) {
			t.Fatalf("profile ls missing %q:\n%s", want, out)
		}
	}
}

func TestProfileUseUpdatesActiveProfile(t *testing.T) {
	root := writeProfiles(t)
	buf := new(bytes.Buffer)
	cmd := NewRootCommandWithOptions(RootOptions{RootDir: root})
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"profile", "use", "class-a"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "VULNSKY_ACTIVE_PROFILE=class-a") {
		t.Fatalf(".env was not updated:\n%s", string(data))
	}
}

func TestProfileShowRedactsSecrets(t *testing.T) {
	root := writeProfiles(t)
	buf := new(bytes.Buffer)
	cmd := NewRootCommandWithOptions(RootOptions{RootDir: root})
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"profile", "show", "class-a"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "secret") {
		t.Fatalf("profile show leaked secret:\n%s", out)
	}
	if !strings.Contains(out, "ALIBABA_CLOUD_ACCESS_KEY_SECRET=<redacted>") {
		t.Fatalf("profile show did not redact secret:\n%s", out)
	}
}

func TestProfileListReturnsMalformedEnvError(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, ".env"), "VULNSKY_ACTIVE_PROFILE\n")
	buf := new(bytes.Buffer)
	cmd := NewRootCommandWithOptions(RootOptions{RootDir: root})
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"profile", "ls"})

	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected malformed .env error\n%s", buf.String())
	}
}

func TestProfileShowReturnsMalformedProfileError(t *testing.T) {
	root := writeProfiles(t)
	mustWriteFile(t, filepath.Join(root, "profiles", "broken.env"), "ALIBABA_CLOUD_ACCESS_KEY_ID\n")
	buf := new(bytes.Buffer)
	cmd := NewRootCommandWithOptions(RootOptions{RootDir: root})
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"profile", "show", "broken"})

	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "load profile") {
		t.Fatalf("Execute() error = %v, want malformed profile error\n%s", err, buf.String())
	}
}

func TestProfileUseReturnsMalformedProfileError(t *testing.T) {
	root := writeProfiles(t)
	mustWriteFile(t, filepath.Join(root, "profiles", "broken.env"), "ALIBABA_CLOUD_ACCESS_KEY_ID\n")
	buf := new(bytes.Buffer)
	cmd := NewRootCommandWithOptions(RootOptions{RootDir: root})
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"profile", "use", "broken"})

	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "load profile") {
		t.Fatalf("Execute() error = %v, want malformed profile error\n%s", err, buf.String())
	}
	data, err := os.ReadFile(filepath.Join(root, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "VULNSKY_ACTIVE_PROFILE=broken") {
		t.Fatalf("malformed profile was activated:\n%s", string(data))
	}
}

func TestProfileShowRejectsPathTraversalName(t *testing.T) {
	root := writeProfiles(t)
	buf := new(bytes.Buffer)
	cmd := NewRootCommandWithOptions(RootOptions{RootDir: root})
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"profile", "show", "../default"})

	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "invalid profile name") {
		t.Fatalf("Execute() error = %v, want invalid profile name error\n%s", err, buf.String())
	}
}

func writeProfiles(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "profiles"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(root, ".env"), "VULNSKY_ACTIVE_PROFILE=default\nVULNSKY_PROFILE_DIR=./profiles\nVULNSKY_DB_PATH=./vulnsky.db\n")
	mustWriteFile(t, filepath.Join(root, "profiles", "default.env"), "VULNSKY_PROFILE_LABEL=default\nALIBABA_CLOUD_ACCESS_KEY_SECRET=secret\n")
	mustWriteFile(t, filepath.Join(root, "profiles", "class-a.env"), "VULNSKY_PROFILE_LABEL=class-a\nALIBABA_CLOUD_ACCESS_KEY_SECRET=secret\n")
	return root
}
