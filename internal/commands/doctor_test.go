package commands

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vulnsky/internal/aliyun"
	"vulnsky/internal/config"
)

func TestDoctorPassesWithFakeClients(t *testing.T) {
	root := writeDoctorProfile(t, "class-a", "123456789", "cn-hangzhou", "cn-hangzhou", "i-lab")
	buf := new(bytes.Buffer)
	cmd := NewRootCommandWithOptions(RootOptions{
		RootDir: root,
		Factories: fakeFactories(
			"123456789",
			"cn-hangzhou",
			[]aliyun.Instance{{ID: "i-lab", Name: "lab", Status: "Stopped", ImageID: "m-old"}},
		),
	})
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"doctor"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v\n%s", err, buf.String())
	}
	out := buf.String()
	for _, want := range []string{
		"PASS profile 已加载: class-a",
		"PASS STS 账号有效: 123456789",
		"PASS OSS bucket 区域: cn-hangzhou",
		"PASS 默认 ECS 可用: i-lab lab Stopped",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("doctor output missing %q:\n%s", want, out)
		}
	}
}

func TestDoctorRedactsSensitiveOutput(t *testing.T) {
	root := writeDoctorProfile(t, "class-a", "123456789", "cn-hangzhou", "cn-hangzhou", "i-secret")
	buf := new(bytes.Buffer)
	cmd := NewRootCommandWithOptions(RootOptions{
		RootDir: root,
		Factories: fakeFactories(
			"123456789",
			"cn-hangzhou",
			[]aliyun.Instance{{ID: "i-secret", Name: "secret-instance", Status: "Stopped", ImageID: "m-old"}},
		),
	})
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"doctor", "--redact"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v\n%s", err, buf.String())
	}
	out := buf.String()
	for _, leaked := range []string{
		root,
		"123456789",
		"acs:ram",
		"i-secret",
		"secret-instance",
	} {
		if strings.Contains(out, leaked) {
			t.Fatalf("doctor --redact leaked %q:\n%s", leaked, out)
		}
	}
	if count := strings.Count(out, "<redacted>"); count < 5 {
		t.Fatalf("doctor --redact did not redact enough values, count=%d:\n%s", count, out)
	}
	if !strings.Contains(out, "PASS 默认 ECS 可用: <redacted> <redacted> Stopped") {
		t.Fatalf("doctor --redact missing redacted ECS status:\n%s", out)
	}
}

func TestDoctorFailsOnAccountMismatch(t *testing.T) {
	root := writeDoctorProfile(t, "class-a", "999999999", "cn-hangzhou", "cn-hangzhou", "i-lab")
	buf := new(bytes.Buffer)
	cmd := NewRootCommandWithOptions(RootOptions{
		RootDir: root,
		Factories: fakeFactories(
			"123456789",
			"cn-hangzhou",
			[]aliyun.Instance{{ID: "i-lab", Name: "lab", Status: "Stopped", ImageID: "m-old"}},
		),
	})
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"doctor"})

	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected account mismatch error\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "FAIL 账号不匹配") {
		t.Fatalf("doctor output missing account mismatch:\n%s", buf.String())
	}
}

func TestDoctorRedactsAccountMismatch(t *testing.T) {
	root := writeDoctorProfile(t, "class-a", "999999999", "cn-hangzhou", "cn-hangzhou", "i-lab")
	buf := new(bytes.Buffer)
	cmd := NewRootCommandWithOptions(RootOptions{
		RootDir: root,
		Factories: fakeFactories(
			"123456789",
			"cn-hangzhou",
			[]aliyun.Instance{{ID: "i-lab", Name: "lab", Status: "Stopped", ImageID: "m-old"}},
		),
	})
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"doctor", "--redact"})

	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected account mismatch error\n%s", buf.String())
	}
	out := buf.String()
	for _, leaked := range []string{"999999999", "123456789"} {
		if strings.Contains(out, leaked) {
			t.Fatalf("doctor --redact leaked account %q:\n%s", leaked, out)
		}
	}
	if !strings.Contains(out, "FAIL 账号不匹配: 期望=<redacted> 实际=<redacted>") {
		t.Fatalf("doctor --redact missing redacted mismatch:\n%s", out)
	}
}

func TestDoctorFailsOnRegionMismatch(t *testing.T) {
	root := writeDoctorProfile(t, "class-a", "123456789", "cn-beijing", "cn-hangzhou", "i-lab")
	buf := new(bytes.Buffer)
	cmd := NewRootCommandWithOptions(RootOptions{
		RootDir: root,
		Factories: fakeFactories(
			"123456789",
			"cn-hangzhou",
			[]aliyun.Instance{{ID: "i-lab", Name: "lab", Status: "Stopped", ImageID: "m-old"}},
		),
	})
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"doctor"})

	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected region mismatch error\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "FAIL ECS/OSS 配置区域不一致") {
		t.Fatalf("doctor output missing region mismatch:\n%s", buf.String())
	}
}

