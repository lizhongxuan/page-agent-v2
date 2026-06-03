# Workflow Backend

The workflow backend stores Page Agent WebOps memory for one local extension
configuration. It is not a generic RAG service. The current memory model has two
long-lived data lines:

- `SiteTaskGuide`: reusable task guides generated from real successful user
  tasks on a specific site.
- `SiteManualWiki`: concise site manual knowledge compiled from user-imported
  manuals.

The backend must not expose or preserve the previous generic document or global
business-summary model.

## Local Startup

```bash
npm run workflow:deploy
```

This is the only local startup command for normal use. It starts the workflow
backend, prints the backend URL, and copies the Chrome extension package to the
desktop.

Useful defaults:

```bash
WORKFLOW_STORAGE_BACKEND=postgres
WORKFLOW_DATA_DIR=$HOME/.page-agent/workflow-backend
WORKFLOW_BACKEND_ADDR=127.0.0.1:38402
```

The script closes existing listeners on the backend port before starting a new
process.

## API Overview

The Page Agent extension should use one backend URL, for example:

```text
Workflow Backend: http://127.0.0.1:38402
Project ID: default
API Key: leave empty for local backend
```

Core endpoints:

```text
POST /api/memory/site-manuals/import
GET  /api/memory/site-manuals
GET  /api/memory/site-manuals/{id}
GET  /api/memory/site-manuals/{id}/wiki
POST /api/memory/site-manuals/{id}/rebuild
POST /api/memory/site-manuals/{id}/disable
POST /api/memory/site-manuals/{id}/enable
DELETE /api/memory/site-manuals/{id}
POST /api/memory/site-manuals/preview-context

POST /api/memory/page-observations
POST /api/memory/context
POST /api/memory/task-runs
GET  /api/memory/site-task-guides
GET  /api/memory/site-task-guides/{id}
POST /api/memory/site-task-guides/{id}/disable
POST /api/memory/site-task-guides/{id}/feedback
GET  /api/memory/inspector
POST /api/memory/maintenance/prune
```

Legacy generic document endpoints are removed, not deprecated, and should not be
called by the extension.

## Import A Site Manual

Manual import requires a site. Global, unscoped documents are rejected.

```bash
curl -s http://127.0.0.1:38402/api/memory/site-manuals/import \
  -H 'content-type: application/json' \
  -d '{
    "projectId": "default",
    "site": "ops.example.com",
    "module": "backup",
    "title": "Instance backup and restore manual",
    "sourceType": "markdown",
    "content": "The backup page has Full Backup and Incremental Backup tabs. Data restore starts from a Full Backup record and requires selecting a node IP before confirmation."
  }'
```

The backend stores the raw source, compiles it into `SiteManualWikiPage` records,
and creates short `SiteManualWikiChunk` records for context retrieval. Raw source
text is not directly injected into Page Agent prompts.

## Preview Manual Context

The manual library UI uses the same backend preview endpoint that task context
uses. This prevents the UI from simulating ranking differently from runtime.

```bash
curl -s http://127.0.0.1:38402/api/memory/site-manuals/preview-context \
  -H 'content-type: application/json' \
  -d '{
    "projectId": "default",
    "site": "ops.example.com",
    "module": "backup",
    "task": "restore an instance from the latest full backup",
    "currentUrl": "https://ops.example.com/instances/pg-1/backups",
    "pageObservation": {
      "title": "Data backup",
      "visibleText": ["Data backup", "Full Backup", "Data Restore"],
      "controls": [{"role": "button", "name": "Data Restore"}]
    }
  }'
```

The response includes matched wiki chunks, filtered reasons, and the exact
`<site_manual_knowledge>` prompt fragment.

## Observe A Page

Page observations are lightweight signals for hard gating and debugging. They do
not create a long-lived page knowledge base.

