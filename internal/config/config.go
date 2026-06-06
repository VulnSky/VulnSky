package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	RootDir              string
	ProfileName          string
	ProfileLabel         string
	DBPath               string
	ProfileDir           string
	ExpectedAccountID    string
	CloudAccessKeyID     string
	CloudAccessKeySecret string
	CloudRegionID        string
	OSSAccessKeyID       string
	OSSAccessKeySecret   string
	OSSRegionID          string
	OSSEndpoint          string
	OSSBucket            string
	DefaultECSInstanceID string
	DefaultObjectPrefix  string
	DefaultArchitecture  string
	DefaultOSType        string
	DefaultPlatform      string
	AutoStopInstance     bool
	AllowForceStop       bool
	StopTimeoutSeconds   int
	StartAfterReimage    bool
}

func Load(rootDir string, explicitProfile string) (Config, error) {
	global, err := readEnvIfExists(filepath.Join(rootDir, ".env"))
	if err != nil {
		return Config{}, err
	}
	profile := explicitProfile
	if profile == "" {
		profile = first(global["VULNSKY_ACTIVE_PROFILE"], "default")
	}
	if err := ValidateProfileName(profile); err != nil {
		return Config{}, err
	}
	profileDir := filepath.Join(rootDir, first(global["VULNSKY_PROFILE_DIR"], "./profiles"))
	profileEnv, err := readEnvIfExists(filepath.Join(profileDir, profile+".env"))
	if err != nil {
		return Config{}, err
	}
	env := merge(global, profileEnv)

	cfg := Config{
		RootDir:              rootDir,
		ProfileName:          profile,
		ProfileLabel:         first(env["VULNSKY_PROFILE_LABEL"], profile),
		DBPath:               filepath.Join(rootDir, first(env["VULNSKY_DB_PATH"], "./vulnsky.db")),
		ProfileDir:           profileDir,
		ExpectedAccountID:    env["VULNSKY_EXPECTED_ACCOUNT_ID"],
		CloudAccessKeyID:     env["ALIBABA_CLOUD_ACCESS_KEY_ID"],
		CloudAccessKeySecret: env["ALIBABA_CLOUD_ACCESS_KEY_SECRET"],
		CloudRegionID:        env["ALIBABA_CLOUD_REGION_ID"],
		OSSAccessKeyID:       first(env["ALIBABA_OSS_ACCESS_KEY_ID"], env["ALIBABA_CLOUD_ACCESS_KEY_ID"]),
		OSSAccessKeySecret:   first(env["ALIBABA_OSS_ACCESS_KEY_SECRET"], env["ALIBABA_CLOUD_ACCESS_KEY_SECRET"]),
		OSSRegionID:          first(env["ALIBABA_OSS_REGION_ID"], env["ALIBABA_CLOUD_REGION_ID"]),
		OSSEndpoint:          env["ALIBABA_OSS_ENDPOINT"],
		OSSBucket:            env["ALIBABA_OSS_BUCKET"],
		DefaultECSInstanceID: env["VULNSKY_DEFAULT_ECS_INSTANCE_ID"],
		DefaultObjectPrefix:  first(env["VULNSKY_DEFAULT_OBJECT_PREFIX"], "qcow2/"),
		DefaultArchitecture:  first(env["VULNSKY_DEFAULT_ARCHITECTURE"], "x86_64"),
		DefaultOSType:        first(env["VULNSKY_DEFAULT_OS_TYPE"], "linux"),
		DefaultPlatform:      first(env["VULNSKY_DEFAULT_PLATFORM"], "Others Linux"),
		AutoStopInstance:     boolValue(env["VULNSKY_AUTO_STOP_INSTANCE"], true),
		AllowForceStop:       boolValue(env["VULNSKY_ALLOW_FORCE_STOP"], false),
		StopTimeoutSeconds:   intValue(env["VULNSKY_STOP_TIMEOUT_SECONDS"], 60),
		StartAfterReimage:    boolValue(env["VULNSKY_START_AFTER_REIMAGE"], true),
	}
	return cfg, nil
}

func ValidateProfileName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("invalid profile name: empty")
	}
	if name != strings.TrimSpace(name) || name == "." || name == ".." || strings.ContainsAny(name, `/\`) || strings.ContainsRune(name, 0) {
		return fmt.Errorf("invalid profile name %q: use a simple name without path separators", name)
	}
	return nil
}

func readEnvIfExists(path string) (map[string]string, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	env, err := godotenv.Read(path)
	if err != nil {
		return nil, fmt.Errorf("load env file %s: %w", path, err)
	}
	return env, nil
}

func (cfg Config) ValidateECS() error {
	if cfg.CloudAccessKeyID == "" || cfg.CloudAccessKeySecret == "" || cfg.CloudRegionID == "" {
		return fmt.Errorf("missing ECS credentials or region in profile %q", cfg.ProfileName)
	}
	return nil
}

func (cfg Config) ValidateOSS() error {
	if cfg.OSSAccessKeyID == "" || cfg.OSSAccessKeySecret == "" || cfg.OSSRegionID == "" || cfg.OSSEndpoint == "" || cfg.OSSBucket == "" {
		return fmt.Errorf("missing OSS credentials, endpoint, bucket, or region in profile %q", cfg.ProfileName)
	}
	return nil
}

func merge(a, b map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

func first(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func boolValue(value string, fallback bool) bool {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func intValue(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
