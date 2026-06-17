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
		"VULNSKY_OSS_CONNECT_TIMEOUT_SECONDS=45\n"+
		"VULNSKY_OSS_READ_WRITE_TIMEOUT_SECONDS=600\n"+
		"VULNSKY_OSS_RETRY_MAX_ATTEMPTS=7\n"+
		"VULNSKY_OSS_UPLOAD_PART_SIZE_MIB=128\n"+
		"VULNSKY_OSS_UPLOAD_PARALLEL=4\n"+
		"VULNSKY_OSS_UPLOAD_CHECKPOINT=false\n"+
		"VULNSKY_OSS_UPLOAD_CHECKPOINT_DIR=./oss-cp\n"+
		"VULNSKY_STOP_TIMEOUT_SECONDS=60\n")

	cfg, err := Load(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ProfileName != "class-a" || cfg.OSSBucket != "lab-bucket" || cfg.StopTimeoutSeconds != 60 {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if cfg.OSSConnectTimeoutSeconds != 45 ||
		cfg.OSSReadWriteTimeoutSeconds != 600 ||
		cfg.OSSRetryMaxAttempts != 7 ||
		cfg.OSSUploadPartSizeMiB != 128 ||
		cfg.OSSUploadParallel != 4 ||
		cfg.OSSUploadCheckpoint ||
		cfg.OSSUploadCheckpointDir != filepath.Join(root, "oss-cp") {
		t.Fatalf("unexpected OSS upload config: %+v", cfg)
	}
}

func TestLoadUsesLargeOSSUploadDefaults(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".env"), "VULNSKY_ACTIVE_PROFILE=class-a\nVULNSKY_PROFILE_DIR=./profiles\n")
	if err := os.Mkdir(filepath.Join(root, "profiles"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(root, "profiles", "class-a.env"), ""+
		"ALIBABA_CLOUD_ACCESS_KEY_ID=ak\n"+
		"ALIBABA_CLOUD_ACCESS_KEY_SECRET=secret\n"+
		"ALIBABA_CLOUD_REGION_ID=cn-hangzhou\n"+
		"ALIBABA_OSS_ENDPOINT=https://oss-cn-hangzhou.aliyuncs.com\n"+
		"ALIBABA_OSS_BUCKET=lab-bucket\n")

	cfg, err := Load(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OSSConnectTimeoutSeconds != 30 ||
		cfg.OSSReadWriteTimeoutSeconds != 300 ||
		cfg.OSSRetryMaxAttempts != 5 ||
		cfg.OSSUploadPartSizeMiB != 64 ||
		cfg.OSSUploadParallel != 3 ||
		!cfg.OSSUploadCheckpoint ||
		cfg.OSSUploadCheckpointDir != filepath.Join(root, ".vulnsky-checkpoints") {
		t.Fatalf("unexpected default OSS upload config: %+v", cfg)
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
