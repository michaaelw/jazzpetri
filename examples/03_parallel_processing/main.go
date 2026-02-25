// Copyright 2026 The JazzPetri Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package main demonstrates AND-split/join concurrent workflow execution.
//
// This example models a document enrichment pipeline where a single uploaded
// document is processed by three independent analysis stages in parallel,
// and the results are joined before the enriched document is stored:
//
//	                     ┌─(ocr)──────► [ocr_result]──────┐
//	[document]─(split)───┼─(nlp)──────► [nlp_result]──────┼─(join)─► [enriched]
//	                     └─(metadata)─► [metadata_result]──┘
//
// The AND-split fires once and produces three tokens simultaneously (one to
// each parallel branch input place). Each branch task processes its token
// independently. The AND-join requires one token from all three result places
// before it fires — enforcing synchronisation.
//
// The engine runs in ModeConcurrent so all three analysis tasks execute
// at the same time in separate goroutines.
//
// Run with:
//
//	go run ./examples/03_parallel_processing
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jazzpetri/engine/clock"
	execContext "github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/engine"
	"github.com/jazzpetri/engine/petri"
	"github.com/jazzpetri/engine/task"
	"github.com/jazzpetri/engine/token"
)

func main() {
	fmt.Println("=== Example 03: Parallel Document Processing (AND-split/join) ===")
	fmt.Println()

	net := petri.NewPetriNet("document-workflow", "Document Enrichment Pipeline")

	// ------------------------------------------------------------------
	// Places
	//
	// The pipeline has six places:
	//   1. document       — holds the incoming document (initial)
	//   2. ocr_pending    — branch input: waiting for OCR
	//   3. nlp_pending    — branch input: waiting for NLP
	//   4. meta_pending   — branch input: waiting for metadata extraction
	//   5-7. *_result     — branch outputs: results of each analysis
	//   8. enriched       — final place (terminal)
	// ------------------------------------------------------------------
	pDocument := petri.NewPlace("document", "Document Uploaded", 10)
	pDocument.IsInitial = true

	// Branch input places — one token each arrives from the AND-split.
	pOCRPending := petri.NewPlace("ocr_pending", "Awaiting OCR", 10)
	pNLPPending := petri.NewPlace("nlp_pending", "Awaiting NLP", 10)
	pMetaPending := petri.NewPlace("meta_pending", "Awaiting Metadata", 10)

	// Branch output places — the AND-join waits for one token in each.
	pOCRResult := petri.NewPlace("ocr_result", "OCR Result", 10)
	pNLPResult := petri.NewPlace("nlp_result", "NLP Result", 10)
	pMetaResult := petri.NewPlace("meta_result", "Metadata Result", 10)

	// Final place.
	pEnriched := petri.NewPlace("enriched", "Document Enriched", 10)
	pEnriched.IsTerminal = true

	// ------------------------------------------------------------------
	// Transitions
	// ------------------------------------------------------------------

	// AND-split: consumes one token from 'document', produces one token
	// in each of the three pending places simultaneously.
	tSplit := petri.NewTransition("split", "AND-Split: Fan Out to 3 branches").
		WithTask(&task.InlineTask{
			Name: "split",
			Fn: func(ctx *execContext.ExecutionContext) error {
				fmt.Println("  [split]    Dispatching document to OCR, NLP, and metadata workers.")
				return nil
			},
		})

	// Three parallel analysis transitions.
	tOCR := petri.NewTransition("ocr", "OCR Analysis").
		WithTask(&task.InlineTask{
			Name: "ocr",
			Fn: func(ctx *execContext.ExecutionContext) error {
				fmt.Println("  [ocr]      Running optical character recognition...")
				time.Sleep(30 * time.Millisecond) // simulate real work
				fmt.Println("  [ocr]      OCR complete: extracted 1,024 words.")
				return nil
			},
		})

	tNLP := petri.NewTransition("nlp", "NLP Analysis").
		WithTask(&task.InlineTask{
			Name: "nlp",
			Fn: func(ctx *execContext.ExecutionContext) error {
				fmt.Println("  [nlp]      Running natural language analysis...")
				time.Sleep(50 * time.Millisecond) // longest branch
				fmt.Println("  [nlp]      NLP complete: sentiment=positive, topics=[finance, Q3].")
				return nil
			},
		})

	tMetadata := petri.NewTransition("metadata", "Metadata Extraction").
		WithTask(&task.InlineTask{
			Name: "metadata",
			Fn: func(ctx *execContext.ExecutionContext) error {
				fmt.Println("  [metadata] Extracting document metadata...")
				time.Sleep(20 * time.Millisecond) // shortest branch
				fmt.Println("  [metadata] Metadata complete: author=Alice, pages=12.")
				return nil
			},
		})

	// AND-join: requires one token from EACH of the three result places.
	// It will not fire until all parallel branches have completed.
	tJoin := petri.NewTransition("join", "AND-Join: Merge 3 results").
		WithTask(&task.InlineTask{
			Name: "join",
			Fn: func(ctx *execContext.ExecutionContext) error {
				fmt.Println("  [join]     All branches complete. Merging results...")
				fmt.Println("  [join]     Indexing enriched document.")
				return nil
			},
		})

	// ------------------------------------------------------------------
	// Assemble: add all elements then wire arcs.
	// ------------------------------------------------------------------
	net.AddPlace(pDocument)
	net.AddPlace(pOCRPending)
	net.AddPlace(pNLPPending)
	net.AddPlace(pMetaPending)
	net.AddPlace(pOCRResult)
	net.AddPlace(pNLPResult)
	net.AddPlace(pMetaResult)
	net.AddPlace(pEnriched)
	net.AddTransition(tSplit)
	net.AddTransition(tOCR)
	net.AddTransition(tNLP)
	net.AddTransition(tMetadata)
	net.AddTransition(tJoin)

	// AND-split: document ──(split)──► three pending places
	net.AddArc(petri.NewArc("arc-doc-split", "document", "split", 1))
	net.AddArc(petri.NewArc("arc-split-ocr", "split", "ocr_pending", 1))
	net.AddArc(petri.NewArc("arc-split-nlp", "split", "nlp_pending", 1))
	net.AddArc(petri.NewArc("arc-split-meta", "split", "meta_pending", 1))

	// Parallel branches: pending ──(task)──► result
	net.AddArc(petri.NewArc("arc-ocr-in", "ocr_pending", "ocr", 1))
	net.AddArc(petri.NewArc("arc-ocr-out", "ocr", "ocr_result", 1))

	net.AddArc(petri.NewArc("arc-nlp-in", "nlp_pending", "nlp", 1))
	net.AddArc(petri.NewArc("arc-nlp-out", "nlp", "nlp_result", 1))

	net.AddArc(petri.NewArc("arc-meta-in", "meta_pending", "metadata", 1))
	net.AddArc(petri.NewArc("arc-meta-out", "metadata", "meta_result", 1))

	// AND-join: all three results ──(join)──► enriched
	net.AddArc(petri.NewArc("arc-join-ocr", "ocr_result", "join", 1))
	net.AddArc(petri.NewArc("arc-join-nlp", "nlp_result", "join", 1))
	net.AddArc(petri.NewArc("arc-join-meta", "meta_result", "join", 1))
	net.AddArc(petri.NewArc("arc-join-enriched", "join", "enriched", 1))

	if err := net.Resolve(); err != nil {
		log.Fatalf("failed to resolve net: %v", err)
	}

	// ------------------------------------------------------------------
	// Add the initial document token.
	// ------------------------------------------------------------------
	docToken := &token.Token{
		ID: "doc-report-q3",
		Data: &petri.MapToken{Value: map[string]interface{}{
			"filename": "Q3_Financial_Report.pdf",
			"size_kb":  4096,
		}},
	}
	pDocument.AddToken(docToken)
	fmt.Printf("Document '%v' queued for enrichment.\n\n",
		docToken.Data.(*petri.MapToken).Value["filename"])

	// ------------------------------------------------------------------
	// Run with concurrent engine so all three analysis tasks execute
	// in parallel goroutines without waiting for each other.
	// ------------------------------------------------------------------
	ctx := execContext.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)
	cfg := engine.Config{
		Mode:                     engine.ModeConcurrent,
		MaxConcurrentTransitions: 5,
		PollInterval:             5 * time.Millisecond,
	}
	eng := engine.NewEngine(net, ctx, cfg)

	start := time.Now()
	fmt.Println("Engine started (ModeConcurrent, MaxConcurrent=5).")
	fmt.Println("Parallel branch output (order may vary between runs):")
	fmt.Println()

	eng.Start()
	eng.Wait()
	eng.Stop()

	elapsed := time.Since(start)
	fmt.Printf("\nAll branches complete in %v (NLP branch alone takes ~50ms).\n",
		elapsed.Round(time.Millisecond))
	fmt.Printf("Tokens in '%s': %d\n", pEnriched.Name, pEnriched.TokenCount())
	fmt.Println("\nDocument enrichment pipeline complete.")
}
