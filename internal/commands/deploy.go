package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vulnsky/internal/aliyun"
	"vulnsky/internal/config"
	"vulnsky/internal/store"
	"vulnsky/internal/util"

	"github.com/spf13/cobra"
)

type deployOptions struct {
	InstanceID    string
	ImageID       string
	ImageName     string
	ObjectKey     string
	RoleName      string
	ForceStop     bool
	NoStart       bool
	PollInterval  time.Duration
	ImportTimeout time.Duration
	StopTimeout   time.Duration
	StartTimeout  time.Duration
	DiskSizeGiB   int32
}

func newDeployCommand(state *rootState) *cobra.Command {
	opts := deployOptions{}
	cmd := &cobra.Command{
		Use:   "deploy [qcow2-path|oss-object-key]",
		Short: "Upload or import a QCOW2 image and reimage the default ECS instance",
		Args: func(cmd *cobra.Command, args []string) error {
			if opts.ImageID != "" {
				return cobra.MaximumNArgs(1)(cmd, args)
			}
			return cobra.ExactArgs(1)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			source := ""
			if len(args) > 0 {
				source = args[0]
			}
			return runDeploy(cmd, state, source, opts)
		},
	}
	cmd.Flags().StringVar(&opts.InstanceID, "instance-id", "", "ECS instance id; defaults to VULNSKY_DEFAULT_ECS_INSTANCE_ID")
	cmd.Flags().StringVar(&opts.ImageID, "image-id", "", "existing ECS image id; skips OSS upload and image import when set")
	cmd.Flags().StringVar(&opts.ImageName, "image-name", "", "custom image name for ImportImage")
	cmd.Flags().StringVar(&opts.ObjectKey, "key", "", "OSS object key when deploying a local qcow2 path")
	cmd.Flags().StringVar(&opts.RoleName, "role-name", "", "RAM role name for ImportImage")
	cmd.Flags().BoolVar(&opts.ForceStop, "force-stop", false, "force stop the ECS instance before replacing the system disk")
	cmd.Flags().BoolVar(&opts.NoStart, "no-start", false, "leave the ECS instance stopped after replacing the system disk")
	cmd.Flags().DurationVar(&opts.PollInterval, "poll-interval", 15*time.Second, "poll interval for cloud tasks")
	cmd.Flags().DurationVar(&opts.ImportTimeout, "import-timeout", 90*time.Minute, "maximum time to wait for ImportImage")
	cmd.Flags().DurationVar(&opts.StopTimeout, "stop-timeout", 0, "maximum time to wait for the ECS instance to stop; defaults to profile stop timeout")
	cmd.Flags().DurationVar(&opts.StartTimeout, "start-timeout", 10*time.Minute, "maximum time to wait for the ECS instance to start")
	cmd.Flags().Int32Var(&opts.DiskSizeGiB, "disk-size-gib", 0, "optional imported system disk size in GiB")
	return cmd
}

