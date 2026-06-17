package commands

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vulnsky/internal/aliyun"
	"vulnsky/internal/config"
	"vulnsky/internal/store"
	"vulnsky/internal/util"
)

func TestOSSListPrintsObjects(t *testing.T) {
	root := writeDoctorProfile(t, "class-a", "123456789", "cn-hangzhou", "cn-hangzhou", "")
	buf := new(bytes.Buffer)
	cmd := NewRootCommandWithOptions(RootOptions{
		RootDir: root,
		Factories: ClientFactories{
			NewOSS: func(config.Config) (aliyun.OSSClient, error) {
				return commandFakeOSS{objects: []string{"images/lab.qcow2", "images/readme.txt"}}, nil
			},
		},
	})
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"oss", "ls", "images/"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "images/lab.qcow2") || !strings.Contains(out, "images/readme.txt") {
		t.Fatalf("oss ls output missing objects:\n%s", out)
	}
}

func TestOSSLinkPrintsPresignedURL(t *testing.T) {
	root := writeDoctorProfile(t, "class-a", "123456789", "cn-hangzhou", "cn-hangzhou", "")
	buf := new(bytes.Buffer)
	cmd := NewRootCommandWithOptions(RootOptions{
		RootDir: root,
		Factories: ClientFactories{
			NewOSS: func(config.Config) (aliyun.OSSClient, error) {
				return commandFakeOSS{signedURL: "https://example.com/signed"}, nil
			},
		},
	})
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"oss", "link", "images/lab.qcow2", "--expires", "30m"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(buf.String(), "https://example.com/signed") {
		t.Fatalf("oss link output missing url:\n%s", buf.String())
	}
}

func TestOSSUploadStoresRecord(t *testing.T) {
	root := writeDoctorProfile(t, "class-a", "123456789", "cn-hangzhou", "cn-hangzhou", "")
	qcow2 := filepath.Join(root, "lab.qcow2")
	if err := os.WriteFile(qcow2, []byte("vulnsky"), 0o644); err != nil {
		t.Fatal(err)
	}
	buf := new(bytes.Buffer)
	cmd := NewRootCommandWithOptions(RootOptions{
		RootDir: root,
		Factories: ClientFactories{
			NewSTS: func(config.Config) (aliyun.STSClient, error) {
				return commandFakeSTS{accountID: "123456789", arn: "acs:ram::123456789:user/test"}, nil
			},
			NewOSS: func(config.Config) (aliyun.OSSClient, error) {
				return commandFakeOSS{}, nil
			},
		},
	})
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"oss", "upload", qcow2, "--key", "images/lab.qcow2"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v\n%s", err, buf.String())
	}
	if !strings.Contains(buf.String(), "uploaded images/lab.qcow2") {
		t.Fatalf("oss upload output missing object key:\n%s", buf.String())
	}

	sum, _, err := util.FileSHA256(qcow2)
	if err != nil {
		t.Fatal(err)
	}
	st := store.New(filepath.Join(root, "vulnsky.db"))
	record, err := st.FindUploadedBySHA256("class-a", "123456789", "cn-hangzhou", sum)
	if err != nil {
		t.Fatal(err)
	}
	if record == nil || record.ObjectKey != "images/lab.qcow2" {
		t.Fatalf("upload record not found or wrong: %#v", record)
	}
}

func TestOSSUploadPrintsProgress(t *testing.T) {
	root := writeDoctorProfile(t, "class-a", "123456789", "cn-hangzhou", "cn-hangzhou", "")
	qcow2 := filepath.Join(root, "lab.qcow2")
	if err := os.WriteFile(qcow2, []byte("vulnsky"), 0o644); err != nil {
		t.Fatal(err)
	}
	buf := new(bytes.Buffer)
	cmd := NewRootCommandWithOptions(RootOptions{
		RootDir: root,
		Factories: ClientFactories{
			NewSTS: func(config.Config) (aliyun.STSClient, error) {
				return commandFakeSTS{accountID: "123456789", arn: "acs:ram::123456789:user/test"}, nil
			},
			NewOSS: func(config.Config) (aliyun.OSSClient, error) {
				return commandFakeOSS{}, nil
			},
		},
	})
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"oss", "upload", qcow2, "--key", "images/lab.qcow2"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v\n%s", err, buf.String())
	}
	out := buf.String()
	if !strings.Contains(out, "upload progress 100.0%") {
		t.Fatalf("oss upload output missing progress:\n%s", out)
	}
}

