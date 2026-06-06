package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Store struct {
	path string
}

type UploadRecord struct {
	ID           int64
	ProfileName  string
	AccountID    string
	RegionID     string
	OSSRegionID  string
	Bucket       string
	ObjectKey    string
	LocalPath    string
	FileName     string
	FileSize     int64
	SHA256       string
	UploadStatus string
	RequestID    string
	UploadedAt   string
}

type ImageImportRecord struct {
	ID           int64
	ProfileName  string
	AccountID    string
	RegionID     string
	UploadID     int64
	OSSBucket    string
	OSSObject    string
	ImageName    string
	ImageID      string
	TaskID       string
	TaskStatus   string
	Platform     string
	Architecture string
	OSType       string
	RequestID    string
	ErrorMessage string
	StartedAt    string
	FinishedAt   string
}

type DeploymentRecord struct {
	ID              int64
	ProfileName     string
	AccountID       string
	RegionID        string
	InstanceID      string
	InstanceName    string
	SourceUploadID  int64
	SourceImageID   string
	PreviousImageID string
	NewDiskID       string
	StopMode        string
	Status          string
	RequestID       string
	ErrorMessage    string
	StartedAt       string
	FinishedAt      string
}

type DeployedImageMark struct {
	ImageID         string
	InstanceID      string
	InstanceName    string
	PreviousImageID string
	NewDiskID       string
	StopMode        string
	Status          string
	StartedAt       string
	FinishedAt      string
}

func New(path string) *Store {
	return &Store{path: path}
}

func (s *Store) open() (*sql.DB, error) {
	if err := ensureParentDir(s.path); err != nil {
		return nil, err
	}
	return sql.Open("sqlite", s.path)
}

