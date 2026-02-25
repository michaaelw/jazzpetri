# Workflow Routing Patterns in Jazz3

## Introduction

Workflow routing patterns define how work flows through a process - where it splits, joins, branches, and merges. These patterns are fundamental building blocks for designing any workflow system, from simple sequential processes to complex distributed systems.

Jazz3 implements all four fundamental workflow routing patterns using Colored Petri Net primitives (places, transitions, arcs, and guards). Understanding these patterns is essential for effective workflow design.

### What are Workflow Routing Patterns?

Routing patterns describe the control flow logic in workflows:

- **How work splits** into parallel branches
- **How parallel work synchronizes** before continuing
- **How decisions route** to different paths
- **How alternative paths merge** back together

### Why They Matter

These patterns appear in virtually every business process:

- **Order fulfillment**: Parallel inventory and credit checks, then synchronize
- **Approval workflows**: Route based on amount, merge approval/rejection paths
- **Data processing pipelines**: Fan-out to workers, fan-in results
- **Error handling**: Route to retry/failure paths, merge to notification

### How Jazz3 Implements Them

Jazz3 uses Petri net theory to provide formal, well-defined implementations:

- **AND-split/AND-join**: Multiple arcs from/to transitions
- **OR-split/OR-join**: Multiple transitions with guards
- **Guards**: Enable data-driven, deterministic routing
- **No special syntax**: Just compose basic Petri net elements

---

## The Four Fundamental Patterns

### 1. AND-Split (Parallel Fan-out)

**Definition**: One transition creates multiple parallel flows by having multiple output arcs.

**Purpose**: Start multiple tasks simultaneously (fan-out, parallel processing).

**Visual Diagram**:
```
         ┌─────────────────────┐
         │  split_transition   │ (transition with multiple outputs)
         └─────────────────────┘
                    │
          ┌─────────┴─────────┐
          ▼                   ▼
    output_place_1      output_place_2
```

**Implementation**:
```go
// Create transition
split := petri.NewTransition("split", "Split Work")

// ONE input arc
net.AddArc(petri.NewArc("a1", "input", "split", 1))

// MULTIPLE output arcs - this creates the AND-split!
net.AddArc(petri.NewArc("a2", "split", "branch_a", 1))
net.AddArc(petri.NewArc("a3", "split", "branch_b", 1))
net.AddArc(petri.NewArc("a4", "split", "branch_c", 1))
```

**How It Works**:
- When `split` fires, it consumes 1 token from `input`
- Produces 1 token in `branch_a`
- Produces 1 token in `branch_b`
- Produces 1 token in `branch_c`
- **Result**: 3 parallel flows from 1 input

**Use Cases**:
- Parallel validation (check inventory AND credit AND shipping)
- Fan-out to multiple workers
- Concurrent notifications (email AND SMS AND webhook)
- Parallel data enrichment

**When to Use**:
- Need to execute multiple tasks simultaneously
- Tasks are independent and can run in parallel
- All parallel branches should start at the same time

**Example Scenario**:
```go
// Order processing: Split into parallel validation tasks
order_received → split_validations → (inventory_queue AND credit_queue AND shipping_queue)
```

---

### 2. AND-Join (Synchronization)

**Definition**: One transition waits for ALL parallel flows to complete by having multiple input arcs.

**Purpose**: Synchronize parallel branches (fan-in, barrier, wait for all).

**Visual Diagram**:
```
    input_place_1      input_place_2
          │                   │
          └─────────┬─────────┘
                    ▼
         ┌─────────────────────┐
         │   join_transition   │ (transition with multiple inputs)
         └─────────────────────┘
                    │
                    ▼
              output_place
```

**Implementation**:
```go
// Create transition
join := petri.NewTransition("join", "Synchronize")

// MULTIPLE input arcs - this creates the AND-join!
net.AddArc(petri.NewArc("a1", "branch_a", "join", 1))
net.AddArc(petri.NewArc("a2", "branch_b", "join", 1))
net.AddArc(petri.NewArc("a3", "branch_c", "join", 1))

// ONE output arc
net.AddArc(petri.NewArc("a4", "join", "output", 1))
```

