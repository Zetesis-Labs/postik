package jobs

import (
	"context"
	"time"

	"github.com/riverqueue/river"
)

// sweepWindow is how far back the sweep relaunches overdue posts (F13).
const sweepWindow = 48 * time.Hour

// SweepWorker relaunches the scheduled posts that were left without a publication.
type SweepWorker struct {
	river.WorkerDefaults[SweepScheduledArgs]
	deps Deps
	jobs *Jobs
}

func (w *SweepWorker) Work(ctx context.Context, _ *river.Job[SweepScheduledArgs]) error {
	now := w.deps.Now()
	posts, err := w.deps.Store.Overdue(ctx, now.Add(-sweepWindow), now)
	if err != nil {
		return err
	}
	for _, p := range posts {
		if err := w.jobs.SchedulePost(ctx, nil, p.ID, p.PublishAt); err != nil {
			return err
		}
	}
	return nil
}
