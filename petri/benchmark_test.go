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

package petri

import (
	"context"
	"testing"

	ctx "github.com/jazzpetri/engine/context"
	"github.com/jazzpetri/engine/clock"
	"github.com/jazzpetri/engine/token"
)

// Missing Benchmarks
//
// EXPECTED FAILURE: These benchmarks will fail because they test
// performance of critical paths that haven't been benchmarked before.
//
// Benchmarks enable:
//   - Performance regression detection
//   - Optimization target identification
//   - Capacity planning
//   - Performance comparisons

func BenchmarkTransition_Fire(b *testing.B) {
	// Benchmark transition firing (hot path)
	net := NewPetriNet("bench", "Benchmark")
	p1 := NewPlace("p1", "Input", 1000)
	p2 := NewPlace("p2", "Output", 1000)
	t1 := NewTransition("t1", "Process")

	_ = net.AddPlace(p1)
	_ = net.AddPlace(p2)
	_ = net.AddTransition(t1)
	_ = net.AddArc(NewArc("a1", "p1", "t1", 1))
	_ = net.AddArc(NewArc("a2", "t1", "p2", 1))
	_ = net.Resolve()

	// Add tokens
	for i := 0; i < b.N; i++ {
		tok := token.NewToken("string", "data")
		_ = p1.AddToken(tok)
	}

	execCtx := ctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = t1.Fire(execCtx)
	}
}

func BenchmarkTransition_IsEnabled(b *testing.B) {
	// Benchmark enablement checking (hot path - called frequently)
	net := NewPetriNet("bench", "Benchmark")
	p1 := NewPlace("p1", "Input", 100)
	t1 := NewTransition("t1", "Process")

	_ = net.AddPlace(p1)
	_ = net.AddTransition(t1)
	_ = net.AddArc(NewArc("a1", "p1", "t1", 1))
	_ = net.Resolve()

	// Add tokens
	tok := token.NewToken("string", "data")
	_ = p1.AddToken(tok)

	execCtx := ctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = t1.IsEnabled(execCtx)
	}
}

func BenchmarkTransition_IsEnabled_WithGuard(b *testing.B) {
	// Benchmark enablement checking with guard function
	net := NewPetriNet("bench", "Benchmark")
	p1 := NewPlace("p1", "Input", 100)
	t1 := NewTransition("t1", "Process")
	t1.WithGuard(func(execCtx *ctx.ExecutionContext, tokens []*token.Token) (bool, error) {
		// Simple guard
		return len(tokens) > 0, nil
	})

	_ = net.AddPlace(p1)
	_ = net.AddTransition(t1)
	_ = net.AddArc(NewArc("a1", "p1", "t1", 1))
	_ = net.Resolve()

	tok := token.NewToken("string", "data")
	_ = p1.AddToken(tok)

	execCtx := ctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = t1.IsEnabled(execCtx)
	}
}

func BenchmarkPlace_AddToken(b *testing.B) {
	// Benchmark token addition (hot path)
	place := NewPlace("p1", "Test Place", b.N)

	tokens := make([]*token.Token, b.N)
	for i := 0; i < b.N; i++ {
		tokens[i] = token.NewToken("string", "data")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = place.AddToken(tokens[i])
	}
}

func BenchmarkPlace_RemoveToken(b *testing.B) {
	// Benchmark token removal (hot path)
	place := NewPlace("p1", "Test Place", b.N)

	// Pre-fill place
	for i := 0; i < b.N; i++ {
		tok := token.NewToken("string", "data")
		_ = place.AddToken(tok)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = place.RemoveToken()
	}
}

func BenchmarkPlace_TokenCount(b *testing.B) {
	// Benchmark token count operation
	place := NewPlace("p1", "Test Place", 100)

	// Add some tokens
	for i := 0; i < 50; i++ {
		tok := token.NewToken("string", "data")
		_ = place.AddToken(tok)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = place.TokenCount()
	}
}

func BenchmarkPlace_PeekTokens(b *testing.B) {
	// Benchmark peeking at tokens (guard evaluation path)
	place := NewPlace("p1", "Test Place", 100)

	// Add tokens
	for i := 0; i < 10; i++ {
		tok := token.NewToken("string", "data")
		_ = place.AddToken(tok)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = place.PeekTokens()
	}
}

