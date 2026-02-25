# JazzPetri Engine

**A Colored Petri Net execution engine for Go. Formally verifiable. Zero dependencies.**

JazzPetri is a production-grade workflow orchestration engine grounded in Colored Petri Net (CPN) theory. It models workflows as graphs of places (states), transitions (actions), and typed tokens (data), then executes them with sequential or concurrent firing, formal correctness proofs, deterministic time control, and checkpoint/restore. If you need to coordinate multi-step processes — order pipelines, approval gates, data transformation chains, event-driven microservices — and you want mathematical guarantees that your workflow cannot deadlock, JazzPetri gives you all of that with zero runtime dependencies and pure Go standard library.

## Quick Start

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jazzpetri/engine/clock"
	execctx "github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/engine"
	"github.com/jazzpetri/engine/petri"
	"github.com/jazzpetri/engine/task"
	"github.com/jazzpetri/engine/token"
)

func main() {
	// 1. Build the Petri net: received → validate → validated → fulfill → complete
	net := petri.NewPetriNet("order-flow", "Order Processing")

	received  := petri.NewPlace("received",  "Order Received",  10)
	validated := petri.NewPlace("validated", "Order Validated", 10)
	complete  := petri.NewPlace("complete",  "Order Complete",  10)
	complete.IsTerminal = true

	validate := petri.NewTransition("validate", "Validate Order").
		WithTask(&task.InlineTask{
			Name: "validate-order",
			Fn: func(ctx *execctx.ExecutionContext) error {
				fmt.Println("  validating order...")
				return nil
			},
		})

	fulfill := petri.NewTransition("fulfill", "Fulfill Order").
		WithTask(&task.InlineTask{
			Name: "fulfill-order",
			Fn: func(ctx *execctx.ExecutionContext) error {
				fmt.Println("  fulfilling order...")
				return nil
			},
		})

	net.AddPlace(received)
	net.AddPlace(validated)
	net.AddPlace(complete)
	net.AddTransition(validate)
	net.AddTransition(fulfill)
	net.AddArc(petri.NewArc("a1", "received",  "validate", 1))
	net.AddArc(petri.NewArc("a2", "validate",  "validated", 1))
	net.AddArc(petri.NewArc("a3", "validated", "fulfill",   1))
	net.AddArc(petri.NewArc("a4", "fulfill",   "complete",  1))

	if err := net.Resolve(); err != nil {
		log.Fatal(err)
	}

	// 2. Add a token (the order data)
	order := &token.Token{
		ID:   "order-001",
		Data: &token.MapToken{Value: map[string]interface{}{"orderId": "ORD-001", "amount": 99.99}},
	}
	if err := received.AddToken(order); err != nil {
		log.Fatal(err)
	}

	// 3. Create execution context and run
	ctx := execctx.NewExecutionContext(context.Background(), clock.NewRealTimeClock(), nil)
	eng := engine.NewEngine(net, ctx, engine.Config{
		Mode:         engine.ModeSequential,
		PollInterval: 10 * time.Millisecond,
	})

	fmt.Println("Starting order processing workflow...")
	eng.Start()
	eng.Wait()
	eng.Stop()

	fmt.Printf("Tokens in 'complete': %d\n", complete.TokenCount())
}
```

```
Starting order processing workflow...
  validating order...
  fulfilling order...
