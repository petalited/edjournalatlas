package main

// Persistence for the in-memory Store -- a single gob-encoded cache file, not SQLite (see
// types.go's top comment for why). gob is Go's own stdlib binary serialization, so this adds
// no external dependency and keeps the whole program pure standard library.

import (
	"bufio"
	"encoding/gob"
	"os"
)

func LoadStore(path string) (*Store, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return NewStore(), nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	store := NewStore()
	dec := gob.NewDecoder(bufio.NewReader(f))
	if err := dec.Decode(store); err != nil {
		return nil, err
	}
	if store.Systems == nil {
		store.Systems = make(map[int64]*System)
	}
	if store.ParseProgress == nil {
		store.ParseProgress = make(map[string]FileProgress)
	}
	return store, nil
}

func SaveStore(path string, store *Store) error {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	enc := gob.NewEncoder(w)
	if err := enc.Encode(store); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := w.Flush(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
