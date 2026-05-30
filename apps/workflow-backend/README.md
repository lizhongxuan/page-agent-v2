# Workflow Backend

The workflow backend stores PageAgent WebOps memory: business profiles, page
observations, page states, page transitions, task runs, experience memories,
failure memories, review items, and knowledge documents.

The PageAgent main flow should use the V2 memory endpoints:

```text
POST /api/memory/documents
POST /api/memory/page-observations
POST /api/memory/context
POST /api/memory/task-runs
GET  /api/memory/inspector
GET  /api/memory/reviews
POST /api/memory/reviews/{id}/approve
POST /api/memory/reviews/{id}/reject
POST /api/memory/maintenance/prune
```

## Local Startup

```bash
npm run workflow:deploy
```

This is the only local startup command for normal use. It starts PostgreSQL with
pgvector, builds and starts the workflow backend, disables Qdrant, and copies the
Chrome extension package to the desktop.

Useful defaults:

```bash
WORKFLOW_STORAGE_BACKEND=postgres
WORKFLOW_DATA_DIR=$HOME/.page-agent/workflow-backend
WORKFLOW_BACKEND_ADDR=127.0.0.1:38402
WORKFLOW_DISABLE_QDRANT=true
```

The script closes existing listeners on the backend and PostgreSQL ports before
starting new processes.

Memory records are upserted by stable keys where they represent reusable
knowledge:

- documents are keyed by project, source, and URL/title, so re-importing the
  same document updates it instead of creating a timestamped copy;
- document chunks replace previous chunks for the same document, so removed
  paragraphs stop being searchable;
- page observation events are keyed by project, site, URL pattern, and title,
  then updated with `seenCount` and `lastSeenAt`;
- repeated failure memories are keyed by the reusable task/page/action failure
  signature and updated with `occurrenceCount`.

Task runs, context events, and observation events are still useful audit data,
but they are bounded by retention limits instead of growing forever:

```bash
WORKFLOW_MEMORY_MAX_TASK_RUNS_PER_SITE=200
WORKFLOW_MEMORY_MAX_PAGE_OBSERVATIONS_PER_SITE=200
WORKFLOW_MEMORY_MAX_CONTEXT_EVENTS_PER_PROJECT=500
WORKFLOW_MEMORY_MAX_FAILURES_PER_SITE=100
```

The local database uses:

```text
database: page_agent_workflow
user: page_agent
port: 54329
```

Do not reuse the compose password outside local development.

## WebOps Memory V2

Use one backend URL in the extension, for example:

```text
Workflow Memory Backend: http://127.0.0.1:38402
Project ID: default
API Key: leave empty for local backend
```

`Knowledge enabled` in the extension only controls whether document evidence and
page summaries are injected into the agent context. It does not require a
separate knowledge backend URL.

### Import Documents

Import one document:

```bash
curl -s http://127.0.0.1:38402/api/memory/documents \
  -H 'content-type: application/json' \
  -d '{
    "projectId": "default",
    "documents": [{
      "title": "Service management manual",
      "source": "manual",
      "url": "https://ops.example.com/services",
      "content": "The service page can search by service name and read the status column.",
      "tags": ["service", "status"]
    }]
  }'
```

The import stores knowledge chunks and updates `business_system_profiles`.
Re-importing the same project/source/URL updates the existing document and
replaces its chunks.

### Observe A Page

```bash
curl -s http://127.0.0.1:38402/api/memory/page-observations \
  -H 'content-type: application/json' \
  -d '{
    "projectId": "default",
    "task": "check service status",
    "url": "https://ops.example.com/services?k=kme-prod-001",
    "title": "Service management",
    "visibleText": ["Service management", "Service name", "Status"],
    "controls": [{"role": "textbox", "name": "Service name"}],
    "source": "before_task"
  }'
```

The backend normalizes URL and page fingerprints. Variable instance values stay
out of searchable memory. Re-observing the same page updates the existing
observation event instead of appending a new timestamped event.

