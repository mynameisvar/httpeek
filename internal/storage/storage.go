package storage

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"

	"github.com/google/uuid"
	bolt "go.etcd.io/bbolt"

	"httpeek/internal/types"
)

var (
	bucketEntries = []byte("entries")
)

type Store struct {
	db *bolt.DB
}

func New(path string) (*Store, error) {
	db, err := bolt.Open(path, 0o644, nil)
	if err != nil {
		return nil, err
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		_, e := tx.CreateBucketIfNotExists(bucketEntries)
		return e
	}); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Put(e *types.Entry) (string, error) {
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(e); err != nil {
		return "", err
	}
	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketEntries)
		return b.Put([]byte(e.ID), buf.Bytes())
	})
	return e.ID, err
}

func (s *Store) Get(id string) (*types.Entry, error) {
	var e types.Entry
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketEntries)
		v := b.Get([]byte(id))
		if v == nil {
			return errors.New("not found")
		}
		return gob.NewDecoder(bytes.NewReader(v)).Decode(&e)
	})
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (s *Store) List(limit int) ([]*types.Entry, error) {
	res := make([]*types.Entry, 0, limit)
	err := s.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(bucketEntries).Cursor()
		for k, v := c.Last(); k != nil && len(res) < limit; k, v = c.Prev() {
			var e types.Entry
			if err := gob.NewDecoder(bytes.NewReader(v)).Decode(&e); err != nil {
				return err
			}
			res = append(res, &e)
		}
		return nil
	})
	return res, err
}

func (s *Store) DeleteAll() error {
	return s.db.Update(func(tx *bolt.Tx) error {
		if err := tx.DeleteBucket(bucketEntries); err != nil {
			return fmt.Errorf("delete bucket: %w", err)
		}
		_, err := tx.CreateBucket(bucketEntries)
		return err
	})
}
