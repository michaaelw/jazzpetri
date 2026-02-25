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
	"fmt"
	"strings"
)

// ToDOT generates a Graphviz DOT representation of the Petri net.
// Visualization support
func (pn *PetriNet) ToDOT() string {
	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("digraph \"%s\" {\n", escapeLabel(pn.Name)))
	sb.WriteString("  rankdir=LR;\n")
	sb.WriteString("  node [fontname=\"Helvetica\"];\n")
	sb.WriteString("  edge [fontname=\"Helvetica\"];\n\n")

	// Places (circles)
	sb.WriteString("  // Places\n")
	for _, place := range pn.Places {
		label := escapeLabel(place.Name)
		if place.Capacity > 0 {
			label = fmt.Sprintf("%s\\n(cap:%d)", label, place.Capacity)
		}
		sb.WriteString(fmt.Sprintf("  %s [label=\"%s\" shape=circle];\n",
			place.ID, label))
	}
	sb.WriteString("\n")

	// Transitions (boxes)
	sb.WriteString("  // Transitions\n")
	for _, trans := range pn.Transitions {
		label := escapeLabel(trans.Name)
		sb.WriteString(fmt.Sprintf("  %s [label=\"%s\" shape=box];\n",
			trans.ID, label))
	}
	sb.WriteString("\n")

	// Arcs (edges)
	sb.WriteString("  // Arcs\n")
	for _, arc := range pn.Arcs {
		weight := ""
		if arc.Weight > 1 {
			weight = fmt.Sprintf(" [label=\"%d\"]", arc.Weight)
		}
		sb.WriteString(fmt.Sprintf("  %s -> %s%s;\n",
			arc.Source, arc.Target, weight))
	}

	sb.WriteString("}\n")
	return sb.String()
}

// ToMermaid generates a Mermaid diagram representation of the Petri net.
// Visualization support
func (pn *PetriNet) ToMermaid() string {
	var sb strings.Builder

	// Header
	sb.WriteString("graph LR\n")

	// Places (double circles)
	for _, place := range pn.Places {
		label := place.Name
		if place.Capacity > 0 {
			label = fmt.Sprintf("%s<br/>cap:%d", label, place.Capacity)
		}
		sb.WriteString(fmt.Sprintf("  %s((%s))\n",
			place.ID, escapeMermaidLabel(label)))
	}

	// Transitions (rectangles)
	for _, trans := range pn.Transitions {
		sb.WriteString(fmt.Sprintf("  %s[%s]\n",
			trans.ID, escapeMermaidLabel(trans.Name)))
	}

	// Arcs (edges)
	for _, arc := range pn.Arcs {
		if arc.Weight > 1 {
			sb.WriteString(fmt.Sprintf("  %s -->|%d| %s\n",
				arc.Source, arc.Weight, arc.Target))
		} else {
			sb.WriteString(fmt.Sprintf("  %s --> %s\n",
				arc.Source, arc.Target))
		}
	}

	return sb.String()
}

// escapeLabel escapes special characters for DOT format.
func escapeLabel(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}

// escapeMermaidLabel escapes special characters for Mermaid format.
func escapeMermaidLabel(s string) string {
	// Mermaid uses different escaping rules
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