**How It Works**:
- Transition `join` is only enabled when:
  - `branch_a` has ≥1 token AND
  - `branch_b` has ≥1 token AND
  - `branch_c` has ≥1 token
- When it fires:
  - Consumes 1 token from each input place
  - Produces 1 token in `output`
- **Result**: Barrier - won't proceed until all parallel work completes

**Use Cases**:
- Wait for all validations to pass
- Aggregate results from parallel workers
- Barrier synchronization in pipelines
- Join after fan-out processing

**When to Use**:
- Need to wait for ALL parallel tasks to complete
- Cannot proceed until all branches finish
- Need to aggregate/combine results from parallel flows

**Example Scenario**:
```go
// Order processing: Wait for all validations before proceeding
(inventory_checked AND credit_checked AND shipping_checked) → synchronize → ready_for_payment
```

**Critical Difference from OR-Join**:
- **AND-join**: Waits for ALL inputs (synchronization)
- **OR-join**: Accepts ANY input (simple merge)

---

### 3. OR-Split (Exclusive Choice)

**Definition**: Multiple transitions compete for the same input, with guards determining which ONE fires.

**Purpose**: Route to exactly one path based on data or conditions (branching, decision point).

**Visual Diagram**:
```
              decision_place
                    │
          ┌─────────┼─────────┐
          ▼         ▼         ▼
    route_high  route_med  route_low
    (guard:     (guard:    (guard:
    priority    priority   priority
    == "high")  == "med")  == "low")
          │         │         │
          ▼         ▼         ▼
    high_path   med_path   low_path
```

**Implementation**:
```go
// Create THREE transitions, all reading from SAME place

// High priority path
highTrans := petri.NewTransition("route_high", "Route High Priority")
highTrans.WithGuard(func(tokens []*token.Token) bool {
    order := tokens[0].Data.(*Order)
    return order.Priority == "high"
})
net.AddArc(petri.NewArc("a1", "decision", "route_high", 1))
net.AddArc(petri.NewArc("a2", "route_high", "high_path", 1))

// Medium priority path
medTrans := petri.NewTransition("route_med", "Route Medium Priority")
medTrans.WithGuard(func(tokens []*token.Token) bool {
    order := tokens[0].Data.(*Order)
    return order.Priority == "medium"
})
net.AddArc(petri.NewArc("a3", "decision", "route_med", 1))  // Same source!
net.AddArc(petri.NewArc("a4", "route_med", "med_path", 1))

// Low priority path
lowTrans := petri.NewTransition("route_low", "Route Low Priority")
lowTrans.WithGuard(func(tokens []*token.Token) bool {
    order := tokens[0].Data.(*Order)
    return order.Priority == "low"
})
net.AddArc(petri.NewArc("a5", "decision", "route_low", 1))  // Same source!
net.AddArc(petri.NewArc("a6", "route_low", "low_path", 1))
```

**How It Works**:
- All three transitions see the token in `decision`
- Engine evaluates guards for each transition
- Only ONE guard returns true (mutual exclusivity)
- That transition fires, consuming the token
- Other transitions cannot fire
- **Result**: Exactly one path is chosen

**Critical Requirements**:
1. **Guards MUST be mutually exclusive** - only one can be true
2. **Guards SHOULD cover all cases** - or token gets stuck
3. **Guards SHOULD be deterministic** - same input → same result

**Use Cases**:
- Route by priority (high/medium/low)
- Route by type (credit card/debit/PayPal)
- Route by status (success/failure/retry)
- Route by threshold (amount < 1000 / ≥ 1000)

**When to Use**:
- Need to choose exactly ONE path from multiple alternatives
- Decision is based on token data
- Alternative paths require different processing

**Example Scenario**:
```go
// Order processing: Route based on payment result
payment_processed → route_by_status → (success_path OR failure_path OR retry_path)
```

