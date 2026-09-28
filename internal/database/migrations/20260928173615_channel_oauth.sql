-- Create "channel_credentials" table
CREATE TABLE "channel_credentials" (
  "channel_id" uuid NOT NULL,
  "access_token" bytea NOT NULL,
  "access_secret" bytea NULL,
  "refresh_token" bytea NULL,
  "expires_at" timestamptz NULL,
  "refresh_expires_at" timestamptz NULL,
  "expiry_warned_for" timestamptz NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("channel_id"),
  CONSTRAINT "channel_credentials_channel_fkey" FOREIGN KEY ("channel_id") REFERENCES "channels" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create "oauth_authorizations" table
CREATE TABLE "oauth_authorizations" (
  "state" text NOT NULL,
  "organization_id" uuid NOT NULL,
  "user_id" uuid NOT NULL,
  "provider" text NOT NULL,
  "channel_id" uuid NULL,
  "secret" bytea NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("state"),
  CONSTRAINT "oauth_authorizations_channel_fkey" FOREIGN KEY ("channel_id") REFERENCES "channels" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "oauth_authorizations_organization_fkey" FOREIGN KEY ("organization_id") REFERENCES "organizations" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "oauth_authorizations_user_fkey" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "oauth_authorizations_created_at_idx" to table: "oauth_authorizations"
CREATE INDEX "oauth_authorizations_created_at_idx" ON "oauth_authorizations" ("created_at");
