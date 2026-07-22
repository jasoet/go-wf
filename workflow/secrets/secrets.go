// Package secrets resolves secret references worker-side so plaintext secrets
// never enter Temporal workflow history. Payloads carry references such as
// "secret://PGPASS"; activities resolve them at runtime via a Resolver.
package secrets

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// RefPrefix marks a payload value as a secret reference.
const RefPrefix = "secret://"

// Resolver resolves a secret reference (the part after "secret://") to its value.
type Resolver interface {
	Resolve(ctx context.Context, ref string) (string, error)
}

// ResolverFunc adapts a function to Resolver.
type ResolverFunc func(ctx context.Context, ref string) (string, error)

// Resolve implements Resolver.
func (f ResolverFunc) Resolve(ctx context.Context, ref string) (string, error) { return f(ctx, ref) }

// EnvResolver resolves refs from environment variables: ref "PGPASS" reads
// the variable prefix+"PGPASS" (default prefix "SECRET_").
func EnvResolver(prefix string) Resolver {
	return ResolverFunc(func(_ context.Context, ref string) (string, error) {
		v, ok := os.LookupEnv(prefix + ref)
		if !ok {
			return "", fmt.Errorf("secret %q not found (env %s%s)", ref, prefix, ref)
		}
		return v, nil
	})
}

var defaultResolver Resolver = EnvResolver("SECRET_")

// SetDefault replaces the process-wide resolver used by activities. Pass nil
// to restore the built-in SECRET_-prefixed env resolver.
func SetDefault(r Resolver) {
	if r == nil {
		defaultResolver = EnvResolver("SECRET_")
		return
	}
	defaultResolver = r
}

// Resolve returns value unchanged unless it has the secret:// prefix, in which
// case the reference is resolved via the default resolver.
func Resolve(ctx context.Context, value string) (string, error) {
	if !strings.HasPrefix(value, RefPrefix) {
		return value, nil
	}
	return defaultResolver.Resolve(ctx, strings.TrimPrefix(value, RefPrefix))
}

// ResolveMap resolves every value in m, returning a new map; m is not mutated.
func ResolveMap(ctx context.Context, m map[string]string) (map[string]string, error) {
	if len(m) == 0 {
		return m, nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		rv, err := Resolve(ctx, v)
		if err != nil {
			return nil, fmt.Errorf("key %s: %w", k, err)
		}
		out[k] = rv
	}
	return out, nil
}
