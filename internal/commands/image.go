package commands

import (
	"fmt"
	"path/filepath"
	"strings"

	"vulnsky/internal/aliyun"
	"vulnsky/internal/config"
	"vulnsky/internal/store"

	"github.com/spf13/cobra"
)

type imageImportOptions struct {
	Name             string
	RoleName         string
	Architecture     string
	OSType           string
	Platform         string
	BootMode         string
	DiskImageSizeGiB int32
}

func newImageCommand(state *rootState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "image",
		Short: "Import and query ECS images",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	listCmd := &cobra.Command{
		Use:   "ls",
		Short: "List ECS images and mark VulnSky deployments",
		RunE: func(cmd *cobra.Command, args []string) error {
			owner, _ := cmd.Flags().GetString("owner")
			return runImageList(cmd, state, owner)
		},
	}
	listCmd.Flags().String("owner", "self", "image owner alias: self, system, others, marketplace, or all")
	cmd.AddCommand(listCmd)

	cmd.AddCommand(&cobra.Command{
		Use:   "source <image-id>",
		Short: "Show the OSS QCOW2 source of an imported ECS image",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runImageSource(cmd, state, args[0])
		},
	})

	importCmd := &cobra.Command{
		Use:   "import <object-key>",
		Short: "Import an OSS QCOW2 object as an ECS custom image",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := imageImportOptions{}
			opts.Name, _ = cmd.Flags().GetString("name")
			opts.RoleName, _ = cmd.Flags().GetString("role-name")
			opts.Architecture, _ = cmd.Flags().GetString("architecture")
			opts.OSType, _ = cmd.Flags().GetString("os-type")
			opts.Platform, _ = cmd.Flags().GetString("platform")
			opts.BootMode, _ = cmd.Flags().GetString("boot-mode")
			opts.DiskImageSizeGiB, _ = cmd.Flags().GetInt32("disk-size-gib")
			return runImageImport(cmd, state, args[0], opts)
		},
	}
	importCmd.Flags().String("name", "", "ECS custom image name")
	importCmd.Flags().String("role-name", "", "RAM role name for ImportImage")
	importCmd.Flags().String("architecture", "", "ImportImage architecture; defaults to VULNSKY_DEFAULT_ARCHITECTURE")
	importCmd.Flags().String("os-type", "", "ImportImage OS type; defaults to VULNSKY_DEFAULT_OS_TYPE")
	importCmd.Flags().String("platform", "", "ImportImage platform, for example Debian; defaults to VULNSKY_DEFAULT_PLATFORM")
	importCmd.Flags().String("boot-mode", "", "optional ImportImage boot mode")
	importCmd.Flags().Int32("disk-size-gib", 0, "optional imported system disk size in GiB")
	cmd.AddCommand(importCmd)
	cmd.AddCommand(&cobra.Command{
		Use:   "status <task-id>",
		Short: "Show image import task status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runImageStatus(cmd, state, args[0])
		},
	})
	return cmd
}

func runImageList(cmd *cobra.Command, state *rootState, ownerAlias string) error {
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

	images, err := ecsClient.ListImages(cmd.Context(), ownerAlias)
	if err != nil {
		return err
	}
	st := store.New(cfg.DBPath)
	if err := st.Init(); err != nil {
		return err
	}
	deployed, err := st.DeployedImages(cfg.ProfileName, accountID, cfg.CloudRegionID)
	if err != nil {
		return err
	}

	fmt.Fprintln(cmd.OutOrStdout(), "IMAGE_ID\tNAME\tSTATUS\tPROGRESS\tCREATED\tVULNSKY\tDEPLOYED_TO")
	for _, image := range images {
		vulnsky := "-"
		deployedTo := "-"
		if mark, ok := deployed[image.ID]; ok {
			vulnsky = "deployed"
			deployedTo = mark.InstanceID
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			image.ID,
			dash(image.Name),
			dash(image.Status),
			dash(image.Progress),
			dash(image.CreationTime),
			vulnsky,
			deployedTo,
		)
	}
	return nil
}

