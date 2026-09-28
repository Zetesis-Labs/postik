// Package jobs runs postik's scheduled work on River: publishing post values
// and sweeping the posts that were left behind.
package jobs

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverdatabasesql"
	"github.com/riverqueue/river/rivertype"
	"github.com/uptrace/bun"

	"github.com/zetesis-labs/postik/internal/core/publish"
	"github.com/zetesis-labs/postik/internal/database/migrations"
	"github.com/zetesis-labs/postik/internal/linkedin"
	"github.com/zetesis-labs/postik/internal/postgres"
	"github.com/zetesis-labs/postik/internal/storage"
	"github.com/zetesis-labs/postik/internal/telegram"
	"github.com/zetesis-labs/postik/internal/tokens"
)

// PublishAttempts is how many times River tries a publish_value job.
const PublishAttempts = publish.MaxAttempts

// uniqueStates leave completed jobs out, so a post moved back to a date whose
// job already ran (and did nothing) still gets a new one.
var uniqueStates = []rivertype.JobState{
	rivertype.JobStateAvailable,
	rivertype.JobStatePending,
	rivertype.JobStateRetryable,
	rivertype.JobStateRunning,
	rivertype.JobStateScheduled,
}

// PublishValueArgs is the value Index of a post, for its PublishAt date.
type PublishValueArgs struct {
	PostID    uuid.UUID `json:"post_id"`
	PublishAt time.Time `json:"publish_at"`
	Index     int       `json:"index"`
}

func (PublishValueArgs) Kind() string { return "publish_value" }

func (PublishValueArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		MaxAttempts: PublishAttempts,
		UniqueOpts:  river.UniqueOpts{ByArgs: true, ByState: uniqueStates},
	}
}

type SweepScheduledArgs struct{}

func (SweepScheduledArgs) Kind() string { return "sweep_scheduled" }

type Deps struct {
	DB       *bun.DB
	Store    *postgres.Publications
	Telegram *telegram.Client
	LinkedIn *linkedin.Client
	Tokens   *tokens.Keeper
	Channels *postgres.Channels
	Files    storage.Files
	Notifier Notifier
	Digester Digester
	Now      func() time.Time
	Logger   *slog.Logger
}

type Jobs struct {
	client         *river.Client[*sql.Tx]
	PublishValue   *PublishValueWorker
	SweepScheduled *SweepWorker
	SuccessDigest  *DigestWorker
	TokenExpiry    *ExpiryWorker
}

func New(d Deps) (*Jobs, error) {
	if d.Notifier == nil {
		d.Notifier = nopNotifier{}
	}
	j := &Jobs{}
	j.PublishValue = &PublishValueWorker{deps: d, jobs: j, publishers: publishers(d)}
	j.SweepScheduled = &SweepWorker{deps: d, jobs: j}
	j.SuccessDigest = &DigestWorker{deps: d}
	j.TokenExpiry = &ExpiryWorker{deps: d}
	workers := river.NewWorkers()
	river.AddWorker(workers, j.PublishValue)
	river.AddWorker(workers, j.SweepScheduled)
	river.AddWorker(workers, j.SuccessDigest)
	river.AddWorker(workers, j.TokenExpiry)

	client, err := river.NewClient(riverdatabasesql.New(d.DB.DB), &river.Config{
		Schema:               migrations.RiverSchema,
		Queues:               map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 10}},
		Workers:              workers,
		PollOnly:             true,
		JobTimeout:           15 * time.Minute,
		RescueStuckJobsAfter: 20 * time.Minute,
		Logger:               d.Logger,
		PeriodicJobs: []*river.PeriodicJob{
			river.NewPeriodicJob(river.PeriodicInterval(time.Hour), func() (river.JobArgs, *river.InsertOpts) {
				return SweepScheduledArgs{}, nil
			}, &river.PeriodicJobOpts{RunOnStart: true}),
			river.NewPeriodicJob(river.PeriodicInterval(time.Hour), func() (river.JobArgs, *river.InsertOpts) {
				return SuccessDigestArgs{}, nil
			}, nil),
			river.NewPeriodicJob(river.PeriodicInterval(time.Hour), func() (river.JobArgs, *river.InsertOpts) {
				return TokenExpiryArgs{}, nil
			}, &river.PeriodicJobOpts{RunOnStart: true}),
		},
	})
	if err != nil {
		return nil, err
	}
	j.client = client
	return j, nil
}

// publishers are the networks postik can publish to.
func publishers(d Deps) map[string]Publisher {
	out := map[string]Publisher{
		"telegram": &telegramPublisher{client: d.Telegram, files: d.Files, logger: d.Logger},
	}
	if d.LinkedIn != nil {
		li := &linkedInPublisher{client: d.LinkedIn, tokens: d.Tokens, files: d.Files}
		out["linkedin"] = li
		out["linkedin-page"] = li
	}
	return out
}

// Start runs the workers until Stop.
func (j *Jobs) Start(ctx context.Context) error { return j.client.Start(ctx) }

// Stop waits for the running jobs to finish or ctx to end.
func (j *Jobs) Stop(ctx context.Context) error { return j.client.Stop(ctx) }

// SchedulePost queues the principal value of a scheduled post at its date,
// in tx when there is one. Queuing the same post and date twice is a no-op.
func (j *Jobs) SchedulePost(ctx context.Context, tx *sql.Tx, postID uuid.UUID, publishAt time.Time) error {
	return j.enqueue(ctx, tx, PublishValueArgs{PostID: postID, PublishAt: publishAt.UTC(), Index: 0}, publishAt)
}

func (j *Jobs) enqueue(ctx context.Context, tx *sql.Tx, args PublishValueArgs, at time.Time) error {
	opts := &river.InsertOpts{ScheduledAt: at.UTC()}
	if tx != nil {
		_, err := j.client.InsertTx(ctx, tx, args, opts)
		return err
	}
	_, err := j.client.Insert(ctx, args, opts)
	return err
}
