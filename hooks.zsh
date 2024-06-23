[[ $_axl_hooks_sourced ]] && return 0
_axl_hooks_sourced=true

_axl_log="/tmp/$(whoami).axl"

_axl_cmd=_axl_nil
_axl_start_time=_axl_nil

_axl_preexec() {
	_axl_cmd=$1
	_axl_start_time=$(date +%s)
	print "+ $_axl_start_time $_axl_cmd" >> "$_axl_log"
	chmod 600 "$_axl_log"
}

_axl_precmd() {
	# Note: it's important this be the first statement to capture the exit code.
	local code=$?
	[[ $_axl_cmd == _axl_nil ]] && return $code
	print -- "- $code $_axl_start_time $_axl_cmd" >> "$_axl_log"
	if [[ $AXL_NOTIFY != "" ]]; then
		local msg
		msg=$(axl internal notify \
			--cmd="$_axl_cmd" --start-time="$_axl_start_time" --code=$code)
		if [[ $msg != "" ]]; then
			print -- "$msg" | eval "$AXL_NOTIFY" &>/dev/null &!
		fi
	fi
	# Reset back to nil since it's possible for _axl_precmd to be called without
	# _axl_preexec (on Ctrl-C, for example).
	_axl_cmd=_axl_nil
	return $code
}

preexec_functions+=(_axl_preexec)
precmd_functions+=(_axl_precmd)
