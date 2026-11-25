// Command zerox renders a template set into a target folder with the values its command line gives, and renders a target again from its `zerox.yml`.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/litebitus/zerox/cmd"
)

var version = "dev"

func main() {
	workingFolder, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "zerox: failed to read the working folder:", err)
		os.Exit(1)
	}
	os.Exit(cmd.Run(version, os.Args[1:], workingFolder, os.Stdout, os.Stderr, time.Now))
}
