// Package store provides a generic, typed key-value storage layer used by
// workflow tasks to persist and retrieve intermediate data.
//
// The two main interfaces are:
//
//   - [RawStore] — byte-level storage with Upload, Download, Delete, Exists, and
//     List operations.  Concrete implementations include [LocalStore] (filesystem)
//     and [S3Store] (S3-compatible object storage).
//
//   - [Store] — a typed wrapper around [RawStore] that uses a [Codec] to
//     serialise and deserialise Go values of any type T.  [JSONCodec] is the
//     default codec.
//
// Keys are built with the [Key] helper to ensure consistent, hierarchical
// naming across stores.  The [OTelStore] decorator adds OpenTelemetry tracing
// to any [RawStore] implementation.
package store
