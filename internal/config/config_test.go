package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadProfileConfig(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".env"), "VULNSKY_ACTIVE_PROFILE=class-a\nVULNSKY_DB_PATH=./vulnsky.db\nVULNSKY_PROFILE_DIR=./profiles\n")
	if err := os.Mkdir(filepath.Join(root, "profiles"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(root, "profiles", "class-a.env"), ""+
		"VULNSKY_PROFILE_LABEL=class-a\n"+
		"ALIBABA_CLOUD_ACCESS_KEY_ID=ak\n"+
		"ALIBABA_CLOUD_ACCESS_KEY_SECRET=secret\n"+
		"ALIBABA_CLOUD_REGION_ID=cn-hangzhou\n"+
		"ALIBABA_OSS_REGION_ID=cn-hangzhou\n"+
		"ALIBABA_OSS_ENDPOINT=https://oss-cn-hangzhou.aliyuncs.com\n"+
		"ALIBABA_OSS_BUCKET=lab-bucket\n"+
		"VULNSKY_STOP_TIMEOUT_SECONDS=60\n")

	cfg, err := Load(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ProfileName != "class-a" || cfg.OSSBucket != "lab-bucket" || cfg.StopTimeoutSeconds != 60 {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestLoadExplicitProfileOverridesActiveProfile(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".env"), "VULNSKY_ACTIVE_PROFILE=default\nVULNSKY_DB_PATH=./vulnsky.db\nVULNSKY_PROFILE_DIR=./profiles\n")
	if err := os.Mkdir(filepath.Join(root, "profiles"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(root, "profiles", "class-b.env"), ""+
		"ALIBABA_CLOUD_ACCESS_KEY_ID=ak-b\n"+
		"ALIBABA_CLOUD_ACCESS_KEY_SECRET=secret-b\n"+
		"ALIBABA_CLOUD_REGION_ID=cn-shanghai\n"+
		"ALIBABA_OSS_REGION_ID=cn-shanghai\n"+
		"ALIBABA_OSS_ENDPOINT=https://oss-cn-shanghai.aliyuncs.com\n"+
		"ALIBABA_OSS_BUCKET=bucket-b\n")

	cfg, err := Load(root, "class-b")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ProfileName != "class-b" || cfg.CloudRegionID != "cn-shanghai" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestLoadReturnsMalformedEnvError(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".env"), "VULNSKY_ACTIVE_PROFILE\n")

	if _, err := Load(root, ""); err == nil {
		t.Fatal("expected malformed .env error")
	}
}

func TestLoadRejectsInvalidProfileName(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".env"), "VULNSKY_ACTIVE_PROFILE=..\\secret\n")

	if _, err := Load(root, ""); err == nil {
		t.Fatal("expected invalid profile name error")
	}
}

func TestLoadRejectsInvalidExplicitProfileName(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".env"), "VULNSKY_ACTIVE_PROFILE=default\n")

	if _, err := Load(root, "../secret"); err == nil {
		t.Fatal("expected invalid explicit profile name error")
	}
}

func mustWrite(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
