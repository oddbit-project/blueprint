package threadpool

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewThreadPool(t *testing.T) {
	tests := []struct {
		name        string
		workerCount int
		queueSize   int
		expectErr   bool
		errorTypeIs error
	}{
		{
			name:        "ErrorWithZeroWorkerCount",
			workerCount: 0,
			queueSize:   10,
			expectErr:   true,
			errorTypeIs: ErrInvalidWorkerCount,
		},
		{
			name:        "ErrorWithZeroQueueSize",
			workerCount: 1,
			queueSize:   0,
			expectErr:   true,
			errorTypeIs: ErrInvalidQueueSize,
		},
		{
			name:        "SuccessWithOneWorkerAndQueueSize",
			workerCount: 1,
			queueSize:   1,
			expectErr:   false,
		},
		{
			name:        "SuccessWithMultipleWorkerAndQueueSize",
			workerCount: 12,
			queueSize:   128,
			expectErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotThreadPool, gotErr := NewThreadPool(tt.workerCount, tt.queueSize)
			if tt.expectErr {
				require.Error(t, gotErr)
				require.Equal(t, tt.errorTypeIs, gotErr)
				return
			}
			require.NoError(t, gotErr)
			require.NotNil(t, gotThreadPool)
			require.Equal(t, gotThreadPool.workerCount, tt.workerCount)
			require.Equal(t, tt.queueSize, cap(gotThreadPool.jobQueue))
			require.Equal(t, gotThreadPool.GetWorkerCount(), 0)
			require.Equal(t, gotThreadPool.GetRequestCount(), uint64(0))
			require.Equal(t, gotThreadPool.GetQueueLen(), 0)
			require.Equal(t, gotThreadPool.GetQueueCapacity(), tt.queueSize)
		})
	}
}

func TestThreadPool_MultipleStartStop(t *testing.T) {
	var err error
	var pool *ThreadPool
	pool, err = NewThreadPool(8, 32)
	require.NoError(t, err)
	_ = pool.Start(context.Background())
	err = pool.Start(context.Background())
	require.Equal(t, err, ErrPoolAlreadyStarted)

	_ = pool.Stop()
	err = pool.Stop()
	require.Equal(t, err, ErrPoolNotStarted)
}
