package settings

import (
	"sync"

	"helios-backend/internal/db"
)

const KeyApprovalRequired = "approval_required"

var (
	mu    sync.RWMutex
	cache = make(map[string]string)
)

func Load() error {
	rows, err := db.DB.Query(`SELECT setting_key, setting_value FROM system_settings`)
	if err != nil {
		return err
	}
	defer rows.Close()

	mu.Lock()
	defer mu.Unlock()
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return err
		}
		cache[k] = v
	}
	return nil
}

func Get(key string) string {
	mu.RLock()
	defer mu.RUnlock()
	return cache[key]
}

func Set(key, value string) error {
	if _, err := db.DB.Exec(
		`INSERT INTO system_settings (setting_key, setting_value) VALUES (?, ?)
		 ON DUPLICATE KEY UPDATE setting_value = VALUES(setting_value)`,
		key, value,
	); err != nil {
		return err
	}
	mu.Lock()
	cache[key] = value
	mu.Unlock()
	return nil
}

func ApprovalRequired() bool {
	return Get(KeyApprovalRequired) == "true"
}

func SetApprovalRequired(v bool) error {
	val := "false"
	if v {
		val = "true"
	}
	return Set(KeyApprovalRequired, val)
}