func TestECSCurrentImageUsesDefaultInstance(t *testing.T) {
	root := writeDoctorProfile(t, "class-a", "123456789", "cn-hangzhou", "cn-hangzhou", "i-lab")
	buf := new(bytes.Buffer)
	cmd := NewRootCommandWithOptions(RootOptions{
		RootDir: root,
		Factories: ClientFactories{
			NewECS: func(config.Config) (aliyun.ECSClient, error) {
				return commandFakeECS{instances: []aliyun.Instance{{ID: "i-lab", Name: "lab", Status: "Running", ImageID: "m-current"}}}, nil
			},
		},
	})
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"ecs", "current-image"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(buf.String(), "m-current") {
		t.Fatalf("ecs current-image output missing image id:\n%s", buf.String())
	}
}

func TestECSUseUpdatesProfileDefault(t *testing.T) {
	root := writeDoctorProfile(t, "class-a", "123456789", "cn-hangzhou", "cn-hangzhou", "")
	buf := new(bytes.Buffer)
	cmd := NewRootCommandWithOptions(RootOptions{
		RootDir: root,
		Factories: ClientFactories{
			NewECS: func(config.Config) (aliyun.ECSClient, error) {
				return commandFakeECS{instances: []aliyun.Instance{{ID: "i-new", Name: "lab", Status: "Stopped", ImageID: "m-current"}}}, nil
			},
		},
	})
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"ecs", "use", "i-new"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "profiles", "class-a.env"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "VULNSKY_DEFAULT_ECS_INSTANCE_ID=i-new") {
		t.Fatalf("profile default ECS was not updated:\n%s", string(data))
	}
}

func TestImageImportStoresTask(t *testing.T) {
	root := writeDoctorProfile(t, "class-a", "123456789", "cn-hangzhou", "cn-hangzhou", "")
	buf := new(bytes.Buffer)
	cmd := NewRootCommandWithOptions(RootOptions{
		RootDir: root,
		Factories: ClientFactories{
			NewSTS: func(config.Config) (aliyun.STSClient, error) {
				return commandFakeSTS{accountID: "123456789", arn: "acs:ram::123456789:user/test"}, nil
			},
			NewECS: func(config.Config) (aliyun.ECSClient, error) {
				return commandFakeECS{importImageID: "m-new", importTaskID: "t-123", importRequestID: "req-import"}, nil
			},
		},
	})
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"image", "import", "images/lab.qcow2", "--name", "vulnsky-lab"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v\n%s", err, buf.String())
	}
	if !strings.Contains(buf.String(), "image=m-new task=t-123") {
		t.Fatalf("image import output missing ids:\n%s", buf.String())
	}
}

func TestImageImportRequiresOSSBucket(t *testing.T) {
	root := writeECSOnlyProfile(t, "class-a", "123456789", "cn-hangzhou", "")
	buf := new(bytes.Buffer)
	cmd := NewRootCommandWithOptions(RootOptions{
		RootDir: root,
		Factories: ClientFactories{
			NewSTS: func(config.Config) (aliyun.STSClient, error) {
				return commandFakeSTS{accountID: "123456789", arn: "acs:ram::123456789:user/test"}, nil
			},
			NewECS: func(config.Config) (aliyun.ECSClient, error) {
				return commandFakeECS{}, nil
			},
		},
	})
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"image", "import", "qcow2/lab.qcow2"})

	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "missing ALIBABA_OSS_BUCKET") {
		t.Fatalf("Execute() error = %v, want missing bucket\n%s", err, buf.String())
	}
}

func TestImageListMarksVulnSkyDeployments(t *testing.T) {
	root := writeDoctorProfile(t, "class-a", "123456789", "cn-hangzhou", "cn-hangzhou", "i-lab")
	st := store.New(filepath.Join(root, "vulnsky.db"))
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := st.InsertDeployment(store.DeploymentRecord{
		ProfileName:     "class-a",
		AccountID:       "123456789",
		RegionID:        "cn-hangzhou",
		InstanceID:      "i-lab",
		InstanceName:    "lab",
		SourceUploadID:  0,
		SourceImageID:   "m-deployed",
		PreviousImageID: "m-old",
		NewDiskID:       "d-new",
		StopMode:        "force",
		Status:          "reimaged",
		RequestID:       "req-replace",
	}); err != nil {
		t.Fatal(err)
	}

	buf := new(bytes.Buffer)
	cmd := NewRootCommandWithOptions(RootOptions{
		RootDir: root,
		Factories: ClientFactories{
			NewSTS: func(config.Config) (aliyun.STSClient, error) {
				return commandFakeSTS{accountID: "123456789", arn: "acs:ram::123456789:user/test"}, nil
			},
			NewECS: func(config.Config) (aliyun.ECSClient, error) {
				return commandFakeECS{images: []aliyun.Image{
					{ID: "m-deployed", Name: "vulnsky-sample-lab", Status: "Available", Progress: "100%", CreationTime: "2026-06-06T10:14Z"},
					{ID: "m-other", Name: "manual-image", Status: "Available", Progress: "100%", CreationTime: "2026-06-06T09:00Z"},
				}}, nil
			},
		},
	})
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"image", "ls"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v\n%s", err, buf.String())
	}
	out := buf.String()
	if !strings.Contains(out, "m-deployed\tvulnsky-sample-lab\tAvailable\t100%\t2026-06-06T10:14Z\tdeployed\ti-lab") {
		t.Fatalf("deployed image was not marked:\n%s", out)
	}
	if !strings.Contains(out, "m-other\tmanual-image\tAvailable\t100%\t2026-06-06T09:00Z\t-\t-") {
		t.Fatalf("manual image marker is wrong:\n%s", out)
	}
}

