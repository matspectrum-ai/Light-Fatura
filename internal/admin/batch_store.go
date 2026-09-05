package admin

import "context"

type batchingStore struct {
	Store
	batchSize int
}

// Batched keeps the admin Store contract intact while splitting large CSV
// imports into bounded persistence batches. All other operations delegate to
// the original Store.
func Batched(store Store, batchSize int) Store {
	if batchSize <= 0 {
		batchSize = 200
	}
	return &batchingStore{Store: store, batchSize: batchSize}
}

func (s *batchingStore) Import(ctx context.Context, rows []ImportRow) (ImportResult, error) {
	result := ImportResult{Rejected: []string{}}
	for start := 0; start < len(rows); start += s.batchSize {
		end := start + s.batchSize
		if end > len(rows) {
			end = len(rows)
		}
		batch, err := s.Store.Import(ctx, rows[start:end])
		if err != nil {
			return ImportResult{}, err
		}
		result.Imported += batch.Imported
		result.InvoicesCreated += batch.InvoicesCreated
		result.InvoicesUpdated += batch.InvoicesUpdated
		result.Rejected = append(result.Rejected, batch.Rejected...)
	}
	return result, nil
}
