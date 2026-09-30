#!/bin/sh
# Installs kforward for the current user, from a release on GitHub:
#
#   curl -fsSL https://raw.githubusercontent.com/gsontag/kforward/main/install.sh | sh
#
# KFORWARD_VERSION picks a release, the latest by default; PREFIX the
# installation directory, ~/.local by default. KFORWARD_SOURCE installs from
# a directory holding out/kforward and data/ instead, as make install does.
# With --uninstall, removes what it installs.
set -eu

REPO=gsontag/kforward
APP_ID=fr.gsontag.kforward

main() {
	prefix=${PREFIX:-$HOME/.local}
	bindir=$prefix/bin
	appdir=$prefix/share/applications
	icondir=$prefix/share/icons/hicolor/scalable/apps

	if [ "${1:-}" = --uninstall ]; then
		rm -f "$bindir/kforward" "$icondir/$APP_ID.svg" "$appdir/$APP_ID.desktop"
		echo "kforward removed from $prefix"
		return
	fi

	src=${KFORWARD_SOURCE:-}
	if [ -z "$src" ]; then
		download
	fi

	install -Dm755 "$src/out/kforward" "$bindir/kforward"
	install -Dm644 "$src/data/$APP_ID.svg" "$icondir/$APP_ID.svg"
	install -d "$appdir"
	sed "s|@BINDIR@|$bindir|" "$src/data/$APP_ID.desktop.in" >"$appdir/$APP_ID.desktop"
	chmod 644 "$appdir/$APP_ID.desktop"
	echo "installed in $prefix: $("$bindir/kforward" --version 2>/dev/null || echo kforward)"

	check "$bindir/kforward"
}

# download fetches the release and checks it, into a temporary directory
# that src then designates.
download() {
	case $(uname -m) in
	x86_64) arch=amd64 ;;
	*) die "no release for $(uname -m): build from source, see https://github.com/$REPO" ;;
	esac
	version=${KFORWARD_VERSION:-$(latest)}
	name=kforward-$version-linux-$arch
	url=https://github.com/$REPO/releases/download/$version/$name.tar.gz

	tmp=$(mktemp -d)
	trap 'rm -rf "$tmp"' EXIT
	echo "downloading $name"
	curl -fsSL -o "$tmp/$name.tar.gz" "$url" || die "cannot download $url"
	curl -fsSL -o "$tmp/$name.tar.gz.sha256" "$url.sha256" || die "cannot download $url.sha256"
	(cd "$tmp" && sha256sum -c --quiet "$name.tar.gz.sha256") || die "$name.tar.gz: checksum mismatch"
	tar -xzf "$tmp/$name.tar.gz" -C "$tmp"
	src=$tmp/$name
}

# latest prints the tag of the latest release, read from where the latest
# release page redirects: the API would limit anonymous calls.
latest() {
	url=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest") ||
		die "cannot find the latest release"
	echo "${url##*/}"
}

# check warns of what would keep the installed program from working.
check() {
	if ! "$1" --version >/dev/null 2>&1; then
		echo "warning: $1 does not start on this system:" >&2
		"$1" --version 2>&1 | head -n 3 >&2 || true
	fi
	if pgrep -x kforward >/dev/null 2>&1; then
		echo "kforward is running: quit it and start it again to use this version"
	fi
}

die() {
	echo "error: $*" >&2
	exit 1
}

# Read in full before running: a download cut short runs nothing.
main "$@"
