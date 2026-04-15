// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rie

import "testing"

func TestIsValidRuntime(t *testing.T) {
	tests := []struct {
		name     string
		runtime  string
		expected bool
	}{
		// Valid Node.js runtimes (official AWS identifiers)
		{"nodejs20.x", "nodejs20.x", true},
		{"nodejs22.x", "nodejs22.x", true},
		{"nodejs24.x", "nodejs24.x", true},

		// Valid Python runtimes
		{"python3.10", "python3.10", true},
		{"python3.11", "python3.11", true},
		{"python3.12", "python3.12", true},
		{"python3.13", "python3.13", true},
		{"python3.14", "python3.14", true},

		// Invalid/unsupported runtimes - old aliases without .x suffix
		{"nodejs20 without .x", "nodejs20", false},
		{"nodejs22 without .x", "nodejs22", false},
		{"nodejs24 without .x", "nodejs24", false},
		{"nodejs alias", "nodejs", false},
		{"python3 alias", "python3", false},

		// Invalid/unsupported runtimes - other languages/versions
		{"nodejs18.x", "nodejs18.x", false},
		{"java17", "java17", false},
		{"java21", "java21", false},
		{"go1.x", "go1.x", false},
		{"python3.9", "python3.9", false},
		{"dotnet6", "dotnet6", false},
		{"ruby3.2", "ruby3.2", false},
		{"provided", "provided", false},
		{"provided.al2", "provided.al2", false},
		{"provided.al2023", "provided.al2023", false},
		{"empty string", "", false},
		{"random string", "foobar", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidRuntime(tt.runtime)
			if result != tt.expected {
				t.Errorf("isValidRuntime(%q) = %v, expected %v", tt.runtime, result, tt.expected)
			}
		})
	}
}