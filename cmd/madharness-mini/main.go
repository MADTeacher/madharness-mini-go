package main

import (
	"os"

	"github.com/MADTeacher/madharness-mini-go/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:], os.Stdout, os.Stderr))
}
