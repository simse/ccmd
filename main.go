package main

import (
	"fmt"

	"github.com/alexflint/go-arg"
	"github.com/simse/ccmd/commands"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

type args struct {
	CacheList *commands.CacheListCmd   `arg:"subcommand:cache-list"`
	Run       *commands.RunCommandArgs `arg:"subcommand:run"`
}

func (args) Version() string {
	return fmt.Sprintf("ccmd %s", version)
}

func main() {
	var args args

	arg.MustParse(&args)

	switch {
	case args.Run != nil:
		commands.RunCommand(args.Run)
		return
	case args.CacheList != nil:
		commands.CacheListCommand()
		return
	}
}
