package main

import (
	"os"

	"github.com/Abdullah1738/juno-addrgen/internal/cli"
	"github.com/Abdullah1738/juno-addrgen/pkg/addrgen"
)

type deriver struct{}

func (deriver) Derive(ufvk string, index uint32) (string, error) {
	return addrgen.Derive(ufvk, index)
}

func (deriver) Batch(ufvk string, start uint32, count uint32) ([]string, error) {
	return addrgen.Batch(ufvk, start, count)
}

func main() {
	os.Exit(cli.Run(os.Args[1:], deriver{}))
}
