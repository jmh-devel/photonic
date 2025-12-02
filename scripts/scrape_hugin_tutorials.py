#!/usr/bin/env python3
"""Fetch the index page and download the first page of every link it contains.

Usage:
    python scripts/scrape_hugin_tutorials.py \
        --url https://hugin.sourceforge.io/tutorials/index.shtml \
        --out output/hugin-tutorials
"""
from __future__ import annotations

import argparse
import json
import re
import sys
from html.parser import HTMLParser
from pathlib import Path
from typing import List, Set
from urllib import parse, request

DEFAULT_URL = "https://hugin.sourceforge.io/tutorials/index.shtml"
DEFAULT_OUT_DIR = "output/hugin-tutorials"
USER_AGENT = "photonic-hugin-scraper/1.0"


class LinkCollector(HTMLParser):
    def __init__(self, base_url: str) -> None:
        super().__init__()
        self.base_url = base_url
        self.links: Set[str] = set()

    def handle_starttag(self, tag: str, attrs) -> None:  # type: ignore[override]
        if tag.lower() != "a":
            return
        href = dict(attrs).get("href")
        if not href:
            return
        href = href.strip()
        if href.startswith(("#", "mailto:", "javascript:", "tel:")):
            return
        absolute = parse.urljoin(self.base_url, href)
        parsed = parse.urlparse(absolute)
        if parsed.scheme not in {"http", "https"}:
            return
        cleaned = parsed._replace(fragment="").geturl()
        self.links.add(cleaned)


def fetch_url(url: str, timeout: int = 20) -> bytes:
    headers = {"User-Agent": USER_AGENT}
    req = request.Request(url, headers=headers)
    with request.urlopen(req, timeout=timeout) as resp:  # nosec B310
        return resp.read()


def slug_from_url(url: str, index: int) -> str:
    parsed = parse.urlparse(url)
    path = Path(parsed.path)
    suffix = path.suffix or ".html"
    parts: List[str] = []
    if parsed.netloc:
        parts.append(parsed.netloc)
    for part in path.parts:
        if part in {"", "/"}:
            continue
        if Path(part).suffix:
            parts.append(Path(part).stem)
        else:
            parts.append(part)
    if parsed.query:
        parts.append(parsed.query)
    raw_slug = "-".join(parts) or "page"
    slug = re.sub(r"[^A-Za-z0-9]+", "-", raw_slug).strip("-") or "page"
    return f"{index:03d}-{slug}{suffix}"


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Scrape Hugin tutorial links")
    parser.add_argument("--url", default=DEFAULT_URL, help="Index URL to scrape")
    parser.add_argument("--out", default=DEFAULT_OUT_DIR, help="Directory to write downloads")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    out_dir = Path(args.out)
    out_dir.mkdir(parents=True, exist_ok=True)

    print(f"Fetching index: {args.url}")
    try:
        index_html = fetch_url(args.url)
    except Exception as exc:  # pragma: no cover - runtime safety
        print(f"Failed to fetch index: {exc}", file=sys.stderr)
        return 1

    parser = LinkCollector(args.url)
    parser.feed(index_html.decode("utf-8", errors="ignore"))

    links = sorted(parser.links)
    print(f"Found {len(links)} links")

    manifest = []
    for idx, link in enumerate(links, start=1):
        filename = slug_from_url(link, idx)
        target = out_dir / filename
        print(f"[{idx:03d}/{len(links)}] {link} -> {target}")
        try:
            content = fetch_url(link)
        except Exception as exc:  # pragma: no cover - runtime safety
            print(f"  ! failed to fetch: {exc}", file=sys.stderr)
            continue
        target.write_bytes(content)
        manifest.append({"url": link, "file": str(target)})

    (out_dir / "manifest.json").write_text(json.dumps(manifest, indent=2), encoding="utf-8")
    print("Done.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
