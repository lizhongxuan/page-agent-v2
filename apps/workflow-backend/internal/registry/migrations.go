package registry

var RegistryMigrations = []string{
	`create table if not exists workflow_candidates (
  id text primary key,
  project_id text not null,
  source text not null,
  task text not null,
  start_url text not null,
  recipe_draft json not null,
  status text not null,
  searchable boolean not null default false,
  created_at text not null,
  reviewed_at text
)`,
	`create table if not exists workflow_recipes (
  id text primary key,
  project_id text not null,
  site text not null,
  intent text not null,
  status text not null,
  searchable boolean not null default false,
  current_version integer not null,
  recipe json not null
)`,
	`create table if not exists workflow_outbox (
  id text primary key,
  type text not null,
  idempotency_key text not null,
  payload json not null,
  status text not null,
  created_at text not null,
  processed_at text
)`,
}
