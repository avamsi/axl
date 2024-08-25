if [[ $_axl_hooks_sourced ]]; then
	return 0
fi
_axl_hooks_sourced=true

_axl_log="/tmp/$(whoami).axl"

_axl_cmd=_axl_nil
_axl_start_time=_axl_nil

_axl_cmd_start() {
	_axl_cmd=$1
	_axl_start_time=$(date +%s)
	print "+ $_axl_start_time $_axl_cmd" >> "$_axl_log"
	chmod 600 "$_axl_log"
}

_axl_cmd_finish() {
	# Note: it's important this be the first statement to capture the exit code.
	local code=$?
	print -- "- $code $_axl_start_time $_axl_cmd" >> "$_axl_log"
	if [[ $_axl_cmd == _axl_nil ]]; then
		return $code
	fi
	if [[ $AXL_NOTIFY ]]; then
		local msg
		msg=$(axl internal notify \
			--cmd="$_axl_cmd" --start-time="$_axl_start_time" --code=$code)
		if [[ $msg ]]; then
			print -- "$msg" | eval "$AXL_NOTIFY" &>/dev/null &!
		fi
	fi
	# Reset back to nil since it's possible for _axl_precmd to be called without
	# _axl_preexec (on Ctrl-C, for example).
	_axl_cmd=_axl_nil
	return $code
}

preexec_functions+=(_axl_cmd_start)
precmd_functions+=(_axl_cmd_finish)
