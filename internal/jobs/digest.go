package jobs

import (
	"context"

	"github.com/riverqueue/river"
)

type SuccessDigestArgs struct{}

func (SuccessDigestArgs) Kind() string { return "success_digest" }

// DigestWorker sends, once an hour, the summary of the successful publications.
type DigestWorker struct {
	river.WorkerDefaults[SuccessDigestArgs]
	deps Deps
}

func (w *DigestWorker) Work(ctx context.Context, _ *river.Job[SuccessDigestArgs]) error {
	if w.deps.Digester == nil {
		return nil
	}
	return w.deps.Digester.SendDigests(ctx)
}
