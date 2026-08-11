#!/usr/bin/env python3
"""Delete orphaned GHCR container versions, without breaking tagged images.

A multi-arch image is an index that points at per-platform child manifests, and
GHCR records every one of those children as its own *untagged* package version.
"Delete everything untagged" therefore rips the platforms out from under tags
that are still in use -- that is how ghcr.io/loxilb-io/loxilb:v0.9.4 lost its
linux/amd64 manifest and stopped being pullable.

So this walks the reference graph from every tagged version first, and only
considers a version deletable when nothing tagged can reach it.

Dry run unless --apply is passed.
"""

import argparse
import base64
import json
import os
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from datetime import datetime, timedelta, timezone

REGISTRY = "ghcr.io"
API = "https://api.github.com"

# Both the OCI and the older Docker media types, indexes first. Without these a
# registry hands back a v2 schema1 blob and the child manifests stay invisible.
MANIFEST_ACCEPT = ", ".join(
    [
        "application/vnd.oci.image.index.v1+json",
        "application/vnd.docker.distribution.manifest.list.v2+json",
        "application/vnd.oci.image.manifest.v1+json",
        "application/vnd.docker.distribution.manifest.v2+json",
    ]
)


class Fatal(Exception):
    pass


def request(url, headers=None, method="GET", retries=3):
    """GET/DELETE with retries. Raises HTTPError for 4xx so callers can tell a
    404 (already gone) apart from a transient failure."""
    last = None
    for attempt in range(retries):
        req = urllib.request.Request(url, headers=headers or {}, method=method)
        try:
            with urllib.request.urlopen(req, timeout=60) as resp:
                return resp.status, resp.headers, resp.read()
        except urllib.error.HTTPError as exc:
            # 4xx is an answer, not a glitch -- report it rather than retrying.
            if 400 <= exc.code < 500 and exc.code not in (429,):
                raise
            last = exc
        except Exception as exc:  # noqa: BLE001 - network stack, anything goes
            last = exc
        if attempt < retries - 1:
            time.sleep(2 ** attempt)
    raise Fatal(f"{method} {url} failed after {retries} attempts: {last}")


def api_paged(path, token):
    """Walk a paginated REST collection via the Link header."""
    url = f"{API}{path}"
    headers = {
        "Authorization": f"Bearer {token}",
        "Accept": "application/vnd.github+json",
        "X-GitHub-Api-Version": "2022-11-28",
        "User-Agent": "loxilb-ghcr-prune",
    }
    out = []
    while url:
        _, resp_headers, body = request(url, headers)
        out.extend(json.loads(body))
        url = None
        for part in (resp_headers.get("Link") or "").split(","):
            if 'rel="next"' in part:
                url = part[part.find("<") + 1 : part.find(">")]
    return out


def registry_token(owner, package, token):
    """Exchange the GitHub token for a registry pull token."""
    basic = base64.b64encode(f"x:{token}".encode()).decode()
    scope = urllib.parse.quote(f"repository:{owner}/{package}:pull", safe=":/")
    _, _, body = request(
        f"https://{REGISTRY}/token?service={REGISTRY}&scope={scope}",
        {"Authorization": f"Basic {basic}", "User-Agent": "loxilb-ghcr-prune"},
    )
    return json.loads(body)["token"]


