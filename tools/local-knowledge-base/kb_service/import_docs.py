import argparse
import json
from pathlib import Path
from typing import Any

from .store import KnowledgeStore


SUPPORTED_SUFFIXES = {".md", ".markdown", ".txt", ".json"}


def main() -> None:
    parser = argparse.ArgumentParser(description="Import local articles into the knowledge base.")
    parser.add_argument("paths", nargs="+", help="Files or directories to import.")
    parser.add_argument("--db-path", default="/data/knowledge.sqlite3")
    parser.add_argument("--project-key", default="")
    parser.add_argument("--source", default="local")
    args = parser.parse_args()

    store = KnowledgeStore(args.db_path)
    try:
        count = 0
        for document in load_documents(args.paths, args.project_key, args.source):
            store.upsert_document(document)
            count += 1
        print(f"Imported {count} document(s)")
    finally:
        store.close()


def load_documents(paths: list[str], project_key: str, source: str) -> list[dict[str, Any]]:
    documents: list[dict[str, Any]] = []
    for raw_path in paths:
        path = Path(raw_path)
        files = [path] if path.is_file() else sorted(path.rglob("*"))
        for file in files:
            if not file.is_file() or file.suffix.lower() not in SUPPORTED_SUFFIXES:
                continue
            documents.extend(load_file(file, project_key, source))
    return documents


def load_file(path: Path, project_key: str, source: str) -> list[dict[str, Any]]:
    raw = path.read_text(encoding="utf-8")
    if path.suffix.lower() == ".json":
        payload = json.loads(raw)
        if isinstance(payload, dict) and isinstance(payload.get("documents"), list):
            return [_with_defaults(item, path, project_key, source) for item in payload["documents"]]
        if isinstance(payload, list):
            return [_with_defaults(item, path, project_key, source) for item in payload]
        if isinstance(payload, dict):
            return [_with_defaults(payload, path, project_key, source)]
        raise ValueError(f"Unsupported JSON document shape: {path}")

    return [
        {
            "title": infer_title(path, raw),
            "source": source,
            "content": raw,
            "projectKey": project_key,
            "url": str(path),
        }
    ]


def infer_title(path: Path, content: str) -> str:
    for line in content.splitlines():
        stripped = line.strip()
        if stripped.startswith("#"):
            return stripped.lstrip("#").strip() or path.stem
    return path.stem.replace("-", " ").replace("_", " ")


def _with_defaults(
    document: dict[str, Any],
    path: Path,
    project_key: str,
    source: str,
) -> dict[str, Any]:
    return {
        **document,
        "title": document.get("title") or path.stem,
        "source": document.get("source") or source,
        "projectKey": document.get("projectKey") or project_key,
    }


if __name__ == "__main__":
    main()
