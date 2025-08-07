package main

import (
	"flag"
	"fmt"
	"os"
	"path"

	"github.com/quantifyearth/reclaimer/clms"
	"github.com/quantifyearth/reclaimer/zenodo"
)

type subcommand func([]string)

var subcommands = map[string]subcommand{
	"zenodo": zenodo.ZenodoMain,
	"clms":   clms.CLMSMain,
}

func main() {
	flag.Parse()
	args := flag.Args()

	if len(args) == 0 {
		execPath, err := os.Executable()
		if nil != err {
			panic(err)
		}
		fmt.Fprintf(os.Stderr, "Usage: %s [subcommand] [subcommand args]\n", path.Base(execPath))
		for cmd := range subcommands {
			fmt.Fprintf(os.Stderr, "\t%s\n", cmd)
		}
		os.Exit(1)
	}

	cmd, args := args[0], args[1:]

	if subcmd, ok := subcommands[cmd]; ok {
		subcmd(args)
	} else {
		fmt.Fprintf(os.Stderr, "Unrecognised subcommand. Options are:\n")
		for cmd := range subcommands {
			fmt.Fprintf(os.Stderr, "\t%s\n", cmd)
		}
		os.Exit(1)
	}
}
