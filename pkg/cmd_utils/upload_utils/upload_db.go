package upload_utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/coscene-io/cocli/internal/constants"
	"github.com/minio/sha256-simd"
	"github.com/pkg/errors"
	bolt "go.etcd.io/bbolt"
)

const (
	multipartUploadsBucket = "multipart_uploads"
)

type UploadDB struct {
	*bolt.DB
}

func NewUploadDB(filename string, recordId string, hash string) (*UploadDB, error) {
	// Compute the db file name by hashing the filepath and recordId
	// todo: add part size
	h := sha256.New()
	h.Write([]byte(recordId + hash + filename))

	boltDB, err := bolt.Open(filepath.Join(constants.DefaultUploaderDirPath, fmt.Sprintf("%x.db", h.Sum(nil))), 0600, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "open db %s", filename)
	}

	// Create a bucket for the multipart uploads
	if err = boltDB.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(multipartUploadsBucket))
		return err
	}); err != nil {
		return nil, errors.Wrapf(err, "create bucket %s", multipartUploadsBucket)
	}

	return &UploadDB{DB: boltDB}, nil
}

// Get retrieves the value of a key from the database.
// if the key does not exist, it returns nil.
func (db *UploadDB) Get(key string, objectPtr interface{}) error {
	var value []byte
	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(multipartUploadsBucket))
		value = bucket.Get([]byte(key))
		return nil
	})
	if err != nil {
		return err
	}

	if err = json.Unmarshal(value, objectPtr); err != nil {
		return errors.Wrapf(err, "unmarshal %s", key)
	}

	return nil
}

// Reset removes all the keys from the database multipart_uploads bucket.
func (db *UploadDB) Reset() error {
	return db.Update(func(tx *bolt.Tx) error {
		err := tx.DeleteBucket([]byte(multipartUploadsBucket))
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte(multipartUploadsBucket))
		return err
	})
}

// Delete removes the database file from the filesystem.
func (db *UploadDB) Delete() error {
	if err := os.Remove(db.Path()); err != nil {
		return errors.Wrapf(err, "remove db %s", db.Path())
	}

	return db.Close()
}
