package bq

import (
	"os"

	"github.com/alecthomas/kong"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Read the command-line arguments and return the parsed structure.
func ParseAndRun() error {
	var args Args

	options := []kong.Option{
		kong.Name("bq"),
		kong.Description("The binary query and modification tool."),
		kong.UsageOnError(),
	}

	kong.Parse(&args, options...)
	return args.Run()
}

// The command-line interface of the `bq` that setup and runs the
// processing based on user inputs.
type Args struct {
	// The verbosity level.
	Verbose int `help:"Increase verbosity level." short:"v" type:"counter"`

	// The expression to be applied on the file content, omitted means reading and
	// printing the content as is.
	Expr *string `help:"The expression to be applied on the file content." arg:"" optional:""`

	// The file content to be processed, or read from stdin if '-' is given.
	File *os.File `help:"The file to be processed, or '-' for stdin." short:"f" arg:"" default:"-"`
}

// Run and return any error encountered during processing.
func (a *Args) Run() error {
	a.prologue()
	defer a.epilogue()

	return a.run()
}

// Setup everything before running the main logic, such as logging or others
func (a *Args) prologue() {
	switch a.Verbose {
	case 0:
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	case 1:
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case 2:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case 3:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	}

	writer := zerolog.ConsoleWriter{Out: os.Stderr}
	log.Logger = zerolog.New(writer).With().Timestamp().Logger()

	log.Debug().Int("verbosity", a.Verbose).Msg("completed prologue ...")
}

// Clean-up everything after running the main logic.
func (a *Args) epilogue() {
	log.Debug().Msg("completed epilogue ...")
}

// The main logic of the `bq` to be executed after setup.
func (a *Args) run() error {
	log.Debug().Any("args", a).Msg("running ...")

	if a.Expr == nil {
		log.Info().Msg("no expression provided, nothing to do")
		return nil
	}

	return Execute(*a.Expr, a.File)
}
