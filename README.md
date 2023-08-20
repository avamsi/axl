```
$ go install github.com/avamsi/axl@latest
```

```shell
# https://github.com/avamsi/axl
source <(axl hooks zsh)
```

```
$ axl --help

axl watches over your commands.

Usage:
  axl [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  hooks       Shell hooks
  internal    Axl internal commands, not for general use
  list        List currently running commands
  wait        Wait for a command to finish running

Flags:
  -h, --help  help for axl

Use "axl [command] --help" for more information about a command.
```

Export AXL_NOTIFY to be notified when long running commands finish. Some examples:

```shell
export AXL_NOTIFY=cat
export AXL_NOTIFY="mail -s axl $USER"
export AXL_NOTIFY="slack-cli -d $USER"' "$(cat)"'
```

Export AXL_MESSAGE (per https://pkg.go.dev/text/template) to customize the message.  
command, start, elapsed, code and host are available. This is the default:

```shell
export AXL_MESSAGE='💲 {{.command}}
⌚ {{.start}} + ⌛ {{.elapsed}}{{if ne .code 0}} -> 🙅 {{.code}}{{end}} @ 💻 {{.host}}'
```
