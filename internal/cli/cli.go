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

	"github.com/Abdullah1738/juno-addrgen/pkg/addrgen"
)

func Run(args []string) int {
	if len(args) == 0 {
		writeUsage(os.Stdout)
		return 2
	}

	switch args[0] {
	case "-h", "--help", "help":
		writeUsage(os.Stdout)
		return 0
	case "derive":
		return runDerive(args[1:], os.Stdout, os.Stderr)
	case "batch":
		return runBatch(args[1:], os.Stdout, os.Stderr)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", args[0])
		writeUsage(os.Stderr)
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

func runDerive(args []string, stdout, stderr io.Writer) int {
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

	address, err := addrgen.Derive(ufvk, idx)
	if err != nil {
		return writeAddrgenErr(stdout, stderr, jsonOut, err)
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

func runBatch(args []string, stdout, stderr io.Writer) int {
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

	addresses, err := addrgen.Batch(ufvk, s, c)
	if err != nil {
		return writeAddrgenErr(stdout, stderr, jsonOut, err)
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

func writeAddrgenErr(stdout, stderr io.Writer, jsonOut bool, err error) int {
	var ae *addrgen.Error
	if errors.As(err, &ae) {
		return writeErr(stdout, stderr, jsonOut, string(ae.Code), "")
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
