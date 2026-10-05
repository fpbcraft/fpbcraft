package planning

import "time"

type BackupFile struct {
	SourcePath string `json:"source_path"`
	BackupPath string `json:"backup_path"`
	SHA512     string `json:"sha512"`
	Bytes      int64  `json:"bytes"`
}

type BackupManifest struct {
	SchemaVersion int          `json:"schema_version"`
	ID            string       `json:"id"`
	PlanID        string       `json:"plan_id"`
	CreatedAt     time.Time    `json:"created_at"`
	Files         []BackupFile `json:"files"`
}
