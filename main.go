package main

import (
	"fmt"
	"os"
)

func main() {
	opts, err := loadOptions()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	runBenchmark(opts)
}
