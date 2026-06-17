package aliyun

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrNotFound = errors.New("not found")

type STSClient interface {
	GetCallerIdentity(ctx context.Context) (accountID string, arn string, err error)
}

type OSSClient interface {
	ListObjects(ctx context.Context, bucket string, prefix string) ([]string, error)
	ObjectExists(ctx context.Context, bucket string, key string) (bool, error)
	UploadFile(ctx context.Context, bucket string, key string, path string, onProgress func(done, total int64)) (requestID string, err error)
	PresignGet(ctx context.Context, bucket string, key string, expires time.Duration) (string, error)
	GetBucketLocation(ctx context.Context, bucket string) (string, error)
}

type ECSClient interface {
	ListInstances(ctx context.Context) ([]Instance, error)
	DescribeInstance(ctx context.Context, instanceID string) (Instance, error)
	ListImages(ctx context.Context, ownerAlias string) ([]Image, error)
	ImportImage(ctx context.Context, input ImportImageInput) (imageID string, taskID string, requestID string, err error)
	DescribeTask(ctx context.Context, taskID string) (TaskStatus, error)
	StopInstance(ctx context.Context, instanceID string, force bool) error
	StartInstance(ctx context.Context, instanceID string) error
	ReplaceSystemDisk(ctx context.Context, instanceID string, imageID string) (diskID string, requestID string, err error)
}

type STSOptions struct {
	AccessKeyID     string
	AccessKeySecret string
	RegionID        string
	Endpoint        string
}

type OSSOptions struct {
	AccessKeyID         string
	AccessKeySecret     string
	SecurityToken       string
	RegionID            string
	Endpoint            string
	ConnectTimeout      time.Duration
	ReadWriteTimeout    time.Duration
	RetryMaxAttempts    int
	UploadPartSizeBytes int64
	UploadParallel      int
	UploadCheckpoint    bool
	UploadCheckpointDir string
}

type ECSOptions struct {
	AccessKeyID     string
	AccessKeySecret string
	RegionID        string
	Endpoint        string
}

type Instance struct {
	ID         string
	Name       string
	Status     string
	ImageID    string
	Type       string
	ZoneID     string
	RegionID   string
	PublicIP   string
	PrivateIP  string
	CreateTime string
}

type Image struct {
	ID              string
	Name            string
	Status          string
	Progress        string
	CreationTime    string
	OwnerAlias      string
	Platform        string
	Architecture    string
	OSType          string
	SizeGiB         int32
	Usage           string
	SourceOSSBucket string
	SourceOSSObject string
	SourceFormat    string
}

type ImportImageInput struct {
	RegionID         string
	ImageName        string
	Description      string
	OSSBucket        string
	OSSObject        string
	Architecture     string
	OSType           string
	Platform         string
	BootMode         string
	RoleName         string
	ClientToken      string
	DiskImageSizeGiB int32
}

type TaskStatus struct {
	ID           string
	ResourceID   string
	Action       string
	Status       string
	CreationTime string
	FinishedTime string
}

func validateAccessOptions(service string, accessKeyID string, accessKeySecret string, regionID string) error {
	missing := make([]string, 0, 3)
	if accessKeyID == "" {
		missing = append(missing, "access key id")
	}
	if accessKeySecret == "" {
		missing = append(missing, "access key secret")
	}
	if regionID == "" {
		missing = append(missing, "region id")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing %s option(s) for %s", strings.Join(missing, ", "), service)
	}
	return nil
}

func ptr[T any](value T) *T {
	return &value
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func int32Value(value *int32) int32 {
	if value == nil {
		return 0
	}
	return *value
}

func firstString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func firstPtrString(values []*string) string {
	for _, value := range values {
		if value != nil && *value != "" {
			return *value
		}
	}
	return ""
}

func NormalizeOSSLocation(location string) string {
	return strings.TrimPrefix(strings.TrimSpace(location), "oss-")
}
