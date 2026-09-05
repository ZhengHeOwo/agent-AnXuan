package analyse

import (
	"encoding/json"
	"errors"
	"fmt"

	bolt "go.etcd.io/bbolt"
)

// Preference 定义bbolt键值数据库value的信息结构。
type Preference struct {
	Name    string `json:"name"`
	Content string `json:"content"`
	Reason  string `json:"reason"`
}

// modelPreference 定义主模型需要接收的信息结构。
type modelPreference struct {
	Preference  string `json:"preference"`
	Description string `json:"description"`
}

// toModelPreference 将遍历出的数据库value还原为数据库信息结构。
func toModelPreference(
	modelPreferences []modelPreference,
	value []byte,
) (Preference, error) {
	var preference Preference
	if err := json.Unmarshal(value, &preference); err != nil {
		return Preference{}, fmt.Errorf(
			"decode database value failed: %w",
			err,
		)
	}
	return preference, nil
}

// ListModelPreference 列出数据库所有偏好并转换为主模型使用的内部信息结构。
func (s *preferencesStore) ListModelPreference() (string, error) {
	modelPreferences := make([]modelPreference, 0)

	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(preferencesBucketName))
		if bucket == nil {
			return errPreferencesBucketNotFound
		}

		return bucket.ForEach(func(key, value []byte) error {
			preference, err := toModelPreference(modelPreferences, value)
			if err != nil {
				return err
			}

			modelPreferences = append(modelPreferences, modelPreference{
				Preference:  preference.Name,
				Description: preference.Content,
			})

			return nil
		})
	})

	if err != nil {
		return "", err
	}

	if len(modelPreferences) == 0 {
		return "No information available.", nil
	}

	data, err := json.Marshal(modelPreferences)
	if err != nil {
		return "", fmt.Errorf(
			"marshal modelPreferences failed: %w",
			err,
		)
	}
	
	return string(data), nil
}

// listPreferenceKeys 列出 键值数据库所有健
func (s *preferencesStore) listPreferenceKeys() (string, error) {
	var keys string

	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(preferencesBucketName))
		if bucket == nil {
			return errPreferencesBucketNotFound
		}

		return bucket.ForEach(func(key, value []byte) error {
			k := string(key)

			if keys == "" {
				keys = k
			} else {
				keys += "、" + k
			}

			return nil
		})
	})

	if err != nil {
		return "", err
	}

	if keys == "" {
		keys = "No information available."
	}

	return keys, nil
}

// preferenceGet 实现 键值数据库bolt 以键取值。
func (s *preferencesStore) preferenceGet(name string) (Preference, error) {
	var preference Preference

	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(preferencesBucketName))
		if bucket == nil {
			return errPreferencesBucketNotFound
		}

		data := bucket.Get([]byte(name))
		if data == nil {
			return errPreferenceNotFound
		}

		if err := json.Unmarshal(data, &preference); err != nil {
			return fmt.Errorf(
				"decode database value failed: %w",
				err,
			)
		}

		return nil
	})

	if err != nil {
		return Preference{}, err
	}

	return preference, nil
}

// PutPreference 新增或覆盖偏好。
func (s *preferencesStore) PutPreference(preference Preference) error {
	if preference.Name == "" {
		return errors.New("preference name cannot be empty")
	}

	data, err := json.Marshal(preference)
	if err != nil {
		return fmt.Errorf("encode preference: %w", err)
	}

	return s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(preferencesBucketName))
		if bucket == nil {
			return errors.New("preferences bucket does not exist")
		}

		if err := bucket.Put([]byte(preference.Name), data); err != nil {
			return fmt.Errorf("put preference: %w", err)
		}

		return nil
	})
}

// DeletePreference 删除偏好。
func (s *preferencesStore) DeletePreference(name string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(preferencesBucketName))
		if bucket == nil {
			return errors.New("preferences bucket does not exist")
		}

		if bucket.Get([]byte(name)) == nil {
			return errPreferenceNotFound
		}

		if err := bucket.Delete([]byte(name)); err != nil {
			return fmt.Errorf("delete preference: %w", err)
		}

		return nil
	})
}

// RenamePreference 重命名偏好键。
func (s *preferencesStore) RenamePreference(oldName, newName string) error {
	if newName == "" {
		return errors.New("new preference name cannot be empty")
	}

	return s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(preferencesBucketName))
		if bucket == nil {
			return errors.New("preferences bucket does not exist")
		}

		data := bucket.Get([]byte(oldName))
		if data == nil {
			return errPreferenceNotFound
		}

		if bucket.Get([]byte(newName)) != nil {
			return errors.New("new preference name already exists")
		}

		var preference Preference
		if err := json.Unmarshal(data, &preference); err != nil {
			return fmt.Errorf("decode preference: %w", err)
		}

		preference.Name = newName

		newData, err := json.Marshal(preference)
		if err != nil {
			return fmt.Errorf("encode renamed preference: %w", err)
		}

		if err := bucket.Put([]byte(newName), newData); err != nil {
			return fmt.Errorf("write renamed preference: %w", err)
		}

		if err := bucket.Delete([]byte(oldName)); err != nil {
			return fmt.Errorf("delete old preference: %w", err)
		}

		return nil
	})
}
