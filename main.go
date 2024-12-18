package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path"
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
	"github.com/google/shlex"
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

const timeLayout = "02 Jan 03:04 PM"

func beautify(cmd string) string {
	secs, cmd, ok := strings.Cut(cmd, " ")
	assert.Truef(ok, "not start time and command: %v", cmd)
	t := time.Unix(assert.Ok(strconv.ParseInt(secs, 10, 64)), 0)
	return fmt.Sprintf("[⌚ %s] 💲 %s", t.Format(timeLayout), cmd)
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
				secs, cmd, ok := strings.Cut(cmd, " ")
				assert.Truef(ok, "not start time and command: %v", cmd)
				var (
					start   = time.Unix(assert.Ok(strconv.ParseInt(secs, 10, 64)), 0)
					elapsed = time.Since(start).Round(time.Second)
					status  string
				)
				if code != "0" {
					status = " -> 🙅 " + code
				}
				fmt.Printf("\033[F[⌚ %s + ⌛ %v%s] 💲 %s\n",
					start.Format(timeLayout), elapsed, status, cmd)
				return climate.ErrExit(assert.Ok(strconv.Atoi(code)))
			}
		default:
			panic(line)
		}
	}
	panic(io.ErrUnexpectedEOF)
}

// Internal commands, not for general use.
type internal struct{}

type renderOptions struct {
	StartTime int64 // seconds from epoch
	Code      int   // exit code
}

const threshold = 42 * time.Second

// Render a command that has finished running.
func (*internal) Render(opts *renderOptions, cmd string) {
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
			"command": cmd,
			"start":   start.Format(timeLayout),
			"elapsed": elapsed.Round(time.Second),
			"code":    opts.Code,
			"host":    assert.Ok(os.Hostname()),
		})
	)
	assert.Nil(err)
}

// Suggest a command to run based on the most recently executed command.
func (*internal) Suggest(cmd string) {
	switch parts := assert.Ok(shlex.Split(cmd)); len(parts) {
	case 2:
		if parts[0] == "mkdir" {
			fmt.Println("cd", parts[1])
		}
	case 3:
		if parts[0] == "git" && parts[1] == "clone" {
			fmt.Println("cd", path.Base(parts[2]))
		}
	case 4:
		if parts[0] == "gh" && parts[1] == "repo" && parts[2] == "clone" {
			fmt.Println("cd", path.Base(parts[3]))
		}
	}
}

// Custom plugins for when CLIs are not readily available.
type plugins struct{}

type mattermostOptions struct {
	webhookEndpoint string
}

// Mattermost a simple text message to the webhook endpoint.
func (*plugins) Mattermost(opts *mattermostOptions) {
	var (
		msg     = assert.Ok(io.ReadAll(os.Stdin))
		payload = assert.Ok(json.Marshal(map[string]string{"text": string(msg)}))
		resp    = assert.Ok(http.Post(
			opts.webhookEndpoint, "application/json", bytes.NewReader(payload)))
	)
	defer resp.Body.Close()
	assert.True(resp.StatusCode == http.StatusOK, string(assert.Ok(io.ReadAll(resp.Body))))
}

//go:generate go run github.com/avamsi/climate/cmd/climate --out=md.cli
//go:embed md.cli
var md []byte

func main() {
	p := climate.Struct[axl](
		climate.Struct[hooks](), climate.Struct[internal](), climate.Struct[plugins]())
	climate.RunAndExit(p, climate.WithMetadata(md))
}
