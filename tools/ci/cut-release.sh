#!/usr/bin/env bash
#
# Cut a loxilb release with one command.
#
# usage: cut-release.sh <version> [--execute] [options]
#
# A release spans three repositories -- loxilb, loxilb-ebpf and loxicmd -- because
# pkg/debian/make.sh in loxilb-io/tools checks out the *same tag name* in all three
# and ignores the submodule pointer recorded in the loxilb tag.
# tools/ci/check-release-tags.sh fails a release build when that arrangement is
# wrong; this script is the other half, laying the tags down so it is right in the
# first place. Both resolve tags the same way on purpose, so a clean preflight here
# means a passing check there.
#
# Without --execute nothing is written anywhere. The script resolves every commit it
# would tag, compares it against what is already published, and prints the plan.
# Run it that way first -- it is by far the cheapest place to catch a bad release.
#
# What --execute does, in order:
#   1. create the three tags, loxilb-ebpf first, so the loxilb tag is never briefly
#      published alongside an ebpf tag that does not exist yet
#   2. open a draft GitHub release with generated notes, so the approver in step 4
#      has something to read
#   3. dispatch the packaging build and the multi-arch image build
#   4. stop, and print what a human still has to do: approve the `release`
#      environment, then publish the draft
#
# Tags are never moved. When one already exists somewhere other than where this
# release wants it, the script stops and prints both commits. Deciding what to do
# about an already-published tag is not a decision to hand to a script.

set -euo pipefail

OWNER="loxilb-io"
LOXILB_REPO="$OWNER/loxilb"
EBPF_REPO="$OWNER/loxilb-ebpf"
LOXICMD_REPO="$OWNER/loxicmd"

PKG_WORKFLOW="package.yaml"
DOCKER_WORKFLOW="docker-multiarch.yml"

say()  { printf '%s\n' "$*"; }
info() { printf '  %s\n' "$*"; }
warn() { printf 'cut-release: warning: %s\n' "$*" >&2; }
die()  { printf 'cut-release: %s\n' "$*" >&2; exit 1; }

usage() {
  cat <<'EOF'
usage: cut-release.sh <version> [--execute] [options]

  <version>              release version without the leading 'v' (e.g. 0.9.8.9)

  --execute              actually create tags and dispatch builds.
                         Omit it to run the preflight only, which writes nothing.
  --yes                  skip the confirmation prompt (implies you have already
                         read a preflight for this exact version)

  --loxilb-ref <ref>     commit/branch to tag in loxilb   (default: main)
  --loxicmd-ref <ref>    commit/branch to tag in loxicmd  (default: main)
  --workflow-ref <ref>   branch whose workflow files drive the packaging build
                         (default: main -- you almost never want the tag here,
                         see the comment on dispatch_packaging below)

  --no-docker            skip the multi-arch container image build
  --no-notes             do not open a draft release with generated notes
  --watch                poll the packaging run until it finishes or stops for
                         approval, instead of just printing its URL

The loxilb-ebpf commit is never chosen by hand: it is read from the submodule
pointer of the loxilb commit being tagged, which is the only value that keeps the
packaged eBPF code identical to what the release was built against.
EOF
}

# --- argument parsing --------------------------------------------------------
version=""
execute=false
assume_yes=false
watch_run_flag=false
do_docker=true
do_notes=true
loxilb_ref="main"
loxicmd_ref="main"
workflow_ref="main"

while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help)      usage; exit 0 ;;
    --execute)      execute=true; shift ;;
    --yes)          assume_yes=true; shift ;;
    --watch)        watch_run_flag=true; shift ;;
    --no-docker)    do_docker=false; shift ;;
    --no-notes)     do_notes=false; shift ;;
    --loxilb-ref)   loxilb_ref="${2:-}"; shift 2 ;;
    --loxicmd-ref)  loxicmd_ref="${2:-}"; shift 2 ;;
    --workflow-ref) workflow_ref="${2:-}"; shift 2 ;;
    -*)             die "unknown option: $1" ;;
    *)
      [[ -n "$version" ]] && die "unexpected argument: $1"
      version="$1"; shift ;;
  esac
done