func runDeploy(cmd *cobra.Command, state *rootState, source string, opts deployOptions) error {
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

	imageID := opts.ImageID
	taskID := ""
	if imageID == "" {
		if err := cfg.ValidateOSS(); err != nil {
			return err
		}
		if cfg.CloudRegionID != cfg.OSSRegionID {
			return fmt.Errorf("ECS/OSS regions must match for ImportImage: ecs=%s oss=%s", cfg.CloudRegionID, cfg.OSSRegionID)
		}
		ossClient, err := state.factories.NewOSS(cfg)
		if err != nil {
			return err
		}
		objectKey, localPath, isLocalFile, err := deploySource(source)
		if err != nil {
			return err
		}
		if isLocalFile {
			objectKey, err = uploadDeployLocalSource(cmd, cfg, ossClient, localPath, opts.ObjectKey, accountID)
			if err != nil {
				return err
			}
		} else {
			exists, err := ossClient.ObjectExists(cmd.Context(), cfg.OSSBucket, objectKey)
			if err != nil {
				return err
			}
			if !exists {
				return fmt.Errorf("OSS object not found: oss://%s/%s", cfg.OSSBucket, objectKey)
			}
			logDeploy(cmd, "oss", "found %s", objectKey)
		}

		imageName := opts.ImageName
		if imageName == "" {
			imageName = deployImageName(objectKey, time.Now())
		}
		if reusedImageID, reused, err := findReusableImportedImage(cmd, cfg, accountID, ecsClient, objectKey, imageName); err != nil {
			return err
		} else if reused {
			imageID = reusedImageID
		} else {
			requestID := ""
			imageID, taskID, requestID, err = startImageImport(cmd, cfg, ecsClient, objectKey, imageName, opts)
			if err != nil {
				return err
			}
			if taskID == "" {
				return fmt.Errorf("ImportImage returned empty task id for image %s", imageID)
			}
			if imageID, err = waitImageImport(cmd.Context(), cmd, ecsClient, taskID, imageID, opts.ImportTimeout, opts.PollInterval); err != nil {
				return err
			}
			if err := recordImageImport(cfg, accountID, imageName, imageID, taskID, requestID, cfg.OSSBucket, objectKey, "Finished"); err != nil {
				return err
			}
		}
	} else {
		logDeploy(cmd, "image", "using existing image=%s", imageID)
	}

	_, err = reimageInstanceWithImage(cmd, cfg, accountID, ecsClient, instance, imageID, opts)
	return err
}

func deploySource(source string) (objectKey string, localPath string, isLocalFile bool, err error) {
	info, statErr := os.Stat(source)
	if statErr == nil {
		if info.IsDir() {
			return "", "", false, fmt.Errorf("deploy source is a directory, expected qcow2 file: %s", source)
		}
		return source, source, true, nil
	}
	if looksLikeLocalPath(source) {
		return "", "", false, fmt.Errorf("local qcow2 file not found: %s", source)
	}
	return source, "", false, nil
}

