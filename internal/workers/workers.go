package workers

import (
	"fmt"
	"gotest/internal/config"
	"gotest/internal/database"
	wm "gotest/internal/workerModels"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"go.etcd.io/bbolt"
)

type WorkerPool struct {
	taskQueue        chan wm.TaskModel
	taskCount        uint32
	storageDirectory string

	db *bbolt.DB

	wgWorkers sync.WaitGroup
	wgTasks   sync.WaitGroup

	mu sync.Mutex
}

func NewWorkerPool(cfg *config.Config, database *database.Database) *WorkerPool {
	wp := &WorkerPool{
		taskQueue:        make(chan wm.TaskModel, 1024),
		storageDirectory: cfg.Service.StorageDir,
		db:               database.DB,
		taskCount:        0,
		mu:               sync.Mutex{},
		wgWorkers:        sync.WaitGroup{},
		wgTasks:          sync.WaitGroup{},
	}

	workerCount := cfg.Service.WorkerCount
	taskCount, err := database.GetTaskCount()
	if err != nil {
		slog.Error("failed to get task count from database",
			slog.String("error", err.Error()))
		taskCount = 0
	}
	wp.taskCount = taskCount

	progressTaskQueue, err := database.GetInProgressTasks() // помещенение в очередь незавершенных задач
	if err != nil {
		slog.Error("failed to get in-progress tasks from database",
			slog.String("error", err.Error()))
	}
	for _, task := range progressTaskQueue {
		wp.taskQueue <- task
	}

	awaitTaskQueue, err := database.GetAwaitTasks() // помещение в очередь ожидающих задач
	if err != nil {
		slog.Error("failed to get await tasks from database",
			slog.String("error", err.Error()))
	}
	for _, task := range awaitTaskQueue {
		wp.taskQueue <- task
	}

	for i := uint16(0); i < workerCount; i++ {
		wp.wgWorkers.Add(1)
		go wp.worker()
	}

	return wp
}

// добаотать воркер, мб добавить горутины на каждый файл
func (wp *WorkerPool) worker() {
	wp.wgTasks.Add(1)
	localTask := <-wp.taskQueue
	if localTask.Status == wm.TaskAwait {
		localTask.Status = wm.TaskInProgress
	}

	for idx := range localTask.Files {
		err := downloadFile(string(localTask.ID), wp.storageDirectory, &localTask.Files[idx])
		if err != nil {
			// сообщение об ошибке статус файла на failured, занесение в бд
		}
		// смена статуса файла, занесение в бд
	}
}

// добавить доозагрузку файлов
func downloadFile(taskID, dir string, fModel *wm.FileModel) error {
	resp, err := http.Get(fModel.URL)
	if err != nil {
		return fmt.Errorf("url request error: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("url response error")
	}

	fileDir := filepath.Join(dir, taskID)
	if err := os.MkdirAll(fileDir, 0755); err != nil {
		return fmt.Errorf("failed to make directory: %w", err)
	}

	urlfName := detectFilename(fModel.URL, resp)
	filename := taskID + "-" + string(fModel.ID) + "-" + urlfName

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

	// в конце добавляем имя в структуру для корректного сохранения в бд
	fModel.Filename = filename
	return nil
}

// функция для определения имени файла
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

// функция для добавления задачи в очередь, возвращает ID задачи
func (wp *WorkerPool) AddTask(urls []string) string {
	wp.mu.Lock()
	wp.taskCount++
	wp.mu.Unlock()

	NewTask := &wm.TaskModel{
		ID:        wm.TaskID(strconv.Itoa(int(wp.taskCount))), // уделить внимание
		Status:    wm.TaskAwait,
		Files:     make([]wm.FileModel, len(urls)),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	for idx, url := range urls {
		NewTask.Files[idx] = wm.FileModel{
			ID:        wm.FileID(strconv.Itoa(idx)),
			URL:       url,
			Filename:  "",
			Status:    wm.FileAwait,
			Error:     "",
			UpdatedAt: "",
		}
	}

	wp.taskQueue <- *NewTask

	return string(NewTask.ID)
}

// функция для просмотра статуса по задаче, требуется бд
