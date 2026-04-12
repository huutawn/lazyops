package ui

import (
	"fmt"
	"io"
	"sync"

	"lazyops-cli/internal/redact"
)

type Output interface {
	Info(format string, args ...any)
	Success(format string, args ...any)
	Warn(format string, args ...any)
	Error(format string, args ...any)
	Print(format string, args ...any)
	Stdout() io.Writer
	Stderr() io.Writer
}

type ConsoleOutput struct {
	stdout io.Writer
	stderr io.Writer
	mu     sync.Mutex
}

func NewConsoleOutput(stdout io.Writer, stderr io.Writer) *ConsoleOutput {
	return &ConsoleOutput{
		stdout: stdout,
		stderr: stderr,
	}
}

func (o *ConsoleOutput) Info(format string, args ...any) {
	o.write(o.stdout, "[info] ", format, args...)
}

func (o *ConsoleOutput) Success(format string, args ...any) {
	o.write(o.stdout, "[ok] ", format, args...)
}

func (o *ConsoleOutput) Warn(format string, args ...any) {
	o.write(o.stderr, "[warn] ", format, args...)
}

func (o *ConsoleOutput) Error(format string, args ...any) {
	o.write(o.stderr, "[error] ", format, args...)
}

func (o *ConsoleOutput) Print(format string, args ...any) {
	o.write(o.stdout, "", format, args...)
}

func (o *ConsoleOutput) Stdout() io.Writer {
	return o.stdout
}

func (o *ConsoleOutput) Stderr() io.Writer {
	return o.stderr
}

func (o *ConsoleOutput) write(w io.Writer, prefix string, format string, args ...any) {
	o.mu.Lock()
	defer o.mu.Unlock()

	line := format
	if len(args) == 0 {
		fmt.Fprintf(w, "%s%s\n", prefix, redact.Text(line))
		return
	}

	line = fmt.Sprintf(format, args...)
	fmt.Fprintf(w, "%s%s\n", prefix, redact.Text(line))
}
