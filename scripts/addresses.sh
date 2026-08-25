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
		#
		# Both halves matter under set -e. The || true keeps a failing lookup
		# from ending the script, and the if keeps the loop from *finishing*
		# on a false condition: the status of a for loop is the status of the
		# last command in its body, and the last interface macOS lists is
		# typically a bridge or a tunnel with no address at all. Written as an
		# && chain this exited 1 after having found every address correctly.
		for i in $(ifconfig -l); do
			a=$(ipconfig getifaddr "$i" 2>/dev/null || true)
			if [ -n "$a" ]; then
				printf '%s\t%s\n' "$i" "$a"
			fi
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

# Interfaces no phone can reach, and the address a machine gives itself when
# DHCP answered nobody:
#
#   docker, br-, veth, virbr, vmnet, bridge   virtual switches for containers and VMs
#   utun                                      VPN tunnels
#   awdl, llw                                 AirDrop and friends, macOS
#   gif, stf, anpi, ap                        macOS pseudo-devices
#   169.254.x                                 self-assigned, reachable by nobody
#
# Sorted down rather than dropped, because "172.x is Docker" is only true until
# somebody's office network is on 172.16/12 — and because a machine with
# nothing but these should show them rather than claim to have no address.
aside='^(docker|br-|veth|virbr|vmnet|utun|awdl|llw|bridge|gif|stf|anpi|ap[0-9])|[[:space:]]169\.254\.'

# Both passes are allowed to match nothing: under set -e a grep that finds
# nothing would otherwise end the script.
printf '%s\n' "$all" | grep -Ev "$aside" || true
printf '%s\n' "$all" | grep -E "$aside" || true
