// Package system holds the production implementations that touch the host:
// rebooting and writing the Alby Hub .env file.
package system

import (
	"os"

	"github.com/getAlby/hub-os-config/internal/envfile"
)

// EnvStore reads and updates a fixed set of managed keys in a dotenv file,
// leaving all other content untouched. It implements web.EnvStore.
type EnvStore struct {
	path string
	keys []string
}

// NewEnvStore returns a store for the dotenv file at path that manages keys.
func NewEnvStore(path string, keys []string) *EnvStore {
	return &EnvStore{path: path, keys: keys}
}

// Get returns the current values of managed keys that are present in the file.
func (e *EnvStore) Get() (map[string]string, error) {
	doc, err := envfile.Load(e.path)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, k := range e.keys {
		if v, ok := doc.Get(k); ok {
			out[k] = v
		}
	}
	return out, nil
}

// Apply updates the given managed keys and writes the file atomically,
// preserving its original ownership where possible.
func (e *EnvStore) Apply(updates map[string]string) error {
	doc, err := envfile.Load(e.path)
	if err != nil {
		return err
	}
	for _, k := range e.keys {
		if v, ok := updates[k]; ok {
			doc.Set(k, v)
		}
	}

	uid, gid, hadOwner := fileOwner(e.path)
	if err := doc.Save(e.path); err != nil {
		return err
	}
	if hadOwner {
		// Best-effort: the atomic rename creates a root-owned file; restore the
		// original owner so a non-root Alby Hub can still read its .env.
		_ = os.Chown(e.path, uid, gid)
	}
	return nil
}