func TestImageSourcePrintsImportedQCOW2(t *testing.T) {
	root := writeDoctorProfile(t, "class-a", "123456789", "cn-hangzhou", "cn-hangzhou", "i-lab")
	buf := new(bytes.Buffer)
	cmd := NewRootCommandWithOptions(RootOptions{
		RootDir: root,
		Factories: ClientFactories{
			NewECS: func(config.Config) (aliyun.ECSClient, error) {
				return commandFakeECS{images: []aliyun.Image{
					{
						ID:              "m-imported",
						Name:            "vulnsky-sample-lab",
						Status:          "Available",
						SourceOSSBucket: "lab-bucket",
						SourceOSSObject: "qcow2/sample-lab.qcow2",
						SourceFormat:    "qcow2",
					},
				}}, nil
			},
		},
	})
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"image", "source", "m-imported"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v\n%s", err, buf.String())
	}
	out := buf.String()
	for _, want := range []string{
		"ImageId=m-imported",
		"ImageName=vulnsky-sample-lab",
		"SourceOSS=oss://lab-bucket/qcow2/sample-lab.qcow2",
		"SourceObject=qcow2/sample-lab.qcow2",
		"SourceFormat=qcow2",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("image source output missing %q:\n%s", want, out)
		}
	}
}

func TestDeployImportsObjectAndReimagesDefaultECS(t *testing.T) {
	root := writeDoctorProfile(t, "class-a", "123456789", "cn-hangzhou", "cn-hangzhou", "i-lab")
	fakeECS := &deployFakeECS{
		instance:      aliyun.Instance{ID: "i-lab", Name: "lab", Status: "Running", ImageID: "m-old"},
		task:          aliyun.TaskStatus{ID: "t-123", ResourceID: "m-new", Action: "ImportImage", Status: "Finished"},
		startFailures: 1,
	}
	buf := new(bytes.Buffer)
	cmd := NewRootCommandWithOptions(RootOptions{
		RootDir: root,
		Factories: ClientFactories{
			NewSTS: func(config.Config) (aliyun.STSClient, error) {
				return commandFakeSTS{accountID: "123456789", arn: "acs:ram::123456789:user/test"}, nil
			},
			NewOSS: func(config.Config) (aliyun.OSSClient, error) {
				return commandFakeOSS{exists: true}, nil
			},
			NewECS: func(config.Config) (aliyun.ECSClient, error) {
				return fakeECS, nil
			},
		},
	})
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"deploy", "qcow2/sample-lab.qcow2",
		"--image-name", "vulnsky-sample-lab",
		"--poll-interval", "1ms",
		"--import-timeout", "100ms",
		"--stop-timeout", "100ms",
		"--start-timeout", "100ms",
		"--force-stop",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v\n%s", err, buf.String())
	}
	out := buf.String()
	for _, want := range []string{
		"[oss] found qcow2/sample-lab.qcow2",
		"[image] import started image=m-new task=t-123",
		"[ecs] replace system disk disk=d-new",
		"[done] instance=i-lab image=m-new status=Running",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("deploy output missing %q:\n%s", want, out)
		}
	}
	if !fakeECS.stopped || !fakeECS.replaced || !fakeECS.started || !fakeECS.forceStop || fakeECS.startCalls != 2 {
		t.Fatalf("deploy did not run expected ECS actions: %#v", fakeECS)
	}
	if strings.Contains(out, "SDKError") || !strings.Contains(out, "[ecs] start not accepted yet; retrying") {
		t.Fatalf("deploy retry log is too noisy:\n%s", out)
	}
}

