import hashlib
import json
import re
import sqlite3
import threading
from datetime import datetime, timezone
from typing import Any


ASCII_TOKEN_RE = re.compile(r"[a-zA-Z0-9][a-zA-Z0-9_.:-]*")
CJK_SEQUENCE_RE = re.compile(r"[\u3400-\u9fff]+")


class KnowledgeStore:
    def __init__(self, db_path: str):
        self.db_path = db_path
        self.connection = sqlite3.connect(db_path, check_same_thread=False)
        self.connection.row_factory = sqlite3.Row
        self.lock = threading.RLock()
        self._initialize()

    def close(self) -> None:
        with self.lock:
            self.connection.close()

    def upsert_document(self, document: dict[str, Any]) -> str:
        title = str(document.get("title", "")).strip()
        content = str(document.get("content", "")).strip()
        if not title:
            raise ValueError("Document title is required")
        if not content:
            raise ValueError("Document content is required")

        source = str(document.get("source") or "local").strip()
        url = str(document.get("url") or "").strip()
        project_key = str(document.get("projectKey") or document.get("project_key") or "").strip()
        tags = _normalize_tags(document.get("tags"))
        document_id = str(document.get("id") or _stable_id(title, source, project_key, content)).strip()
        now = datetime.now(timezone.utc).isoformat()

        with self.lock:
            self.connection.execute(
                """
                insert into documents (
                    id, title, source, content, url, project_key, tags_json, created_at, updated_at
                ) values (?, ?, ?, ?, ?, ?, ?, ?, ?)
                on conflict(id) do update set
                    title = excluded.title,
                    source = excluded.source,
                    content = excluded.content,
                    url = excluded.url,
                    project_key = excluded.project_key,
                    tags_json = excluded.tags_json,
                    updated_at = excluded.updated_at
                """,
                (
                    document_id,
                    title,
                    source,
                    content,
                    url,
                    project_key,
                    json.dumps(tags, ensure_ascii=False),
                    now,
                    now,
                ),
            )
            self.connection.commit()

        return document_id

    def search(self, request: dict[str, Any]) -> list[dict[str, Any]]:
        limit = _coerce_limit(request.get("limit"))
        project_key = str(request.get("projectKey") or request.get("project_key") or "").strip()
        query_text = _build_query_text(request)
        terms = extract_terms(query_text)

        with self.lock:
            if project_key:
                rows = self.connection.execute(
                    """
                    select * from documents
                    where project_key = '' or project_key = ?
                    order by updated_at desc
                    """,
                    (project_key,),
                ).fetchall()
            else:
                rows = self.connection.execute(
                    "select * from documents order by updated_at desc"
                ).fetchall()

        hits: list[dict[str, Any]] = []
        for row in rows:
            tags = json.loads(row["tags_json"] or "[]")
            score = _score_row(row, tags, terms, query_text)
            if score <= 0:
                continue

            hits.append(
                {
                    "id": row["id"],
                    "title": row["title"],
                    "source": row["source"],
                    "snippet": _make_snippet(row["content"], terms),
                    "url": row["url"] or None,
                    "score": round(score, 4),
                    "tags": tags,
                }
            )

        hits.sort(key=lambda hit: hit["score"], reverse=True)
        return hits[:limit]

    def count_documents(self) -> int:
        with self.lock:
            row = self.connection.execute("select count(*) as count from documents").fetchone()
        return int(row["count"])

    def _initialize(self) -> None:
        with self.lock:
            self.connection.execute(
                """
                create table if not exists documents (
                    id text primary key,
                    title text not null,
                    source text not null,
                    content text not null,
                    url text not null default '',
                    project_key text not null default '',
                    tags_json text not null default '[]',
                    created_at text not null,
                    updated_at text not null
                )
                """
            )
            self.connection.execute(
                "create index if not exists idx_documents_project_key on documents(project_key)"
            )
            self.connection.commit()


def extract_terms(value: str) -> list[str]:
    normalized = value.lower()
    terms: list[str] = []

    for match in ASCII_TOKEN_RE.findall(normalized):
        token = match.strip("._:-")
        if len(token) >= 2:
            terms.append(token)

    for sequence in CJK_SEQUENCE_RE.findall(normalized):
        if 2 <= len(sequence) <= 8:
            terms.append(sequence)
        for width in (2, 3, 4):
            if len(sequence) >= width:
                terms.extend(sequence[index : index + width] for index in range(len(sequence) - width + 1))

    deduped: list[str] = []
    seen: set[str] = set()
    for term in terms:
        if term not in seen:
            deduped.append(term)
            seen.add(term)
    return deduped


def _initialize_text(value: Any) -> str:
    if isinstance(value, list):
        return " ".join(str(item) for item in value)
    return str(value or "")


def _build_query_text(request: dict[str, Any]) -> str:
    weighted_parts = [
        _initialize_text(request.get("task")),
        _initialize_text(request.get("task")),
        _initialize_text(request.get("title")),
        _initialize_text(request.get("url")),
        _initialize_text(request.get("hints")),
        _initialize_text(request.get("visibleText")),
    ]
    return " ".join(part for part in weighted_parts if part).strip()


def _score_row(
    row: sqlite3.Row,
    tags: list[str],
    terms: list[str],
    query_text: str,
) -> float:
    title = row["title"].lower()
    source = row["source"].lower()
    content = row["content"].lower()
    url = row["url"].lower()
    tag_text = " ".join(tags).lower()
    score = 0.0

    for term in terms:
        if term in title:
            score += 6.0
        if term in tag_text:
            score += 4.0
        if term in source:
            score += 2.0
        if term in url:
            score += 1.5
        content_count = content.count(term)
        if content_count:
            score += min(6.0, 1.5 * content_count)

    compact_query = " ".join(query_text.lower().split())
    if compact_query and compact_query in content:
        score += 10.0
    if compact_query and compact_query in title:
        score += 14.0

    return score


def _make_snippet(content: str, terms: list[str], max_length: int = 500) -> str:
    lowered = content.lower()
    best_index = -1
    for term in terms:
        index = lowered.find(term)
        if index >= 0 and (best_index < 0 or index < best_index):
            best_index = index

    if best_index < 0:
        snippet = content[:max_length]
    else:
        start = max(0, best_index - 160)
        end = min(len(content), start + max_length)
        snippet = content[start:end]
        if start > 0:
            snippet = "..." + snippet
        if end < len(content):
            snippet = snippet + "..."

    return " ".join(snippet.split())


def _normalize_tags(value: Any) -> list[str]:
    if value is None:
        return []
    if isinstance(value, str):
        return [item.strip() for item in value.split(",") if item.strip()]
    if isinstance(value, list):
        return [str(item).strip() for item in value if str(item).strip()]
    return []


def _stable_id(title: str, source: str, project_key: str, content: str) -> str:
    digest = hashlib.sha1(
        f"{project_key}\n{source}\n{title}\n{content}".encode("utf-8")
    ).hexdigest()
    return digest[:16]


def _coerce_limit(value: Any) -> int:
    try:
        limit = int(value)
    except (TypeError, ValueError):
        limit = 5
    return min(20, max(1, limit))