**Common Mistake - Non-Exclusive Guards**:
```go
// BAD: Overlapping guards
t1.WithGuard(func(tokens) bool { return amount >= 100 })  // true for 100-999
t2.WithGuard(func(tokens) bool { return amount >= 500 })  // ALSO true for 500-999!

// GOOD: Exclusive guards
t1.WithGuard(func(tokens) bool { return amount >= 100 && amount < 500 })
t2.WithGuard(func(tokens) bool { return amount >= 500 })
```

---

### 4. OR-Join (Simple Merge)

**Definition**: Multiple alternative paths merge to the same destination through separate transitions.

**Purpose**: Merge alternative flows without synchronization (reconvergence).

**Visual Diagram**:
```
    high_path      med_path       low_path
          │              │              │
          ▼              ▼              ▼
    merge_high    merge_med     merge_low
          │              │              │
          └──────────────┼──────────────┘
                         ▼
                  merged_place
```

**Implementation**:
```go
// Create THREE transitions, all outputting to SAME place

// Merge from high path
mergeHigh := petri.NewTransition("merge_high", "Merge High")
net.AddArc(petri.NewArc("a1", "high_path", "merge_high", 1))
net.AddArc(petri.NewArc("a2", "merge_high", "merged", 1))  // Same dest

// Merge from medium path
mergeMed := petri.NewTransition("merge_med", "Merge Medium")
net.AddArc(petri.NewArc("a3", "med_path", "merge_med", 1))
net.AddArc(petri.NewArc("a4", "merge_med", "merged", 1))   // Same dest

// Merge from low path
mergeLow := petri.NewTransition("merge_low", "Merge Low")
net.AddArc(petri.NewArc("a5", "low_path", "merge_low", 1))
net.AddArc(petri.NewArc("a6", "merge_low", "merged", 1))   // Same dest
```

**How It Works**:
- Whichever path was taken (from OR-split), that token arrives
- That path's merge transition fires
- Token moves to `merged` place
- Other merge transitions don't fire (their inputs are empty)
- **Result**: Simple, non-blocking merge

**Critical Difference from AND-Join**:
- **AND-join**: Waits for ALL inputs (synchronization)
- **OR-join**: Accepts ANY input (simple merge, no waiting)

**Use Cases**:
- Merge after exclusive choice routing
- Reconverge alternative processing paths
- Common next step after branching
- Merge error handling paths

**When to Use**:
- After an OR-split (exclusive choice)
- Alternative paths need to reconverge
- No synchronization needed (only one path will have token)

**Example Scenario**:
```go
// Order processing: All payment outcomes merge to notification
(success_path OR failure_path OR retry_path) → merge → notification_queue
```

---

## Combining Patterns

Real workflows combine these four patterns into complete processes.

### Pattern Combination 1: Parallel-then-Sequence

```
Input → AND-split → Parallel work → AND-join → Sequential work
```

**Example**: Gather data from multiple sources, then process combined result
```go
Order → Split → (Check inventory, Check credit, Check shipping) → Join → Process payment
```

### Pattern Combination 2: Sequence-then-Choice

```
Sequential work → OR-split → Alternative paths
```

**Example**: Validate request, then route based on validation result
```go
Receive order → Validate → Route by priority → (High path OR Low path)
```

### Pattern Combination 3: Choice-then-Merge

```
OR-split → Alternative processing → OR-join
```

**Example**: Different processing based on type, then common notification
```go
Route by type → (Credit card process OR PayPal process) → Merge → Notify
```

### Pattern Combination 4: Full Workflow (Complete Example)

```
Sequence → AND-split → Parallel → AND-join → Sequence → OR-split → Alternatives → OR-join → Sequence
```

**Example**: Order processing with parallel validation, payment, routing, and notification
```
Order → Split → (Inventory check AND Credit check) → Join → 
Payment → Route → (Success OR Failure OR Retry) → Merge → Notify
```

---

## Common Workflow Scenarios

### Scenario 1: Parallel Validation with Conditional Routing

**Business Process**: Validate order in parallel, then route based on result