[[ -n "$version" ]] || { usage >&2; exit 1; }

# The packaging build takes the version bare and the release action prefixes it, so
# 'v1.0.2' as input would end up asking for tag 'vv1.0.2'.
if [[ "$version" == v* ]]; then
  version="${version#v}"
  info "dropping the leading 'v': using version '$version'"
fi

[[ "$version" =~ ^[0-9]+(\.[0-9]+){1,3}$ ]] ||
  die "version '$version' is not a dotted numeric version (e.g. 0.9.8.9)"

[[ "$version" == "latest" ]] && die "'latest' is the rolling build, not a release"

tag="v$version"

# --- prerequisites -----------------------------------------------------------
command -v gh  >/dev/null 2>&1 || die "the GitHub CLI (gh) is required"
command -v git >/dev/null 2>&1 || die "git is required"
gh auth status >/dev/null 2>&1 || die "gh is not authenticated -- run 'gh auth login'"

# --- resolution --------------------------------------------------------------

# Resolve a branch, tag or abbreviated sha to a full commit sha.
resolve_commit() {
  local repo="$1" ref="$2"
  gh api "repos/$repo/commits/$ref" --jq '.sha' 2>/dev/null || true
}

# Resolve a remote tag to the commit it names. Annotated tags need the peeled ref,
# lightweight tags only have the plain one, so ask for both and prefer the peeled
# value. Deliberately identical to check-release-tags.sh: preflight and CI must not
# be able to disagree about where a tag points.
peel_remote_tag() {
  local url="https://github.com/$1.git" ref="refs/tags/$2"
  git ls-remote "$url" "$ref" "$ref^{}" 2>/dev/null |
    awk -v peeled="$ref^{}" -v plain="$ref" '
      $2 == peeled { p = $1 }
      $2 == plain  { l = $1 }
      END { print (p != "" ? p : l) }'
}

short() { printf '%.12s' "$1"; }

say "cut-release: resolving $tag"

loxilb_sha="$(resolve_commit "$LOXILB_REPO" "$loxilb_ref")"
[[ -n "$loxilb_sha" ]] || die "could not resolve '$loxilb_ref' in $LOXILB_REPO"

# The ebpf commit comes from the loxilb tree, never from a branch tip: the release
# has to ship the eBPF code this loxilb commit was built and tested against.
ebpf_sha="$(gh api "repos/$LOXILB_REPO/contents/loxilb-ebpf?ref=$loxilb_sha" \
             --jq 'select(.type == "submodule") | .sha' 2>/dev/null || true)"
[[ -n "$ebpf_sha" ]] ||
  die "could not read the loxilb-ebpf submodule pointer at $(short "$loxilb_sha")"

[[ -n "$(resolve_commit "$EBPF_REPO" "$ebpf_sha")" ]] ||
  die "loxilb pins loxilb-ebpf $(short "$ebpf_sha"), which is not pushed to $EBPF_REPO"

loxicmd_sha="$(resolve_commit "$LOXICMD_REPO" "$loxicmd_ref")"
[[ -n "$loxicmd_sha" ]] || die "could not resolve '$loxicmd_ref' in $LOXICMD_REPO"

repos=("$EBPF_REPO" "$LOXILB_REPO" "$LOXICMD_REPO")
declare -A target=(
  ["$EBPF_REPO"]="$ebpf_sha"
  ["$LOXILB_REPO"]="$loxilb_sha"
  ["$LOXICMD_REPO"]="$loxicmd_sha"
)
declare -A origin=(
  ["$EBPF_REPO"]="submodule pointer of $LOXILB_REPO $(short "$loxilb_sha")"
  ["$LOXILB_REPO"]="$loxilb_ref"
  ["$LOXICMD_REPO"]="$loxicmd_ref"
)
declare -A action=() existing=()

conflicts=0
for repo in "${repos[@]}"; do
  have="$(peel_remote_tag "$repo" "$tag")"
  existing["$repo"]="$have"
  if [[ -z "$have" ]]; then
    action["$repo"]="create"
  elif [[ "$have" == "${target[$repo]}" ]]; then
    action["$repo"]="ok"
  else
    action["$repo"]="conflict"
    conflicts=$((conflicts + 1))
  fi
