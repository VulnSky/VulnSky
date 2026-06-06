package commands

import (
	"vulnsky/internal/aliyun"
	"vulnsky/internal/config"

	"github.com/spf13/cobra"
)

type RootOptions struct {
	RootDir   string
	Profile   string
	Factories ClientFactories
}

type ClientFactories struct {
	NewSTS func(config.Config) (aliyun.STSClient, error)
	NewOSS func(config.Config) (aliyun.OSSClient, error)
	NewECS func(config.Config) (aliyun.ECSClient, error)
}

type rootState struct {
	rootDir   string
	profile   string
	factories ClientFactories
}

func NewRootCommand() *cobra.Command {
	return NewRootCommandWithOptions(RootOptions{})
}

func NewRootCommandWithOptions(options RootOptions) *cobra.Command {
	rootDir := options.RootDir
	if rootDir == "" {
		rootDir = "."
	}
	state := &rootState{
		rootDir:   rootDir,
		profile:   options.Profile,
		factories: options.Factories.withDefaults(),
	}
	cmd := &cobra.Command{
		Use:           "vulnsky",
		Short:         "VulnSky manages OSS QCOW2 uploads, ECS image imports, and ECS reimages.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInteractiveShell(cmd, state)
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if shouldSkipConfigBootstrap(cmd) {
				return nil
			}
			return ensureDefaultConfigFiles(state.rootDir)
		},
	}
	cmd.PersistentFlags().StringVar(&state.profile, "profile", state.profile, "profile name")
	cmd.PersistentFlags().StringVar(&state.rootDir, "root", state.rootDir, "project root directory")
	cmd.AddCommand(newProfileCommand(state))
	cmd.AddCommand(newOSSCommand(state))
	cmd.AddCommand(newImageCommand(state))
	cmd.AddCommand(newECSCommand(state))
	cmd.AddCommand(newDeployCommand(state))
	cmd.AddCommand(newDoctorCommand(state))
	cmd.AddCommand(newRecordsCommand(state))
	cmd.AddCommand(newShellCommand(state))
	cmd.AddCommand(newVersionCommand())
	return cmd
}

func (f ClientFactories) withDefaults() ClientFactories {
	if f.NewSTS == nil {
		f.NewSTS = func(cfg config.Config) (aliyun.STSClient, error) {
			return aliyun.NewSTSClient(aliyun.STSOptions{
				AccessKeyID:     cfg.CloudAccessKeyID,
				AccessKeySecret: cfg.CloudAccessKeySecret,
				RegionID:        cfg.CloudRegionID,
			})
		}
	}
	if f.NewOSS == nil {
		f.NewOSS = func(cfg config.Config) (aliyun.OSSClient, error) {
			return aliyun.NewOSSClient(aliyun.OSSOptions{
				AccessKeyID:     cfg.OSSAccessKeyID,
				AccessKeySecret: cfg.OSSAccessKeySecret,
				RegionID:        cfg.OSSRegionID,
				Endpoint:        cfg.OSSEndpoint,
			})
		}
	}
	if f.NewECS == nil {
		f.NewECS = func(cfg config.Config) (aliyun.ECSClient, error) {
			return aliyun.NewECSClient(aliyun.ECSOptions{
				AccessKeyID:     cfg.CloudAccessKeyID,
				AccessKeySecret: cfg.CloudAccessKeySecret,
				RegionID:        cfg.CloudRegionID,
			})
		}
	}
	return f
}

func shouldSkipConfigBootstrap(cmd *cobra.Command) bool {
	for current := cmd; current != nil; current = current.Parent() {
		switch current.Name() {
		case "version", "completion", "help":
			return true
		}
	}
	return false
}
