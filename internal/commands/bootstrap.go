package commands

import (
	"os"
	"path/filepath"
)

const defaultRootEnv = `VULNSKY_ACTIVE_PROFILE=default
VULNSKY_DB_PATH=./vulnsky.db
VULNSKY_PROFILE_DIR=./profiles
`

const defaultProfileEnv = `VULNSKY_PROFILE_LABEL=default
VULNSKY_EXPECTED_ACCOUNT_ID=

ALIBABA_CLOUD_ACCESS_KEY_ID=
ALIBABA_CLOUD_ACCESS_KEY_SECRET=
ALIBABA_CLOUD_REGION_ID=

ALIBABA_OSS_ACCESS_KEY_ID=
ALIBABA_OSS_ACCESS_KEY_SECRET=
ALIBABA_OSS_REGION_ID=
ALIBABA_OSS_ENDPOINT=
ALIBABA_OSS_BUCKET=
VULNSKY_OSS_CONNECT_TIMEOUT_SECONDS=30
VULNSKY_OSS_READ_WRITE_TIMEOUT_SECONDS=300
VULNSKY_OSS_RETRY_MAX_ATTEMPTS=5
VULNSKY_OSS_UPLOAD_PART_SIZE_MIB=64
VULNSKY_OSS_UPLOAD_PARALLEL=3
VULNSKY_OSS_UPLOAD_CHECKPOINT=true
VULNSKY_OSS_UPLOAD_CHECKPOINT_DIR=./.vulnsky-checkpoints

VULNSKY_DEFAULT_ECS_INSTANCE_ID=
VULNSKY_DEFAULT_OBJECT_PREFIX=qcow2/
VULNSKY_DEFAULT_ARCHITECTURE=x86_64
VULNSKY_DEFAULT_OS_TYPE=linux
VULNSKY_DEFAULT_PLATFORM=Others Linux
VULNSKY_AUTO_STOP_INSTANCE=true
VULNSKY_ALLOW_FORCE_STOP=false
VULNSKY_STOP_TIMEOUT_SECONDS=60
VULNSKY_START_AFTER_REIMAGE=true
`

func ensureDefaultConfigFiles(rootDir string) error {
	envPath := filepath.Join(rootDir, ".env")
	if _, err := os.Stat(envPath); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := os.MkdirAll(rootDir, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(envPath, []byte(defaultRootEnv), 0o644); err != nil {
			return err
		}
	}

	global, err := readRootEnv(rootDir)
	if err != nil {
		return err
	}
	profileDir := filepath.Join(rootDir, firstNonEmpty(global["VULNSKY_PROFILE_DIR"], "./profiles"))
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		return err
	}
	defaultProfilePath := filepath.Join(profileDir, "default.env")
	if _, err := os.Stat(defaultProfilePath); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := os.WriteFile(defaultProfilePath, []byte(defaultProfileEnv), 0o644); err != nil {
			return err
		}
	}
	return nil
}
