package workers

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type fileID string
type fileStatus int
type taskID string
type taskStatus int

const (
	taskAwait taskStatus = iota
	taskInProgress
	task
)

const (
	fileAwait fileStatus = iota
	fileDownloaded
	fileDone
	fileFailured
)

type WorkerPool struct {
	taskQueue chan TaskModel
	taskCount uint32
	Directory string `json:"directory"`

	wgWorkers sync.WaitGroup
	wgTasks   sync.WaitGroup

	mu sync.Mutex
}

type TaskModel struct {
	ID        taskID      `json:"id"`
	Status    taskStatus  `json:"status"`
	Files     []FileModel `json:"file"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

type FileModel struct {
	ID        fileID     `json:"id"`
	URL       string     `json:"url"`
	Filename  string     `json:"filename"`
	Status    fileStatus `json:"status"`
	Error     string     `json:"error,omitempty"`
	UpdatedAt string     `json:"updated_at"`
}

func NewWorkerPool(workerCount uint16) *WorkerPool {
	wp := &WorkerPool{
		taskQueue: make(chan TaskModel, 1024),
	}

	// проверка, есть ли сохраненный счетчик задач в бд, если нет taskCount = 0

	// добавление в очередь неоконченных задач из бд
	// добавление в очередь ожидающих задач из бд

	for i := uint16(0); i < workerCount; i++ {
		wp.wgWorkers.Add(1)
		go wp.worker()
	}

	return wp
}

func (wp *WorkerPool) worker() {
	wp.wgTasks.Add(1)
	localTask := <-wp.taskQueue
	if localTask.Status == taskAwait {
		localTask.Status = taskInProgress
	}

	for _, val := range localTask.Files {
		err := downloadFile(string(localTask.ID), wp.Directory, &val) // проверить передается ли ссылка на изначальный объект
		if err != nil {
			// сообщение об ошибке статус файла на filed, занесение в бд
		}
		// смена статуса файла, занесение в бд
	}
}

// получить ID таски и файл
func downloadFile(TaskID, dir string, FModel *FileModel) error {
	resp, err := http.Get(FModel.URL)
	if err != nil {
		return fmt.Errorf("url request error: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("url response error")
	}

	fileDir := filepath.Join(dir, TaskID)
	if err := os.MkdirAll(fileDir, 0755); err != nil {
		return fmt.Errorf("failed to make directory: %w", err)
	}

	urlfName := detectFilename(FModel.URL, resp)
	filename := TaskID + "-" + string(FModel.ID) + "-" + urlfName

	filePath := filepath.Join(fileDir, filename)

	out, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	// добавить попытку переподключиться

	// в конце добавляем имя
	FModel.Filename = filename
	return nil
}

func detectFilename(url string, resp *http.Response) string {
	if cd := resp.Header.Get("Content-Disposition"); cd != "" { // Проверяем Content-Disposition
		if _, params, err := mime.ParseMediaType(cd); err == nil {
			if fname, ok := params["filename"]; ok {
				return fname
			}
		}
	}

	if ctype := resp.Header.Get("Content-Type"); ctype != "" { // Пробуем Content-Type и расширение
		if exts, _ := mime.ExtensionsByType(ctype); len(exts) > 0 {
			return exts[0]
		}
	}

	if ext := filepath.Ext(url); ext != "" { // Берём из URL
		return ext
	}

	return "downloaded.bin"
}

// добавить функцию для добавления таски в очередь. возвращает id задачи
func (wp *WorkerPool) AddTask(urls []string) string {
	wp.mu.Lock()
	wp.taskCount++
	wp.mu.Unlock()

	NewTask := &TaskModel{
		ID:        taskID(wp.taskCount), // подкорректировать конвертацию в taskID
		Status:    taskAwait,
		Files:     make([]FileModel, len(urls)),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	for idx, url := range urls {
		NewTask.Files[idx] = FileModel{
			ID:        fileID(idx),
			URL:       url,
			Filename:  "",
			Status:    fileAwait,
			Error:     "",
			UpdatedAt: "",
		}
	}

	wp.taskQueue <- *NewTask

	return string(NewTask.ID)
}

// функция для просмотра статуса по задаче, требуется бд
