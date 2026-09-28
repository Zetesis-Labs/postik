CREATE TABLE sessions (
  id_hash bytea NOT NULL,
  kind text NOT NULL,
  user_id uuid NULL,
  created_at timestamptz NOT NULL,
  last_seen_at timestamptz NOT NULL,
  CONSTRAINT sessions_pkey PRIMARY KEY (id_hash),
  CONSTRAINT sessions_kind_check CHECK (kind IN ('superadmin', 'member'))
);

CREATE INDEX sessions_user_id_idx ON sessions (user_id);
