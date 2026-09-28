CREATE TABLE users (
  id uuid NOT NULL,
  issuer text NOT NULL,
  subject text NOT NULL,
  email text NOT NULL,
  name text NOT NULL,
  created_at timestamptz NOT NULL,
  CONSTRAINT users_pkey PRIMARY KEY (id),
  CONSTRAINT users_issuer_subject_key UNIQUE (issuer, subject)
);

CREATE TABLE organizations (
  id uuid NOT NULL,
  name text NOT NULL,
  created_at timestamptz NOT NULL,
  CONSTRAINT organizations_pkey PRIMARY KEY (id)
);

CREATE TABLE memberships (
  organization_id uuid NOT NULL,
  user_id uuid NOT NULL,
  role text NOT NULL,
  created_at timestamptz NOT NULL,
  CONSTRAINT memberships_pkey PRIMARY KEY (organization_id, user_id),
  CONSTRAINT memberships_role_check CHECK (role IN ('USER', 'ADMIN', 'OWNER')),
  CONSTRAINT memberships_organization_fkey FOREIGN KEY (organization_id) REFERENCES organizations (id) ON DELETE CASCADE,
  CONSTRAINT memberships_user_fkey FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

CREATE INDEX memberships_user_id_idx ON memberships (user_id);

CREATE TABLE sessions (
  id_hash bytea NOT NULL,
  kind text NOT NULL,
  user_id uuid NULL,
  created_at timestamptz NOT NULL,
  last_seen_at timestamptz NOT NULL,
  CONSTRAINT sessions_pkey PRIMARY KEY (id_hash),
  CONSTRAINT sessions_kind_check CHECK (kind IN ('superadmin', 'member')),
  CONSTRAINT sessions_user_fkey FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

CREATE INDEX sessions_user_id_idx ON sessions (user_id);

CREATE TABLE oidc_logins (
  state_hash bytea NOT NULL,
  nonce text NOT NULL,
  code_verifier text NOT NULL,
  created_at timestamptz NOT NULL,
  CONSTRAINT oidc_logins_pkey PRIMARY KEY (state_hash)
);

CREATE INDEX oidc_logins_created_at_idx ON oidc_logins (created_at);

CREATE TABLE customers (
  id uuid NOT NULL,
  organization_id uuid NOT NULL,
  name text NOT NULL,
  created_at timestamptz NOT NULL,
  CONSTRAINT customers_pkey PRIMARY KEY (id),
  CONSTRAINT customers_organization_name_key UNIQUE (organization_id, name),
  CONSTRAINT customers_organization_fkey FOREIGN KEY (organization_id) REFERENCES organizations (id) ON DELETE CASCADE
);

CREATE TABLE channels (
  id uuid NOT NULL,
  organization_id uuid NOT NULL,
  provider text NOT NULL,
  external_id text NOT NULL,
  name text NOT NULL,
  username text NOT NULL DEFAULT '',
  picture text NULL,
  disabled boolean NOT NULL DEFAULT false,
  refresh_needed boolean NOT NULL DEFAULT false,
  in_between_steps boolean NOT NULL DEFAULT false,
  customer_id uuid NULL,
  posting_times integer[] NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT channels_pkey PRIMARY KEY (id),
  CONSTRAINT channels_organization_provider_external_key UNIQUE (organization_id, provider, external_id),
  CONSTRAINT channels_organization_fkey FOREIGN KEY (organization_id) REFERENCES organizations (id) ON DELETE CASCADE,
  CONSTRAINT channels_customer_fkey FOREIGN KEY (customer_id) REFERENCES customers (id) ON DELETE SET NULL
);

CREATE INDEX channels_organization_id_idx ON channels (organization_id);

CREATE TABLE telegram_connections (
  code text NOT NULL,
  organization_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  channel_id uuid NULL,
  CONSTRAINT telegram_connections_pkey PRIMARY KEY (code),
  CONSTRAINT telegram_connections_organization_fkey FOREIGN KEY (organization_id) REFERENCES organizations (id) ON DELETE CASCADE,
  CONSTRAINT telegram_connections_channel_fkey FOREIGN KEY (channel_id) REFERENCES channels (id) ON DELETE SET NULL
);

CREATE TABLE telegram_state (
  id smallint NOT NULL DEFAULT 1,
  next_offset bigint NOT NULL,
  CONSTRAINT telegram_state_pkey PRIMARY KEY (id),
  CONSTRAINT telegram_state_single_row CHECK (id = 1)
);

CREATE TABLE media (
  id uuid NOT NULL,
  organization_id uuid NOT NULL,
  name text NOT NULL,
  path text NOT NULL,
  kind text NOT NULL,
  mime text NOT NULL,
  size bigint NOT NULL,
  alt text NOT NULL DEFAULT '',
  thumbnail_path text NULL,
  thumbnail_seconds integer NULL,
  created_at timestamptz NOT NULL,
  deleted_at timestamptz NULL,
  CONSTRAINT media_pkey PRIMARY KEY (id),
  CONSTRAINT media_kind_check CHECK (kind IN ('image', 'video')),
  CONSTRAINT media_organization_fkey FOREIGN KEY (organization_id) REFERENCES organizations (id) ON DELETE CASCADE
);

CREATE INDEX media_organization_created_idx ON media (organization_id, created_at DESC);

CREATE TABLE tags (
  id uuid NOT NULL,
  organization_id uuid NOT NULL,
  name text NOT NULL,
  color text NOT NULL,
  created_at timestamptz NOT NULL,
  CONSTRAINT tags_pkey PRIMARY KEY (id),
  CONSTRAINT tags_organization_name_key UNIQUE (organization_id, name),
  CONSTRAINT tags_organization_fkey FOREIGN KEY (organization_id) REFERENCES organizations (id) ON DELETE CASCADE
);

CREATE TABLE post_groups (
  id uuid NOT NULL,
  organization_id uuid NOT NULL,
  origin text NOT NULL,
  created_by uuid NULL,
  created_at timestamptz NOT NULL,
  CONSTRAINT post_groups_pkey PRIMARY KEY (id),
  CONSTRAINT post_groups_origin_check CHECK (origin IN ('web', 'mcp')),
  CONSTRAINT post_groups_organization_fkey FOREIGN KEY (organization_id) REFERENCES organizations (id) ON DELETE CASCADE,
  CONSTRAINT post_groups_created_by_fkey FOREIGN KEY (created_by) REFERENCES users (id) ON DELETE SET NULL
);

CREATE TABLE post_group_tags (
  group_id uuid NOT NULL,
  tag_id uuid NOT NULL,
  CONSTRAINT post_group_tags_pkey PRIMARY KEY (group_id, tag_id),
  CONSTRAINT post_group_tags_group_fkey FOREIGN KEY (group_id) REFERENCES post_groups (id) ON DELETE CASCADE,
  CONSTRAINT post_group_tags_tag_fkey FOREIGN KEY (tag_id) REFERENCES tags (id) ON DELETE CASCADE
);

CREATE TABLE posts (
  id uuid NOT NULL,
  organization_id uuid NOT NULL,
  group_id uuid NOT NULL,
  channel_id uuid NOT NULL,
  status text NOT NULL,
  publish_at timestamptz NOT NULL,
  post_values jsonb NOT NULL,
  settings jsonb NOT NULL DEFAULT '{}',
  release_url text NULL,
  error text NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT posts_pkey PRIMARY KEY (id),
  CONSTRAINT posts_status_check CHECK (status IN ('draft', 'scheduled', 'published', 'error')),
  CONSTRAINT posts_organization_fkey FOREIGN KEY (organization_id) REFERENCES organizations (id) ON DELETE CASCADE,
  CONSTRAINT posts_group_fkey FOREIGN KEY (group_id) REFERENCES post_groups (id) ON DELETE CASCADE,
  CONSTRAINT posts_channel_fkey FOREIGN KEY (channel_id) REFERENCES channels (id) ON DELETE CASCADE
);

CREATE INDEX posts_organization_publish_at_idx ON posts (organization_id, publish_at);
CREATE INDEX posts_group_id_idx ON posts (group_id);
CREATE INDEX posts_channel_id_idx ON posts (channel_id);
