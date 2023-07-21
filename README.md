```
$ go install github.com/avamsi/axl@latest
```

```shell
# https://github.com/avamsi/axl
source <(axl hooks zsh)

export AXL_NOTIFY=...
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
  -h, --help   help for axl

Use "axl [command] --help" for more information about a command.
```
