#!/bin/sh
# Prints one "interface<TAB>address" line for every address this machine
# answers on, likeliest candidate first.
#
# Shared by office:ip and office:setup rather than written twice in the
# Taskfile: the two would drift, and the second one to drift is the one
# somebody uses while the office waits.
#
# A Docker bridge is never the address a phone has to reach, so those go last.
# They are sorted down rather than dropped, because "172.x is Docker" is only
# true until somebody's office network is on 172.16/12.
set -eu

list() {
	if command -v ip >/dev/null 2>&1; then
		ip -4 -o addr show scope global | awk '{split($4, a, "/"); print $2 "\t" a[1]}'
	elif command -v ipconfig >/dev/null 2>&1; then
		# macOS: ipconfig answers per interface, and only for the ones that
		# actually have an address.
		for i in $(ifconfig -l); do
			a=$(ipconfig getifaddr "$i" 2>/dev/null) && [ -n "$a" ] && printf '%s\t%s\n' "$i" "$a"
		done
	elif hostname -I >/dev/null 2>&1; then
		# No interface names from this one, only the addresses.
		for a in $(hostname -I); do printf '?\t%s\n' "$a"; done
	else
		echo "No ip, ipconfig or hostname -I here — read the address off the network settings." >&2
		exit 1
	fi
}

all=$(list)

# Two passes, both allowed to match nothing: under set -e a grep that finds
# nothing would otherwise end the script.
printf '%s\n' "$all" | grep -Ev '^(docker|br-|veth|virbr|vmnet|utun)' || true
printf '%s\n' "$all" | grep -E '^(docker|br-|veth|virbr|vmnet|utun)' || true