```go
// Pattern: AND-split → Parallel → AND-join → OR-split

// 1. AND-SPLIT: Create parallel validation tasks
split := petri.NewTransition("split", "Split Validations")
net.AddArc(petri.NewArc("a1", "order", "split", 1))
net.AddArc(petri.NewArc("a2", "split", "inventory_queue", 1))
net.AddArc(petri.NewArc("a3", "split", "credit_queue", 1))

// 2. Parallel tasks (with tasks attached)
checkInventory := petri.NewTransition("check_inv", "Check Inventory")
checkInventory.WithTask(&task.InlineTask{/* ... */})
net.AddArc(petri.NewArc("a4", "inventory_queue", "check_inv", 1))
net.AddArc(petri.NewArc("a5", "check_inv", "inv_checked", 1))

checkCredit := petri.NewTransition("check_credit", "Check Credit")
checkCredit.WithTask(&task.InlineTask{/* ... */})
net.AddArc(petri.NewArc("a6", "credit_queue", "check_credit", 1))
net.AddArc(petri.NewArc("a7", "check_credit", "credit_checked", 1))

// 3. AND-JOIN: Wait for both validations
join := petri.NewTransition("join", "Join Validations")
net.AddArc(petri.NewArc("a8", "inv_checked", "join", 1))
net.AddArc(petri.NewArc("a9", "credit_checked", "join", 1))
net.AddArc(petri.NewArc("a10", "join", "validated", 1))

// 4. OR-SPLIT: Route based on validation result
approveRoute := petri.NewTransition("route_approve", "Route Approve")
approveRoute.WithGuard(func(tokens []*token.Token) bool {
    return tokens[0].Data.(*Order).AllChecksPass()
})
net.AddArc(petri.NewArc("a11", "validated", "route_approve", 1))
net.AddArc(petri.NewArc("a12", "route_approve", "approved_path", 1))

rejectRoute := petri.NewTransition("route_reject", "Route Reject")
rejectRoute.WithGuard(func(tokens []*token.Token) bool {
    return !tokens[0].Data.(*Order).AllChecksPass()
})
net.AddArc(petri.NewArc("a13", "validated", "route_reject", 1))
net.AddArc(petri.NewArc("a14", "route_reject", "rejected_path", 1))
```

### Scenario 2: Priority-Based Processing Pipeline

**Business Process**: Route by priority, process, then merge to output

```go
// Pattern: OR-split → Parallel alternatives → OR-join

// 1. OR-SPLIT: Route by priority
high := petri.NewTransition("route_high", "High Priority")
high.WithGuard(func(tokens []*token.Token) bool {
    return tokens[0].Data.(*Request).Priority > 8
})
net.AddArc(petri.NewArc("a1", "input", "route_high", 1))
net.AddArc(petri.NewArc("a2", "route_high", "high_queue", 1))

low := petri.NewTransition("route_low", "Low Priority")
low.WithGuard(func(tokens []*token.Token) bool {
    return tokens[0].Data.(*Request).Priority <= 8
})
net.AddArc(petri.NewArc("a3", "input", "route_low", 1))
net.AddArc(petri.NewArc("a4", "route_low", "low_queue", 1))

// 2. Alternative processing (different tasks)
processHigh := petri.NewTransition("proc_high", "Process High")
processHigh.WithTask(&task.InlineTask{/* fast path */})
net.AddArc(petri.NewArc("a5", "high_queue", "proc_high", 1))
net.AddArc(petri.NewArc("a6", "proc_high", "high_done", 1))

processLow := petri.NewTransition("proc_low", "Process Low")
processLow.WithTask(&task.InlineTask{/* batch path */})
net.AddArc(petri.NewArc("a7", "low_queue", "proc_low", 1))
net.AddArc(petri.NewArc("a8", "proc_low", "low_done", 1))

// 3. OR-JOIN: Merge back to output
mergeHigh := petri.NewTransition("merge_high", "Merge High")
net.AddArc(petri.NewArc("a9", "high_done", "merge_high", 1))
net.AddArc(petri.NewArc("a10", "merge_high", "output", 1))

mergeLow := petri.NewTransition("merge_low", "Merge Low")
net.AddArc(petri.NewArc("a11", "low_done", "merge_low", 1))
net.AddArc(petri.NewArc("a12", "merge_low", "output", 1))
```