func TestDeployReusesFinishedImageImportWhenImageStillExists(t *testing.T) {
	root := writeDoctorProfile(t, "class-a", "123456789", "cn-hangzhou", "cn-hangzhou", "i-lab")
	st := store.New(filepath.Join(root, "vulnsky.db"))
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := st.InsertImageImport(store.ImageImportRecord{
		ProfileName:  "class-a",
		AccountID:    "123456789",
		RegionID:     "cn-hangzhou",
		OSSBucket:    "lab-bucket",
		OSSObject:    "qcow2/sample-lab.qcow2",
		ImageName:    "vulnsky-sample-lab",
		ImageID:      "m-reused",
		TaskID:       "t-reused",
		TaskStatus:   "Finished",
		Platform:     "Others Linux",
		Architecture: "x86_64",
		OSType:       "linux",
	}); err != nil {
		t.Fatal(err)
	}

	fakeECS := &deployFakeECS{
		instance: aliyun.Instance{ID: "i-lab", Name: "lab", Status: "Running", ImageID: "m-old"},
		images: []aliyun.Image{
			{
				ID:              "m-reused",
				Name:            "vulnsky-sample-lab",
				Status:          "Available",
				SourceOSSBucket: "lab-bucket",
				SourceOSSObject: "qcow2/sample-lab.qcow2",
			},
		},
	}
	buf := new(bytes.Buffer)
	cmd := NewRootCommandWithOptions(RootOptions{
		RootDir: root,
		Factories: ClientFactories{
			NewSTS: func(config.Config) (aliyun.STSClient, error) {
				return commandFakeSTS{accountID: "123456789", arn: "acs:ram::123456789:user/test"}, nil
			},
			NewOSS: func(config.Config) (aliyun.OSSClient, error) {
				return commandFakeOSS{exists: true}, nil
			},
			NewECS: func(config.Config) (aliyun.ECSClient, error) {
				return fakeECS, nil
			},
		},
	})
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"deploy", "qcow2/sample-lab.qcow2",
		"--poll-interval", "1ms",
		"--import-timeout", "100ms",
		"--stop-timeout", "100ms",
		"--start-timeout", "100ms",
		"--force-stop",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v\n%s", err, buf.String())
	}
	out := buf.String()
	if !strings.Contains(out, "[image] reused image=m-reused object=qcow2/sample-lab.qcow2 source=history status=Available") {
		t.Fatalf("deploy did not report image reuse:\n%s", out)
	}
	if strings.Contains(out, "[image] import started") || fakeECS.importCalls != 0 {
		t.Fatalf("deploy imported instead of reusing image, importCalls=%d:\n%s", fakeECS.importCalls, out)
	}
	if !strings.Contains(out, "[done] instance=i-lab image=m-reused status=Running") {
		t.Fatalf("deploy did not reimage with reused image:\n%s", out)
	}
}

func TestDeployReusesECSImageByOSSObjectWithoutLocalHistory(t *testing.T) {
	root := writeDoctorProfile(t, "class-a", "123456789", "cn-hangzhou", "cn-hangzhou", "i-lab")
	fakeECS := &deployFakeECS{
		instance: aliyun.Instance{ID: "i-lab", Name: "lab", Status: "Running", ImageID: "m-old"},
		images: []aliyun.Image{
			{
				ID:              "m-from-ecs",
				Name:            "vulnsky-sample-lab",
				Status:          "Available",
				SourceOSSBucket: "lab-bucket",
				SourceOSSObject: "qcow2/sample-lab.qcow2",
			},
		},
	}
	buf := new(bytes.Buffer)
	cmd := NewRootCommandWithOptions(RootOptions{
		RootDir: root,
		Factories: ClientFactories{
			NewSTS: func(config.Config) (aliyun.STSClient, error) {
				return commandFakeSTS{accountID: "123456789", arn: "acs:ram::123456789:user/test"}, nil
			},
			NewOSS: func(config.Config) (aliyun.OSSClient, error) {
				return commandFakeOSS{exists: true}, nil
			},
			NewECS: func(config.Config) (aliyun.ECSClient, error) {
				return fakeECS, nil
			},
		},
	})
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"deploy", "qcow2/sample-lab.qcow2",
		"--poll-interval", "1ms",
		"--import-timeout", "100ms",
		"--stop-timeout", "100ms",
		"--start-timeout", "100ms",
		"--force-stop",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v\n%s", err, buf.String())
	}
	out := buf.String()
	if !strings.Contains(out, "[image] reused image=m-from-ecs object=qcow2/sample-lab.qcow2 source=ecs status=Available") {
		t.Fatalf("deploy did not report ECS image reuse:\n%s", out)
	}
	if strings.Contains(out, "[image] import started") || fakeECS.importCalls != 0 {
		t.Fatalf("deploy imported instead of reusing ECS image, importCalls=%d:\n%s", fakeECS.importCalls, out)
	}

	st := store.New(filepath.Join(root, "vulnsky.db"))
	found, err := st.FindFinishedImageImportByOSSObject("class-a", "123456789", "cn-hangzhou", "lab-bucket", "qcow2/sample-lab.qcow2")
	if err != nil {
		t.Fatal(err)
	}
	if found == nil || found.ImageID != "m-from-ecs" || found.TaskStatus != "Reused" {
		t.Fatalf("ECS image reuse was not recorded locally: %#v", found)
	}
}

