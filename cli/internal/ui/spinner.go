package ui

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"lazyops-cli/internal/redact"
)

type Spinner interface {
	Start(message string)
	Update(message string)
	Stop(message string)
}

type SpinnerFactory interface {
	New() Spinner
}

type spinnerFactory struct {
	writer  io.Writer
	enabled bool
}

func NewSpinnerFactory(writer io.Writer) SpinnerFactory {
	return &spinnerFactory{
		writer:  writer,
		enabled: isTerminal(writer),
	}
}

func (f *spinnerFactory) New() Spinner {
	if !f.enabled {
		return noopSpinner{}
	}

	return &terminalSpinner{
		writer: f.writer,
		frames: []string{"-", "\\", "|", "/"},
	}
}

type noopSpinner struct{}

func (noopSpinner) Start(string)  {}
func (noopSpinner) Update(string) {}
func (noopSpinner) Stop(string)   {}

type terminalSpinner struct {
	writer  io.Writer
	frames  []string
	message string

	mu      sync.Mutex
	stopCh  chan struct{}
	doneCh  chan struct{}
	running bool
}

func (s *terminalSpinner) Start(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		s.message = message
		return
	}

	s.message = message
	s.stopCh = make(chan struct{})
	s.doneCh = make(chan struct{})
	s.running = true

	go s.loop()
}

func (s *terminalSpinner) Update(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.message = message
}

func (s *terminalSpinner) Stop(message string) {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}

	stopCh := s.stopCh
	doneCh := s.doneCh
	s.running = false
	s.stopCh = nil
	s.doneCh = nil
	s.mu.Unlock()

	close(stopCh)
	<-doneCh

	if message != "" {
		fmt.Fprintf(s.writer, "%s\n", redact.Text(message))
	}
}

func (s *terminalSpinner) loop() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	defer close(s.doneCh)

	frameIndex := 0
	for {
		s.mu.Lock()
		message := s.message
		s.mu.Unlock()

		fmt.Fprintf(s.writer, "\r%s %s", s.frames[frameIndex%len(s.frames)], redact.Text(message))
		frameIndex++

		select {
		case <-ticker.C:
		case <-s.stopCh:
			fmt.Fprint(s.writer, "\r\033[K")
			return
		}
	}
}

func isTerminal(writer io.Writer) bool {
	file, ok := writer.(*os.File)
	if !ok {
		return false
	}

	info, err := file.Stat()
	if err != nil {
		return false
	}

	return (info.Mode() & os.ModeCharDevice) != 0
}