### Scenario 3: Fan-out/Fan-in Data Processing

**Business Process**: Split work to multiple workers, aggregate results

```go
// Pattern: AND-split → Parallel workers → AND-join

// 1. AND-SPLIT: Fan-out to 3 workers
split := petri.NewTransition("split", "Split Work")
net.AddArc(petri.NewArc("a1", "input", "split", 1))
net.AddArc(petri.NewArc("a2", "split", "worker_1", 1))
net.AddArc(petri.NewArc("a3", "split", "worker_2", 1))
net.AddArc(petri.NewArc("a4", "split", "worker_3", 1))

// 2. Parallel workers
for i := 1; i <= 3; i++ {
    worker := petri.NewTransition(fmt.Sprintf("work_%d", i), fmt.Sprintf("Worker %d", i))
    worker.WithTask(&task.InlineTask{/* process chunk */})
    net.AddArc(petri.NewArc(fmt.Sprintf("a%d", 4+i*2-1), fmt.Sprintf("worker_%d", i), fmt.Sprintf("work_%d", i), 1))
    net.AddArc(petri.NewArc(fmt.Sprintf("a%d", 4+i*2), fmt.Sprintf("work_%d", i), fmt.Sprintf("done_%d", i), 1))
}

// 3. AND-JOIN: Wait for all workers
join := petri.NewTransition("join", "Aggregate Results")
join.WithTask(&task.InlineTask{/* combine results */})
net.AddArc(petri.NewArc("a11", "done_1", "join", 1))
net.AddArc(petri.NewArc("a12", "done_2", "join", 1))
net.AddArc(petri.NewArc("a13", "done_3", "join", 1))
net.AddArc(petri.NewArc("a14", "join", "output", 1))
```

---

## Implementation Notes

### Arc Weights and Routing Patterns

Arc weights affect how patterns behave:

**AND-split with weights**:
```go
// Split with different weights
net.AddArc(petri.NewArc("a1", "split", "branch_a", 2))  // 2 tokens to branch_a
net.AddArc(petri.NewArc("a2", "split", "branch_b", 1))  // 1 token to branch_b
// Fires when input has ≥1 token, produces 2 to branch_a, 1 to branch_b
```

**AND-join with weights**:
```go
// Join requiring different token counts
net.AddArc(petri.NewArc("a1", "branch_a", "join", 2))  // Need 2 tokens from branch_a
net.AddArc(petri.NewArc("a2", "branch_b", "join", 1))  // Need 1 token from branch_b
// Fires only when branch_a has ≥2 tokens AND branch_b has ≥1 token
```

### Guard Function Best Practices

**1. Keep Guards Pure and Deterministic**:
```go
// GOOD: Pure function based on token data
guard := func(tokens []*token.Token) bool {
    return tokens[0].Data.(*Order).Amount >= 1000
}

// BAD: Non-deterministic (uses random, time, external state)
guard := func(tokens []*token.Token) bool {
    return rand.Float64() > 0.5  // Random choice - wrong!
}
```

**2. Ensure Mutual Exclusivity for OR-Split**:
```go
// GOOD: Mutually exclusive ranges
t1.WithGuard(func(t []*token.Token) bool { return amount < 100 })
t2.WithGuard(func(t []*token.Token) bool { return amount >= 100 && amount < 1000 })
t3.WithGuard(func(t []*token.Token) bool { return amount >= 1000 })

// BAD: Overlapping conditions
t1.WithGuard(func(t []*token.Token) bool { return amount >= 100 })
t2.WithGuard(func(t []*token.Token) bool { return amount >= 500 })  // Conflict!
```

**3. Handle All Cases (Avoid Stuck Tokens)**:
```go
// GOOD: All possible states covered
t1.WithGuard(func(t []*token.Token) bool { return status == "pending" })
t2.WithGuard(func(t []*token.Token) bool { return status == "approved" })
t3.WithGuard(func(t []*token.Token) bool { return status == "rejected" })

// BAD: Missing "rejected" case - token gets stuck
t1.WithGuard(func(t []*token.Token) bool { return status == "pending" })
t2.WithGuard(func(t []*token.Token) bool { return status == "approved" })
```