func TestDeployReimportsWhenRecordedImageWasDeleted(t *testing.T) {
	root := writeDoctorProfile(t, "class-a", "123456789", "cn-hangzhou", "cn-hangzhou", "i-lab")
	st := store.New(filepath.Join(root, "vulnsky.db"))
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := st.InsertImageImport(store.ImageImportRecord{
		ProfileName:  "class-a",
		AccountID:    "123456789",
		RegionID:     "cn-hangzhou",
		OSSBucket:    "lab-bucket",
		OSSObject:    "qcow2/sample-lab.qcow2",
		ImageName:    "vulnsky-sample-lab",
		ImageID:      "m-deleted",
		TaskID:       "t-deleted",
		TaskStatus:   "Finished",
		Platform:     "Others Linux",
		Architecture: "x86_64",
		OSType:       "linux",
	}); err != nil {
		t.Fatal(err)
	}

	fakeECS := &deployFakeECS{
		instance: aliyun.Instance{ID: "i-lab", Name: "lab", Status: "Running", ImageID: "m-old"},
		task:     aliyun.TaskStatus{ID: "t-123", ResourceID: "m-new", Action: "ImportImage", Status: "Finished"},
	}
	buf := new(bytes.Buffer)
	cmd := NewRootCommandWithOptions(RootOptions{
		RootDir: root,
		Factories: ClientFactories{
			NewSTS: func(config.Config) (aliyun.STSClient, error) {
				return commandFakeSTS{accountID: "123456789", arn: "acs:ram::123456789:user/test"}, nil
			},
			NewOSS: func(config.Config) (aliyun.OSSClient, error) {
				return commandFakeOSS{exists: true}, nil
			},
			NewECS: func(config.Config) (aliyun.ECSClient, error) {
				return fakeECS, nil
			},
		},
	})
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"deploy", "qcow2/sample-lab.qcow2",
		"--poll-interval", "1ms",
		"--import-timeout", "100ms",
		"--stop-timeout", "100ms",
		"--start-timeout", "100ms",
		"--force-stop",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v\n%s", err, buf.String())
	}
	out := buf.String()
	if !strings.Contains(out, "[image] recorded image=m-deleted object=qcow2/sample-lab.qcow2 not available; importing again") {
		t.Fatalf("deploy did not report stale image history:\n%s", out)
	}
	if fakeECS.importCalls != 1 || fakeECS.importOSSObject != "qcow2/sample-lab.qcow2" {
		t.Fatalf("deploy did not import expected object, fakeECS=%#v\n%s", fakeECS, out)
	}
	if !strings.Contains(out, "[image] import started image=m-new task=t-123") {
		t.Fatalf("deploy did not start replacement import:\n%s", out)
	}
}

