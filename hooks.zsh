if [[ $_axl_hooks_sourced ]]; then
	return 0
fi
_axl_hooks_sourced=true

_axl_log="/tmp/$(whoami).axl"

_axl_cmd_run=_axl_nil
_axl_cmd=_axl_nil
_axl_start_time=_axl_nil

_axl_cmd_start() {
	_axl_start_time=$(date +%s)
	_axl_cmd_run=$1
	print "+ $_axl_start_time $_axl_cmd_run" >> "$_axl_log"
	chmod 600 "$_axl_log"
}

# TODO: would be nice to handle errors with `axl internal render`.
_axl_cmd_finish() {
	# Note: it's important this be the first statement to capture the exit code.
	local code=$?
	_axl_cmd=$_axl_cmd_run
	if [[ $_axl_cmd_run == _axl_nil ]]; then
		return $code
	fi
	# Reset back to nil since it's possible for _axl_cmd_finish to be called
	# without an associated _axl_cmd_start (on ctrl-c, for example).
	_axl_cmd_run=_axl_nil
	print -- "- $code $_axl_start_time $_axl_cmd" >> "$_axl_log"
	if [[ $AXL_NOTIFY ]]; then
		local msg
		msg=$(axl internal render "$_axl_cmd" \
			--start-time="$_axl_start_time" --code=$code)
		if [[ $msg ]]; then
			print -- "$msg" | eval "$AXL_NOTIFY" &>/dev/null &!
		fi
	fi
	return $code
}

preexec_functions+=(_axl_cmd_start)
precmd_functions+=(_axl_cmd_finish)

# TODO: would be nice to handle errors with `axl internal suggest`.
_axl_suggest_cmd() {
	if [[ $_axl_cmd != _axl_nil && -z $BUFFER ]]; then
		BUFFER=$(axl internal suggest "$_axl_cmd")
		CURSOR=$#BUFFER
	fi
}

autoload -Uz add-zle-hook-widget
add-zle-hook-widget zle-line-init _axl_suggest_cmd
