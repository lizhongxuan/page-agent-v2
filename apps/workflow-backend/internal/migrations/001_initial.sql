create extension if not exists vector;

create table if not exists projects (
  id text primary key,
  name text not null,
  created_at timestamptz not null default now()
);

create table if not exists users (
  id text primary key,
  name text not null,
  created_at timestamptz not null default now()
);

create table if not exists workflow_recipes (
  id text primary key,
  project_id text not null,
  site text not null,
  name text not null,
  intent text not null,
  description text not null default '',
  status text not null,
  current_version int not null,
  tags text[] not null default '{}',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table if not exists workflow_versions (
  workflow_id text not null references workflow_recipes(id),
  version int not null,
  recipe jsonb not null,
  summary text not null,
  created_by text,
  created_at timestamptz not null default now(),
  primary key (workflow_id, version)
);

create table if not exists workflow_candidates (
  id text primary key,
  project_id text not null,
  source text not null,
  task text not null,
  start_url text not null,
  recipe_draft jsonb not null,
  sensitive_redaction_report jsonb not null default '{}',
  artifact_refs jsonb not null default '[]',
  review_status text not null,
  notification_status text not null default 'pending_notify',
  searchable boolean not null default false,
  user_confirmed_at timestamptz,
  searchable_at timestamptz,
  created_at timestamptz not null default now()
);

create table if not exists workflow_runs (
  id text primary key,
  workflow_id text,
  workflow_version int,
  project_id text not null,
  task text not null,
  url text not null,
  variables_used jsonb not null default '{}',
  result text not null,
  failed_chunk_id text,
  failed_step_id text,
  fallback_reason text,
  duration_ms int,
  artifact_refs jsonb not null default '[]',
  created_at timestamptz not null default now()
);

create table if not exists selector_stats (
  workflow_id text not null,
  workflow_version int not null,
  step_id text not null,
  strategy text not null,
  selector text not null,
  success_count int not null default 0,
  fail_count int not null default 0,
  last_success_at timestamptz,
  last_fail_at timestamptz,
  last_failure_reason text,
  primary key (workflow_id, workflow_version, step_id, strategy, selector)
);

create table if not exists knowledge_documents (
  id text primary key,
  project_id text not null,
  type text not null,
  title text not null,
  source text not null,
  url text,
  tags text[] not null default '{}',
  content text not null,
  scope jsonb not null default '{}',
  confidence double precision not null default 0.5,
  status text not null default 'active',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table if not exists knowledge_embeddings (
  document_id text primary key references knowledge_documents(id),
  embedding vector(1536) not null,
  content text not null,
  updated_at timestamptz not null default now()
);

create table if not exists workflow_embeddings (
  workflow_id text not null,
  version int not null,
  embedding vector(1536) not null,
  content text not null,
  updated_at timestamptz not null default now(),
  primary key (workflow_id, version)
);

create table if not exists artifacts (
  id text primary key,
  project_id text not null,
  owner_type text not null,
  owner_id text not null,
  kind text not null,
  content_type text not null,
  size_bytes bigint not null,
  sha256 text not null,
  storage_backend text not null default 'local',
  relative_path text not null,
  created_at timestamptz not null default now(),
  expires_at timestamptz
);

create table if not exists workflow_patch_candidates (
  id text primary key,
  project_id text not null,
  workflow_id text not null,
  workflow_version int not null,
  patch_type text not null,
  status text not null default 'pending_review',
  patch jsonb not null,
  evidence jsonb not null default '{}',
  created_at timestamptz not null default now()
);

create table if not exists audit_logs (
  id text primary key,
  project_id text not null,
  actor text,
  action text not null,
  target_type text not null,
  target_id text not null,
  created_at timestamptz not null default now()
);

create index if not exists idx_workflow_recipes_project_status
  on workflow_recipes(project_id, status);
create index if not exists idx_workflow_recipes_site
  on workflow_recipes(site);
create index if not exists idx_artifacts_owner
  on artifacts(project_id, owner_type, owner_id);
