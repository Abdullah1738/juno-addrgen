package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Deriver interface {
	Derive(ufvk string, index uint32) (string, error)
	Batch(ufvk string, start uint32, count uint32) ([]string, error)
}

func Run(args []string, deriver Deriver) int {
	return RunWithIO(args, deriver, os.Stdout, os.Stderr)
}

func RunWithIO(args []string, deriver Deriver, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		writeUsage(stdout)
		return 2
	}

	switch args[0] {
	case "-h", "--help", "help":
		writeUsage(stdout)
		return 0
	case "derive":
		if deriver == nil {
			return writeErr(stdout, stderr, false, "internal", "missing deriver")
		}
		return runDerive(args[1:], deriver, stdout, stderr)
	case "batch":
		if deriver == nil {
			return writeErr(stdout, stderr, false, "internal", "missing deriver")
		}
		return runBatch(args[1:], deriver, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n\n", args[0])
		writeUsage(stderr)
		return 2
	}
}

func writeUsage(w io.Writer) {
	fmt.Fprintln(w, "juno-addrgen")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Offline address derivation (UFVK + index -> j1...) for Juno Cash.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  juno-addrgen derive --ufvk <jview1...> --index <n> [--json]")
	fmt.Fprintln(w, "  juno-addrgen batch  --ufvk <jview1...> --start <n> --count <k> [--json]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Notes:")
	fmt.Fprintln(w, "  - UFVKs are sensitive (watch-only, but reveal incoming transaction details).")
	fmt.Fprintln(w, "  - This tool is offline; it never talks to junocashd or the network.")
}

func runDerive(args []string, deriver Deriver, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("derive", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var ufvkFlag string
	var ufvkFile string
	var ufvkEnv string
	var index uint64
	var jsonOut bool

	fs.StringVar(&ufvkFlag, "ufvk", "", "UFVK (jview1...)")
	fs.StringVar(&ufvkFile, "ufvk-file", "", "Read UFVK from file")
	fs.StringVar(&ufvkEnv, "ufvk-env", "", "Read UFVK from env var (name)")
	fs.Uint64Var(&index, "index", 0, "Diversifier index (0..2^32-1)")
	fs.BoolVar(&jsonOut, "json", false, "JSON output")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 2
	}

	ufvk, err := readUFVK(ufvkFlag, ufvkFile, ufvkEnv)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 2
	}

	idx, ok := uint64ToUint32(index)
	if !ok {
		return writeErr(stdout, stderr, jsonOut, "index_invalid", "index out of range")
	}

	address, err := deriver.Derive(ufvk, idx)
	if err != nil {
		return writeDeriverErr(stdout, stderr, jsonOut, err)
	}

	if jsonOut {
		_ = json.NewEncoder(stdout).Encode(map[string]any{
			"status":  "ok",
			"address": address,
		})
		return 0
	}

	fmt.Fprintln(stdout, address)
	return 0
}

func runBatch(args []string, deriver Deriver, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("batch", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var ufvkFlag string
	var ufvkFile string
	var ufvkEnv string
	var start uint64
	var count uint64
	var jsonOut bool

	fs.StringVar(&ufvkFlag, "ufvk", "", "UFVK (jview1...)")
	fs.StringVar(&ufvkFile, "ufvk-file", "", "Read UFVK from file")
	fs.StringVar(&ufvkEnv, "ufvk-env", "", "Read UFVK from env var (name)")
	fs.Uint64Var(&start, "start", 0, "Start diversifier index (0..2^32-1)")
	fs.Uint64Var(&count, "count", 0, "Number of addresses (1..100000)")
	fs.BoolVar(&jsonOut, "json", false, "JSON output")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 2
	}

	ufvk, err := readUFVK(ufvkFlag, ufvkFile, ufvkEnv)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 2
	}

	s, ok := uint64ToUint32(start)
	if !ok {
		return writeErr(stdout, stderr, jsonOut, "index_invalid", "start out of range")
	}
	c, ok := uint64ToUint32(count)
	if !ok || c == 0 {
		return writeErr(stdout, stderr, jsonOut, "count_invalid", "count out of range")
	}

	addresses, err := deriver.Batch(ufvk, s, c)
	if err != nil {
		return writeDeriverErr(stdout, stderr, jsonOut, err)
	}

	if jsonOut {
		_ = json.NewEncoder(stdout).Encode(map[string]any{
			"status":    "ok",
			"start":     s,
			"count":     c,
			"addresses": addresses,
		})
		return 0
	}

	for _, a := range addresses {
		fmt.Fprintln(stdout, a)
	}
	return 0
}

func readUFVK(ufvkFlag, ufvkFile, ufvkEnv string) (string, error) {
	var sources int
	if strings.TrimSpace(ufvkFlag) != "" {
		sources++
	}
	if strings.TrimSpace(ufvkFile) != "" {
		sources++
	}
	if strings.TrimSpace(ufvkEnv) != "" {
		sources++
	}
	if sources == 0 {
		return "", fmt.Errorf("ufvk is required (use --ufvk, --ufvk-file, or --ufvk-env)")
	}
	if sources > 1 {
		return "", fmt.Errorf("ufvk source conflict (use only one of --ufvk, --ufvk-file, --ufvk-env)")
	}

	if strings.TrimSpace(ufvkFlag) != "" {
		return strings.TrimSpace(ufvkFlag), nil
	}

	if strings.TrimSpace(ufvkEnv) != "" {
		return strings.TrimSpace(os.Getenv(strings.TrimSpace(ufvkEnv))), nil
	}

	path := strings.TrimSpace(ufvkFile)
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read ufvk file (%s): %w", filepath.Base(path), err)
	}
	return strings.TrimSpace(string(b)), nil
}

func uint64ToUint32(v uint64) (uint32, bool) {
	if v > uint64(^uint32(0)) {
		return 0, false
	}
	return uint32(v), true
}

type codedError interface {
	error
	CodeString() string
}

func writeDeriverErr(stdout, stderr io.Writer, jsonOut bool, err error) int {
	var ce codedError
	if errors.As(err, &ce) {
		return writeErr(stdout, stderr, jsonOut, ce.CodeString(), "")
	}
	return writeErr(stdout, stderr, jsonOut, "internal", err.Error())
}

func writeErr(stdout, stderr io.Writer, jsonOut bool, code, message string) int {
	if jsonOut {
		_ = json.NewEncoder(stdout).Encode(map[string]any{
			"status": "err",
			"error":  code,
		})
		return 1
	}

	if message == "" {
		fmt.Fprintln(stderr, code)
		return 1
	}
	fmt.Fprintf(stderr, "%s: %s\n", code, message)
	return 1
}
