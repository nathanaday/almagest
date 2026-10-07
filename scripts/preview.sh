#!/bin/sh
# Installs this checkout on this machine, to try it before a release:
#   on    the binary, as version <plugin version>-preview.<commit>, where the launcher runs it
#         and the link ~/.almagest/bin/almagest points; the agent plugin, from a copy of the
#         checkout in a local marketplace, almagest-preview, in place of the release plugin;
#         and, when VAULT names a vault, Almagest for Obsidian built from OBSIDIAN_SRC.
#   off   the release plugin and its binary again.
# CLAUDE_CONFIG_DIR picks the Claude Code account, as for claude itself.
set -eu

mode=${1:-on}
root=$(cd "$(dirname "$0")/.." && pwd)
home=${ALMAGEST_HOME:-$HOME/.almagest}
claude=${CLAUDE:-claude}
stage=$home/preview/marketplace
release=almagest@nathanaday-almagest
preview=almagest@almagest-preview

say() { printf '\033[1m==> %s\033[0m\n' "$*"; }

# The id of each installed plugin that is on, one per line.
enabled() {
	"$claude" plugin list --json 2>/dev/null | node -e '
		let s = ""; process.stdin.on("data", (d) => (s += d)).on("end", () => {
			const list = JSON.parse(s || "[]");
			for (const p of Array.isArray(list) ? list : list.plugins ?? []) if (p.enabled) console.log(p.id ?? p.name);
		});'
}

if [ "$mode" = off ]; then
	say "The release plugin, $release"
	"$claude" plugin disable "$preview" >/dev/null 2>&1 || true
	"$claude" plugin enable "$release"
	latest=$(ls "$home/bin" | grep -E '^[0-9]+\.[0-9]+\.[0-9]+$' | sort -t. -k1,1n -k2,2n -k3,3n | tail -1 || true)
	if [ -n "$latest" ]; then
		ln -sfn "$latest/almagest" "$home/bin/almagest"
		say "The link ~/.almagest/bin/almagest names almagest $latest again"
	fi
	echo "Start a new session. Almagest for Obsidian stays in the vault until Obsidian's community plugins update it."
	exit 0
fi

base=$(sed -n 's/.*"version": "\([^"]*\)".*/\1/p' "$root/.claude-plugin/plugin.json" | head -1)
commit=$(git -C "$root" rev-parse --short HEAD)
dirty=$(git -C "$root" status --porcelain | grep -q . && echo .dirty || true)
version=$base-preview.$commit$dirty

say "The binary, almagest $version"
mkdir -p "$home/bin/$version"
(cd "$root" && go build -ldflags "-X github.com/nathanaday/almagest/internal/cli.Version=$version" -o "$home/bin/$version/almagest" ./cmd/almagest)
ln -sfn "$version/almagest" "$home/bin/almagest"
for old in "$home"/bin/*-preview.*; do
	[ "$old" = "$home/bin/$version" ] || rm -rf "$old"
done

say "The agent plugin, $preview $version"
rm -rf "$stage"
mkdir -p "$stage/.claude-plugin" "$stage/almagest"
# The files a release ships: every file git tracks or would track, as the working tree holds them.
(cd "$root" && git ls-files -co --exclude-standard -z | xargs -0 -I{} sh -c 'mkdir -p "$1/$(dirname "$2")" && cp -p "$2" "$1/$2"' _ "$stage/almagest" {})
sed -i.bak "s/\"version\": \"$base\"/\"version\": \"$version\"/" "$stage/almagest/.claude-plugin/plugin.json"
sed -i.bak "s/^version='$base'$/version='$version'/" "$stage/almagest/bin/almagest"
rm -f "$stage/almagest/.claude-plugin/plugin.json.bak" "$stage/almagest/bin/almagest.bak"
grep -q "^version='$version'$" "$stage/almagest/bin/almagest" || { echo "The launcher names no version $base; run make pin." >&2; exit 1; }
cat >"$stage/.claude-plugin/marketplace.json" <<EOF
{
  "name": "almagest-preview",
  "owner": { "name": "nathanaday" },
  "metadata": { "description": "A build of the almagest checkout at $root, to try before a release.", "version": "$version" },
  "plugins": [{ "name": "almagest", "source": "./almagest", "version": "$version", "category": "productivity" }]
}
EOF
if "$claude" plugin marketplace list 2>/dev/null | grep -q almagest-preview; then
	"$claude" plugin marketplace update almagest-preview >/dev/null
else
	"$claude" plugin marketplace add "$stage" >/dev/null
fi
# Two plugins named almagest would serve the tools and run the hooks twice.
if enabled | grep -qx "$release"; then "$claude" plugin disable "$release" >/dev/null; fi
"$claude" plugin uninstall "$preview" >/dev/null 2>&1 || true
"$claude" plugin install "$preview" >/dev/null
"$claude" plugin enable "$preview" >/dev/null 2>&1 || true
enabled | grep -qx "$preview" || { echo "Claude Code did not turn on $preview." >&2; exit 1; }

if [ -n "${VAULT:-}" ]; then
	# make passes a ~ on as typed.
	case $VAULT in "~"/*) VAULT=$HOME/${VAULT#"~"/} ;; esac
	vault=$(cd "$VAULT" && pwd)
	src=$(cd "${OBSIDIAN_SRC:-$root/../obsidian-almagest}" && pwd)
	say "Almagest for Obsidian, from $src, in $vault"
	(cd "$src" && npm run --silent build >/dev/null)
	dir=$vault/.obsidian/plugins/almagest
	mkdir -p "$dir"
	cp "$src/dist/main.js" "$src/dist/manifest.json" "$src/dist/styles.css" "$dir/"
	list=$vault/.obsidian/community-plugins.json
	node -e '
		const fs = require("fs"), file = process.argv[1];
		const list = fs.existsSync(file) ? JSON.parse(fs.readFileSync(file, "utf8")) : [];
		if (!list.includes("almagest")) fs.writeFileSync(file, JSON.stringify([...list, "almagest"], null, 2) + "\n");' "$list"
	echo "In Obsidian, run the command \"Reload app without saving\" to load it."
fi
echo "Start a new Claude Code session; almagest version says $version. make preview-off goes back to the release."
