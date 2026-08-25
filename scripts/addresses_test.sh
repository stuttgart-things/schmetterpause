#!/bin/sh
# Checks scripts/addresses.sh against both platforms it has to work on.
#
# This is the one piece of code in the repository that never runs in the
# pipeline or in the image — it runs on whoever's laptop stands at the table.
# Stubbing ifconfig/ipconfig and ip is the only way to exercise it at all, and
# an address picked wrong is printed into a QR code nobody can reach.
set -eu

here=$(cd "$(dirname "$0")" && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
mkdir "$work/bin"

fail() {
	echo "FAIL: $1"
	echo "got:"
	printf '%s\n' "$2" | sed 's/^/  /'
	exit 1
}

# --- macOS -------------------------------------------------------------
# Pseudo-devices first, which is the order ifconfig -l really answers in.
cat > "$work/bin/ifconfig" <<'STUB'
#!/bin/sh
[ "$1" = "-l" ] && echo "lo0 bridge0 awdl0 utun0 en0 en1"
STUB
cat > "$work/bin/ipconfig" <<'STUB'
#!/bin/sh
case "$2" in
	en0)     echo "192.168.1.42" ;;
	en1)     echo "10.0.5.7" ;;
	bridge0) echo "192.168.64.1" ;;
	awdl0)   echo "169.254.7.9" ;;
	utun0)   echo "10.8.0.3" ;;
	*) exit 1 ;;
esac
STUB
chmod +x "$work/bin/ifconfig" "$work/bin/ipconfig"

mac=$(PATH="$work/bin:/usr/bin:/bin" sh "$here/addresses.sh")

# The first line is what office:setup offers as the default, so it is the one
# that has to be reachable from a phone.
first=$(printf '%s\n' "$mac" | head -1)
case "$first" in
	en0*) ;;
	*) fail "macOS offers $first first, which no phone can reach" "$mac" ;;
esac

# Nothing is dropped: a machine with only a VPN up should still see it.
for want in en0 en1 bridge0 awdl0 utun0; do
	printf '%s\n' "$mac" | grep -q "^$want" || fail "macOS lost $want" "$mac"
done

# Every interface a phone cannot reach sits below every one it can.
lowest_real=$(printf '%s\n' "$mac" | grep -n '^en' | tail -1 | cut -d: -f1)
highest_aside=$(printf '%s\n' "$mac" | grep -nE '^(bridge|awdl|utun)' | head -1 | cut -d: -f1)
[ "$lowest_real" -lt "$highest_aside" ] ||
	fail "macOS mixes reachable and unreachable addresses" "$mac"

# --- Linux -------------------------------------------------------------
cat > "$work/bin/ip" <<'STUB'
#!/bin/sh
cat <<'OUT'
1: docker0    inet 172.17.0.1/16 scope global docker0
2: ens192    inet 10.100.136.190/24 scope global ens192
OUT
STUB
chmod +x "$work/bin/ip"
rm -f "$work/bin/ifconfig" "$work/bin/ipconfig"

linux=$(PATH="$work/bin:/usr/bin:/bin" sh "$here/addresses.sh")
first=$(printf '%s\n' "$linux" | head -1)
case "$first" in
	ens192*10.100.136.190*) ;;
	*) fail "Linux offers $first first, expected ens192" "$linux" ;;
esac
printf '%s\n' "$linux" | grep -q '^docker0' || fail "Linux dropped docker0" "$linux"

# --- neither -----------------------------------------------------------
# No tool to ask: say so on stderr and fail, rather than answering nothing and
# letting the caller write an empty address into the QR code.
rm -f "$work/bin/ip"
if PATH="$work/bin" sh "$here/addresses.sh" >/dev/null 2>&1; then
	echo "FAIL: a machine with no networking tools reported success"
	exit 1
fi

echo "addresses.sh: passed"
