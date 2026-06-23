package app

import "github.com/getAlby/hub-os-config/internal/config"

// FileStateStore persists App state to a JSON file at a fixed path.
type FileStateStore struct {
	path string
}

// NewFileStateStore returns a store backed by the file at path.
func NewFileStateStore(path string) *FileStateStore {
	return &FileStateStore{path: path}
}

func (s *FileStateStore) Load() (config.State, error) {
	return config.LoadState(s.path)
}

func (s *FileStateStore) Save(st config.State) error {
	return config.SaveState(s.path, st)
}