### Request Memory Context

Call this before a task. The response includes `<webops_memory>` plus business
context, current page, navigation hints, experience hints, failure warnings,
and knowledge evidence.

```bash
curl -s http://127.0.0.1:38402/api/memory/context \
  -H 'content-type: application/json' \
  -d '{
    "projectId": "default",
    "task": "check service status",
    "currentUrl": "https://ops.example.com/services",
    "pageObservation": {
      "title": "Service management",
      "visibleText": ["Service management", "Status"],
      "controls": [{"role": "textbox", "name": "Service name"}]
    },
    "riskPolicy": {"blocked": ["destructive"]},
    "mode": "before_task"
  }'
```

### Submit A Task Run

Call this after a task. The backend saves the task run, optimizes the path,
updates transitions, and writes experience or failure memory.

```bash
curl -s http://127.0.0.1:38402/api/memory/task-runs \
  -H 'content-type: application/json' \
  -d '{
    "projectId": "default",
    "site": "ops.example.com",
    "taskTemplate": "check {{service_name}} status",
    "summary": "Checked the service status.",
    "originalPath": ["page_service_list", "page_service_detail"],
    "status": "success",
    "actionSteps": [{
      "pageStateId": "page_service_list",
      "stepIndex": 1,
      "actionType": "fill",
      "targetName": "Service name",
      "valueTemplate": "{{service_name}}"
    }]
  }'
```

### Inspect Saved Memory

```bash
curl -s 'http://127.0.0.1:38402/api/memory/inspector?projectId=default'
```

### Memory Attribution Feedback

Memory attribution treats a task run as evidence about which recalled memory was
actually useful. A final task `success` or `failed` result is only a weak
outcome signal. It must not directly reward or punish every recalled memory
item.

The backend should instead compare recalled evidence with the real task path and
action targets:

- `helpful`: the recalled evidence was actually used and aligned with the real
  path or target without immediately causing a bad branch or selector failure;
- `unused`: the evidence was recalled but the agent did not actually use it;
- `misleading`: the evidence was used, but it pushed the task into a wrong
  branch, rollback, selector failure, or other avoidable error path;
- `stale`: the evidence points to a page rule, control, or route that no longer
  matches the current product state;
- `neutral`: the backend does not have enough execution evidence to decide.

Use the inspector to review both what the backend injected and how it evaluated
that injection. The intended debug flow is:

```bash
curl -s 'http://127.0.0.1:38402/api/memory/inspector?projectId=default'
```

Inspector output should let you inspect:

- the recalled `evidence` items or `evidenceRefs` attached to a memory context;
- the attribution `label` assigned to each evidence item;
- the per-evidence `signals` used for that decision;
- the accumulated evidence `stats` used by reranking.

To reproduce an attribution experiment, seed two similar memories for the same
task, then run the task so the agent first follows the wrong hint and later
finishes through the correct path. After that:

1. check the first context and confirm both hints can be recalled;
2. inspect the saved task run and attribution events;
3. request context again and verify the misleading hint has lower rank or is no
   longer injected into top results.

### Prune Old Memory Logs

The backend automatically applies the configured retention limits after memory
writes. To force cleanup manually:

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

Reusable records such as page states, transitions, approved experiences,
business profiles, and current knowledge documents are not deleted by this
endpoint. It only removes older log-like records beyond the requested limits.

## WebOps Memory E2E

Run the V2 WebOps memory E2E from the repository root:

```bash
npm run webops-memory:v2:e2e
```

The V2 test only calls `/api/memory/*`, drives Chrome with Playwright, and writes:

```text
report.json
backend-saved-data.json
chrome-user-flow.png
user-flow-report.html
backend.log
postgres.log
```

## Qdrant

The local startup path does not use Qdrant. `npm run workflow:deploy` explicitly
sets `WORKFLOW_DISABLE_QDRANT=true`.
