// Command g is a personal Git and GitHub workflow assistant.
//
// Running g with no arguments prints a short help listing; `g --help` prints the
// full command reference.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/roshandroids/g/internal/command"
	"github.com/roshandroids/g/internal/config"
	"github.com/roshandroids/g/internal/git"
	"github.com/roshandroids/g/internal/github"
	"github.com/roshandroids/g/internal/process"
	"github.com/roshandroids/g/internal/workflow"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

// run wires the layers together and returns the process exit code.
//
// The dependency chain mirrors the architecture: commands drive workflows, which
// drive the git client, which drives the process runner that executes the
// installed git executable.
func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "g: cannot determine working directory: %v\n", err)
		return command.ExitError
	}

	cfg, err := config.Load(config.DefaultPath())
	if err != nil {
		fmt.Fprintf(stderr, "g: %v; using defaults\n", err)
		cfg = config.Default()
	}

	runner := process.ExecRunner{}
	env := &command.Env{
		Service: workflow.NewService(git.NewClient(runner)),
		GitHub:  github.NewExecClient(runner),
		Config:  cfg,
		Dir:     dir,
		Out:     stdout,
		Err:     stderr,
	}

	return command.Run(ctx, env, args)
}
