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

package engine

import "time"

// DefaultMaxConcurrentTransitions is the default number of goroutines
// used for concurrent transition firing in ModeConcurrent.
//
// This value balances parallelism with resource consumption. For workflows
// with many independent transitions, you may want to increase this. For
// workflows with heavy tasks, consider decreasing it to avoid overwhelming
// system resources.
//
// Set to a custom value in Config to override this default.
const DefaultMaxConcurrentTransitions = 10

// DefaultPollInterval is the default time interval between checks for enabled transitions.
//
// This value (100ms) provides responsive workflow execution without excessive CPU usage
// from tight polling loops. For latency-sensitive workflows, you may decrease this.
// For low-priority background workflows, you may increase this to reduce system load.
//
// Set to a custom value in Config to override this default.
const DefaultPollInterval = 100 * time.Millisecond

// DefaultPlaceCapacity is the default maximum number of tokens a place can hold.
//
// This value (100) is suitable for most workflows and prevents unbounded memory growth.
// Places with high token volumes should specify a larger capacity explicitly.
//
// Note: This constant is defined in the engine package but used by the petri package.
// It represents a system-wide default capacity policy.
const DefaultPlaceCapacity = 100
