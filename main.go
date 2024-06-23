package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"text/template"
	"time"

	_ "embed"

	"github.com/avamsi/climate"
	"github.com/avamsi/ergo/assert"
	"github.com/djherbis/atime"
	"github.com/erikgeiser/promptkit"
	"github.com/erikgeiser/promptkit/selection"
)

// axl watches over your commands.
type axl struct{}

//go:embed hooks.zsh
var zsh string

// Shell hooks.
type hooks struct{}

// Zsh hooks.
//
//	source <(axl hooks zsh)
func (*hooks) Zsh() {
	fmt.Print(zsh)
}

// TODO: maybe consider making the log file (or the username) configurable.
func (*axl) log() string {
	return fmt.Sprintf("/tmp/%s.axl", assert.Ok(user.Current()).Username)
}

func tail(ctx context.Context, file string) <-chan string {
	cmd := exec.CommandContext(ctx, "tail", "-n 42", "-f", file)
	cmd.Stderr = os.Stderr
	var (
		out    = bufio.NewScanner(assert.Ok(cmd.StdoutPipe()))
		stream = make(chan string)
	)
	assert.Nil(cmd.Start())
	go func() {
		defer close(stream)
		for out.Scan() {
			stream <- out.Text()
		}
		assert.Nil(out.Err())
	}()
	return stream
}

func (*axl) list(stream <-chan string) []string {
	var (
		cmds []string
		done = make(map[string]bool)
	)
	for {
		select {
		case line := <-stream:
			switch line[:2] {
			case "+ ":
				cmds = append(cmds, line[2:])
			case "- ":
				_, cmd, ok := strings.Cut(line[2:], " ")
				assert.Truef(ok, "not exit code and command: %v", line[2:])
				done[cmd] = true
			default:
				panic(line)
			}
		case <-time.After(42 * time.Millisecond):
			var out []string
			for _, cmd := range cmds {
				if !done[cmd] {
					// TODO: maybe weed out axl list and axl wait?
					out = append(out, cmd)
				}
			}
			return out
		}
	}
}

func beautify(cmd string) string {
	secs, cmd, ok := strings.Cut(cmd, " ")
	assert.Truef(ok, "not start time and command: %v", cmd)
	t := time.Unix(assert.Ok(strconv.ParseInt(secs, 10, 64)), 0)
	return fmt.Sprintf("[⌚ %s] 💲 %s", t.Format("02 Jan 15:04"), cmd)
}

// List currently running commands.
func (a *axl) List(ctx context.Context) {
	for _, cmd := range a.list(tail(ctx, a.log())) {
		fmt.Println(beautify(cmd))
	}
}

type choice string

func (c choice) String() string {
	return beautify(string(c))
}

// Wait for a command to finish running.
func (a *axl) Wait(ctx context.Context) error {
	var (
		stream  = tail(ctx, a.log())
		cmds    = a.list(stream)
		waitFor string
	)
	switch len(cmds) {
	case 0:
		return nil
	case 1:
		waitFor = cmds[0]
	default:
		var choices []choice
		for _, cmd := range cmds {
			choices = append(choices, choice(cmd))
		}
		sp := selection.New("", choices)
		sp.FilterPlaceholder = ""
		sp.ResultTemplate = ""
		sel, err := sp.RunPrompt()
		waitFor = string(sel)
		if errors.Is(err, promptkit.ErrAborted) {
			return climate.ErrExit(130)
		}
		assert.Nil(err)
	}
	fmt.Println("⌛", beautify(waitFor))
	for line := range stream {
		switch line[:2] {
		case "+ ": // Do nothing.
		case "- ":
			code, cmd, ok := strings.Cut(line[2:], " ")
			assert.Truef(ok, "not exit code and command: %v", line[2:])
			if cmd == waitFor {
				return climate.ErrExit(assert.Ok(strconv.Atoi(code)))
			}
		default:
			panic(line)
		}
	}
	return io.ErrUnexpectedEOF
}

// axl internal commands, not for general use.
type internal struct{}

type notifyOptions struct {
	Cmd       string
	StartTime int64 // seconds from epoch
	Code      int   // exit code
}

const threshold = 42 * time.Second

// Notify the user that a command has finished running.
func (*internal) Notify(opts *notifyOptions) {
	if opts.Code == 130 {
		// 130 is SIGINT (mostly from Ctrl-C), no need to notify.
		return
	}
	var (
		start   = time.Unix(opts.StartTime, 0)
		now     = time.Now()
		elapsed = now.Sub(start)
	)
	if elapsed < threshold {
		return
	}
	// Use last access time of stdin as a proxy for user interaction.
	interaction := atime.Get(assert.Ok(os.Stdin.Stat()))
	if now.Sub(interaction) < threshold {
		return
	}
	msg := `💲 {{.command}}
⌚ {{.start}} + ⌛ {{.elapsed}}{{if ne .code 0}} -> 🙅 {{.code}}{{end}} @ 💻 {{.host}}`
	if v, ok := os.LookupEnv("AXL_MESSAGE"); ok {
		msg = v
	}
	var (
		t   = template.Must(template.New("msg").Parse(msg))
		err = t.Execute(os.Stdout, map[string]any{
			"command": opts.Cmd,
			"start":   start.Format(time.Kitchen),
			"elapsed": elapsed.Round(time.Second),
			"code":    opts.Code,
			"host":    assert.Ok(os.Hostname()),
		})
	)
	assert.Nil(err)
}

//go:generate go run github.com/avamsi/climate/cmd/climate --out=md.cli
//go:embed md.cli
var md []byte

func main() {
	p := climate.Struct[axl](climate.Struct[hooks](), climate.Struct[internal]())
	climate.RunAndExit(p, climate.WithMetadata(md))
}
