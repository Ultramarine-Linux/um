package util

import (
	"os"

	bolt "go.etcd.io/bbolt"
)

func GetDB() (*bolt.DB, error) {
	if err := os.MkdirAll(GetStateDir(), 0755); err != nil {
		return nil, err
	}
	return bolt.Open(GetStateDir()+"/um.db", 0600, nil)
}
