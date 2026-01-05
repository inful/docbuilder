package main

import (
	"log/slog"

	"github.com/alecthomas/kong"

	"git.home.luguber.info/inful/docbuilder/cmd/docbuilder/commands"
	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
	"git.home.luguber.info/inful/docbuilder/internal/version"
)

func main() {
	cli := &commands.CLI{}
	parser := kong.Parse(cli,
		kong.Description("DocBuilder: aggregate multi-repo documentation into a Hugo site."),
		kong.Vars{"version": version.Version},
	)

	// Set up structured error handling
	logger := slog.Default()
	errorAdapter := errors.NewCLIErrorAdapter(cli.Verbose, logger)

	// Prepare globals (currently just logger already installed in AfterApply)
	globals := &commands.Global{Logger: logger}

	// Run command and handle errors uniformly
	if err := parser.Run(globals, cli); err != nil {
		errorAdapter.HandleError(err)
	}
}
