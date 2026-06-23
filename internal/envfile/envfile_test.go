package envfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseRenderRoundTripsUnchanged(t *testing.T) {
	input := "# Alby Hub config\n" +
		"RELAY=wss://relay.example.com\n" +
		"\n" +
		"# backend\n" +
		"LN_BACKEND_TYPE=LDK\n"

	doc := Parse([]byte(input))
	got := string(doc.Render())

	if got != input {
		t.Fatalf("round-trip changed content.\n got: %q\nwant: %q", got, input)
	}
}

func TestParseRoundTripsWithoutTrailingNewline(t *testing.T) {
	input := "RELAY=wss://relay.example.com"
	doc := Parse([]byte(input))
	if got := string(doc.Render()); got != input {
		t.Fatalf("round-trip changed content.\n got: %q\nwant: %q", got, input)
	}
}

func TestGet(t *testing.T) {
	doc := Parse([]byte("RELAY=wss://r\nLN_BACKEND_TYPE=BARK\n"))

	if v, ok := doc.Get("RELAY"); !ok || v != "wss://r" {
		t.Fatalf("Get(RELAY) = %q,%v; want wss://r,true", v, ok)
	}
	if v, ok := doc.Get("LN_BACKEND_TYPE"); !ok || v != "BARK" {
		t.Fatalf("Get(LN_BACKEND_TYPE) = %q,%v; want BARK,true", v, ok)
	}
	if _, ok := doc.Get("MISSING"); ok {
		t.Fatalf("Get(MISSING) reported present")
	}
}

func TestSetUpdatesExistingKeyInPlace(t *testing.T) {
	input := "# header\n" +
		"RELAY=old\n" +
		"OTHER=keepme\n" +
		"LN_BACKEND_TYPE=LDK\n"
	doc := Parse([]byte(input))

	doc.Set("RELAY", "new")

	want := "# header\n" +
		"RELAY=new\n" +
		"OTHER=keepme\n" +
		"LN_BACKEND_TYPE=LDK\n"
	if got := string(doc.Render()); got != want {
		t.Fatalf("Set in place failed.\n got: %q\nwant: %q", got, want)
	}
}

func TestSetAppendsNewKey(t *testing.T) {
	input := "RELAY=r\n"
	doc := Parse([]byte(input))

	doc.Set("LDK_ESPLORA_SERVER", "https://esplora.example.com")

	want := "RELAY=r\nLDK_ESPLORA_SERVER=https://esplora.example.com\n"
	if got := string(doc.Render()); got != want {
		t.Fatalf("Set append failed.\n got: %q\nwant: %q", got, want)
	}
}

func TestSetAppendsNewKeyWhenFileLacksTrailingNewline(t *testing.T) {
	doc := Parse([]byte("RELAY=r"))
	doc.Set("LN_BACKEND_TYPE", "BARK")

	want := "RELAY=r\nLN_BACKEND_TYPE=BARK\n"
	if got := string(doc.Render()); got != want {
		t.Fatalf("append without trailing newline failed.\n got: %q\nwant: %q", got, want)
	}
}

func TestValueWithEqualsSignPreserved(t *testing.T) {
	doc := Parse([]byte("RELAY=wss://r?token=abc=def\n"))
	if v, _ := doc.Get("RELAY"); v != "wss://r?token=abc=def" {
		t.Fatalf("value with '=' mishandled: got %q", v)
	}
}

func TestLoadMissingFileReturnsEmptyDoc(t *testing.T) {
	doc, err := Load(filepath.Join(t.TempDir(), "does-not-exist.env"))
	if err != nil {
		t.Fatalf("Load missing file errored: %v", err)
	}
	if _, ok := doc.Get("ANYTHING"); ok {
		t.Fatalf("empty doc reported a key")
	}
	doc.Set("RELAY", "x")
	if got := string(doc.Render()); got != "RELAY=x\n" {
		t.Fatalf("empty doc set/render wrong: %q", got)
	}
}

func TestSaveWritesAtomicallyAndLeavesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "albyhub.env")

	doc := Parse([]byte("# keep\nRELAY=old\n"))
	doc.Set("RELAY", "new")
	doc.Set("LN_BACKEND_TYPE", "LDK")
	if err := doc.Save(path); err != nil {
		t.Fatalf("Save errored: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading saved file: %v", err)
	}
	want := "# keep\nRELAY=new\nLN_BACKEND_TYPE=LDK\n"
	if string(data) != want {
		t.Fatalf("saved content wrong.\n got: %q\nwant: %q", string(data), want)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("expected only the target file, found: %v", names)
	}
}

func TestLoadThenSaveRoundTripsOnDisk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "albyhub.env")
	original := "# comment\nUNRELATED=stays\nRELAY=old\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	doc, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	doc.Set("RELAY", "changed")
	if err := doc.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	data, _ := os.ReadFile(path)
	want := "# comment\nUNRELATED=stays\nRELAY=changed\n"
	if string(data) != want {
		t.Fatalf("disk round-trip wrong.\n got: %q\nwant: %q", string(data), want)
	}
}
