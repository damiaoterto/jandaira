package mcp

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"
)

// StdioTransport launches an MCP server as a child process and communicates
// with it via its stdin/stdout using newline-delimited JSON-RPC.
type StdioTransport struct {
	// Command is the executable and its arguments, e.g. ["npx", "-y", "@mcp/server-postgres", "postgres://..."].
	Command []string
	// Env holds extra environment variables passed to the child process (KEY=VALUE format).
	Env []string

	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	outChan chan []byte
}

// NewStdioTransport creates a StdioTransport for the given command tokens.
func NewStdioTransport(command []string, env []string) *StdioTransport {
	return &StdioTransport{
		Command: command,
		Env:     env,
		outChan: make(chan []byte, 64),
	}
}

// Start launches the child process and wires up the I/O pipes.
// The process lifetime is managed by Close(), not by ctx — ctx is used only
// for blocking handshake operations (initialize, ListTools) performed after Start returns.
func (t *StdioTransport) Start(_ context.Context) error {
	if len(t.Command) == 0 {
		return fmt.Errorf("mcp stdio: empty command")
	}

	t.cmd = exec.Command(t.Command[0], t.Command[1:]...)

	// Inherit the parent environment and append extras.
	if len(t.Env) > 0 {
		t.cmd.Env = append(t.cmd.Environ(), t.Env...)
	}

	// MCP servers must not write protocol output to stderr; redirect it to the
	// parent process logger so operator can debug server-side issues.
	t.cmd.Stderr = &stderrLogger{prefix: "[mcp-stdio] "}

	var err error
	t.stdin, err = t.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("mcp stdio: stdin pipe: %w", err)
	}

	t.stdout, err = t.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("mcp stdio: stdout pipe: %w", err)
	}

	if err := t.cmd.Start(); err != nil {
		return fmt.Errorf("mcp stdio: start process: %w", err)
	}

	go t.readLoop()
	return nil
}

// Send writes a JSON-RPC message followed by a newline to the child's stdin.
func (t *StdioTransport) Send(_ context.Context, msg []byte) error {
	_, err := t.stdin.Write(append(msg, '\n'))
	return err
}

// Receive returns the channel that delivers lines read from the child's stdout.
func (t *StdioTransport) Receive() (<-chan []byte, error) {
	return t.outChan, nil
}

// Close kills the child process and reaps it to prevent zombies.
func (t *StdioTransport) Close() error {
	if t.stdin != nil {
		_ = t.stdin.Close()
	}
	if t.cmd != nil && t.cmd.Process != nil {
		_ = t.cmd.Process.Kill()
		_ = t.cmd.Wait() // reap; error expected (killed), intentionally ignored
	}
	return nil
}

// readLoop scans stdout line by line and forwards each JSON line to outChan.
func (t *StdioTransport) readLoop() {
	defer close(t.outChan)
	scanner := bufio.NewScanner(t.stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1 MiB max line
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		cp := make([]byte, len(line))
		copy(cp, line)
		t.outChan <- cp
	}
}

// stderrLogger forwards MCP server stderr to the Go logger.
type stderrLogger struct {
	prefix string
	buf    strings.Builder
}

func (w *stderrLogger) Write(p []byte) (int, error) {
	for _, b := range p {
		if b == '\n' {
			log.Printf("%s%s", w.prefix, w.buf.String())
			w.buf.Reset()
		} else {
			w.buf.WriteByte(b)
		}
	}
	return len(p), nil
}