**4. Use Type Assertions Safely**:
```go
// GOOD: Safe type assertion with error handling
guard := func(tokens []*token.Token) bool {
    if order, ok := tokens[0].Data.(*Order); ok {
        return order.Amount >= 1000
    }
    return false  // Default to false on type mismatch
}
```

### Thread Safety Considerations

**Concurrent Execution Mode**:
- AND-split creates truly parallel goroutines
- Guards are evaluated atomically per transition
- Places use channels (thread-safe by design)
- No explicit locking needed

**Sequential Execution Mode**:
- OR-split evaluation is deterministic (no race conditions)
- Guards evaluated in transition registration order
- Useful for testing and debugging

### Performance Implications

**AND-split/AND-join**:
- Parallel execution improves throughput
- Synchronization adds latency (waiting)
- Use `ModeConcurrent` for true parallelism

**OR-split/OR-join**:
- Guard evaluation overhead (check all guards)
- Make guards fast (avoid expensive operations)
- Consider guard evaluation order for optimization

---

## Best Practices

### 1. Name Transitions Clearly

Use naming conventions that indicate the pattern:

```go
// AND-split
split := petri.NewTransition("split_validations", "Split Validations")

// AND-join
join := petri.NewTransition("join_validations", "Join Validations")
// OR: synchronize_validations, wait_for_all_validations

// OR-split
route := petri.NewTransition("route_by_priority", "Route by Priority")

// OR-join
merge := petri.NewTransition("merge_to_output", "Merge to Output")
```

### 2. Document Your Guards

Always comment guard logic:

```go
highPriority := petri.NewTransition("route_high", "High Priority")
highPriority.WithGuard(func(tokens []*token.Token) bool {
    // Guard: Route to express shipping if priority is "high"
    // This ensures high-priority orders are processed faster
    order := tokens[0].Data.(*Order)
    return order.Priority == "high"
})
```

### 3. Test All OR-Split Paths

For each OR-split, create test cases for every guard:

```go
func TestOrderRouting(t *testing.T) {
    tests := []struct {
        name     string
        status   string
        expected string
    }{
        {"Success path", "success", "success_path"},
        {"Failure path", "failure", "failure_path"},
        {"Retry path", "retry", "retry_path"},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test each path independently
        })
    }
}
```

### 4. Validate Guard Mutual Exclusivity

Write tests to ensure guards are mutually exclusive:

```go
func TestGuardExclusivity(t *testing.T) {
    // Create test tokens for all possible values
    testCases := []string{"success", "failure", "retry"}
    
    for _, status := range testCases {
        token := &token.Token{Data: &Order{PaymentStatus: status}}
        
        // Count how many guards return true
        trueCount := 0
        if successGuard([]*token.Token{token}) { trueCount++ }
        if failureGuard([]*token.Token{token}) { trueCount++ }
        if retryGuard([]*token.Token{token}) { trueCount++ }
        
        // Exactly ONE guard should return true
        if trueCount != 1 {
            t.Errorf("Guards not mutually exclusive for status=%s (true count=%d)", status, trueCount)
        }
    }
}
```

### 5. Use Arc Expressions for Transformation

Transform tokens as they flow through arcs:

```go
// Add timestamp when merging
mergeArc := petri.NewArc("a1", "merge", "output", 1)
mergeArc.WithExpression(func(tok *token.Token) *token.Token {
    data := tok.Data.(*Order)
    data.ProcessedAt = time.Now()
    return tok
})
```

### 6. Draw Workflow Diagrams

Visualize your workflow before implementing:

```
Input → Split → (Task A, Task B) → Join → Route → (High, Low) → Merge → Output
```

Use ASCII art, Mermaid, or tools like draw.io to document complex workflows.

---

## Common Pitfalls