func (s *Store) Init() error {
	db, err := s.open()
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS uploads (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	profile_name TEXT NOT NULL,
	account_id TEXT NOT NULL,
	region_id TEXT NOT NULL,
	oss_region_id TEXT NOT NULL,
	bucket TEXT NOT NULL,
	object_key TEXT NOT NULL,
	local_path TEXT NOT NULL,
	file_name TEXT NOT NULL,
	file_size INTEGER NOT NULL,
	sha256 TEXT NOT NULL,
	upload_status TEXT NOT NULL,
	request_id TEXT,
	uploaded_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_uploads_lookup
ON uploads(profile_name, account_id, region_id, sha256, upload_status);

CREATE TABLE IF NOT EXISTS image_imports (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	profile_name TEXT NOT NULL,
	account_id TEXT NOT NULL,
	region_id TEXT NOT NULL,
	upload_id INTEGER NOT NULL,
	oss_bucket TEXT NOT NULL DEFAULT '',
	oss_object TEXT NOT NULL DEFAULT '',
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
);

CREATE TABLE IF NOT EXISTS images (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	profile_name TEXT NOT NULL,
	account_id TEXT NOT NULL,
	region_id TEXT NOT NULL,
	image_id TEXT NOT NULL,
	image_name TEXT NOT NULL,
	source_upload_id INTEGER,
	source_import_id INTEGER,
	status TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	last_seen_at TEXT
);

CREATE TABLE IF NOT EXISTS ecs_instances_cache (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	profile_name TEXT NOT NULL,
	account_id TEXT NOT NULL,
	region_id TEXT NOT NULL,
	instance_id TEXT NOT NULL,
	instance_name TEXT,
	status TEXT NOT NULL,
	image_id TEXT,
	public_ip TEXT,
	private_ip TEXT,
	instance_type TEXT,
	zone_id TEXT,
	last_seen_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS deployments (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	profile_name TEXT NOT NULL,
	account_id TEXT NOT NULL,
	region_id TEXT NOT NULL,
	instance_id TEXT NOT NULL,
	instance_name TEXT,
	source_upload_id INTEGER NOT NULL,
	source_image_id TEXT NOT NULL,
	previous_image_id TEXT,
	new_disk_id TEXT,
	stop_mode TEXT NOT NULL,
	status TEXT NOT NULL,
	request_id TEXT,
	error_message TEXT,
	started_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	finished_at TEXT
);

CREATE TABLE IF NOT EXISTS events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	profile_name TEXT NOT NULL,
	account_id TEXT NOT NULL,
	region_id TEXT NOT NULL,
	entity_type TEXT NOT NULL,
	entity_id TEXT NOT NULL,
	action TEXT NOT NULL,
	status TEXT NOT NULL,
	message TEXT NOT NULL,
	request_id TEXT,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`)
	if err != nil {
		return err
	}
	if err := ensureColumn(db, "image_imports", "oss_bucket", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureColumn(db, "image_imports", "oss_object", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	return nil
}

func (s *Store) TableNames() (map[string]bool, error) {
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type = 'table'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tables := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables[name] = true
	}
	return tables, rows.Err()
}

func (s *Store) InsertUpload(record UploadRecord) (int64, error) {
	db, err := s.open()
	if err != nil {
		return 0, err
	}
	defer db.Close()

	result, err := db.Exec(`
INSERT INTO uploads (
	profile_name, account_id, region_id, oss_region_id, bucket, object_key,
	local_path, file_name, file_size, sha256, upload_status, request_id
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.ProfileName,
		record.AccountID,
		record.RegionID,
		record.OSSRegionID,
		record.Bucket,
		record.ObjectKey,
		record.LocalPath,
		record.FileName,
		record.FileSize,
		record.SHA256,
		record.UploadStatus,
		record.RequestID,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (s *Store) FindUploadedBySHA256(profileName, accountID, regionID, sha256 string) (*UploadRecord, error) {
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	row := db.QueryRow(`
SELECT profile_name, account_id, region_id, oss_region_id, bucket, object_key,
       local_path, file_name, file_size, sha256, upload_status, COALESCE(request_id, '')
FROM uploads
WHERE profile_name = ?
  AND account_id = ?
  AND region_id = ?
  AND sha256 = ?
  AND upload_status = 'uploaded'
ORDER BY uploaded_at DESC
LIMIT 1`,
		profileName,
		accountID,
		regionID,
		sha256,
	)

	var record UploadRecord
	if err := row.Scan(
		&record.ProfileName,
		&record.AccountID,
		&record.RegionID,
		&record.OSSRegionID,
		&record.Bucket,
		&record.ObjectKey,
		&record.LocalPath,
		&record.FileName,
		&record.FileSize,
		&record.SHA256,
		&record.UploadStatus,
		&record.RequestID,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

func (s *Store) ListUploads(limit int) ([]UploadRecord, error) {
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if limit <= 0 {
		limit = 50
	}

	rows, err := db.Query(`
SELECT id, profile_name, account_id, region_id, oss_region_id, bucket, object_key,
       local_path, file_name, file_size, sha256, upload_status,
       COALESCE(request_id, ''), uploaded_at
FROM uploads
ORDER BY id DESC
LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []UploadRecord
	for rows.Next() {
		var record UploadRecord
		if err := rows.Scan(
			&record.ID,
			&record.ProfileName,
			&record.AccountID,
			&record.RegionID,
			&record.OSSRegionID,
			&record.Bucket,
			&record.ObjectKey,
			&record.LocalPath,
			&record.FileName,
			&record.FileSize,
			&record.SHA256,
			&record.UploadStatus,
			&record.RequestID,
			&record.UploadedAt,
		); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *Store) InsertImageImport(record ImageImportRecord) (int64, error) {
	db, err := s.open()
	if err != nil {
		return 0, err
	}
	defer db.Close()

	result, err := db.Exec(`
INSERT INTO image_imports (
	profile_name, account_id, region_id, upload_id, oss_bucket, oss_object, image_name, image_id,
	task_id, task_status, platform, architecture, os_type, request_id, error_message
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.ProfileName,
		record.AccountID,
		record.RegionID,
		record.UploadID,
		record.OSSBucket,
		record.OSSObject,
		record.ImageName,
		record.ImageID,
		record.TaskID,
		record.TaskStatus,
		record.Platform,
		record.Architecture,
		record.OSType,
		record.RequestID,
		record.ErrorMessage,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (s *Store) ListImageImports(limit int) ([]ImageImportRecord, error) {
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if limit <= 0 {
		limit = 50
	}

	rows, err := db.Query(`
SELECT id, profile_name, account_id, region_id, upload_id, image_name, image_id,
       task_id, task_status, platform, architecture, os_type,
       COALESCE(oss_bucket, ''), COALESCE(oss_object, ''),
       COALESCE(request_id, ''), COALESCE(error_message, ''),
       started_at, COALESCE(finished_at, '')
FROM image_imports
ORDER BY id DESC
LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []ImageImportRecord
	for rows.Next() {
		var record ImageImportRecord
		if err := rows.Scan(
			&record.ID,
			&record.ProfileName,
			&record.AccountID,
			&record.RegionID,
			&record.UploadID,
			&record.ImageName,
			&record.ImageID,
			&record.TaskID,
			&record.TaskStatus,
			&record.Platform,
			&record.Architecture,
			&record.OSType,
			&record.OSSBucket,
			&record.OSSObject,
			&record.RequestID,
			&record.ErrorMessage,
			&record.StartedAt,
			&record.FinishedAt,
		); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *Store) FindFinishedImageImportByOSSObject(profileName, accountID, regionID, bucket, objectKey string) (*ImageImportRecord, error) {
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	row := db.QueryRow(`
SELECT id, profile_name, account_id, region_id, upload_id,
       COALESCE(oss_bucket, ''), COALESCE(oss_object, ''),
       image_name, image_id, task_id, task_status, platform, architecture, os_type,
       COALESCE(request_id, ''), COALESCE(error_message, ''),
       started_at, COALESCE(finished_at, '')
FROM image_imports
WHERE profile_name = ?
  AND account_id = ?
  AND region_id = ?
  AND oss_bucket = ?
  AND oss_object = ?
  AND LOWER(task_status) IN ('finished', 'reused')
ORDER BY id DESC
LIMIT 1`,
		profileName,
		accountID,
		regionID,
		bucket,
		objectKey,
	)

	var record ImageImportRecord
	if err := row.Scan(
		&record.ID,
		&record.ProfileName,
		&record.AccountID,
		&record.RegionID,
		&record.UploadID,
		&record.OSSBucket,
		&record.OSSObject,
		&record.ImageName,
		&record.ImageID,
		&record.TaskID,
		&record.TaskStatus,
		&record.Platform,
		&record.Architecture,
		&record.OSType,
		&record.RequestID,
		&record.ErrorMessage,
		&record.StartedAt,
		&record.FinishedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

func (s *Store) InsertDeployment(record DeploymentRecord) (int64, error) {
	db, err := s.open()
	if err != nil {
		return 0, err
	}
	defer db.Close()

	result, err := db.Exec(`
INSERT INTO deployments (
	profile_name, account_id, region_id, instance_id, instance_name,
	source_upload_id, source_image_id, previous_image_id, new_disk_id,
	stop_mode, status, request_id, error_message, finished_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		record.ProfileName,
		record.AccountID,
		record.RegionID,
		record.InstanceID,
		record.InstanceName,
		record.SourceUploadID,
		record.SourceImageID,
		record.PreviousImageID,
		record.NewDiskID,
		record.StopMode,
		record.Status,
		record.RequestID,
		record.ErrorMessage,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (s *Store) ListDeployments(limit int) ([]DeploymentRecord, error) {
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if limit <= 0 {
		limit = 50
	}

	rows, err := db.Query(`
SELECT id, profile_name, account_id, region_id, instance_id, COALESCE(instance_name, ''),
       source_upload_id, source_image_id, COALESCE(previous_image_id, ''),
       COALESCE(new_disk_id, ''), stop_mode, status,
       COALESCE(request_id, ''), COALESCE(error_message, ''),
       started_at, COALESCE(finished_at, '')
FROM deployments
ORDER BY id DESC
LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []DeploymentRecord
	for rows.Next() {
		var record DeploymentRecord
		if err := rows.Scan(
			&record.ID,
			&record.ProfileName,
			&record.AccountID,
			&record.RegionID,
			&record.InstanceID,
			&record.InstanceName,
			&record.SourceUploadID,
			&record.SourceImageID,
			&record.PreviousImageID,
			&record.NewDiskID,
			&record.StopMode,
			&record.Status,
			&record.RequestID,
			&record.ErrorMessage,
			&record.StartedAt,
			&record.FinishedAt,
		); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *Store) DeployedImages(profileName, accountID, regionID string) (map[string]DeployedImageMark, error) {
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`
SELECT source_image_id, instance_id, COALESCE(instance_name, ''),
       COALESCE(previous_image_id, ''), COALESCE(new_disk_id, ''),
       stop_mode, status, started_at, COALESCE(finished_at, '')
FROM deployments
WHERE profile_name = ?
  AND account_id = ?
  AND region_id = ?
ORDER BY id DESC`,
		profileName,
		accountID,
		regionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	marks := map[string]DeployedImageMark{}
	for rows.Next() {
		var mark DeployedImageMark
		if err := rows.Scan(
			&mark.ImageID,
			&mark.InstanceID,
			&mark.InstanceName,
			&mark.PreviousImageID,
			&mark.NewDiskID,
			&mark.StopMode,
			&mark.Status,
			&mark.StartedAt,
			&mark.FinishedAt,
		); err != nil {
			return nil, err
		}
		if _, exists := marks[mark.ImageID]; !exists {
			marks[mark.ImageID] = mark
		}
	}
	return marks, rows.Err()
}

func ensureParentDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func ensureColumn(db *sql.DB, tableName, columnName, definition string) error {
	if !isSafeIdentifier(tableName) || !isSafeIdentifier(columnName) {
		return fmt.Errorf("unsafe sqlite identifier: table=%q column=%q", tableName, columnName)
	}
	rows, err := db.Query("PRAGMA table_info(" + tableName + ")")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var typ string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == columnName {
			return rows.Err()
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = db.Exec("ALTER TABLE " + tableName + " ADD COLUMN " + columnName + " " + definition)
	return err
}

func isSafeIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for i, r := range value {
		if r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (i > 0 && r >= '0' && r <= '9') {
			continue
		}
		return false
	}
	return true
}
