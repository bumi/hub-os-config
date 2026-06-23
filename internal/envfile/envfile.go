// Package envfile reads and edits dotenv-style files (KEY=VALUE) while
// preserving the original line order, comments, blank lines, and any keys the
// caller does not touch. It writes atomically so a crash mid-write can never
// leave a partially-written file. This matters because it edits Alby Hub's
// /opt/albyhub/.env, which contains keys this tool does not own.
package envfile

import (
	"os"
	"path/filepath"
	"strings"
)

// Doc is a parsed dotenv file. Use Parse or Load to obtain one.
type Doc struct {
	lines        []line
	finalNewline bool
}

type line struct {
	raw      string // verbatim original line (without its newline)
	isKV     bool
	key      string
	value    string
	modified bool // true once Set changed/added this line
}

// Parse parses dotenv content. Unrecognized lines (comments, blanks, anything
// without a key) are preserved verbatim.
func Parse(data []byte) *Doc {
	d := &Doc{}
	if len(data) == 0 {
		return d
	}
	s := string(data)
	d.finalNewline = strings.HasSuffix(s, "\n")
	if d.finalNewline {
		s = s[:len(s)-1]
	}
	for _, raw := range strings.Split(s, "\n") {
		isKV, key, value := parseLine(raw)
		d.lines = append(d.lines, line{raw: raw, isKV: isKV, key: key, value: value})
	}
	return d
}

func parseLine(raw string) (isKV bool, key, value string) {
	trimmed := strings.TrimLeft(raw, " \t")
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return false, "", ""
	}
	idx := strings.Index(raw, "=")
	if idx < 0 {
		return false, "", ""
	}
	key = strings.TrimSpace(raw[:idx])
	if key == "" {
		return false, "", ""
	}
	return true, key, raw[idx+1:]
}

// Get returns the current value for key and whether it is present.
func (d *Doc) Get(key string) (string, bool) {
	for _, l := range d.lines {
		if l.isKV && l.key == key {
			return l.value, true
		}
	}
	return "", false
}

// Set updates an existing key in place, or appends a new KEY=VALUE line.
func (d *Doc) Set(key, value string) {
	for i := range d.lines {
		if d.lines[i].isKV && d.lines[i].key == key {
			d.lines[i].value = value
			d.lines[i].modified = true
			return
		}
	}
	d.lines = append(d.lines, line{isKV: true, key: key, value: value, modified: true})
	d.finalNewline = true
}

// Render serializes the document back to bytes.
func (d *Doc) Render() []byte {
	if len(d.lines) == 0 {
		return nil
	}
	rendered := make([]string, len(d.lines))
	for i, l := range d.lines {
		if l.isKV && l.modified {
			rendered[i] = l.key + "=" + l.value
		} else {
			rendered[i] = l.raw
		}
	}
	out := strings.Join(rendered, "\n")
	if d.finalNewline {
		out += "\n"
	}
	return []byte(out)
}

// Load reads and parses the file at path. A missing file yields an empty Doc
// with no error, so callers can Load-then-Set-then-Save to create it.
func Load(path string) (*Doc, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Doc{}, nil
		}
		return nil, err
	}
	return Parse(data), nil
}

// Save writes the document to path atomically (temp file + rename). If path
// already exists its file mode is preserved.
func (d *Doc) Save(path string) error {
	mode := os.FileMode(0o600)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".env-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after a successful rename

	if _, err := tmp.Write(d.Render()); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