func writeDoctorProfile(t *testing.T, profile string, expectedAccountID string, ecsRegion string, ossRegion string, defaultInstanceID string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "profiles"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(root, ".env"), "VULNSKY_ACTIVE_PROFILE="+profile+"\nVULNSKY_PROFILE_DIR=./profiles\n")
	mustWriteFile(t, filepath.Join(root, "profiles", profile+".env"), ""+
		"VULNSKY_PROFILE_LABEL=三班\n"+
		"VULNSKY_EXPECTED_ACCOUNT_ID="+expectedAccountID+"\n"+
		"ALIBABA_CLOUD_ACCESS_KEY_ID=ak\n"+
		"ALIBABA_CLOUD_ACCESS_KEY_SECRET=secret\n"+
		"ALIBABA_CLOUD_REGION_ID="+ecsRegion+"\n"+
		"ALIBABA_OSS_REGION_ID="+ossRegion+"\n"+
		"ALIBABA_OSS_ENDPOINT=https://oss-"+ossRegion+".aliyuncs.com\n"+
		"ALIBABA_OSS_BUCKET=lab-bucket\n"+
		"VULNSKY_DEFAULT_ECS_INSTANCE_ID="+defaultInstanceID+"\n")
	return root
}

func writeECSOnlyProfile(t *testing.T, profile string, expectedAccountID string, ecsRegion string, defaultInstanceID string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "profiles"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(root, ".env"), "VULNSKY_ACTIVE_PROFILE="+profile+"\nVULNSKY_PROFILE_DIR=./profiles\n")
	mustWriteFile(t, filepath.Join(root, "profiles", profile+".env"), ""+
		"VULNSKY_EXPECTED_ACCOUNT_ID="+expectedAccountID+"\n"+
		"ALIBABA_CLOUD_ACCESS_KEY_ID=ak\n"+
		"ALIBABA_CLOUD_ACCESS_KEY_SECRET=secret\n"+
		"ALIBABA_CLOUD_REGION_ID="+ecsRegion+"\n"+
		"VULNSKY_DEFAULT_ECS_INSTANCE_ID="+defaultInstanceID+"\n")
	return root
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fakeFactories(accountID string, ossLocation string, instances []aliyun.Instance) ClientFactories {
	return ClientFactories{
		NewSTS: func(config.Config) (aliyun.STSClient, error) {
			return commandFakeSTS{accountID: accountID, arn: "acs:ram::" + accountID + ":user/test"}, nil
		},
		NewOSS: func(config.Config) (aliyun.OSSClient, error) {
			return commandFakeOSS{location: ossLocation}, nil
		},
		NewECS: func(config.Config) (aliyun.ECSClient, error) {
			return commandFakeECS{instances: instances}, nil
		},
	}
}

type commandFakeSTS struct {
	accountID string
	arn       string
}

func (f commandFakeSTS) GetCallerIdentity(context.Context) (string, string, error) {
	return f.accountID, f.arn, nil
}

type commandFakeOSS struct {
	location  string
	objects   []string
	signedURL string
	exists    bool
}

func (f commandFakeOSS) ListObjects(context.Context, string, string) ([]string, error) {
	return f.objects, nil
}

func (f commandFakeOSS) ObjectExists(context.Context, string, string) (bool, error) {
	return f.exists, nil
}

func (f commandFakeOSS) UploadFile(_ context.Context, _ string, _ string, _ string, onProgress func(done int64, total int64)) (string, error) {
	if onProgress != nil {
		onProgress(5*1024*1024, 10*1024*1024)
		onProgress(10*1024*1024, 10*1024*1024)
	}
	return "", nil
}

func (f commandFakeOSS) PresignGet(context.Context, string, string, time.Duration) (string, error) {
	return f.signedURL, nil
}

func (f commandFakeOSS) GetBucketLocation(context.Context, string) (string, error) {
	return f.location, nil
}

type commandFakeECS struct {
	instances       []aliyun.Instance
	images          []aliyun.Image
	importImageID   string
	importTaskID    string
	importRequestID string
	task            aliyun.TaskStatus
}

func (f commandFakeECS) ListInstances(context.Context) ([]aliyun.Instance, error) {
	return f.instances, nil
}

func (f commandFakeECS) DescribeInstance(_ context.Context, instanceID string) (aliyun.Instance, error) {
	for _, instance := range f.instances {
		if instance.ID == instanceID {
			return instance, nil
		}
	}
	return aliyun.Instance{}, aliyun.ErrNotFound
}

func (f commandFakeECS) ListImages(context.Context, string) ([]aliyun.Image, error) {
	return f.images, nil
}

func (f commandFakeECS) ImportImage(context.Context, aliyun.ImportImageInput) (string, string, string, error) {
	return f.importImageID, f.importTaskID, f.importRequestID, nil
}

func (f commandFakeECS) DescribeTask(context.Context, string) (aliyun.TaskStatus, error) {
	return f.task, nil
}

func (f commandFakeECS) StopInstance(context.Context, string, bool) error {
	return nil
}

func (f commandFakeECS) StartInstance(context.Context, string) error {
	return nil
}

func (f commandFakeECS) ReplaceSystemDisk(context.Context, string, string) (string, string, error) {
	return "", "", nil
}
