package main

import (
	"log"
	"os"

	"github.com/waste3d/forge/cmd/forge/cli"
)

func main() {
	log.SetFlags(0)
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
