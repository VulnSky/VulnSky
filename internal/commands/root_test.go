package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vulnsky/internal/aliyun"
	"vulnsky/internal/config"
)

func TestRootHelpShowsCoreCommands(t *testing.T) {
	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := buf.String()
	for _, want := range []string{"profile", "oss", "image", "ecs", "deploy", "doctor", "records", "shell", "version"} {
		if !strings.Contains(out, want) {
			t.Fatalf("help output missing %q: %s", want, out)
		}
	}
}

func TestRootHelpDoesNotExposeWorkingDirectory(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := buf.String()
	if strings.Contains(out, wd) {
		t.Fatalf("help output leaked working directory %q:\n%s", wd, out)
	}
	if !strings.Contains(out, `--root string`) || !strings.Contains(out, `(default ".")`) {
		t.Fatalf("help output missing dot root default:\n%s", out)
	}
}

func TestFirstRunCreatesDefaultEnvFiles(t *testing.T) {
	root := t.TempDir()
	cmd := NewRootCommandWithOptions(RootOptions{RootDir: root})
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"doctor"})

	_ = cmd.Execute()

	envData, err := os.ReadFile(filepath.Join(root, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	profileData, err := os.ReadFile(filepath.Join(root, "profiles", "default.env"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(envData), "VULNSKY_ACTIVE_PROFILE=default") {
		t.Fatalf(".env missing active profile:\n%s", string(envData))
	}
	if !strings.Contains(string(profileData), "ALIBABA_CLOUD_ACCESS_KEY_ID=") ||
		!strings.Contains(string(profileData), "ALIBABA_OSS_BUCKET=") ||
		!strings.Contains(string(profileData), "VULNSKY_STOP_TIMEOUT_SECONDS=60") {
		t.Fatalf("default profile missing expected fields:\n%s", string(profileData))
	}
}

func TestFirstRunDoesNotOverwriteExistingEnvFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "profiles"), 0o755); err != nil {
		t.Fatal(err)
	}
	envPath := filepath.Join(root, ".env")
	profilePath := filepath.Join(root, "profiles", "default.env")
	if err := os.WriteFile(envPath, []byte("VULNSKY_ACTIVE_PROFILE=class-a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(profilePath, []byte("VULNSKY_PROFILE_LABEL=keep-me\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := NewRootCommandWithOptions(RootOptions{RootDir: root})
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"doctor"})
	_ = cmd.Execute()

	envData, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatal(err)
	}
	profileData, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(envData) != "VULNSKY_ACTIVE_PROFILE=class-a\n" {
		t.Fatalf(".env was overwritten:\n%s", string(envData))
	}
	if string(profileData) != "VULNSKY_PROFILE_LABEL=keep-me\n" {
		t.Fatalf("default.env was overwritten:\n%s", string(profileData))
	}
}

func TestFirstRunReturnsMalformedEnvError(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("VULNSKY_ACTIVE_PROFILE\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := NewRootCommandWithOptions(RootOptions{RootDir: root})
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"doctor"})

	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected malformed .env error\n%s", buf.String())
	}
	if _, err := os.Stat(filepath.Join(root, "profiles", "default.env")); !os.IsNotExist(err) {
		t.Fatalf("malformed .env should not create default profile, stat err=%v", err)
	}
}

func TestInteractiveShellSwitchesContextAndRunsCommand(t *testing.T) {
	root := writeDoctorProfile(t, "class-a", "123456789", "cn-hangzhou", "cn-hangzhou", "i-lab")
	buf := new(bytes.Buffer)
	cmd := NewRootCommandWithOptions(RootOptions{
		RootDir: root,
		Factories: ClientFactories{
			NewOSS: func(config.Config) (aliyun.OSSClient, error) {
				return commandFakeOSS{objects: []string{"qcow2/sample-lab.qcow2"}}, nil
			},
		},
	})
	cmd.SetIn(strings.NewReader("oss\nls qcow2/\nback\nexit\n"))
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v\n%s", err, buf.String())
	}
	out := buf.String()
	for _, want := range []string{
		"vulnsky/> ",
		"vulnsky/oss> ",
		"qcow2/sample-lab.qcow2",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("interactive shell output missing %q:\n%s", want, out)
		}
	}
}

func TestShellSplitKeepsQuotedArguments(t *testing.T) {
	fields, err := splitShellFields(`deploy "C:\Labs\sample-lab.qcow2" --key qcow2/sample-lab.qcow2`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"deploy", `C:\Labs\sample-lab.qcow2`, "--key", "qcow2/sample-lab.qcow2"}
	if len(fields) != len(want) {
		t.Fatalf("field count = %d, want %d: %#v", len(fields), len(want), fields)
	}
	for i := range want {
		if fields[i] != want[i] {
			t.Fatalf("field[%d] = %q, want %q; all=%#v", i, fields[i], want[i], fields)
		}
	}
}

func TestVersionCommandPrintsBuildInfo(t *testing.T) {
	root := t.TempDir()
	buf := new(bytes.Buffer)
	cmd := NewRootCommandWithOptions(RootOptions{RootDir: root})
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v\n%s", err, buf.String())
	}
	out := buf.String()
	for _, want := range []string{"Version=", "Commit=", "BuildDate="} {
		if !strings.Contains(out, want) {
			t.Fatalf("version output missing %q:\n%s", want, out)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".env")); !os.IsNotExist(err) {
		t.Fatalf("version should not create .env, stat err=%v", err)
	}
}
