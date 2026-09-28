-- Modify "users" table
ALTER TABLE "users" ADD COLUMN "notifications_read_at" timestamptz NULL, ADD COLUMN "email_success" boolean NOT NULL DEFAULT true, ADD COLUMN "email_failure" boolean NOT NULL DEFAULT true;
-- Create "notifications" table
CREATE TABLE "notifications" (
  "id" uuid NOT NULL,
  "organization_id" uuid NOT NULL,
  "kind" text NOT NULL,
  "template" text NOT NULL,
  "params" jsonb NOT NULL DEFAULT '{}',
  "created_at" timestamptz NOT NULL,
  "digested_at" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "notifications_organization_fkey" FOREIGN KEY ("organization_id") REFERENCES "organizations" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "notifications_kind_check" CHECK (kind = ANY (ARRAY['info'::text, 'failure'::text, 'success'::text]))
);
-- Create index "notifications_organization_created_at_idx" to table: "notifications"
CREATE INDEX "notifications_organization_created_at_idx" ON "notifications" ("organization_id", "created_at");
