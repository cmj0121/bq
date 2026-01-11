package main

import (
	"os"

	"github.com/cmj0121/bq"
)

func main() {
	if err := bq.ParseAndRun(); err != nil {
		os.Exit(1)
	}
}