func looksLikeLocalPath(value string) bool {
	return filepath.IsAbs(value) ||
		strings.HasPrefix(value, ".\\") ||
		strings.HasPrefix(value, "./") ||
		strings.HasPrefix(value, "..\\") ||
		strings.HasPrefix(value, "../") ||
		strings.Contains(value, `:\`) ||
		strings.Contains(value, ":/")
}

func uploadDeployLocalSource(cmd *cobra.Command, cfg config.Config, client aliyun.OSSClient, localPath string, objectKey string, accountID string) (string, error) {
	sum, size, err := util.FileSHA256(localPath)
	if err != nil {
		return "", err
	}
	if objectKey == "" {
		objectKey = defaultObjectKey(cfgDefaultObjectPrefix(cfg.DefaultObjectPrefix), localPath)
	}

	st := store.New(cfg.DBPath)
	if err := st.Init(); err != nil {
		return "", err
	}
	if existing, err := st.FindUploadedBySHA256(cfg.ProfileName, accountID, cfg.OSSRegionID, sum); err != nil {
		return "", err
	} else if existing != nil {
		exists, err := client.ObjectExists(cmd.Context(), existing.Bucket, existing.ObjectKey)
		if err != nil {
			return "", err
		}
		if exists {
			logDeploy(cmd, "oss", "reused %s sha256=%s size=%d", existing.ObjectKey, sum, size)
			return existing.ObjectKey, nil
		}
	}

	logDeploy(cmd, "oss", "uploading local=%s key=%s size=%d", localPath, objectKey, size)
	requestID, err := client.UploadFile(cmd.Context(), cfg.OSSBucket, objectKey, localPath, nil)
	if err != nil {
		return "", err
	}
	if _, err := st.InsertUpload(store.UploadRecord{
		ProfileName:  cfg.ProfileName,
		AccountID:    accountID,
		RegionID:     cfg.OSSRegionID,
		OSSRegionID:  cfg.OSSRegionID,
		Bucket:       cfg.OSSBucket,
		ObjectKey:    objectKey,
		LocalPath:    localPath,
		FileName:     filepath.Base(localPath),
		FileSize:     size,
		SHA256:       sum,
		UploadStatus: "uploaded",
		RequestID:    requestID,
	}); err != nil {
		return "", err
	}
	logDeploy(cmd, "oss", "uploaded %s sha256=%s size=%d", objectKey, sum, size)
	return objectKey, nil
}

func findReusableImportedImage(cmd *cobra.Command, cfg config.Config, accountID string, client aliyun.ECSClient, objectKey string, imageName string) (string, bool, error) {
	st := store.New(cfg.DBPath)
	if err := st.Init(); err != nil {
		return "", false, err
	}
	images, err := client.ListImages(cmd.Context(), "self")
	if err != nil {
		return "", false, err
	}

	staleImageID := ""
	if existing, err := st.FindFinishedImageImportByOSSObject(cfg.ProfileName, accountID, cfg.CloudRegionID, cfg.OSSBucket, objectKey); err != nil {
		return "", false, err
	} else if existing != nil {
		if image, ok := findImageByID(images, existing.ImageID); ok && isReusableImageStatus(image.Status) {
			logDeploy(cmd, "image", "reused image=%s object=%s source=history status=%s", image.ID, objectKey, dash(image.Status))
			return image.ID, true, nil
		}
		staleImageID = existing.ImageID
	}

	if image, ok := findImageByOSSObject(images, cfg.OSSBucket, objectKey); ok {
		logDeploy(cmd, "image", "reused image=%s object=%s source=ecs status=%s", image.ID, objectKey, dash(image.Status))
		if err := recordImageImport(cfg, accountID, firstNonEmpty(image.Name, imageName), image.ID, "", "", cfg.OSSBucket, objectKey, "Reused"); err != nil {
			return "", false, err
		}
		return image.ID, true, nil
	}

	if staleImageID != "" {
		logDeploy(cmd, "image", "recorded image=%s object=%s not available; importing again", staleImageID, objectKey)
	}
	return "", false, nil
}

func findImageByID(images []aliyun.Image, imageID string) (aliyun.Image, bool) {
	for _, image := range images {
		if image.ID == imageID {
			return image, true
		}
	}
	return aliyun.Image{}, false
}

func findImageByOSSObject(images []aliyun.Image, bucket string, objectKey string) (aliyun.Image, bool) {
	for _, image := range images {
		if image.SourceOSSBucket == bucket && image.SourceOSSObject == objectKey && isReusableImageStatus(image.Status) {
			return image, true
		}
	}
	return aliyun.Image{}, false
}

func isReusableImageStatus(status string) bool {
	return strings.EqualFold(strings.TrimSpace(status), "Available")
}

func startImageImport(cmd *cobra.Command, cfg config.Config, client aliyun.ECSClient, objectKey string, imageName string, opts deployOptions) (string, string, string, error) {
	imageID, taskID, requestID, err := client.ImportImage(cmd.Context(), aliyun.ImportImageInput{
		RegionID:         cfg.CloudRegionID,
		ImageName:        imageName,
		OSSBucket:        cfg.OSSBucket,
		OSSObject:        objectKey,
		Architecture:     cfg.DefaultArchitecture,
		OSType:           cfg.DefaultOSType,
		Platform:         cfg.DefaultPlatform,
		RoleName:         opts.RoleName,
		Description:      "Imported by VulnSky",
		DiskImageSizeGiB: opts.DiskSizeGiB,
	})
	if err != nil {
		return "", "", "", err
	}
	logDeploy(cmd, "image", "import started image=%s task=%s request=%s name=%s", imageID, taskID, requestID, imageName)
	return imageID, taskID, requestID, nil
}

func waitImageImport(ctx context.Context, cmd *cobra.Command, client aliyun.ECSClient, taskID string, fallbackImageID string, timeout time.Duration, interval time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for {
		task, err := client.DescribeTask(ctx, taskID)
		if err != nil {
			if !errors.Is(err, aliyun.ErrNotFound) || time.Now().After(deadline) {
				return "", err
			}
			logDeploy(cmd, "image", "task=%s status=NotFound; retrying", taskID)
		} else {
			logDeploy(cmd, "image", "task=%s status=%s resource=%s", task.ID, task.Status, task.ResourceID)
			if isTaskSuccess(task.Status) {
				return firstNonEmpty(task.ResourceID, fallbackImageID), nil
			}
			if isTaskFailure(task.Status) {
				return "", fmt.Errorf("ImportImage task %s failed with status %s", taskID, task.Status)
			}
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("timed out waiting for ImportImage task %s after %s", taskID, timeout)
		}
		if err := sleepContext(ctx, interval); err != nil {
			return "", err
		}
	}
}

func stopInstanceForReimage(ctx context.Context, cmd *cobra.Command, client aliyun.ECSClient, instanceID string, force bool, timeout time.Duration, interval time.Duration) (string, error) {
	instance, err := client.DescribeInstance(ctx, instanceID)
	if err != nil {
		return "", err
	}
	if strings.EqualFold(instance.Status, "Stopped") {
		logDeploy(cmd, "ecs", "already stopped")
		return "already-stopped", nil
	}

	if err := client.StopInstance(ctx, instanceID, force); err != nil {
		return "", err
	}
	stopMode := "normal"
	if force {
		stopMode = "force"
	}
	logDeploy(cmd, "ecs", "stop requested mode=%s", stopMode)
	if _, err := waitInstanceStatus(ctx, cmd, client, instanceID, "Stopped", timeout, interval); err != nil {
		return "", err
	}
	return stopMode, nil
}

func waitInstanceStatus(ctx context.Context, cmd *cobra.Command, client aliyun.ECSClient, instanceID string, target string, timeout time.Duration, interval time.Duration) (aliyun.Instance, error) {
	deadline := time.Now().Add(timeout)
	for {
		instance, err := client.DescribeInstance(ctx, instanceID)
		if err != nil {
			return aliyun.Instance{}, err
		}
		logDeploy(cmd, "ecs", "instance=%s status=%s target=%s", instanceID, instance.Status, target)
		if strings.EqualFold(instance.Status, target) {
			return instance, nil
		}
		if time.Now().After(deadline) {
			return aliyun.Instance{}, fmt.Errorf("timed out waiting for instance %s to reach %s after %s", instanceID, target, timeout)
		}
		if err := sleepContext(ctx, interval); err != nil {
			return aliyun.Instance{}, err
		}
	}
}

func startInstanceWithRetry(ctx context.Context, cmd *cobra.Command, client aliyun.ECSClient, instanceID string, timeout time.Duration, interval time.Duration) (aliyun.Instance, error) {
	deadline := time.Now().Add(timeout)
	for {
		instance, err := client.DescribeInstance(ctx, instanceID)
		if err != nil {
			return aliyun.Instance{}, err
		}
		if strings.EqualFold(instance.Status, "Running") {
			logDeploy(cmd, "ecs", "already running")
			return instance, nil
		}

		err = client.StartInstance(ctx, instanceID)
		if err == nil {
			logDeploy(cmd, "ecs", "start requested")
			return waitInstanceStatus(ctx, cmd, client, instanceID, "Running", time.Until(deadline), interval)
		}
		if !isRetryableInstanceStatusError(err) || time.Now().After(deadline) {
			return aliyun.Instance{}, err
		}
		logDeploy(cmd, "ecs", "start not accepted yet; retrying")
		if err := sleepContext(ctx, interval); err != nil {
			return aliyun.Instance{}, err
		}
	}
}

func reimageInstanceWithImage(cmd *cobra.Command, cfg config.Config, accountID string, ecsClient aliyun.ECSClient, instance aliyun.Instance, imageID string, opts deployOptions) (aliyun.Instance, error) {
	stopTimeout := opts.StopTimeout
	if stopTimeout == 0 {
		stopTimeout = time.Duration(cfg.StopTimeoutSeconds) * time.Second
	}
	stopMode, err := stopInstanceForReimage(cmd.Context(), cmd, ecsClient, instance.ID, opts.ForceStop || cfg.AllowForceStop, stopTimeout, opts.PollInterval)
	if err != nil {
		return aliyun.Instance{}, err
	}

	diskID, requestID, err := ecsClient.ReplaceSystemDisk(cmd.Context(), instance.ID, imageID)
	if err != nil {
		return aliyun.Instance{}, err
	}
	logDeploy(cmd, "ecs", "replace system disk disk=%s request=%s", diskID, requestID)
	tryRecordDeployment(cmd, cfg, accountID, instance, instance.ImageID, imageID, diskID, stopMode, requestID, "replace_succeeded")

	if cfg.StartAfterReimage && !opts.NoStart {
		if _, err := startInstanceWithRetry(cmd.Context(), cmd, ecsClient, instance.ID, opts.StartTimeout, opts.PollInterval); err != nil {
			return aliyun.Instance{}, err
		}
	}

	final, err := ecsClient.DescribeInstance(cmd.Context(), instance.ID)
	if err != nil {
		return aliyun.Instance{}, err
	}
	tryRecordDeployment(cmd, cfg, accountID, final, instance.ImageID, imageID, diskID, stopMode, requestID, "reimaged")
	logDeploy(cmd, "done", "instance=%s image=%s status=%s", final.ID, imageID, final.Status)
	return final, nil
}

func recordImageImport(cfg config.Config, accountID string, imageName string, imageID string, taskID string, requestID string, ossBucket string, ossObject string, taskStatus string) error {
	st := store.New(cfg.DBPath)
	if err := st.Init(); err != nil {
		return err
	}
	_, err := st.InsertImageImport(store.ImageImportRecord{
		ProfileName:  cfg.ProfileName,
		AccountID:    accountID,
		RegionID:     cfg.CloudRegionID,
		UploadID:     0,
		OSSBucket:    ossBucket,
		OSSObject:    ossObject,
		ImageName:    imageName,
		ImageID:      imageID,
		TaskID:       taskID,
		TaskStatus:   taskStatus,
		Platform:     cfg.DefaultPlatform,
		Architecture: cfg.DefaultArchitecture,
		OSType:       cfg.DefaultOSType,
		RequestID:    requestID,
	})
	return err
}

func tryRecordDeployment(cmd *cobra.Command, cfg config.Config, accountID string, instance aliyun.Instance, previousImageID string, imageID string, diskID string, stopMode string, requestID string, status string) {
	if err := recordDeployment(cfg, accountID, instance, previousImageID, imageID, diskID, stopMode, requestID, status); err != nil {
		logDeploy(cmd, "warn", "deployment record failed: %v", err)
	}
}

func recordDeployment(cfg config.Config, accountID string, instance aliyun.Instance, previousImageID string, imageID string, diskID string, stopMode string, requestID string, status string) error {
	st := store.New(cfg.DBPath)
	if err := st.Init(); err != nil {
		return err
	}
	_, err := st.InsertDeployment(store.DeploymentRecord{
		ProfileName:     cfg.ProfileName,
		AccountID:       accountID,
		RegionID:        cfg.CloudRegionID,
		InstanceID:      instance.ID,
		InstanceName:    instance.Name,
		SourceUploadID:  0,
		SourceImageID:   imageID,
		PreviousImageID: previousImageID,
		NewDiskID:       diskID,
		StopMode:        stopMode,
		Status:          status,
		RequestID:       requestID,
	})
	return err
}

func deployImageName(objectKey string, now time.Time) string {
	return fmt.Sprintf("%s-%s", imageNameFromObjectKey(objectKey), now.Format("20060102150405"))
}

func isTaskSuccess(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "finished", "success", "succeed", "succeeded", "completed":
		return true
	default:
		return false
	}
}

func isTaskFailure(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "failed", "fail", "canceled", "cancelled":
		return true
	default:
		return false
	}
}

func isRetryableInstanceStatusError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "IncorrectInstanceStatus")
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	if duration <= 0 {
		duration = time.Second
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func logDeploy(cmd *cobra.Command, stage string, format string, args ...any) {
	fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s\n", stage, fmt.Sprintf(format, args...))
}
