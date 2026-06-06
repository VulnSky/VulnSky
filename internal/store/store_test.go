package store

import (
	"path/filepath"
	"testing"
)

func TestStoreInitializesTables(t *testing.T) {
	db := New(filepath.Join(t.TempDir(), "vulnsky.db"))

	if err := db.Init(); err != nil {
		t.Fatal(err)
	}

	tables, err := db.TableNames()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"uploads", "image_imports", "images", "ecs_instances_cache", "deployments", "events"} {
		if !tables[want] {
			t.Fatalf("missing table %q in %#v", want, tables)
		}
	}
}

func TestUploadRoundTripBySHA256(t *testing.T) {
	db := New(filepath.Join(t.TempDir(), "vulnsky.db"))
	if err := db.Init(); err != nil {
		t.Fatal(err)
	}

	record := UploadRecord{
		ProfileName:  "class-a",
		AccountID:    "123456789",
		RegionID:     "cn-hangzhou",
		OSSRegionID:  "cn-hangzhou",
		Bucket:       "lab-bucket",
		ObjectKey:    "images/class-a/lab.qcow2",
		LocalPath:    `C:\Labs\lab.qcow2`,
		FileName:     "lab.qcow2",
		FileSize:     1024,
		SHA256:       "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		UploadStatus: "uploaded",
		RequestID:    "req-1",
	}

	uploadID, err := db.InsertUpload(record)
	if err != nil {
		t.Fatal(err)
	}
	found, err := db.FindUploadedBySHA256("class-a", "123456789", "cn-hangzhou", record.SHA256)
	if err != nil {
		t.Fatal(err)
	}

	if uploadID <= 0 {
		t.Fatalf("uploadID = %d, want positive", uploadID)
	}
	if found == nil {
		t.Fatal("expected upload record, got nil")
	}
	if found.ObjectKey != "images/class-a/lab.qcow2" {
		t.Fatalf("ObjectKey = %q", found.ObjectKey)
	}
}

func TestImageImportRoundTripByOSSObject(t *testing.T) {
	db := New(filepath.Join(t.TempDir(), "vulnsky.db"))
	if err := db.Init(); err != nil {
		t.Fatal(err)
	}

	_, err := db.InsertImageImport(ImageImportRecord{
		ProfileName:  "class-a",
		AccountID:    "123456789",
		RegionID:     "cn-hangzhou",
		UploadID:     0,
		OSSBucket:    "lab-bucket",
		OSSObject:    "qcow2/sample-lab.qcow2",
		ImageName:    "vulnsky-sample-lab",
		ImageID:      "m-reused",
		TaskID:       "t-123",
		TaskStatus:   "Finished",
		Platform:     "Others Linux",
		Architecture: "x86_64",
		OSType:       "linux",
		RequestID:    "req-import",
	})
	if err != nil {
		t.Fatal(err)
	}

	found, err := db.FindFinishedImageImportByOSSObject("class-a", "123456789", "cn-hangzhou", "lab-bucket", "qcow2/sample-lab.qcow2")
	if err != nil {
		t.Fatal(err)
	}
	if found == nil {
		t.Fatal("expected image import record, got nil")
	}
	if found.ImageID != "m-reused" || found.OSSObject != "qcow2/sample-lab.qcow2" {
		t.Fatalf("unexpected image import record: %#v", found)
	}

	records, err := db.ListImageImports(5)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].OSSBucket != "lab-bucket" || records[0].OSSObject != "qcow2/sample-lab.qcow2" {
		t.Fatalf("ListImageImports lost OSS source fields: %#v", records)
	}
}

func TestStoreMigratesImageImportOSSSourceColumns(t *testing.T) {
	db := New(filepath.Join(t.TempDir(), "vulnsky.db"))
	raw, err := db.open()
	if err != nil {
		t.Fatal(err)
	}
	_, err = raw.Exec(`
CREATE TABLE image_imports (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	profile_name TEXT NOT NULL,
	account_id TEXT NOT NULL,
	region_id TEXT NOT NULL,
	upload_id INTEGER NOT NULL,
	image_name TEXT NOT NULL,
	image_id TEXT NOT NULL,
	task_id TEXT NOT NULL,
	task_status TEXT NOT NULL,
	platform TEXT NOT NULL,
	architecture TEXT NOT NULL,
	os_type TEXT NOT NULL,
	request_id TEXT,
	error_message TEXT,
	started_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	finished_at TEXT
)`)
	if closeErr := raw.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatal(err)
	}

	if err := db.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.InsertImageImport(ImageImportRecord{
		ProfileName:  "class-a",
		AccountID:    "123456789",
		RegionID:     "cn-hangzhou",
		UploadID:     0,
		OSSBucket:    "lab-bucket",
		OSSObject:    "qcow2/migrated.qcow2",
		ImageName:    "vulnsky-migrated",
		ImageID:      "m-migrated",
		TaskID:       "t-migrated",
		TaskStatus:   "Finished",
		Platform:     "Others Linux",
		Architecture: "x86_64",
		OSType:       "linux",
	}); err != nil {
		t.Fatal(err)
	}

	found, err := db.FindFinishedImageImportByOSSObject("class-a", "123456789", "cn-hangzhou", "lab-bucket", "qcow2/migrated.qcow2")
	if err != nil {
		t.Fatal(err)
	}
	if found == nil || found.ImageID != "m-migrated" {
		t.Fatalf("migrated image import record not found: %#v", found)
	}
}

func TestDeployedImagesByProfileAccountRegion(t *testing.T) {
	db := New(filepath.Join(t.TempDir(), "vulnsky.db"))
	if err := db.Init(); err != nil {
		t.Fatal(err)
	}

	_, err := db.InsertDeployment(DeploymentRecord{
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
	})
	if err != nil {
		t.Fatal(err)
	}

	marks, err := db.DeployedImages("class-a", "123456789", "cn-hangzhou")
	if err != nil {
		t.Fatal(err)
	}
	mark, ok := marks["m-new"]
	if !ok {
		t.Fatalf("expected deployed image mark, got %#v", marks)
	}
	if mark.InstanceID != "i-lab" || mark.PreviousImageID != "m-old" || mark.NewDiskID != "d-new" {
		t.Fatalf("unexpected mark: %#v", mark)
	}
}

func TestStoreCreatesParentDirectory(t *testing.T) {
	db := New(filepath.Join(t.TempDir(), "nested", "state", "vulnsky.db"))

	if err := db.Init(); err != nil {
		t.Fatal(err)
	}
}
