#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
config_home=${XDG_CONFIG_HOME:-"${HOME:?HOME must be set when XDG_CONFIG_HOME is unset}/.config"}
opencode_home=$config_home/opencode
plugin_dir=$opencode_home/plugins
skill_dir=$opencode_home/skills/lucind-ai
plugin_source=$root/plugin/opencode/lucind-ai.ts
process_source=$root/plugin/opencode/process.mjs
skill_source=$root/plugin/opencode/skills/lucind-ai
plugin_target=$plugin_dir/lucind-ai.ts
process_target=$plugin_dir/process.mjs
owner_marker=$opencode_home/.lucind-ai-opencode-owner

hash_file() {
  set -- $(sha256sum "$1")
  printf '%s' "$1"
}

hash_tree() {
  digest=$(find "$1" -type f -print0 | sort -z | xargs -0 sha256sum | sha256sum)
  set -- $digest
  printf '%s' "$1"
}

marker_content() {
  printf 'lucind-ai-opencode-installer-v1\nplugin=%s\nprocess=%s\nskill=%s\n' \
    "$(hash_file "$plugin_target")" "$(hash_file "$process_target")" "$(hash_tree "$skill_dir")"
}

owned=0
if [ -e "$owner_marker" ]; then
  if [ ! -f "$owner_marker" ] || [ "$(cat "$owner_marker")" != "$(marker_content)" ]; then
    printf '%s\n' "refusing to use invalid ownership marker: $owner_marker" >&2
    exit 1
  fi
  owned=1
fi

mkdir -p "$plugin_dir" "$opencode_home/skills"

if [ "$owned" -eq 0 ] && [ -e "$plugin_target" ]; then
  printf '%s\n' "refusing to overwrite existing unrelated plugin: $plugin_target" >&2
  exit 1
fi
if [ "$owned" -eq 0 ] && [ -e "$process_target" ]; then
  printf '%s\n' "refusing to overwrite existing unrelated module: $process_target" >&2
  exit 1
fi
if [ "$owned" -eq 0 ] && [ -e "$skill_dir" ]; then
  printf '%s\n' "refusing to overwrite existing unrelated skill: $skill_dir" >&2
  exit 1
fi

cp "$plugin_source" "$plugin_target"
cp "$process_source" "$process_target"
rm -rf "$skill_dir"
cp -R "$skill_source" "$skill_dir"
marker_content > "$owner_marker"

printf '%s\n' "installed lucind-ai OpenCode plugin and skill under $opencode_home"
