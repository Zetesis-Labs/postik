-- Create "media" table
CREATE TABLE "media" (
  "id" uuid NOT NULL,
  "organization_id" uuid NOT NULL,
  "name" text NOT NULL,
  "path" text NOT NULL,
  "kind" text NOT NULL,
  "mime" text NOT NULL,
  "size" bigint NOT NULL,
  "alt" text NOT NULL DEFAULT '',
  "thumbnail_path" text NULL,
  "thumbnail_seconds" integer NULL,
  "created_at" timestamptz NOT NULL,
  "deleted_at" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "media_organization_fkey" FOREIGN KEY ("organization_id") REFERENCES "organizations" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "media_kind_check" CHECK (kind = ANY (ARRAY['image'::text, 'video'::text]))
);
-- Create index "media_organization_created_idx" to table: "media"
CREATE INDEX "media_organization_created_idx" ON "media" ("organization_id", "created_at" DESC);