def build_referenced_set(owner, package, tagged_digests, reg_token):
    """Every digest reachable from a tagged version.

    Fails closed: a manifest we cannot read might be an index whose children we
    would then classify as orphans and delete. The one safe exception is a 404,
    which means the manifest is already gone and so can reference nothing.
    """
    headers = {
        "Authorization": f"Bearer {reg_token}",
        "Accept": MANIFEST_ACCEPT,
        "User-Agent": "loxilb-ghcr-prune",
    }
    referenced = set()
    visited = set()
    dangling = []

    def walk(ref, root_tag):
        if ref in visited:
            return
        visited.add(ref)
        try:
            _, _, body = request(
                f"https://{REGISTRY}/v2/{owner}/{package}/manifests/{ref}", headers
            )
        except urllib.error.HTTPError as exc:
            if exc.code == 404:
                # Already missing. Record it -- a tagged image referencing a
                # missing child is broken and worth surfacing -- but it cannot
                # point at anything, so it is safe to stop here.
                dangling.append((root_tag, ref))
                return
            raise Fatal(f"cannot read manifest {ref} (tag {root_tag}): {exc}")
        for child in json.loads(body).get("manifests", []):
            referenced.add(child["digest"])
            walk(child["digest"], root_tag)

    for digest, tags in tagged_digests.items():
        walk(digest, ",".join(tags))

    return referenced, dangling


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--owner", default="loxilb-io")
    ap.add_argument("--package", default="loxilb")
    ap.add_argument(
        "--older-than-days",
        type=int,
        default=90,
        help="only delete orphans last updated at least this long ago",
    )
    ap.add_argument(
        "--apply", action="store_true", help="actually delete (default is a dry run)"
    )
    ap.add_argument(
        "--max-deletes",
        type=int,
        default=0,
        help="stop after this many deletions (0 = no limit)",
    )
    args = ap.parse_args()

    token = os.environ.get("GH_TOKEN") or os.environ.get("GITHUB_TOKEN")
    if not token:
        raise Fatal("set GH_TOKEN (needs read:packages and delete:packages)")

    owner, package = args.owner, args.package
    quoted = urllib.parse.quote(package, safe="")

    print(f"package: {REGISTRY}/{owner}/{package}")
    versions = api_paged(
        f"/orgs/{owner}/packages/container/{quoted}/versions?per_page=100", token
    )
    tagged = {
        v["name"]: v["metadata"]["container"]["tags"]
        for v in versions
        if v["metadata"]["container"]["tags"]
    }
    untagged = [v for v in versions if not v["metadata"]["container"]["tags"]]
    print(f"  versions: {len(versions)}  tagged: {len(tagged)}  untagged: {len(untagged)}")

    reg_token = registry_token(owner, package, token)
    referenced, dangling = build_referenced_set(owner, package, tagged, reg_token)
    print(f"  digests reachable from a tag: {len(referenced)}")

    if dangling:
        print("\n  WARNING: tags pointing at manifests that no longer exist:")
        for tag, ref in dangling:
            print(f"    {tag} -> {ref}")
        print("  (these images are already broken; pruning does not make it worse)")

    cutoff = datetime.now(timezone.utc) - timedelta(days=args.older_than_days)
    keep_referenced, keep_recent, orphans = 0, 0, []
    for v in untagged:
        if v["name"] in referenced:
            keep_referenced += 1
            continue
        updated = datetime.strptime(v["updated_at"], "%Y-%m-%dT%H:%M:%SZ").replace(
            tzinfo=timezone.utc
        )
        if updated > cutoff:
            keep_recent += 1
            continue
        orphans.append(v)

    orphans.sort(key=lambda v: v["updated_at"])
    if args.max_deletes:
        orphans = orphans[: args.max_deletes]

    print()
    print(f"  keep (child of a tagged image): {keep_referenced}")
    print(f"  keep (newer than {args.older_than_days}d): {keep_recent}")
    print(f"  delete: {len(orphans)}")
    if not orphans:
        print("\nnothing to do")
        return 0

    print(f"\n  oldest: {orphans[0]['updated_at']}   newest: {orphans[-1]['updated_at']}")

    if not args.apply:
        print("\nDRY RUN -- pass --apply to delete")
        return 0

    headers = {
        "Authorization": f"Bearer {token}",
        "Accept": "application/vnd.github+json",
        "X-GitHub-Api-Version": "2022-11-28",
        "User-Agent": "loxilb-ghcr-prune",
    }
    deleted, failed = 0, 0
    for i, v in enumerate(orphans, 1):
        url = f"{API}/orgs/{owner}/packages/container/{quoted}/versions/{v['id']}"
        try:
            request(url, headers, method="DELETE")
            deleted += 1
        except urllib.error.HTTPError as exc:
            # 404 means someone else got there first, which is not a failure.
            if exc.code == 404:
                deleted += 1
            else:
                failed += 1
                print(f"  FAILED {v['id']} {v['name'][:19]}: {exc}")
        if i % 50 == 0:
            print(f"  ...{i}/{len(orphans)}")

    print(f"\ndeleted {deleted}, failed {failed}")
    return 1 if failed else 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Fatal as exc:
        print(f"error: {exc}", file=sys.stderr)
        sys.exit(2)
