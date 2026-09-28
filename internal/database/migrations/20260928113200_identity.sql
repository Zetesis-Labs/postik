-- Create "oidc_logins" table
CREATE TABLE "oidc_logins" (
  "state_hash" bytea NOT NULL,
  "nonce" text NOT NULL,
  "code_verifier" text NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("state_hash")
);
-- Create index "oidc_logins_created_at_idx" to table: "oidc_logins"
CREATE INDEX "oidc_logins_created_at_idx" ON "oidc_logins" ("created_at");
-- Create "organizations" table
CREATE TABLE "organizations" (
  "id" uuid NOT NULL,
  "name" text NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("id")
);
-- Create "users" table
CREATE TABLE "users" (
  "id" uuid NOT NULL,
  "issuer" text NOT NULL,
  "subject" text NOT NULL,
  "email" text NOT NULL,
  "name" text NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "users_issuer_subject_key" UNIQUE ("issuer", "subject")
);
-- Create "memberships" table
CREATE TABLE "memberships" (
  "organization_id" uuid NOT NULL,
  "user_id" uuid NOT NULL,
  "role" text NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("organization_id", "user_id"),
  CONSTRAINT "memberships_organization_fkey" FOREIGN KEY ("organization_id") REFERENCES "organizations" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "memberships_user_fkey" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "memberships_role_check" CHECK (role = ANY (ARRAY['USER'::text, 'ADMIN'::text, 'OWNER'::text]))
);
-- Create index "memberships_user_id_idx" to table: "memberships"
CREATE INDEX "memberships_user_id_idx" ON "memberships" ("user_id");
-- Modify "sessions" table
ALTER TABLE "sessions" ADD CONSTRAINT "sessions_user_fkey" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE;