func TestDeployUploadsLocalQCOW2BeforeImport(t *testing.T) {
	root := writeDoctorProfile(t, "class-a", "123456789", "cn-hangzhou", "cn-hangzhou", "i-lab")
	profilePath := filepath.Join(root, "profiles", "class-a.env")
	f, err := os.OpenFile(profilePath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("VULNSKY_DEFAULT_OBJECT_PREFIX=qcow2/\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	localQCOW2 := filepath.Join(root, "sample-lab.qcow2")
	if err := os.WriteFile(localQCOW2, []byte("sample"), 0o644); err != nil {
		t.Fatal(err)
	}

	fakeOSS := &deployUploadFakeOSS{}
	fakeECS := &deployFakeECS{
		instance: aliyun.Instance{ID: "i-lab", Name: "lab", Status: "Stopped", ImageID: "m-old"},
		task:     aliyun.TaskStatus{ID: "t-123", ResourceID: "m-new", Action: "ImportImage", Status: "Finished"},
	}
	buf := new(bytes.Buffer)
	cmd := NewRootCommandWithOptions(RootOptions{
		RootDir: root,
		Factories: ClientFactories{
			NewSTS: func(config.Config) (aliyun.STSClient, error) {
				return commandFakeSTS{accountID: "123456789", arn: "acs:ram::123456789:user/test"}, nil
			},
			NewOSS: func(config.Config) (aliyun.OSSClient, error) {
				return fakeOSS, nil
			},
			NewECS: func(config.Config) (aliyun.ECSClient, error) {
				return fakeECS, nil
			},
		},
	})
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"deploy", localQCOW2,
		"--poll-interval", "1ms",
		"--import-timeout", "100ms",
		"--stop-timeout", "100ms",
		"--start-timeout", "100ms",
		"--force-stop",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v\n%s", err, buf.String())
	}
	if fakeOSS.uploadedKey != "qcow2/sample-lab.qcow2" || fakeOSS.uploadedPath != localQCOW2 {
		t.Fatalf("unexpected upload: key=%q path=%q", fakeOSS.uploadedKey, fakeOSS.uploadedPath)
	}
	if fakeECS.importOSSObject != "qcow2/sample-lab.qcow2" {
		t.Fatalf("ImportImage used wrong object: %q", fakeECS.importOSSObject)
	}
	if !strings.Contains(buf.String(), "[oss] uploaded qcow2/sample-lab.qcow2") {
		t.Fatalf("deploy output missing upload log:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "[oss] upload progress 100.0%") {
		t.Fatalf("deploy output missing upload progress:\n%s", buf.String())
	}
}

func TestECSStartRetriesUntilRunning(t *testing.T) {
	root := writeDoctorProfile(t, "class-a", "123456789", "cn-hangzhou", "cn-hangzhou", "i-lab")
	fakeECS := &deployFakeECS{
		instance:      aliyun.Instance{ID: "i-lab", Name: "lab", Status: "Stopped", ImageID: "m-new"},
		startFailures: 1,
	}
	buf := new(bytes.Buffer)
	cmd := NewRootCommandWithOptions(RootOptions{
		RootDir: root,
		Factories: ClientFactories{
			NewECS: func(config.Config) (aliyun.ECSClient, error) {
				return fakeECS, nil
			},
		},
	})
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"ecs", "start", "--poll-interval", "1ms", "--timeout", "100ms"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v\n%s", err, buf.String())
	}
	if fakeECS.startCalls != 2 || fakeECS.instance.Status != "Running" {
		t.Fatalf("start did not retry until running: %#v", fakeECS)
	}
	if !strings.Contains(buf.String(), "[done] instance=i-lab status=Running") {
		t.Fatalf("start output missing completion:\n%s", buf.String())
	}
	if strings.Contains(buf.String(), "SDKError") || !strings.Contains(buf.String(), "[ecs] start not accepted yet; retrying") {
		t.Fatalf("start retry log is too noisy:\n%s", buf.String())
	}
}

func TestECSReimageUsesExistingImageWithoutOSS(t *testing.T) {
	root := writeECSOnlyProfile(t, "class-a", "123456789", "cn-hangzhou", "i-lab")
	fakeECS := &deployFakeECS{
		instance: aliyun.Instance{ID: "i-lab", Name: "lab", Status: "Running", ImageID: "m-old"},
	}
	ossCalled := false
	buf := new(bytes.Buffer)
	cmd := NewRootCommandWithOptions(RootOptions{
		RootDir: root,
		Factories: ClientFactories{
			NewSTS: func(config.Config) (aliyun.STSClient, error) {
				return commandFakeSTS{accountID: "123456789", arn: "acs:ram::123456789:user/test"}, nil
			},
			NewOSS: func(config.Config) (aliyun.OSSClient, error) {
				ossCalled = true
				return commandFakeOSS{}, nil
			},
			NewECS: func(config.Config) (aliyun.ECSClient, error) {
				return fakeECS, nil
			},
		},
	})
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"ecs", "reimage", "m-existing",
		"--poll-interval", "1ms",
		"--stop-timeout", "100ms",
		"--start-timeout", "100ms",
		"--force-stop",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v\n%s", err, buf.String())
	}
	if ossCalled {
		t.Fatal("ecs reimage should not initialize OSS client")
	}
	out := buf.String()
	for _, want := range []string{
		"[ecs] target instance=i-lab name=lab status=Running image=m-old",
		"[image] using existing image=m-existing",
		"[ecs] replace system disk disk=d-new",
		"[done] instance=i-lab image=m-existing status=Running",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("ecs reimage output missing %q:\n%s", want, out)
		}
	}
	if !fakeECS.stopped || !fakeECS.replaced || !fakeECS.started || !fakeECS.forceStop {
		t.Fatalf("ecs reimage did not run expected ECS actions: %#v", fakeECS)
	}
}

