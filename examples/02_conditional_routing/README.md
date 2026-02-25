# Example 02: Conditional Routing with Guards

An OR-split workflow where guard functions route a loan application based on credit score. Two transitions share the same input place — only the one whose guard passes will fire.

## Petri Net Diagram

```mermaid
graph LR
  application((Application Received))
  approved((Auto-Approved))
  review_queue((Manual Review Queue))
  auto_approve[Auto Approve]
  manual_review[Send to Manual Review]
  application --> auto_approve
  auto_approve --> approved
  application --> manual_review
  manual_review --> review_queue
```

## Run

```bash
go run ./examples/02_conditional_routing
```

## What It Demonstrates

- **Guard functions** receive the execution context and candidate tokens, returning `(bool, error)`.
- **OR-split** pattern: multiple transitions compete for the same input place; only the enabled one fires.
- **Data-driven routing**: the credit score inside the `MapToken` determines the path.
- Running the same net twice with different data produces different execution paths.
