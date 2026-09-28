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
