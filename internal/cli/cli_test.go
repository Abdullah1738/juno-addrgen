package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

type fakeDeriver struct {
	deriveUFVK  string
	deriveIndex uint32
	deriveAddr  string
	deriveErr   error

	batchUFVK  string
	batchStart uint32
	batchCount uint32
	batchAddrs []string
	batchErr   error
}

func (f *fakeDeriver) Derive(ufvk string, index uint32) (string, error) {
	f.deriveUFVK = ufvk
	f.deriveIndex = index
	return f.deriveAddr, f.deriveErr
}

func (f *fakeDeriver) Batch(ufvk string, start uint32, count uint32) ([]string, error) {
	f.batchUFVK = ufvk
	f.batchStart = start
	f.batchCount = count
	return f.batchAddrs, f.batchErr
}

type codedErr string

func (e codedErr) Error() string      { return string(e) }
func (e codedErr) CodeString() string { return string(e) }

func TestDerive_Plain(t *testing.T) {
	d := &fakeDeriver{deriveAddr: "j1abc"}
	var out, err bytes.Buffer

	code := RunWithIO([]string{"derive", "--ufvk", "  jview1test  ", "--index", "5"}, d, &out, &err)
	if code != 0 {
		t.Fatalf("unexpected exit code: %d (stderr=%q)", code, err.String())
	}
	if got := out.String(); got != "j1abc\n" {
		t.Fatalf("unexpected stdout: %q", got)
	}
	if err.Len() != 0 {
		t.Fatalf("unexpected stderr: %q", err.String())
	}
	if d.deriveUFVK != "jview1test" {
		t.Fatalf("unexpected ufvk: %q", d.deriveUFVK)
	}
	if d.deriveIndex != 5 {
		t.Fatalf("unexpected index: %d", d.deriveIndex)
	}
}

func TestDerive_JSON(t *testing.T) {
	d := &fakeDeriver{deriveAddr: "j1abc"}
	var out, err bytes.Buffer

	code := RunWithIO([]string{"derive", "--ufvk", "jview1test", "--index", "0", "--json"}, d, &out, &err)
	if code != 0 {
		t.Fatalf("unexpected exit code: %d (stderr=%q)", code, err.String())
	}

	var v map[string]any
	if e := json.Unmarshal(out.Bytes(), &v); e != nil {
		t.Fatalf("invalid json: %v (%q)", e, out.String())
	}
	if v["status"] != "ok" || v["address"] != "j1abc" {
		t.Fatalf("unexpected json: %v", v)
	}
}

func TestDerive_ErrorCode(t *testing.T) {
	d := &fakeDeriver{deriveErr: codedErr("ufvk_invalid_bech32m")}
	var out, err bytes.Buffer

	code := RunWithIO([]string{"derive", "--ufvk", "bad", "--index", "0"}, d, &out, &err)
	if code != 1 {
		t.Fatalf("unexpected exit code: %d", code)
	}
	if got := strings.TrimSpace(err.String()); got != "ufvk_invalid_bech32m" {
		t.Fatalf("unexpected stderr: %q", got)
	}
}

func TestDerive_IndexOutOfRange(t *testing.T) {
	d := &fakeDeriver{deriveAddr: "j1abc"}
	var out, err bytes.Buffer

	code := RunWithIO([]string{"derive", "--ufvk", "jview1test", "--index", "4294967296"}, d, &out, &err)
	if code != 1 {
		t.Fatalf("unexpected exit code: %d", code)
	}
	if !strings.Contains(err.String(), "index_invalid") {
		t.Fatalf("unexpected stderr: %q", err.String())
	}
}

func TestBatch_Plain(t *testing.T) {
	d := &fakeDeriver{batchAddrs: []string{"j1a", "j1b"}}
	var out, err bytes.Buffer

	code := RunWithIO([]string{"batch", "--ufvk", "jview1test", "--start", "10", "--count", "2"}, d, &out, &err)
	if code != 0 {
		t.Fatalf("unexpected exit code: %d (stderr=%q)", code, err.String())
	}
	if got := out.String(); got != "j1a\nj1b\n" {
		t.Fatalf("unexpected stdout: %q", got)
	}
	if d.batchUFVK != "jview1test" || d.batchStart != 10 || d.batchCount != 2 {
		t.Fatalf("unexpected batch call: ufvk=%q start=%d count=%d", d.batchUFVK, d.batchStart, d.batchCount)
	}
}

func TestUFVKEnv(t *testing.T) {
	t.Setenv("JUNO_TEST_UFVK", "jview1fromenv")

	d := &fakeDeriver{deriveAddr: "j1abc"}
	var out, err bytes.Buffer

	code := RunWithIO([]string{"derive", "--ufvk-env", "JUNO_TEST_UFVK", "--index", "0"}, d, &out, &err)
	if code != 0 {
		t.Fatalf("unexpected exit code: %d (stderr=%q)", code, err.String())
	}
	if d.deriveUFVK != "jview1fromenv" {
		t.Fatalf("unexpected ufvk: %q", d.deriveUFVK)
	}
}

func TestUFVKSourceConflict(t *testing.T) {
	d := &fakeDeriver{deriveAddr: "j1abc"}
	var out, err bytes.Buffer

	code := RunWithIO([]string{"derive", "--ufvk", "jview1a", "--ufvk-env", "JUNO_TEST_UFVK", "--index", "0"}, d, &out, &err)
	if code != 2 {
		t.Fatalf("unexpected exit code: %d", code)
	}
	if err.Len() == 0 {
		t.Fatalf("expected stderr")
	}
	_ = out
}

func TestHelp(t *testing.T) {
	d := &fakeDeriver{deriveAddr: "j1abc"}
	var out, err bytes.Buffer

	code := RunWithIO([]string{"--help"}, d, &out, &err)
	if code != 0 {
		t.Fatalf("unexpected exit code: %d", code)
	}
	if !strings.Contains(out.String(), "Usage:") {
		t.Fatalf("expected usage, got: %q", out.String())
	}
}
