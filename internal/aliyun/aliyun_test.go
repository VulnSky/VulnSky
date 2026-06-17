package aliyun

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	ecsapi "github.com/alibabacloud-go/ecs-20140526/v4/client"
)

func TestOfficialConstructorsValidateRequiredOptions(t *testing.T) {
	tests := []struct {
		name string
		fn   func() error
	}{
		{
			name: "sts",
			fn: func() error {
				_, err := NewSTSClient(STSOptions{AccessKeyID: "ak", AccessKeySecret: "secret"})
				return err
			},
		},
		{
			name: "oss",
			fn: func() error {
				_, err := NewOSSClient(OSSOptions{AccessKeyID: "ak", AccessKeySecret: "secret", RegionID: "cn-hangzhou"})
				return err
			},
		},
		{
			name: "ecs",
			fn: func() error {
				_, err := NewECSClient(ECSOptions{AccessKeyID: "ak", AccessKeySecret: "secret"})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.fn(); err == nil {
				t.Fatal("expected missing option error")
			}
		})
	}
}

func TestOfficialConstructorsDoNotCallNetwork(t *testing.T) {
	stsClient, err := NewSTSClient(STSOptions{
		AccessKeyID:     "ak",
		AccessKeySecret: "secret",
		RegionID:        "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("NewSTSClient() error = %v", err)
	}
	if stsClient == nil {
		t.Fatal("NewSTSClient() returned nil")
	}

	ossClient, err := NewOSSClient(OSSOptions{
		AccessKeyID:     "ak",
		AccessKeySecret: "secret",
		RegionID:        "cn-hangzhou",
		Endpoint:        "https://oss-cn-hangzhou.aliyuncs.com",
	})
	if err != nil {
		t.Fatalf("NewOSSClient() error = %v", err)
	}
	if ossClient == nil {
		t.Fatal("NewOSSClient() returned nil")
	}

	ecsClient, err := NewECSClient(ECSOptions{
		AccessKeyID:     "ak",
		AccessKeySecret: "secret",
		RegionID:        "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("NewECSClient() error = %v", err)
	}
	if ecsClient == nil {
		t.Fatal("NewECSClient() returned nil")
	}
}

func TestOSSClientStoresLargeUploadOptions(t *testing.T) {
	client, err := NewOSSClient(OSSOptions{
		AccessKeyID:         "ak",
		AccessKeySecret:     "secret",
		RegionID:            "cn-hangzhou",
		Endpoint:            "https://oss-cn-hangzhou.aliyuncs.com",
		ConnectTimeout:      45 * time.Second,
		ReadWriteTimeout:    600 * time.Second,
		RetryMaxAttempts:    7,
		UploadPartSizeBytes: 128 * 1024 * 1024,
		UploadParallel:      4,
		UploadCheckpoint:    true,
		UploadCheckpointDir: "/tmp/vulnsky-cp",
	})
	if err != nil {
		t.Fatalf("NewOSSClient() error = %v", err)
	}
	official, ok := client.(*officialOSSClient)
	if !ok {
		t.Fatalf("NewOSSClient() type = %T, want *officialOSSClient", client)
	}
	if official.uploadPartSizeBytes != 128*1024*1024 ||
		official.uploadParallel != 4 ||
		!official.uploadCheckpoint ||
		official.uploadCheckpointDir != "/tmp/vulnsky-cp" {
		t.Fatalf("unexpected upload options: %#v", official)
	}
}

func TestFakeClientsSatisfyWorkflowInterfaces(t *testing.T) {
	ctx := context.Background()
	sts := &fakeSTS{accountID: "123456789", arn: "acs:ram::123456789:user/test"}
	oss := &fakeOSS{objects: []string{"images/lab.qcow2"}, location: "cn-hangzhou"}
	ecs := &fakeECS{
		instances: []Instance{{ID: "i-123", Name: "lab", Status: "Stopped", ImageID: "m-old"}},
		task:      TaskStatus{ID: "t-123", ResourceID: "m-new", Action: "ImportImage", Status: "Finished"},
	}

	var _ STSClient = sts
	var _ OSSClient = oss
	var _ ECSClient = ecs

	accountID, arn, err := sts.GetCallerIdentity(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if accountID != "123456789" || arn == "" {
		t.Fatalf("unexpected identity: %q %q", accountID, arn)
	}

	objects, err := oss.ListObjects(ctx, "bucket", "images/")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(objects, []string{"images/lab.qcow2"}) {
		t.Fatalf("unexpected objects: %#v", objects)
	}

	imageID, taskID, requestID, err := ecs.ImportImage(ctx, ImportImageInput{
		RegionID:     "cn-hangzhou",
		ImageName:    "vulnsky-lab",
		OSSBucket:    "bucket",
		OSSObject:    "images/lab.qcow2",
		Architecture: "x86_64",
		OSType:       "linux",
		Platform:     "Others Linux",
	})
	if err != nil {
		t.Fatal(err)
	}
	if imageID != "m-new" || taskID != "t-123" || requestID == "" {
		t.Fatalf("unexpected import result: %q %q %q", imageID, taskID, requestID)
	}

	diskID, requestID, err := ecs.ReplaceSystemDisk(ctx, "i-123", imageID)
	if err != nil {
		t.Fatal(err)
	}
	if diskID != "d-new" || requestID == "" {
		t.Fatalf("unexpected replace result: %q %q", diskID, requestID)
	}
}

func TestMapImagesIncludesImportOSSSource(t *testing.T) {
	resp := (&ecsapi.DescribeImagesResponse{}).SetBody(
		(&ecsapi.DescribeImagesResponseBody{}).SetImages(
			(&ecsapi.DescribeImagesResponseBodyImages{}).SetImage([]*ecsapi.DescribeImagesResponseBodyImagesImage{
				(&ecsapi.DescribeImagesResponseBodyImagesImage{}).
					SetImageId("m-imported").
					SetImageName("vulnsky-sample-lab").
					SetDiskDeviceMappings(
						(&ecsapi.DescribeImagesResponseBodyImagesImageDiskDeviceMappings{}).SetDiskDeviceMapping([]*ecsapi.DescribeImagesResponseBodyImagesImageDiskDeviceMappingsDiskDeviceMapping{
							(&ecsapi.DescribeImagesResponseBodyImagesImageDiskDeviceMappingsDiskDeviceMapping{}).
								SetImportOSSBucket("lab-bucket").
								SetImportOSSObject("qcow2/sample-lab.qcow2").
								SetFormat("qcow2"),
						}),
					),
			}),
		),
	)

	images := mapImages(resp)
	if len(images) != 1 {
		t.Fatalf("len(images) = %d, want 1", len(images))
	}
	if images[0].SourceOSSBucket != "lab-bucket" || images[0].SourceOSSObject != "qcow2/sample-lab.qcow2" || images[0].SourceFormat != "qcow2" {
		t.Fatalf("source not mapped: %#v", images[0])
	}
}

func TestDescribeImagesRequestIncludesAllImportStatuses(t *testing.T) {
	req := describeImagesRequest("cn-hangzhou", "self", 1)
	if req.Status == nil {
		t.Fatal("DescribeImages status filter is nil")
	}
	for _, want := range []string{"Creating", "Waiting", "Available", "UnAvailable", "CreateFailed", "Deprecated"} {
		if !strings.Contains(*req.Status, want) {
			t.Fatalf("DescribeImages status filter %q missing %q", *req.Status, want)
		}
	}
}

type fakeSTS struct {
	accountID string
	arn       string
	err       error
}

func (f *fakeSTS) GetCallerIdentity(context.Context) (string, string, error) {
	return f.accountID, f.arn, f.err
}

type fakeOSS struct {
	objects  []string
	location string
	err      error
}

func (f *fakeOSS) ListObjects(context.Context, string, string) ([]string, error) {
	return f.objects, f.err
}

func (f *fakeOSS) ObjectExists(context.Context, string, string) (bool, error) {
	return len(f.objects) > 0, f.err
}

func (f *fakeOSS) UploadFile(context.Context, string, string, string, func(done int64, total int64)) (string, error) {
	return "req-oss", f.err
}

func (f *fakeOSS) PresignGet(context.Context, string, string, time.Duration) (string, error) {
	return "https://example.com/signed", f.err
}

func (f *fakeOSS) GetBucketLocation(context.Context, string) (string, error) {
	return f.location, f.err
}

type fakeECS struct {
	instances []Instance
	task      TaskStatus
	err       error
}

func (f *fakeECS) ListInstances(context.Context) ([]Instance, error) {
	return f.instances, f.err
}

func (f *fakeECS) DescribeInstance(_ context.Context, instanceID string) (Instance, error) {
	for _, instance := range f.instances {
		if instance.ID == instanceID {
			return instance, f.err
		}
	}
	return Instance{}, ErrNotFound
}

func (f *fakeECS) ListImages(context.Context, string) ([]Image, error) {
	return nil, f.err
}

func (f *fakeECS) ImportImage(context.Context, ImportImageInput) (string, string, string, error) {
	return f.task.ResourceID, f.task.ID, "req-ecs", f.err
}

func (f *fakeECS) DescribeTask(context.Context, string) (TaskStatus, error) {
	return f.task, f.err
}

func (f *fakeECS) StopInstance(context.Context, string, bool) error {
	return f.err
}

func (f *fakeECS) StartInstance(context.Context, string) error {
	return f.err
}

func (f *fakeECS) ReplaceSystemDisk(context.Context, string, string) (string, string, error) {
	return "d-new", "req-replace", f.err
}
