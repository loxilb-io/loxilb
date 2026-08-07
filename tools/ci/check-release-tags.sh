#!/usr/bin/env bash
#
# Verify a release tag is usable by the packaging build, before that build
# spends time on anything.
#
# usage: check-release-tags.sh <tag>
#
# pkg/debian/make.sh in loxilb-io/tools checks out the *same tag name* in three
# repos -- loxilb, loxilb-ebpf and loxicmd -- and it ignores the submodule
# pointer recorded in the loxilb tag, checking out the ebpf tag instead. Two
# things therefore have to hold:
#
#   A. All three repos carry the tag. A missing one does abort make.sh under
#      set -e, but only after the openssl source build, and with a raw git
#      pathspec error rather than a message naming the repo at fault.
#
#   B. The ebpf tag names the same commit the loxilb tag pins as its submodule.
#      If it does not, the release ships eBPF code other than what the loxilb
#      tag was built and tested against, and nothing anywhere reports it.
#
# A tag of "latest"/"" is a rolling build with no fixed version and is skipped.

set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <tag>" >&2
  exit 1
fi

raw_tag="$1"
tag="${raw_tag#refs/tags/}"
tag="${tag#v}"

if [[ -z "$tag" || "$tag" == "latest" ]]; then
  echo "check-release-tags: rolling build (tag='$raw_tag'), nothing to verify"
  exit 0
fi

tag="v${tag}"
repos=(loxilb loxilb-ebpf loxicmd)

# Resolve a remote tag to the commit it names. Annotated tags need the peeled
# ref, lightweight tags only have the plain one, so ask for both and prefer the
# peeled value.
peel_remote_tag() {
  local url="$1" ref="refs/tags/$2"
  git ls-remote "$url" "$ref" "$ref^{}" 2>/dev/null |
    awk -v peeled="$ref^{}" -v plain="$ref" '
      $2 == peeled { p = $1 }
      $2 == plain  { l = $1 }
      END { print (p != "" ? p : l) }'
}

# --- A. the tag exists in all three repos ------------------------------------
missing=()
declare -A tag_commit=()
for repo in "${repos[@]}"; do
  commit="$(peel_remote_tag "https://github.com/loxilb-io/${repo}.git" "$tag")"
  if [[ -z "$commit" ]]; then
    missing+=("$repo")
    echo "check-release-tags: $repo has NO tag $tag"
  else
    tag_commit["$repo"]="$commit"
    echo "check-release-tags: $repo $tag -> ${commit:0:12}"
  fi
done

if (( ${#missing[@]} > 0 )); then
  cat >&2 <<EOF

check-release-tags: MISSING TAG in ${missing[*]}

The packaging build checks out $tag in loxilb, loxilb-ebpf and loxicmd alike,
so it would abort part-way through. Create $tag in the repos above -- in
loxilb-ebpf it must name the commit the loxilb tag pins as its submodule --
and re-run.
EOF
  exit 1
fi

# --- B. the ebpf tag matches the submodule pointer ---------------------------
if ! git rev-parse -q --verify "refs/tags/$tag" >/dev/null 2>&1; then
  git fetch --depth 1 origin "refs/tags/$tag:refs/tags/$tag" >/dev/null 2>&1 || true
fi

if ! pinned="$(git ls-tree "$tag" loxilb-ebpf 2>/dev/null | awk '{print $3}')" || [[ -z "$pinned" ]]; then
  echo "check-release-tags: could not read the loxilb-ebpf submodule pointer at $tag" >&2
  exit 1
fi

built="${tag_commit[loxilb-ebpf]}"
if [[ "$pinned" == "$built" ]]; then
  echo "check-release-tags: OK -- loxilb-ebpf $tag matches the submodule pointer (${pinned:0:12})"
  exit 0
fi

cat >&2 <<EOF

check-release-tags: SUBMODULE POINTER MISMATCH
  loxilb $tag pins loxilb-ebpf : $pinned
  loxilb-ebpf tag $tag names   : $built

The packaging build checks out the ebpf tag, not the submodule pointer, so this
release would ship eBPF code other than what $tag was built against. Move the
loxilb-ebpf tag onto $pinned, or bump the submodule and re-cut $tag.
EOF
exit 1
