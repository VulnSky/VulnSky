package commands

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newECSCommand(state *rootState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ecs",
		Short: "Query and reimage ECS instances",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "ls",
		Short: "List ECS instances",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runECSList(cmd, state)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "show <instance-id>",
		Short: "Show ECS instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runECSShow(cmd, state, args[0])
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "use <instance-id>",
		Short: "Set default ECS instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runECSUse(cmd, state, args[0])
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "current-image [instance-id]",
		Short: "Show current ECS image",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			instanceID := ""
			if len(args) > 0 {
				instanceID = args[0]
			}
			return runECSCurrentImage(cmd, state, instanceID)
		},
	})
	startCmd := &cobra.Command{
		Use:   "start [instance-id]",
		Short: "Start an ECS instance and wait until it is running",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			instanceID := ""
			if len(args) > 0 {
				instanceID = args[0]
			}
			timeout, _ := cmd.Flags().GetDuration("timeout")
			interval, _ := cmd.Flags().GetDuration("poll-interval")
			return runECSStart(cmd, state, instanceID, timeout, interval)
		},
	}
	startCmd.Flags().Duration("timeout", 10*time.Minute, "maximum time to wait for the ECS instance to start")
	startCmd.Flags().Duration("poll-interval", 10*time.Second, "poll interval while waiting for the ECS instance")
	cmd.AddCommand(startCmd)

	reimageOpts := deployOptions{}
	reimageCmd := &cobra.Command{
		Use:   "reimage <image-id>",
		Short: "Reimage an ECS instance with an existing custom image",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runECSReimage(cmd, state, args[0], reimageOpts)
		},
	}
	reimageCmd.Flags().StringVar(&reimageOpts.InstanceID, "instance-id", "", "ECS instance id; defaults to VULNSKY_DEFAULT_ECS_INSTANCE_ID")
	reimageCmd.Flags().BoolVar(&reimageOpts.ForceStop, "force-stop", false, "force stop the ECS instance before replacing the system disk")
	reimageCmd.Flags().BoolVar(&reimageOpts.NoStart, "no-start", false, "leave the ECS instance stopped after replacing the system disk")
	reimageCmd.Flags().DurationVar(&reimageOpts.PollInterval, "poll-interval", 15*time.Second, "poll interval for cloud tasks")
	reimageCmd.Flags().DurationVar(&reimageOpts.StopTimeout, "stop-timeout", 0, "maximum time to wait for the ECS instance to stop; defaults to profile stop timeout")
	reimageCmd.Flags().DurationVar(&reimageOpts.StartTimeout, "start-timeout", 10*time.Minute, "maximum time to wait for the ECS instance to start")
	cmd.AddCommand(reimageCmd)
	return cmd
}

func runECSList(cmd *cobra.Command, state *rootState) error {
	_, client, err := loadECSClient(state)
	if err != nil {
		return err
	}
	instances, err := client.ListInstances(cmd.Context())
	if err != nil {
		return err
	}
	for _, instance := range instances {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\t%s\n", instance.ID, instance.Name, instance.Status, instance.ImageID, instance.PublicIP)
	}
	return nil
}

func runECSShow(cmd *cobra.Command, state *rootState, instanceID string) error {
	_, client, err := loadECSClient(state)
	if err != nil {
		return err
	}
	instance, err := client.DescribeInstance(cmd.Context(), instanceID)
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "InstanceId=%s\nName=%s\nStatus=%s\nImageId=%s\nType=%s\nZoneId=%s\nPublicIP=%s\nPrivateIP=%s\n",
		instance.ID,
		instance.Name,
		instance.Status,
		instance.ImageID,
		instance.Type,
		instance.ZoneID,
		instance.PublicIP,
		instance.PrivateIP,
	)
	return nil
}

func runECSUse(cmd *cobra.Command, state *rootState, instanceID string) error {
	_, client, err := loadECSClient(state)
	if err != nil {
		return err
	}
	instance, err := client.DescribeInstance(cmd.Context(), instanceID)
	if err != nil {
		return err
	}
	profileName, err := activeProfileName(state)
	if err != nil {
		return err
	}
	path, err := profileEnvPath(state.rootDir, profileName)
	if err != nil {
		return err
	}
	if err := upsertEnvValue(path, "VULNSKY_DEFAULT_ECS_INSTANCE_ID", instance.ID); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "default ecs=%s %s %s\n", instance.ID, instance.Name, instance.Status)
	return nil
}

func runECSCurrentImage(cmd *cobra.Command, state *rootState, instanceID string) error {
	cfg, client, err := loadECSClient(state)
	if err != nil {
		return err
	}
	if instanceID == "" {
		instanceID = cfg.DefaultECSInstanceID
	}
	if instanceID == "" {
		return fmt.Errorf("missing instance id and VULNSKY_DEFAULT_ECS_INSTANCE_ID is not configured")
	}
	instance, err := client.DescribeInstance(cmd.Context(), instanceID)
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", instance.ID, instance.Name, instance.ImageID)
	return nil
}

func runECSStart(cmd *cobra.Command, state *rootState, instanceID string, timeout time.Duration, interval time.Duration) error {
	cfg, client, err := loadECSClient(state)
	if err != nil {
		return err
	}
	if instanceID == "" {
		instanceID = cfg.DefaultECSInstanceID
	}
	if instanceID == "" {
		return fmt.Errorf("missing instance id and VULNSKY_DEFAULT_ECS_INSTANCE_ID is not configured")
	}
	instance, err := startInstanceWithRetry(cmd.Context(), cmd, client, instanceID, timeout, interval)
	if err != nil {
		return err
	}
	logDeploy(cmd, "done", "instance=%s status=%s", instance.ID, instance.Status)
	return nil
}

func runECSReimage(cmd *cobra.Command, state *rootState, imageID string, opts deployOptions) error {
	cfg, ecsClient, err := loadECSClient(state)
	if err != nil {
		return err
	}
	_, stsClient, err := loadSTSClient(state)
	if err != nil {
		return err
	}
	accountID, _, err := stsClient.GetCallerIdentity(cmd.Context())
	if err != nil {
		return err
	}

	instanceID := firstNonEmpty(opts.InstanceID, cfg.DefaultECSInstanceID)
	if instanceID == "" {
		return fmt.Errorf("missing ECS instance id and VULNSKY_DEFAULT_ECS_INSTANCE_ID is not configured")
	}
	instance, err := ecsClient.DescribeInstance(cmd.Context(), instanceID)
	if err != nil {
		return err
	}
	logDeploy(cmd, "ecs", "target instance=%s name=%s status=%s image=%s", instance.ID, instance.Name, instance.Status, instance.ImageID)
	logDeploy(cmd, "image", "using existing image=%s", imageID)

	_, err = reimageInstanceWithImage(cmd, cfg, accountID, ecsClient, instance, imageID, opts)
	return err
}
