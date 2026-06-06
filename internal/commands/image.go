package commands

import (
	"fmt"
	"path/filepath"
	"strings"

	"vulnsky/internal/aliyun"
	"vulnsky/internal/store"

	"github.com/spf13/cobra"
)

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
			name, _ := cmd.Flags().GetString("name")
			return runImageImport(cmd, state, args[0], name)
		},
	}
	importCmd.Flags().String("name", "", "ECS custom image name")
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

func runImageImport(cmd *cobra.Command, state *rootState, objectKey string, imageName string) error {
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
	if imageName == "" {
		imageName = imageNameFromObjectKey(objectKey)
	}
	imageID, taskID, requestID, err := ecsClient.ImportImage(cmd.Context(), aliyun.ImportImageInput{
		RegionID:         cfg.CloudRegionID,
		ImageName:        imageName,
		OSSBucket:        cfg.OSSBucket,
		OSSObject:        objectKey,
		Architecture:     cfg.DefaultArchitecture,
		OSType:           cfg.DefaultOSType,
		Platform:         cfg.DefaultPlatform,
		ClientToken:      "",
		Description:      "Imported by VulnSky",
		DiskImageSizeGiB: 0,
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
		Platform:     cfg.DefaultPlatform,
		Architecture: cfg.DefaultArchitecture,
		OSType:       cfg.DefaultOSType,
		RequestID:    requestID,
	}); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "image=%s task=%s request=%s\n", imageID, taskID, requestID)
	return nil
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
