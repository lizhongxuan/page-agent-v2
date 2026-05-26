# Local Knowledge Base

This is a Dockerized local knowledge base compatible with the Page Agent extension.

## Start

```bash
cd tools/local-knowledge-base
docker compose up -d --build
curl http://127.0.0.1:8787/health
```

## Configure the extension

- Enable knowledge base: on
- Knowledge Base URL: `http://127.0.0.1:8787`
- Project Key: `aiops`
- API Key: empty by default, or the value of `KB_API_KEY` if configured

The extension posts to `/search` and injects returned hits into `<project_knowledge>`.

## Add one article

```bash
curl -X POST http://127.0.0.1:8787/documents \
  -H 'content-type: application/json' \
  -d '{
    "id": "pg-runtime-log",
    "title": "PG runtime log convention",
    "source": "manual",
    "projectKey": "aiops",
    "tags": ["postgresql", "logs"],
    "content": "When log timestamps are ascending, the newest entries are at the bottom. Use bounded scrolling to avoid following live output forever."
  }'
```

## Search

```bash
curl -X POST http://127.0.0.1:8787/search \
  -H 'content-type: application/json' \
  -d '{
    "task": "排查 PG 实例运行日志最新错误",
    "url": "https://platform.local/console",
    "title": "运行日志",
    "projectKey": "aiops",
    "limit": 5
  }'
```

## Import local files

Inside the container:

```bash
docker compose exec local-knowledge-base \
  python -m kb_service.import_docs /data/articles --project-key aiops --source manual
```

Supported file types: `.md`, `.markdown`, `.txt`, `.json`.