Tokens in 'complete': 1
```

## Features

- **Colored Petri Net Model** — Places, transitions, arcs with typed tokens and guard functions. Guards receive the full token set and can route based on data values, making every transition a first-class conditional.
- **Execution Engine** — Sequential and concurrent execution modes, step-by-step debugging, dry-run analysis, pause/resume, and health checks with execution statistics.
- **Formal Verification** — Prove deadlock freedom, boundedness, reachability, mutual exclusion, and place invariants before a single token flows. Get a `ProofCertificate` you can log at startup or embed in test assertions.
- **Time Management** — `VirtualClock` for fully deterministic test execution (advance time programmatically), `RealTimeClock` for production. Swap with a single import — no mocks needed.
- **Trigger System** — Automatic (fire when enabled), timed (delay or cron-like schedule), manual (human approval gate), and event-based triggers compose on any transition.
- **Task Framework** — `InlineTask` (Go closures), `HTTPTask` (REST calls), `CommandTask` (OS processes), `ParallelTask`, `SequentialTask`, `RetryTask` (exponential backoff). All composable.
- **Checkpoint/Restore** — Snapshot the marking (token distribution) at any point, serialize to JSON, restore to any prior state. Full event log replay also supported.
- **Zero Dependencies** — Pure Go standard library. No vendored frameworks, no hidden transitive pulls. `go get` and build.

## Architecture

Package dependency graph (arrows point toward dependencies):

```
token   clock   event
  \       |      /
      context
        / \
     task  petri
        \   / \
        engine  state
             \  /
          verification
```

Each layer depends only on layers below it. `token`, `clock`, and `event` are foundational and have no intra-project dependencies. `verification` sits at the top and can introspect any assembled net.

## Packages

| Package | Description |
|---|---|
| `token` | Token type system: `TokenData` interface, `IntToken`, `StringToken`, `MapToken`, pluggable `TypeRegistry` for serialization |
| `clock` | Time abstraction: `Clock` interface, `VirtualClock` (test), `RealTimeClock` (production) |
| `event` | Pub/sub event system for event-triggered transitions |
| `context` | `ExecutionContext` carrying clock, logger, metrics, tracer, and error handler — all pluggable |
| `task` | `Task` interface and implementations: `InlineTask`, `HTTPTask`, `CommandTask`, `ParallelTask`, `SequentialTask`, `RetryTask`, `WaitTask` |
| `petri` | Petri net model: `PetriNet`, `Place`, `Transition`, `Arc`, `GuardFunc`, trigger types |
| `engine` | Workflow execution: `Engine`, `Config`, `ModeSequential`, `ModeConcurrent`, step/dry-run/pause |
| `state` | Marking capture, event log, checkpoint serialization and restore |
| `verification` | State space exploration: `Verifier`, `CheckDeadlockFreedom`, `CheckBoundedness`, `CheckReachability`, `CheckMutualExclusion`, `CheckPlaceInvariant`, `GenerateCertificate` |

## Examples

Six runnable examples live in `examples/`:

| Example | Description |
|---|---|
| [`01_simple_workflow`](examples/01_simple_workflow/) | Linear order processing pipeline — the fastest path from zero to running |
| [`02_conditional_routing`](examples/02_conditional_routing/) | Guard functions that route tokens based on data values |
| [`03_parallel_processing`](examples/03_parallel_processing/) | Concurrent transition firing with a goroutine pool |
| [`04_formal_verification`](examples/04_formal_verification/) | Build a state space and prove deadlock freedom before execution |
| [`05_timed_triggers`](examples/05_timed_triggers/) | Delay and schedule transitions with `VirtualClock` for deterministic tests |
| [`06_human_approval`](examples/06_human_approval/) | Manual trigger gates that pause execution until explicitly approved |

Run any example:

```bash
go run ./examples/01_simple_workflow
```

## Installation

```bash
go get github.com/jazzpetri/engine
```

Requires Go 1.25 or later. No other dependencies.

## Jazz Platform

> **Looking for YAML workflows, AI agents, a visual designer, and enterprise features?**
>
> The Jazz Platform builds on this engine to provide:
>
> - Declarative YAML/JSON workflow compiler — author nets without writing Go
> - AI agent integration with LLM reasoning loops and tool calls
> - Visual workflow designer and real-time monitoring dashboard
> - Human-in-the-loop approval gates with audit trail
> - PostgreSQL and BadgerDB persistence adapters
> - Prometheus metrics and OpenTelemetry tracing out of the box
>
> [Contact us](https://jazzpetri.dev)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

Apache License 2.0. See [LICENSE](LICENSE).
