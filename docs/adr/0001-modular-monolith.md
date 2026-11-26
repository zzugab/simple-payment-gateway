# ADR-0001: Start with a Modular Monolith

- Status: Accepted
- Date: 2026-11-26

## Context

This repository is both a practical payment integration project and a laboratory for production-oriented backend engineering. Payment workflows require consistency, traceability, and careful failure handling. Starting with multiple independently deployed services would add distributed-systems complexity before there is evidence that it is needed.

## Decision

Start with a Go modular monolith. Keep domain modules and infrastructure adapters separated behind clear boundaries. Use PostgreSQL as the source of truth. Introduce asynchronous processing through a transactional outbox before adding an external broker.

## Consequences

### Positive
- Database transactions are easier to reason about.
- Local development and testing remain simple.
- Domain boundaries can be tested independently.
- Modules can be extracted later when scaling or ownership requires it.

### Trade-offs
- The initial application is deployed as one unit.
- Module boundaries must be enforced through package design and review.

## Future evolution

Add Redis, a message broker, dedicated workers, and service extraction only when measured requirements justify them.
