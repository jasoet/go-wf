// Package datasync provides a Source-Mapper-Sink data synchronization pipeline
// built on Temporal workflows.
//
// A sync [Job] is composed of three stages:
//
//   - [Source] — produces a stream of records of type T.
//   - [Mapper] — transforms each record from type T to type U.
//   - [Sink] — writes the transformed records to a destination.
//
// Jobs are registered with a [Runner] and executed as Temporal workflows,
// benefiting from Temporal's durability, retry, and scheduling capabilities.
package datasync
