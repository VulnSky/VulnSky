package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"vulnsky/internal/config"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

type doctorOptions struct {
	Redact bool
}

func newDoctorCommand(state *rootState) *cobra.Command {
	opts := doctorOptions{}
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check config and Aliyun API access",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctor(cmd.Context(), cmd.OutOrStdout(), state, opts)
		},
	}
	cmd.Flags().BoolVar(&opts.Redact, "redact", false, "redact local paths and cloud identifiers in output")
	return cmd
}

func runDoctor(ctx context.Context, out io.Writer, state *rootState, opts doctorOptions) error {
	failures := 0
	warn := func(format string, args ...any) {
		fmt.Fprintf(out, "WARN "+format+"\n", args...)
	}
	pass := func(format string, args ...any) {
		fmt.Fprintf(out, "PASS "+format+"\n", args...)
	}
	fail := func(format string, args ...any) {
		failures++
		fmt.Fprintf(out, "FAIL "+format+"\n", args...)
	}

	profileName, profilePath, profileInfoErr := doctorProfileInfo(state.rootDir, state.profile)
	envPath := filepath.Join(state.rootDir, ".env")
	if _, err := os.Stat(envPath); err != nil {
		fail(".env 未找到，请从 .env.example 创建")
	} else {
		pass(".env 已存在: %s", doctorValue(envPath, opts.Redact))
	}

	if profileInfoErr != nil {
		fail("profile 名称无效: %v", profileInfoErr)
		return doctorResult(failures)
	}
	if _, err := os.Stat(profilePath); err != nil {
		fail("profile 未找到: %s", doctorValue(profilePath, opts.Redact))
	} else {
		pass("profile 已存在: %s", doctorValue(profilePath, opts.Redact))
	}

	cfg, err := config.Load(state.rootDir, state.profile)
	if err != nil {
		fail("profile 配置加载失败: %v", err)
		return doctorResult(failures)
	}
	pass("profile 已加载: %s", cfg.ProfileName)

	if cfg.CloudRegionID == cfg.OSSRegionID {
		pass("ECS/OSS 配置区域一致: %s", cfg.CloudRegionID)
	} else {
		fail("ECS/OSS 配置区域不一致: ECS=%s OSS=%s", cfg.CloudRegionID, cfg.OSSRegionID)
	}

	stsClient, err := state.factories.NewSTS(cfg)
	if err != nil {
		fail("STS 客户端创建失败: %v", err)
	} else {
		accountID, arn, err := stsClient.GetCallerIdentity(ctx)
		if err != nil {
			fail("STS GetCallerIdentity 失败: %v", err)
		} else {
			pass("STS 账号有效: %s", doctorValue(accountID, opts.Redact))
			if arn != "" {
				pass("STS 身份 ARN: %s", doctorValue(arn, opts.Redact))
			}
			if cfg.ExpectedAccountID != "" && cfg.ExpectedAccountID != accountID {
				fail("账号不匹配: 期望=%s 实际=%s", doctorValue(cfg.ExpectedAccountID, opts.Redact), doctorValue(accountID, opts.Redact))
			}
		}
	}

	ossClient, err := state.factories.NewOSS(cfg)
	if err != nil {
		fail("OSS 客户端创建失败: %v", err)
	} else {
		location, err := ossClient.GetBucketLocation(ctx, cfg.OSSBucket)
		if err != nil {
			fail("OSS bucket 区域查询失败: %v", err)
		} else {
			pass("OSS bucket 区域: %s", location)
			if location != "" && location != cfg.OSSRegionID {
				fail("OSS bucket 实际区域与配置不一致: bucket=%s profile=%s", location, cfg.OSSRegionID)
			}
		}
	}

	ecsClient, err := state.factories.NewECS(cfg)
	if err != nil {
		fail("ECS 客户端创建失败: %v", err)
	} else {
		instances, err := ecsClient.ListInstances(ctx)
		if err != nil {
			fail("ECS 实例查询失败: %v", err)
		} else {
			pass("ECS 区域可访问: %s, 实例数=%d", cfg.CloudRegionID, len(instances))
		}
		if cfg.DefaultECSInstanceID == "" {
			warn("默认 ECS 未配置: VULNSKY_DEFAULT_ECS_INSTANCE_ID")
		} else {
			instance, err := ecsClient.DescribeInstance(ctx, cfg.DefaultECSInstanceID)
			if err != nil {
				fail("默认 ECS 不可用: %s, %v", doctorValue(cfg.DefaultECSInstanceID, opts.Redact), err)
			} else {
				pass("默认 ECS 可用: %s %s %s", doctorValue(instance.ID, opts.Redact), doctorValue(instance.Name, opts.Redact), instance.Status)
			}
		}
	}

	if profileName != cfg.ProfileName {
		warn("profile 名称解析不一致: env=%s loaded=%s", profileName, cfg.ProfileName)
	}
	return doctorResult(failures)
}

func doctorProfileInfo(rootDir string, explicitProfile string) (string, string, error) {
	global, _ := godotenv.Read(filepath.Join(rootDir, ".env"))
	profileName := explicitProfile
	if profileName == "" {
		profileName = firstNonEmpty(global["VULNSKY_ACTIVE_PROFILE"], "default")
	}
	if err := config.ValidateProfileName(profileName); err != nil {
		return profileName, "", err
	}
	profileDir := filepath.Join(rootDir, firstNonEmpty(global["VULNSKY_PROFILE_DIR"], "./profiles"))
	return profileName, filepath.Join(profileDir, profileName+".env"), nil
}

func doctorResult(failures int) error {
	if failures > 0 {
		return fmt.Errorf("doctor found %d failure(s)", failures)
	}
	return nil
}

func doctorValue(value string, redact bool) string {
	if !redact || value == "" {
		return value
	}
	return "<redacted>"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
