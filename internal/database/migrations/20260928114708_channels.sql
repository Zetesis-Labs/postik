-- Create "telegram_state" table
CREATE TABLE "telegram_state" (
  "id" smallint NOT NULL DEFAULT 1,
  "next_offset" bigint NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "telegram_state_single_row" CHECK (id = 1)
);
-- Create "customers" table
CREATE TABLE "customers" (
  "id" uuid NOT NULL,
  "organization_id" uuid NOT NULL,
  "name" text NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "customers_organization_name_key" UNIQUE ("organization_id", "name"),
  CONSTRAINT "customers_organization_fkey" FOREIGN KEY ("organization_id") REFERENCES "organizations" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create "channels" table
CREATE TABLE "channels" (
  "id" uuid NOT NULL,
  "organization_id" uuid NOT NULL,
  "provider" text NOT NULL,
  "external_id" text NOT NULL,
  "name" text NOT NULL,
  "username" text NOT NULL DEFAULT '',
  "picture" text NULL,
  "disabled" boolean NOT NULL DEFAULT false,
  "refresh_needed" boolean NOT NULL DEFAULT false,
  "in_between_steps" boolean NOT NULL DEFAULT false,
  "customer_id" uuid NULL,
  "posting_times" integer[] NOT NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "channels_organization_provider_external_key" UNIQUE ("organization_id", "provider", "external_id"),
  CONSTRAINT "channels_customer_fkey" FOREIGN KEY ("customer_id") REFERENCES "customers" ("id") ON UPDATE NO ACTION ON DELETE SET NULL,
  CONSTRAINT "channels_organization_fkey" FOREIGN KEY ("organization_id") REFERENCES "organizations" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "channels_organization_id_idx" to table: "channels"
CREATE INDEX "channels_organization_id_idx" ON "channels" ("organization_id");
-- Create "telegram_connections" table
CREATE TABLE "telegram_connections" (
  "code" text NOT NULL,
  "organization_id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL,
  "channel_id" uuid NULL,
  PRIMARY KEY ("code"),
  CONSTRAINT "telegram_connections_channel_fkey" FOREIGN KEY ("channel_id") REFERENCES "channels" ("id") ON UPDATE NO ACTION ON DELETE SET NULL,
  CONSTRAINT "telegram_connections_organization_fkey" FOREIGN KEY ("organization_id") REFERENCES "organizations" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
