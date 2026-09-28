-- Create "post_groups" table
CREATE TABLE "post_groups" (
  "id" uuid NOT NULL,
  "organization_id" uuid NOT NULL,
  "origin" text NOT NULL,
  "created_by" uuid NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "post_groups_created_by_fkey" FOREIGN KEY ("created_by") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE SET NULL,
  CONSTRAINT "post_groups_organization_fkey" FOREIGN KEY ("organization_id") REFERENCES "organizations" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "post_groups_origin_check" CHECK (origin = ANY (ARRAY['web'::text, 'mcp'::text]))
);
-- Create "tags" table
CREATE TABLE "tags" (
  "id" uuid NOT NULL,
  "organization_id" uuid NOT NULL,
  "name" text NOT NULL,
  "color" text NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "tags_organization_name_key" UNIQUE ("organization_id", "name"),
  CONSTRAINT "tags_organization_fkey" FOREIGN KEY ("organization_id") REFERENCES "organizations" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create "post_group_tags" table
CREATE TABLE "post_group_tags" (
  "group_id" uuid NOT NULL,
  "tag_id" uuid NOT NULL,
  PRIMARY KEY ("group_id", "tag_id"),
  CONSTRAINT "post_group_tags_group_fkey" FOREIGN KEY ("group_id") REFERENCES "post_groups" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "post_group_tags_tag_fkey" FOREIGN KEY ("tag_id") REFERENCES "tags" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create "posts" table
CREATE TABLE "posts" (
  "id" uuid NOT NULL,
  "organization_id" uuid NOT NULL,
  "group_id" uuid NOT NULL,
  "channel_id" uuid NOT NULL,
  "status" text NOT NULL,
  "publish_at" timestamptz NOT NULL,
  "post_values" jsonb NOT NULL,
  "settings" jsonb NOT NULL DEFAULT '{}',
  "release_url" text NULL,
  "error" text NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "posts_channel_fkey" FOREIGN KEY ("channel_id") REFERENCES "channels" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "posts_group_fkey" FOREIGN KEY ("group_id") REFERENCES "post_groups" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "posts_organization_fkey" FOREIGN KEY ("organization_id") REFERENCES "organizations" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "posts_status_check" CHECK (status = ANY (ARRAY['draft'::text, 'scheduled'::text, 'published'::text, 'error'::text]))
);
-- Create index "posts_channel_id_idx" to table: "posts"
CREATE INDEX "posts_channel_id_idx" ON "posts" ("channel_id");
-- Create index "posts_group_id_idx" to table: "posts"
CREATE INDEX "posts_group_id_idx" ON "posts" ("group_id");
-- Create index "posts_organization_publish_at_idx" to table: "posts"
CREATE INDEX "posts_organization_publish_at_idx" ON "posts" ("organization_id", "publish_at");