func runImageSource(cmd *cobra.Command, state *rootState, imageID string) error {
	_, ecsClient, err := loadECSClient(state)
	if err != nil {
		return err
	}
	images, err := ecsClient.ListImages(cmd.Context(), "self")
	if err != nil {
		return err
	}
	for _, image := range images {
		if image.ID != imageID {
			continue
		}
		sourceURI := "-"
		if image.SourceOSSBucket != "" && image.SourceOSSObject != "" {
			sourceURI = fmt.Sprintf("oss://%s/%s", image.SourceOSSBucket, image.SourceOSSObject)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "ImageId=%s\nImageName=%s\nStatus=%s\nSourceOSS=%s\nSourceBucket=%s\nSourceObject=%s\nSourceFormat=%s\n",
			image.ID,
			dash(image.Name),
			dash(image.Status),
			sourceURI,
			dash(image.SourceOSSBucket),
			dash(image.SourceOSSObject),
			dash(image.SourceFormat),
		)
		return nil
	}
	return fmt.Errorf("image not found: %s", imageID)
}

func runImageImport(cmd *cobra.Command, state *rootState, objectKey string, opts imageImportOptions) error {
	cfg, ecsClient, err := loadECSClient(state)
	if err != nil {
		return err
	}
	if cfg.OSSBucket == "" {
		return fmt.Errorf("missing ALIBABA_OSS_BUCKET in profile %q", cfg.ProfileName)
	}
	if cfg.CloudRegionID != cfg.OSSRegionID {
		return fmt.Errorf("ECS/OSS regions must match for ImportImage: ecs=%s oss=%s", cfg.CloudRegionID, cfg.OSSRegionID)
	}
	_, stsClient, err := loadSTSClient(state)
	if err != nil {
		return err
	}
	accountID, _, err := stsClient.GetCallerIdentity(cmd.Context())
	if err != nil {
		return err
	}
	imageName := opts.Name
	if imageName == "" {
		imageName = imageNameFromObjectKey(objectKey)
	}
	settings := imageImportSettingsFromOptions(cfg, opts)
	imageID, taskID, requestID, err := ecsClient.ImportImage(cmd.Context(), aliyun.ImportImageInput{
		RegionID:         cfg.CloudRegionID,
		ImageName:        imageName,
		OSSBucket:        cfg.OSSBucket,
		OSSObject:        objectKey,
		Architecture:     settings.Architecture,
		OSType:           settings.OSType,
		Platform:         settings.Platform,
		BootMode:         settings.BootMode,
		RoleName:         opts.RoleName,
		ClientToken:      "",
		Description:      "Imported by VulnSky",
		DiskImageSizeGiB: opts.DiskImageSizeGiB,
	})
	if err != nil {
		return err
	}
	st := store.New(cfg.DBPath)
	if err := st.Init(); err != nil {
		return err
	}
	if _, err := st.InsertImageImport(store.ImageImportRecord{
		ProfileName:  cfg.ProfileName,
		AccountID:    accountID,
		RegionID:     cfg.CloudRegionID,
		UploadID:     0,
		OSSBucket:    cfg.OSSBucket,
		OSSObject:    objectKey,
		ImageName:    imageName,
		ImageID:      imageID,
		TaskID:       taskID,
		TaskStatus:   "Processing",
		Platform:     settings.Platform,
		Architecture: settings.Architecture,
		OSType:       settings.OSType,
		RequestID:    requestID,
	}); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "image=%s task=%s request=%s platform=%s os=%s arch=%s\n", imageID, taskID, requestID, settings.Platform, settings.OSType, settings.Architecture)
	return nil
}

func imageImportSettingsFromOptions(cfg config.Config, opts imageImportOptions) imageImportSettings {
	return imageImportSettings{
		Architecture: firstNonEmpty(opts.Architecture, cfg.DefaultArchitecture),
		OSType:       firstNonEmpty(opts.OSType, cfg.DefaultOSType),
		Platform:     firstNonEmpty(opts.Platform, cfg.DefaultPlatform),
		BootMode:     opts.BootMode,
	}
}

func runImageStatus(cmd *cobra.Command, state *rootState, taskID string) error {
	_, client, err := loadECSClient(state)
	if err != nil {
		return err
	}
	task, err := client.DescribeTask(cmd.Context(), taskID)
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "task=%s action=%s status=%s resource=%s created=%s finished=%s\n",
		task.ID,
		task.Action,
		task.Status,
		task.ResourceID,
		task.CreationTime,
		task.FinishedTime,
	)
	return nil
}

func imageNameFromObjectKey(objectKey string) string {
	base := filepath.Base(objectKey)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	base = strings.NewReplacer(" ", "-", "_", "-").Replace(base)
	if base == "" || base == "." {
		return "vulnsky-image"
	}
	return "vulnsky-" + base
}

func dash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}
