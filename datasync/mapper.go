package datasync

import "context"

// Mapper transforms a batch of source records (type T) into sink records (type U).
type Mapper[T any, U any] interface {
	Map(ctx context.Context, records []T) ([]U, error)
}

// MapperFunc is a convenience adapter for simple mapping functions.
type MapperFunc[T any, U any] func(ctx context.Context, records []T) ([]U, error)

// Map calls the underlying function.
func (f MapperFunc[T, U]) Map(ctx context.Context, records []T) ([]U, error) {
	return f(ctx, records)
}

// IdentityMapper returns records unchanged. Use when Source and Sink share the same type.
func IdentityMapper[T any]() Mapper[T, T] {
	return MapperFunc[T, T](func(_ context.Context, records []T) ([]T, error) {
		return records, nil
	})
}
