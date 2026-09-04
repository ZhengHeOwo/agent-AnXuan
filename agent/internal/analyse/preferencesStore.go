package analyse

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	preferencesBucketName = "Behavior_Preferences"
	databaseErrBucketName = "bbolt_Database_Err_Log"
)

type preferencesStore struct {
	db *bolt.DB
}

// 创建/确认：键值数据库、bucket存在。
func NewPreferencesStore(path string) (*preferencesStore, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf(
			"Database path must not be empty",
		)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf(
			"Database file creation failed: %w",
			err,
		)
	}

	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf(
			"Open database failed: %w",
			err,
		)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte(preferencesBucketName)); err != nil {
			return fmt.Errorf(
				"create database bucket %v: %w",
				preferencesBucketName,
				err,
			)
		}

		if _, err := tx.CreateBucketIfNotExists([]byte(databaseErrBucketName)); err != nil {
			return fmt.Errorf(
				"create database bucket %v: %w",
				databaseErrBucketName,
				err,
			)
		}

		return nil
	})

	if err != nil {
		_ = db.Close()
		return nil, err
	}

	return &preferencesStore{
		db: db,
	}, nil
}

func (s *preferencesStore) Close() error {
	return s.db.Close()
}
