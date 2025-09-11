package threadpool

import "context"

type Job interface {
	Run(ctx context.Context)
}

type funcRunner struct {
	task func(ctx context.Context)
}

func (f *funcRunner) Run(ctx context.Context) {
	f.task(ctx)
}

func FuncRunner(job func(ctx context.Context)) Job {
	return &funcRunner{
		task: job,
	}
}
