package franz

import (
	"context"
	"sync"

	"github.com/twmb/franz-go/pkg/kgo"
)

// Transaction represents a Kafka transaction
type Transaction struct {
	producer *Producer
	client   *kgo.Client
	ctx      context.Context
	records  []*kgo.Record

	mu       sync.Mutex
	aborted  bool
	finished bool
}

// TransactionFunc is executed within a transaction context
type TransactionFunc func(tx *Transaction) error

// BeginTransaction starts a new transaction
// The producer must be configured with a TransactionalID
func (p *Producer) BeginTransaction(ctx context.Context) (*Transaction, error) {
	if ctx == nil {
		return nil, ErrNilContext
	}

	if p.config.TransactionalID == "" {
		return nil, ErrNoTransactionalID
	}

	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return nil, ErrClientClosed
	}
	client := p.client
	p.mu.RUnlock()

	if err := client.BeginTransaction(); err != nil {
		p.Logger.Error(err, "Failed to begin transaction")
		return nil, err
	}

	p.Logger.Info("Transaction started")

	return &Transaction{
		producer: p,
		client:   client,
		ctx:      ctx,
		records:  make([]*kgo.Record, 0),
	}, nil
}

// Produce adds a record to the transaction
// The record will be sent when the transaction is committed
func (tx *Transaction) Produce(record *Record) error {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.finished {
		return ErrTransactionAborted
	}
	if tx.aborted {
		return ErrTransactionAborted
	}

	tx.records = append(tx.records, recordToKgo(record, tx.producer.config.DefaultTopic))
	return nil
}

// ProduceMany adds multiple records to the transaction
func (tx *Transaction) ProduceMany(records ...*Record) error {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.finished {
		return ErrTransactionAborted
	}
	if tx.aborted {
		return ErrTransactionAborted
	}

	for _, r := range records {
		tx.records = append(tx.records, recordToKgo(r, tx.producer.config.DefaultTopic))
	}
	return nil
}

// Commit commits the transaction
func (tx *Transaction) Commit() error {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.finished {
		return ErrTransactionAborted
	}
	if tx.aborted {
		return ErrTransactionAborted
	}

	tx.finished = true

	// Produce all records synchronously within the transaction
	if len(tx.records) > 0 {
		results := tx.client.ProduceSync(tx.ctx, tx.records...)
		for _, res := range results {
			if res.Err != nil {
				// If any record fails, abort the transaction
				tx.aborted = true
				tx.client.AbortBufferedRecords(tx.ctx)
				if err := tx.client.EndTransaction(tx.ctx, kgo.TryAbort); err != nil {
					tx.producer.Logger.Error(err, "Failed to abort transaction after produce error")
				}
				tx.producer.Logger.Error(res.Err, "Transaction aborted due to produce error")
				return res.Err
			}
		}
	}

	// Commit the transaction
	if err := tx.client.EndTransaction(tx.ctx, kgo.TryCommit); err != nil {
		tx.producer.Logger.Error(err, "Failed to commit transaction")
		return err
	}

	tx.producer.Logger.Info("Transaction committed", map[string]interface{}{
		"recordCount": len(tx.records),
	})

	return nil
}

// Abort aborts the transaction
func (tx *Transaction) Abort() error {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.finished {
		return nil // Already finished
	}

	tx.aborted = true
	tx.finished = true

	// Abort any buffered records
	tx.client.AbortBufferedRecords(tx.ctx)

	// End the transaction with abort
	if err := tx.client.EndTransaction(tx.ctx, kgo.TryAbort); err != nil {
		tx.producer.Logger.Error(err, "Failed to abort transaction")
		return err
	}

	tx.producer.Logger.Info("Transaction aborted")
	return nil
}

// IsAborted returns true if the transaction was aborted
func (tx *Transaction) IsAborted() bool {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	return tx.aborted
}

// RecordCount returns the number of records in the transaction
func (tx *Transaction) RecordCount() int {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	return len(tx.records)
}

// Transact executes a function within a transaction
// The transaction is committed on success and aborted on error or panic
func (p *Producer) Transact(ctx context.Context, fn TransactionFunc) error {
	tx, err := p.BeginTransaction(ctx)
	if err != nil {
		return err
	}

	// Handle panics by aborting the transaction
	defer func() {
		if r := recover(); r != nil {
			tx.Abort()
			panic(r) // Re-panic after cleanup
		}
	}()

	// Execute the transaction function
	if err := fn(tx); err != nil {
		if abortErr := tx.Abort(); abortErr != nil {
			p.Logger.Error(abortErr, "Failed to abort transaction after function error")
		}
		return err
	}

	// Commit the transaction
	return tx.Commit()
}

// TransactRecords is a convenience method to produce multiple records in a transaction
func (p *Producer) TransactRecords(ctx context.Context, records ...*Record) error {
	return p.Transact(ctx, func(tx *Transaction) error {
		return tx.ProduceMany(records...)
	})
}