func BenchmarkPetriNet_Resolve(b *testing.B) {
	// Benchmark resolution (initialization path)
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		net := createMediumNet()
		b.StartTimer()

		_ = net.Resolve()
	}
}

func BenchmarkEngine_FindEnabledTransitions(b *testing.B) {
	// This benchmark needs access to engine internals
	// Skipped for now - would benchmark findEnabledTransitions()
	b.Skip("Requires engine package access")
}

// Benchmark different network sizes
func BenchmarkTransition_Fire_SmallNet(b *testing.B) {
	benchmarkFiring(b, createSmallNet())
}

func BenchmarkTransition_Fire_MediumNet(b *testing.B) {
	benchmarkFiring(b, createMediumNet())
}

func BenchmarkTransition_Fire_LargeNet(b *testing.B) {
	benchmarkFiring(b, createLargeNet())
}

// Helper functions
func benchmarkFiring(b *testing.B, net *PetriNet) {
	_ = net.Resolve()

	// Get first transition
	var trans *Transition
	for _, t := range net.Transitions {
		trans = t
		break
	}

	// Fill input places
	for _, arc := range trans.InputArcs() {
		if place, ok := arc.Source.(*Place); ok {
			for i := 0; i < b.N; i++ {
				tok := token.NewToken("string", "data")
				_ = place.AddToken(tok)
			}
		}
	}

	execCtx := ctx.NewExecutionContext(
		context.Background(),
		clock.NewRealTimeClock(),
		nil,
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = trans.Fire(execCtx)
	}
}

func createSmallNet() *PetriNet {
	// 2 places, 1 transition
	net := NewPetriNet("small", "Small Net")
	p1 := NewPlace("p1", "In", 10000)
	p2 := NewPlace("p2", "Out", 10000)
	t1 := NewTransition("t1", "T")

	_ = net.AddPlace(p1)
	_ = net.AddPlace(p2)
	_ = net.AddTransition(t1)
	_ = net.AddArc(NewArc("a1", "p1", "t1", 1))
	_ = net.AddArc(NewArc("a2", "t1", "p2", 1))

	return net
}

func createMediumNet() *PetriNet {
	// 10 places, 5 transitions
	net := NewPetriNet("medium", "Medium Net")

	for i := 0; i < 10; i++ {
		p := NewPlace("p"+string(rune('0'+i)), "Place", 10000)
		_ = net.AddPlace(p)
	}

	for i := 0; i < 5; i++ {
		t := NewTransition("t"+string(rune('0'+i)), "Trans")
		_ = net.AddTransition(t)
	}

	// Create linear chain
	for i := 0; i < 5; i++ {
		_ = net.AddArc(NewArc("a"+string(rune('0'+i*2)), "p"+string(rune('0'+i)), "t"+string(rune('0'+i)), 1))
		_ = net.AddArc(NewArc("a"+string(rune('1'+i*2)), "t"+string(rune('0'+i)), "p"+string(rune('1'+i)), 1))
	}

	return net
}

func createLargeNet() *PetriNet {
	// 100 places, 50 transitions
	net := NewPetriNet("large", "Large Net")

	for i := 0; i < 100; i++ {
		p := NewPlace("p"+string(rune('0'+i)), "Place", 10000)
		_ = net.AddPlace(p)
	}

	for i := 0; i < 50; i++ {
		t := NewTransition("t"+string(rune('0'+i)), "Trans")
		_ = net.AddTransition(t)
	}

	// Create connections
	for i := 0; i < 50; i++ {
		_ = net.AddArc(NewArc("a"+string(rune('0'+i*2)), "p"+string(rune('0'+i)), "t"+string(rune('0'+i)), 1))
		_ = net.AddArc(NewArc("a"+string(rune('1'+i*2)), "t"+string(rune('0'+i)), "p"+string(rune('1'+i)), 1))
	}

	return net
}

// Benchmark memory allocations
func BenchmarkToken_Creation(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = token.NewToken("string", "benchmark data")
	}
}

func BenchmarkPlace_Creation(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = NewPlace("p1", "Test", 100)
	}
}