func TestDeployExistingImageDoesNotRequireOSS(t *testing.T) {
	root := writeECSOnlyProfile(t, "class-a", "123456789", "cn-hangzhou", "i-lab")
	fakeECS := &deployFakeECS{
		instance: aliyun.Instance{ID: "i-lab", Name: "lab", Status: "Running", ImageID: "m-old"},
	}
	ossCalled := false
	buf := new(bytes.Buffer)
	cmd := NewRootCommandWithOptions(RootOptions{
		RootDir: root,
		Factories: ClientFactories{
			NewSTS: func(config.Config) (aliyun.STSClient, error) {
				return commandFakeSTS{accountID: "123456789", arn: "acs:ram::123456789:user/test"}, nil
			},
			NewOSS: func(config.Config) (aliyun.OSSClient, error) {
				ossCalled = true
				return commandFakeOSS{}, nil
			},
			NewECS: func(config.Config) (aliyun.ECSClient, error) {
				return fakeECS, nil
			},
		},
	})
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"deploy", "--image-id", "m-existing",
		"--poll-interval", "1ms",
		"--stop-timeout", "100ms",
		"--start-timeout", "100ms",
		"--force-stop",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v\n%s", err, buf.String())
	}
	if ossCalled {
		t.Fatal("deploy --image-id should not initialize OSS client")
	}
	if !strings.Contains(buf.String(), "[done] instance=i-lab image=m-existing status=Running") {
		t.Fatalf("deploy --image-id output missing completion:\n%s", buf.String())
	}
}

func TestRecordsDeploymentsShowsLocalHistory(t *testing.T) {
	root := writeECSOnlyProfile(t, "class-a", "123456789", "cn-hangzhou", "i-lab")
	st := store.New(filepath.Join(root, "vulnsky.db"))
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := st.InsertDeployment(store.DeploymentRecord{
		ProfileName:     "class-a",
		AccountID:       "123456789",
		RegionID:        "cn-hangzhou",
		InstanceID:      "i-lab",
		InstanceName:    "lab",
		SourceUploadID:  0,
		SourceImageID:   "m-new",
		PreviousImageID: "m-old",
		NewDiskID:       "d-new",
		StopMode:        "force",
		Status:          "reimaged",
		RequestID:       "req-replace",
	}); err != nil {
		t.Fatal(err)
	}

	buf := new(bytes.Buffer)
	cmd := NewRootCommandWithOptions(RootOptions{RootDir: root})
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"records", "deployments", "--limit", "5"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v\n%s", err, buf.String())
	}
	out := buf.String()
	if !strings.Contains(out, "ID\tINSTANCE_ID\tSOURCE_IMAGE_ID\tPREVIOUS_IMAGE_ID\tDISK_ID\tSTATUS") {
		t.Fatalf("records deployments output missing header:\n%s", out)
	}
	if !strings.Contains(out, "i-lab\tm-new\tm-old\td-new\treimaged") {
		t.Fatalf("records deployments output missing deployment:\n%s", out)
	}
}

func TestRecordsImagesShowsOSSObject(t *testing.T) {
	root := writeECSOnlyProfile(t, "class-a", "123456789", "cn-hangzhou", "i-lab")
	st := store.New(filepath.Join(root, "vulnsky.db"))
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := st.InsertImageImport(store.ImageImportRecord{
		ProfileName:  "class-a",
		AccountID:    "123456789",
		RegionID:     "cn-hangzhou",
		OSSBucket:    "lab-bucket",
		OSSObject:    "qcow2/sample-lab.qcow2",
		ImageName:    "vulnsky-sample-lab",
		ImageID:      "m-new",
		TaskID:       "t-123",
		TaskStatus:   "Finished",
		Platform:     "Others Linux",
		Architecture: "x86_64",
		OSType:       "linux",
		RequestID:    "req-import",
	}); err != nil {
		t.Fatal(err)
	}

	buf := new(bytes.Buffer)
	cmd := NewRootCommandWithOptions(RootOptions{RootDir: root})
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"records", "images", "--limit", "5"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v\n%s", err, buf.String())
	}
	out := buf.String()
	if !strings.Contains(out, "ID\tIMAGE_ID\tIMAGE_NAME\tOSS_OBJECT\tTASK_ID\tSTATUS") {
		t.Fatalf("records images output missing header:\n%s", out)
	}
	if !strings.Contains(out, "m-new\tvulnsky-sample-lab\tqcow2/sample-lab.qcow2\tt-123\tFinished") {
		t.Fatalf("records images output missing OSS object:\n%s", out)
	}
}

