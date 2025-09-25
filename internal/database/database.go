package database

import (
	"encoding/binary"
	"gotest/internal/config"
	"log/slog"

	"go.etcd.io/bbolt"
	"go.etcd.io/bbolt/errors"
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
			return errors.ErrBucketNotFound
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

// функция для сохранения задачи в бд

// функкция для получения слайса задач in-progress

// функция для получения слайса задач await
