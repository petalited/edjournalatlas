package main

// Persistence for the in-memory Store -- a single JSON-encoded cache file, not SQLite (see
// types.go's top comment for why) and not gob either, despite gob being the more obvious pure-
// stdlib choice: gob has a real, confirmed bug for this data shape. gob's struct encoder skips
// fields whose value is the zero value of their type as a size optimization, but for a pointer
// field that optimization incorrectly triggers off the POINTED-TO value being zero, not the
// pointer itself being nil -- so a `*bool` pointing to `false` (or a `*int` pointing to `0`,
// e.g. ParentStarBodyID for a body orbiting the system's primary star, which is BodyID 0 in the
// overwhelming majority of real systems) round-trips through a single gob Encode/Decode as `nil`,
// indistinguishable from "never set." Reproduced directly (isolated Go program, not a guess):
// encoding a `*bool` pointing to `false` and decoding it back on the very next line already loses
// it. In practice this meant every WasFootfalled/WasDiscovered/WasMapped/WasLogged `false` (and
// every ParentStarBodyID of 0) silently vanished on the run immediately after the one that first
// recorded it. encoding/json has no such issue -- confirmed against the same reproduction -- at
// the cost of a larger, slower-to-(de)serialize cache file, an acceptable trade for a purely
// local, disk-only cache.
//
// LoadStore treats ANY decode failure (including an old gob-format file from before this fix) as
// "start fresh" rather than a hard error -- this cache is always fully rebuildable from the real
// journal, so failing to load it should trigger a full reparse, not abort the program.

import (
	"bufio"
	"encoding/json"
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
	dec := json.NewDecoder(bufio.NewReader(f))
	if err := dec.Decode(store); err != nil {
		return NewStore(), nil
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
	enc := json.NewEncoder(w)
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
