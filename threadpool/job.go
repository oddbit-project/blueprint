package threadpool

import "context"

type Job interface {
	Run(ctx context.Context)
}