### Pitfall 1: Non-Mutual Exclusive Guards (OR-Split)

**Problem**: Multiple guards return true for the same token.

```go
// BAD: Overlapping guards
t1.WithGuard(func(tokens) bool { return score >= 600 })  // true for 600-800
t2.WithGuard(func(tokens) bool { return score >= 700 })  // ALSO true for 700-800!
```

**Consequence**: Non-deterministic behavior (which transition fires is undefined).

**Solution**: Ensure mutual exclusivity.

```go
// GOOD: Exclusive ranges
t1.WithGuard(func(tokens) bool { return score >= 600 && score < 700 })
t2.WithGuard(func(tokens) bool { return score >= 700 })
```

### Pitfall 2: Missing Guard Cases (OR-Split)

**Problem**: No guard returns true for some token values.

```go
// BAD: What if status is "pending"?
t1.WithGuard(func(tokens) bool { return status == "success" })
t2.WithGuard(func(tokens) bool { return status == "failure" })
// Token stuck if status == "pending"!
```

**Consequence**: Token gets stuck, workflow halts.

**Solution**: Add catch-all or validate input.

```go
// GOOD: Handle all cases
t1.WithGuard(func(tokens) bool { return status == "success" })
t2.WithGuard(func(tokens) bool { return status == "failure" })
t3.WithGuard(func(tokens) bool { return status == "pending" })

// OR: Add default catch-all
t_other.WithGuard(func(tokens) bool { 
    return status != "success" && status != "failure"
})
```

### Pitfall 3: Using OR-Join When You Need AND-Join

**Problem**: Proceeding before all parallel work completes.

```go
// BAD: Using OR-join (simple merge) after AND-split
// If parallel_task_1 finishes first, workflow continues without waiting
```

**Consequence**: Race condition, incomplete data, incorrect results.

**Solution**: Use AND-join for synchronization.

```go
// GOOD: AND-join waits for BOTH parallel tasks
join := petri.NewTransition("join", "Synchronize")
net.AddArc(petri.NewArc("a1", "task_1_done", "join", 1))
net.AddArc(petri.NewArc("a2", "task_2_done", "join", 1))
```

### Pitfall 4: Incorrect Arc Weights

**Problem**: Arc weight doesn't match pattern intent.

```go
// BAD: Weight of 2 means "consume 2 tokens from SAME place"
net.AddArc(petri.NewArc("a1", "source", "join", 2))  // Wrong!
```

**Consequence**: Deadlock (waiting for 2 tokens when only 1 arrives).

**Solution**: Use separate arcs for separate inputs.

```go
// GOOD: Two arcs, one from each input place
net.AddArc(petri.NewArc("a1", "source1", "join", 1))
net.AddArc(petri.NewArc("a2", "source2", "join", 1))
```

### Pitfall 5: Forgetting to Resolve the Net

**Problem**: Calling `IsEnabled()` or executing transitions before `net.Resolve()`.

```go
// BAD: Using net before resolving
if trans.IsEnabled() {  // Returns false (not resolved)
    // Won't work
}
```

**Consequence**: Transitions always disabled, workflow doesn't execute.

**Solution**: Always call `Resolve()` after building the net.

```go
// GOOD: Resolve before execution
if err := net.Resolve(); err != nil {
    log.Fatal(err)
}

// Now transitions work correctly
if trans.IsEnabled() {
    // Works as expected
}
```

---

## See Also

- **[Example 07: Routing Patterns](../../examples/07-routing-patterns/)** - Complete working demonstration of all four patterns
- **[Petri Package README](README.md)** - Core Petri net model documentation
- **[Task Package](../task/README.md)** - Task executors for transition work
- **[Engine Package](../engine/README.md)** - Workflow execution modes (sequential/concurrent)

---

## References

- **Workflow Patterns Initiative**: [workflowpatterns.com](http://www.workflowpatterns.com/)
- **WfMC Specification**: Workflow Management Coalition standards
- **Petri Net Theory**: Classical foundations for concurrent systems
- **BPMN Gateways**: Business Process Model and Notation gateway patterns
