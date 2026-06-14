package main

import (
	"os"

	"github.com/MADTeacher/madharness-mini-go/internal/cli"
)

func main() {
	os.Exit(cli.MainWithInput(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