done

# --- the plan ----------------------------------------------------------------
say
say "cut-release: plan for $tag"
say
printf '  %-14s %-14s %-9s %s\n' REPO COMMIT ACTION SOURCE
for repo in "${repos[@]}"; do
  case "${action[$repo]}" in
    create)   verb="create" ;;
    ok)       verb="ok" ;;
    conflict) verb="CONFLICT" ;;
  esac
  printf '  %-14s %-14s %-9s %s\n' \
    "${repo#"$OWNER"/}" "$(short "${target[$repo]}")" "$verb" "${origin[$repo]}"
done
say

if (( conflicts > 0 )); then
  say "cut-release: the following tags already point somewhere else"
  say
  for repo in "${repos[@]}"; do
    [[ "${action[$repo]}" == "conflict" ]] || continue
    say "  $repo $tag"
    info "published : $(short "${existing[$repo]}")"
    info "wanted    : $(short "${target[$repo]}")  (${origin[$repo]})"
    if [[ "$repo" == "$EBPF_REPO" ]]; then
      info "This is the mismatch check-release-tags.sh refuses to build on: the"
      info "release would ship eBPF code other than what $tag was built against."
      info "Move the ebpf tag onto the pinned commit, or bump the submodule and"
      info "re-cut the loxilb tag."
    else
      info "Either delete and re-cut the tag, or point --${repo#"$OWNER"/}-ref at"
      info "the commit it already names if that is the intended release."
    fi
    say
  done
  die "refusing to move a published tag"
fi

release_state="none"
if is_draft="$(gh release view "$tag" --repo "$LOXILB_REPO" --json isDraft --jq '.isDraft' 2>/dev/null)"; then
  [[ "$is_draft" == "true" ]] && release_state="draft" || release_state="published"
fi

case "$release_state" in
  none)      say "  release   $tag does not exist yet$( $do_notes && printf ' (a draft will be opened)')" ;;
  draft)     say "  release   $tag already exists as a draft; its notes are left alone" ;;
  published) warn "release $tag is already published -- new artifacts would attach to a live release" ;;
esac

say "  builds    $PKG_WORKFLOW (on $workflow_ref)$( $do_docker && printf ", %s (on %s)" "$DOCKER_WORKFLOW" "$tag")"
say

if ! $execute; then
  say "cut-release: preflight only, nothing was written."
  say "Re-run with --execute to create the tags and start the builds."
  exit 0
fi

# --- execute -----------------------------------------------------------------
if ! $assume_yes; then
  printf 'Create these tags and start the release builds? [y/N] '
  read -r reply < /dev/tty || reply=""
  [[ "$reply" == "y" || "$reply" == "Y" ]] || die "aborted"
fi

# ebpf first: until its tag exists, a published loxilb tag would fail
# check-release-tags.sh for anyone who tried to build from it.
for repo in "${repos[@]}"; do
  if [[ "${action[$repo]}" == "ok" ]]; then
    say "cut-release: $repo $tag already at $(short "${target[$repo]}")"
    continue
  fi
  say "cut-release: tagging $repo $tag -> $(short "${target[$repo]}")"
  gh api --method POST "repos/$repo/git/refs" \
    -f ref="refs/tags/$tag" -f sha="${target[$repo]}" --silent ||
    die "failed to create $tag in $repo"
done

# Read the tags back rather than trusting the writes: this is the same view
# check-release-tags.sh will take, and a surprise here is far cheaper now than an
# hour into the packaging build.
for repo in "${repos[@]}"; do
  have="$(peel_remote_tag "$repo" "$tag")"
  [[ "$have" == "${target[$repo]}" ]] ||
    die "$repo $tag reads back as '$(short "$have")', expected $(short "${target[$repo]}")"
done
say "cut-release: all three tags verified"

if $do_notes && [[ "$release_state" == "none" ]]; then
  say "cut-release: opening draft release $tag"
  # Generated notes are a starting point for the human who edits and publishes it.
  # The packaging build passes omitBodyDuringUpdate, so whatever is written here
  # survives the artifact upload.
  gh release create "$tag" --repo "$LOXILB_REPO" \
    --draft --verify-tag --title "$tag" --generate-notes ||
    warn "could not open the draft release; the build will create one instead"