func TestECSReimageRecordsReplaceBeforeStartFailure(t *testing.T) {
	root := writeECSOnlyProfile(t, "class-a", "123456789", "cn-hangzhou", "i-lab")
	fakeECS := &deployFakeECS{
		instance:      aliyun.Instance{ID: "i-lab", Name: "lab", Status: "Stopped", ImageID: "m-old"},
		startFailures: 100,
	}
	buf := new(bytes.Buffer)
	cmd := NewRootCommandWithOptions(RootOptions{
		RootDir: root,
		Factories: ClientFactories{
			NewSTS: func(config.Config) (aliyun.STSClient, error) {
				return commandFakeSTS{accountID: "123456789", arn: "acs:ram::123456789:user/test"}, nil
			},
			NewECS: func(config.Config) (aliyun.ECSClient, error) {
				return fakeECS, nil
			},
		},
	})
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"ecs", "reimage", "m-existing",
		"--poll-interval", "1ms",
		"--start-timeout", "3ms",
	})

	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected start failure\n%s", buf.String())
	}
	st := store.New(filepath.Join(root, "vulnsky.db"))
	records, err := st.ListDeployments(5)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("deployment record count = %d, want 1: %#v", len(records), records)
	}
	if records[0].Status != "replace_succeeded" || records[0].SourceImageID != "m-existing" {
		t.Fatalf("unexpected deployment record: %#v", records[0])
	}
}

type deployFakeECS struct {
	instance        aliyun.Instance
	images          []aliyun.Image
	task            aliyun.TaskStatus
	stopped         bool
	replaced        bool
	started         bool
	forceStop       bool
	startFailures   int
	startCalls      int
	importCalls     int
	importOSSObject string
	importImageID   string
	importTaskID    string
	importRequestID string
}

func (f *deployFakeECS) ListInstances(context.Context) ([]aliyun.Instance, error) {
	return []aliyun.Instance{f.instance}, nil
}

func (f *deployFakeECS) DescribeInstance(context.Context, string) (aliyun.Instance, error) {
	return f.instance, nil
}

func (f *deployFakeECS) ListImages(context.Context, string) ([]aliyun.Image, error) {
	return f.images, nil
}

func (f *deployFakeECS) ImportImage(_ context.Context, input aliyun.ImportImageInput) (string, string, string, error) {
	f.importCalls++
	f.importOSSObject = input.OSSObject
	return firstNonEmpty(f.importImageID, "m-new"), firstNonEmpty(f.importTaskID, "t-123"), firstNonEmpty(f.importRequestID, "req-import"), nil
}

func (f *deployFakeECS) DescribeTask(context.Context, string) (aliyun.TaskStatus, error) {
	return f.task, nil
}

func (f *deployFakeECS) StopInstance(_ context.Context, _ string, force bool) error {
	f.stopped = true
	f.forceStop = force
	f.instance.Status = "Stopped"
	return nil
}

func (f *deployFakeECS) StartInstance(context.Context, string) error {
	f.startCalls++
	if f.startCalls <= f.startFailures {
		return fmt.Errorf("SDKError: Code: IncorrectInstanceStatus")
	}
	f.started = true
	f.instance.Status = "Running"
	return nil
}

func (f *deployFakeECS) ReplaceSystemDisk(_ context.Context, _ string, imageID string) (string, string, error) {
	f.replaced = true
	f.instance.ImageID = imageID
	return "d-new", "req-replace", nil
}

type deployUploadFakeOSS struct {
	uploadedKey  string
	uploadedPath string
}

func (f *deployUploadFakeOSS) ListObjects(context.Context, string, string) ([]string, error) {
	return nil, nil
}

func (f *deployUploadFakeOSS) ObjectExists(context.Context, string, string) (bool, error) {
	return false, nil
}

func (f *deployUploadFakeOSS) UploadFile(_ context.Context, _ string, key string, path string, onProgress func(done int64, total int64)) (string, error) {
	f.uploadedKey = key
	f.uploadedPath = path
	if onProgress != nil {
		onProgress(5*1024*1024, 10*1024*1024)
		onProgress(10*1024*1024, 10*1024*1024)
	}
	return "req-upload", nil
}

func (f *deployUploadFakeOSS) PresignGet(context.Context, string, string, time.Duration) (string, error) {
	return "", nil
}

func (f *deployUploadFakeOSS) GetBucketLocation(context.Context, string) (string, error) {
	return "cn-hangzhou", nil
}

func TestImageStatusPrintsTask(t *testing.T) {
	root := writeDoctorProfile(t, "class-a", "123456789", "cn-hangzhou", "cn-hangzhou", "")
	buf := new(bytes.Buffer)
	cmd := NewRootCommandWithOptions(RootOptions{
		RootDir: root,
		Factories: ClientFactories{
			NewECS: func(config.Config) (aliyun.ECSClient, error) {
				return commandFakeECS{task: aliyun.TaskStatus{ID: "t-123", ResourceID: "m-new", Action: "ImportImage", Status: "Finished"}}, nil
			},
		},
	})
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"image", "status", "t-123"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(buf.String(), "Finished") || !strings.Contains(buf.String(), "m-new") {
		t.Fatalf("image status output missing task status:\n%s", buf.String())
	}
}
