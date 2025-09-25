package database

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"gotest/internal/config"
	wm "gotest/internal/workerModels"
	"log/slog"

	"go.etcd.io/bbolt"
	bbErrors "go.etcd.io/bbolt/errors"
)

const (
	CountBucketName = "count"
	TasksBucketName = "tasks"
)

type Database struct {
	DB *bbolt.DB
}

func NewDB(cfg *config.Config) (*Database, error) {
	db, err := bbolt.Open(cfg.Service.DbPath, 0666, nil)
	if err != nil {
		slog.Error("failed to open database",
			slog.String("db_path", cfg.Service.DbPath),
			slog.String("error", err.Error()))
		return nil, err
	}

	// создание бакета в бд для хранения переменной
	err = db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(CountBucketName))
		if err != nil {
			return err
		}

		return bucket.Put([]byte("taskCount"), []byte("0"))
	})
	if err != nil {
		slog.Error("failed to create count bucket in database",
			slog.String("db_path", cfg.Service.DbPath),
			slog.String("error", err.Error()))
		return nil, err
	}

	// создание бакета в бд для хранения тасков
	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(TasksBucketName))
		return err
	})
	if err != nil {
		slog.Error("failed to create tasks bucket in database",
			slog.String("db_path", cfg.Service.DbPath),
			slog.String("error", err.Error()))
		return nil, err
	}

	slog.Info("database successfully opened",
		slog.String("db_path", cfg.Service.DbPath))

	return &Database{DB: db}, nil
}

// функция для получения счетчика задач
func (database *Database) GetTaskCount() (uint32, error) {
	var count uint32
	db := database.DB

	err := db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(CountBucketName))
		if bucket == nil {
			return bbErrors.ErrBucketNotFound
		}

		countBytes := bucket.Get([]byte("taskCount"))
		if countBytes == nil {
			count = 0
			return nil
		} // читаем в байтах, поэтому нужно конвертировать байты в uint32

		count = binary.BigEndian.Uint32(countBytes)
		return nil
	})
	if err != nil {
		slog.Error("failed to get task count from database",
			slog.String("error", err.Error()))
		return 0, err
	}

	return count, nil
}

// функция для обновления счетчика задач
func (database *Database) UpdateTaskCount(newCount uint32) error {
	db := database.DB

	err := db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(CountBucketName))
		if bucket == nil {
			return bbErrors.ErrBucketNotFound
		}
		countBytes := make([]byte, 4)
		binary.BigEndian.PutUint32(countBytes, newCount)
		return bucket.Put([]byte("taskCount"), countBytes) // ключ для счетчика только "taskCount"
	})
	if err != nil {
		slog.Error("failed to update task count in database",
			slog.String("error", err.Error()))
		return err
	}
	return nil
}

// функция для сохранения задачи в бд
func (database *Database) SaveTask(task wm.TaskModel) error {
	db := database.DB
	taskBytes, err := json.Marshal(task) // преобразовываем структуру задачи в json

	if err != nil {
		slog.Error("failed to marshal task",
			slog.String("task_id", string(task.ID)),
			slog.String("error", err.Error()))
		return err
	}

	err = db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(TasksBucketName))
		if bucket == nil {
			return bbErrors.ErrBucketNotFound
		}
		return bucket.Put([]byte(task.ID), taskBytes)
	})
	if err != nil {
		slog.Error("failed to save task to database",
			slog.String("task_id", string(task.ID)),
			slog.String("error", err.Error()))
		return err
	}
	return nil
}

// функция для получения статуса задачи по id
func (database *Database) GetTaskStatusByID(id string) (wm.TaskStatus, map[wm.FileID]wm.FileStatus, error) {
	db := database.DB
	var task wm.TaskModel

	err := db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(TasksBucketName))
		if bucket == nil {
			return bbErrors.ErrBucketNotFound
		}
		taskBytes := bucket.Get([]byte(id))
		if taskBytes == nil {
			return errors.New("task not found")
		}
		return json.Unmarshal(taskBytes, &task) // производим unmarshall в task, возвращаем наверх err функции Unmarshal()
	})

	if err != nil {
		slog.Error("failed to get task from database",
			slog.String("task_id", id),
			slog.String("error", err.Error()))
		return 0, nil, err
	}

	fileStatuses := make(map[wm.FileID]wm.FileStatus)
	for _, file := range task.Files {
		fileStatuses[file.ID] = file.Status
	}
	return task.Status, fileStatuses, nil
}

// функкция для получения слайса задач in-progress
func (database *Database) GetInProgressTasks() ([]wm.TaskModel, error) {
	db := database.DB
	var tasks []wm.TaskModel

	err := db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(TasksBucketName))
		if bucket == nil {
			return bbErrors.ErrBucketNotFound
		}
		return bucket.ForEach(func(k, v []byte) error {
			var task wm.TaskModel
			err := json.Unmarshal(v, &task)
			if err != nil {
				return err
			}
			if task.Status == wm.TaskInProgress {
				tasks = append(tasks, task)
			}
			return nil
		})
	})
	if err != nil {
		slog.Error("failed to get await tasks from database",
			slog.String("error", err.Error()))
		return nil, err
	}
	return tasks, nil
}

// функция для получения слайса задач await
func (database *Database) GetAwaitTasks() ([]wm.TaskModel, error) {
	db := database.DB
	var tasks []wm.TaskModel

	err := db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(TasksBucketName))
		if bucket == nil {
			return bbErrors.ErrBucketNotFound
		}
		return bucket.ForEach(func(k, v []byte) error {
			var task wm.TaskModel
			err := json.Unmarshal(v, &task)
			if err != nil {
				return err
			}
			if task.Status == wm.TaskAwait {
				tasks = append(tasks, task)
			}
			return nil
		})
	})
	if err != nil {
		slog.Error("failed to get await tasks from database",
			slog.String("error", err.Error()))
		return nil, err
	}
	return tasks, nil
}