fi

# The packaging build runs from a branch, not from the tag: it clones
# loxilb-io/tools and that clone checks out the tag itself, so the only thing the
# workflow checkout supplies is the workflow file and tools/ci + tools/vm scripts.
# Running it from main is what makes fixes to those scripts apply to old tags too.
dispatch_packaging() {
  say "cut-release: dispatching $PKG_WORKFLOW (tagName=$version, publishRelease=true)"
  gh workflow run "$PKG_WORKFLOW" --repo "$LOXILB_REPO" --ref "$workflow_ref" \
    -f tagName="$version" \
    -f buildVmImage=true \
    -f smokeTestVmImage=true \
    -f publishRelease=true
}

# The image build is the opposite case: its Dockerfile COPYs the checked-out tree,
# so the run itself has to happen on the tag. tagName is the image tag and stays
# bare; sourceTag is a git ref used for the loxicmd checkout inside the build and
# therefore needs the 'v'.
dispatch_docker() {
  say "cut-release: dispatching $DOCKER_WORKFLOW on $tag"
  gh workflow run "$DOCKER_WORKFLOW" --repo "$LOXILB_REPO" --ref "$tag" \
    -f tagName="$version" \
    -f sourceTag="$tag" ||
    warn "could not dispatch $DOCKER_WORKFLOW -- check that it exists at $tag with a sourceTag input"
}

# gh workflow run reports nothing about the run it started, so find the newest
# dispatched run created since we asked for one.
find_run() {
  local workflow="$1" since="$2" id="" i
  for (( i = 0; i < 20; i++ )); do
    id="$(gh run list --repo "$LOXILB_REPO" --workflow "$workflow" \
            --event workflow_dispatch --limit 10 \
            --json databaseId,createdAt \
            --jq "[.[] | select(.createdAt >= \"$since\")] | max_by(.createdAt) | .databaseId" \
            2>/dev/null || true)"
    if [[ -n "$id" && "$id" != "null" ]]; then
      printf '%s' "$id"
      return 0
    fi
    sleep 3
  done
  return 1
}

started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
dispatch_packaging
pkg_run="$(find_run "$PKG_WORKFLOW" "$started_at" || true)"

if $do_docker; then
  docker_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  dispatch_docker
  docker_run="$(find_run "$DOCKER_WORKFLOW" "$docker_started_at" || true)"
fi

run_url() { printf 'https://github.com/%s/actions/runs/%s' "$LOXILB_REPO" "$1"; }

say
say "cut-release: builds started"
[[ -n "${pkg_run:-}" ]]    && info "packaging : $(run_url "$pkg_run")"
[[ -n "${docker_run:-}" ]] && info "image     : $(run_url "$docker_run")"

# Stop as soon as the run needs a human, rather than holding the terminal for the
# hour the packaging build takes.
if $watch_run_flag && [[ -n "${pkg_run:-}" ]]; then
  say
  say "cut-release: watching the packaging run"
  while :; do
    pending="$(gh api "repos/$LOXILB_REPO/actions/runs/$pkg_run/pending_deployments" \
                --jq 'length' 2>/dev/null || printf '0')"
    if [[ "$pending" != "0" ]]; then
      say "cut-release: the build is waiting for approval on the 'release' environment"
      break
    fi
    status="$(gh run view "$pkg_run" --repo "$LOXILB_REPO" --json status --jq '.status' 2>/dev/null || true)"
    [[ "$status" == "completed" ]] && { say "cut-release: packaging run completed"; break; }
    sleep 30
  done
fi

cat <<EOF

cut-release: what is left, and who does it

  1. a maintainer other than you approves the packaging run's publish job on the
     'release' environment. Only then are the artifacts attached to $tag.
       $( [[ -n "${pkg_run:-}" ]] && run_url "$pkg_run" )
  2. review the draft release, edit the notes, and press Publish.
       https://github.com/$LOXILB_REPO/releases/tag/$tag
EOF
