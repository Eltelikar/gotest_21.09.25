package workermodels

import "time"

type FileID string
type FileStatus int
type TaskID string
type TaskStatus int

const (
	TaskAwait TaskStatus = iota
	TaskInProgress
	TaskDone
	TaskFailed
	TaskPartialDone // в случае частичной загрузки файлов
)

const (
	FileAwait FileStatus = iota
	FileDownloading
	FileDone
	FileFailured
)

type TaskModel struct {
	ID        TaskID      `json:"id"`
	Status    TaskStatus  `json:"status"`
	Files     []FileModel `json:"file"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

type FileModel struct {
	ID        FileID     `json:"id"`
	URL       string     `json:"url"`
	Filename  string     `json:"filename"`
	Status    FileStatus `json:"status"`
	Error     string     `json:"error,omitempty"`
	UpdatedAt string     `json:"updated_at"`
}
