package main

import (
	"log"
	"os"

	"github.com/jsmorph/adj/adc/runtime/cli"
)

func main() {
	if err := cli.Run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		log.Printf("error: %v", err)
		os.Exit(1)
	}
}
