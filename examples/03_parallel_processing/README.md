# Example 03: Parallel Document Processing (AND-split/join)

A document enrichment pipeline that fans out to three concurrent analysis branches (OCR, NLP, metadata extraction) and joins the results before storing the enriched document.

## Petri Net Diagram

```mermaid
graph LR
  document((Document Uploaded))
  ocr_pending((Awaiting OCR))
  nlp_pending((Awaiting NLP))
  meta_pending((Awaiting Metadata))
  ocr_result((OCR Result))
  nlp_result((NLP Result))
  meta_result((Metadata Result))
  enriched((Document Enriched))
  split[AND-Split: Fan Out]
  ocr[OCR Analysis]
  nlp[NLP Analysis]
  metadata[Metadata Extraction]
  join[AND-Join: Merge Results]
  document --> split
  split --> ocr_pending
  split --> nlp_pending
  split --> meta_pending
  ocr_pending --> ocr
  ocr --> ocr_result
  nlp_pending --> nlp
  nlp --> nlp_result
  meta_pending --> metadata
  metadata --> meta_result
  ocr_result --> join
  nlp_result --> join
  meta_result --> join
  join --> enriched
```

## Run

```bash
go run ./examples/03_parallel_processing
```

## What It Demonstrates

- **AND-split**: one transition produces tokens in multiple output places simultaneously.
- **AND-join**: a transition requires tokens from all input places before it can fire.
- **ModeConcurrent**: the engine fires independent transitions in parallel goroutines.
- **MaxConcurrentTransitions**: limits the goroutine pool size.
- Total wall time is bounded by the slowest branch, not the sum of all branches.
