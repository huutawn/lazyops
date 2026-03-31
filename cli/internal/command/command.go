package command

import (
	"context"
	"fmt"

	"lazyops-cli/internal/transport"
	"lazyops-cli/internal/ui"
)

type RuntimeConfig struct {
	TransportMode string
	APIBaseURL    string
}

type Runtime struct {
	Output         ui.Output
	SpinnerFactory ui.SpinnerFactory
	Transport      transport.Transport
	Config         RuntimeConfig
}

type RunFunc func(ctx context.Context, runtime *Runtime, args []string) error

type Command struct {
	Name        string
	Summary     string
	Usage       string
	Subcommands []*Command
	Run         RunFunc
}

func (c *Command) Execute(ctx context.Context, runtime *Runtime, args []string) error {
	if len(args) == 0 {
		if c.Run != nil {
			return c.Run(ctx, runtime, args)
		}

		c.printHelp(runtime)
		return nil
	}

	switch args[0] {
	case "help", "-h", "--help":
		c.printHelp(runtime)
		return nil
	}

	if child := c.lookup(args[0]); child != nil {
		return child.Execute(ctx, runtime, args[1:])
	}

	if c.Run != nil {
		return c.Run(ctx, runtime, args)
	}

	c.printHelp(runtime)
	return fmt.Errorf("unknown command %q", args[0])
}

func (c *Command) lookup(name string) *Command {
	for _, child := range c.Subcommands {
		if child.Name == name {
			return child
		}
	}

	return nil
}

func (c *Command) printHelp(runtime *Runtime) {
	if c.Usage != "" {
		runtime.Output.Print("Usage: %s", c.Usage)
	}

	if c.Summary != "" {
		runtime.Output.Print("")
		runtime.Output.Print("%s", c.Summary)
	}

	if len(c.Subcommands) == 0 {
		return
	}

	runtime.Output.Print("")
	runtime.Output.Print("Commands:")
	for _, child := range c.Subcommands {
		runtime.Output.Print("  %-12s %s", child.Name, child.Summary)
	}
}