```bash
curl -s http://127.0.0.1:38402/api/memory/page-observations \
  -H 'content-type: application/json' \
  -d '{
    "projectId": "default",
    "task": "restore an instance from the latest full backup",
    "url": "https://ops.example.com/instances",
    "title": "Instances",
    "visibleText": ["Instances", "Data backup"],
    "controls": [{"role": "link", "name": "lzxpg"}],
    "source": "before_task"
  }'
```

## Request Memory Context

Call this before a task. The response includes `<site_task_guides>` and
`<site_manual_knowledge>` when same-site, same-module, page-gated candidates are
available.

```bash
curl -s http://127.0.0.1:38402/api/memory/context \
  -H 'content-type: application/json' \
  -d '{
    "projectId": "default",
    "task": "restore an instance from the latest full backup",
    "currentUrl": "https://ops.example.com/instances/pg-1/backups",
    "pageObservation": {
      "title": "Data backup",
      "visibleText": ["Data backup", "Full Backup", "Data Restore"],
      "controls": [{"role": "button", "name": "Data Restore"}]
    },
    "metadata": {"module": "backup"},
    "mode": "before_task"
  }'
```

Runtime rules:

- Current DOM/page observation has higher priority than memory.
- `SiteTaskGuide` is a reference guide, not a replay script.
- If a guide does not match the current page, Page Agent must abandon it.
- `SiteManualWiki` is auxiliary manual knowledge and can be stale.

## Submit A Task Run

Call this after a task. The backend saves the task run, compresses noisy action
paths, and creates or updates a reusable `SiteTaskGuide` when the evidence is
good enough.

```bash
curl -s http://127.0.0.1:38402/api/memory/task-runs \
  -H 'content-type: application/json' \
  -d '{
    "projectId": "default",
    "site": "ops.example.com",
    "taskTemplate": "restore {{instance_name}} from latest full backup",
    "summary": "Restored an instance from the latest full backup.",
    "originalPath": ["instances", "instance_detail", "monitoring", "instance_detail", "backups", "restore_dialog"],
    "status": "success",
    "actionSteps": [{
      "stepIndex": 1,
      "actionType": "click",
      "targetName": "Instance name",
      "valueTemplate": "{{instance_name}}"
    }, {
      "stepIndex": 2,
      "actionType": "click",
      "targetName": "Data backup"
    }, {
      "stepIndex": 3,
      "actionType": "click",
      "targetName": "Full Backup"
    }, {
      "stepIndex": 4,
      "actionType": "click",
      "targetName": "Data Restore"
    }]
  }'
```

Task guide generation must not store concrete instance names, IP addresses,
backup IDs, account names, tokens, or one-time IDs in searchable text.

## Inspect Saved Memory

```bash
curl -s 'http://127.0.0.1:38402/api/memory/inspector?projectId=default'
```

Inspector output should make runtime behavior debuggable:

- current page observation signal;
- candidate task guides and manual wiki chunks;
- hard-gate matched and filtered reasons;
- final prompt fragments injected into Page Agent;
- feedback labels such as `used_helpful`, `abandoned_mismatch`,
  `used_misleading`, `manual_helpful`, `manual_stale`, and `manual_unused`.

## Prune Old Memory Logs

The backend can prune log-like records by policy. Reusable guide and manual wiki
records are updated or disabled by stable IDs rather than written into timestamp
directories.

```bash
curl -s http://127.0.0.1:38402/api/memory/maintenance/prune \
  -H 'content-type: application/json' \
  -d '{
    "projectId": "default",
    "site": "ops.example.com",
    "maxTaskRuns": 100,
    "maxPageObservationEvents": 100,
    "maxMemoryContextEvents": 300,
    "maxFailureMemories": 50
  }'
```

## Tests

Backend:

```bash
cd apps/workflow-backend
go test ./...
```

Extension:

```bash
npm run typecheck
npm test
```

Chrome E2E should cover manual import, wiki preview, context injection, task guide
creation, second-run guide reuse, stale/abandoned feedback, and residual old API
checks.
