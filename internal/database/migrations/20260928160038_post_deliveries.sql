-- Create index "posts_status_publish_at_idx" to table: "posts"
CREATE INDEX "posts_status_publish_at_idx" ON "posts" ("status", "publish_at");
-- Create "post_deliveries" table
CREATE TABLE "post_deliveries" (
  "post_id" uuid NOT NULL,
  "publish_at" timestamptz NOT NULL,
  "value_index" integer NOT NULL,
  "state" text NOT NULL,
  "external_id" text NULL,
  "url" text NULL,
  "error" text NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("post_id", "publish_at", "value_index"),
  CONSTRAINT "post_deliveries_post_fkey" FOREIGN KEY ("post_id") REFERENCES "posts" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "post_deliveries_state_check" CHECK (state = ANY (ARRAY['sending'::text, 'sent'::text, 'failed'::text]))
);
