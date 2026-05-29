/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package endpointslice

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewStaleInformerCache verifies that NewStaleInformerCache always returns
// a non-nil *StaleInformerCache whose Error() method round-trips the message
// passed to the constructor verbatim, including empty and multi-byte (unicode)
// inputs. It also asserts that the returned concrete type satisfies the
// standard error interface (a compile-time guarantee that is reinforced at run
// time via the Error() assertion).
func TestNewStaleInformerCache(t *testing.T) {
	tests := []struct {
		name string
		msg  string
		want string
	}{
		{name: "non_empty_message", msg: "informer cache is stale for foo", want: "informer cache is stale for foo"},
		{name: "empty_message", msg: "", want: ""},
		{name: "unicode_message", msg: "缓存过期", want: "缓存过期"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NewStaleInformerCache(tc.msg)
			require.NotNil(t, got, "NewStaleInformerCache must never return nil")

			// The returned concrete type must satisfy the error interface.
			// Assigning it to a variable of static type error is a
			// compile-time check; the subsequent Error() assertion exercises
			// the implementation at run time.
			var asError error = got
			assert.Equal(t, tc.want, asError.Error(), "Error() must return the constructor message verbatim")
		})
	}
}

// TestStaleInformerCacheError targets the (*StaleInformerCache).Error() method
// directly. It is intentionally a separate dimension of coverage from
// TestNewStaleInformerCache: the focus here is the Error() round-trip across
// edge-value messages (empty and multi-line) rather than constructor
// invariants.
func TestStaleInformerCacheError(t *testing.T) {
	tests := []struct {
		name string
		msg  string
		want string
	}{
		{name: "simple_message", msg: "stale", want: "stale"},
		{name: "empty", msg: "", want: ""},
		{name: "multiline", msg: "line1\nline2", want: "line1\nline2"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := NewStaleInformerCache(tc.msg)
			require.NotNil(t, e)
			assert.Equal(t, tc.want, e.Error())
		})
	}
}

// TestIsStaleInformerCacheErr is the truth table for the IsStaleInformerCacheErr
// type guard. It additionally documents and guards an INTENTIONAL behavior: the
// helper uses a direct type assertion (err.(*StaleInformerCache)) rather than
// errors.As, so an error wrapped via fmt.Errorf("%w", ...) is deliberately NOT
// detected. See the IMPORTANT comment on the wrapped row below.
func TestIsStaleInformerCacheErr(t *testing.T) {
	// inner is the sentinel that the wrapped-error row wraps. It is declared
	// once here because it is referenced both by the truth-table row and by
	// the errors.As contrast subtest. It is only ever read (never mutated), so
	// this does not introduce shared mutable state.
	inner := NewStaleInformerCache("inner")

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "direct_stale_informer_cache", err: NewStaleInformerCache("x"), want: true},
		{name: "direct_with_empty_message", err: NewStaleInformerCache(""), want: true},
		{name: "nil_error", err: nil, want: false},
		{name: "unrelated_errors_New", err: errors.New("not stale"), want: false},
		{name: "unrelated_fmt_Errorf", err: fmt.Errorf("custom %s", "msg"), want: false},
		// IMPORTANT: The following row documents INTENTIONAL behavior.
		//
		// IsStaleInformerCacheErr uses a direct type assertion
		// (err.(*StaleInformerCache)) and does NOT use errors.As.
		// Consequently, an error wrapped via fmt.Errorf("%w", ...) is
		// intentionally NOT detected, and this test guards that contract.
		//
		// This is INTENTIONAL per code review (see AAP section 0.5.2).
		// Do NOT "fix" the production code to use errors.As without explicit
		// approval from the package owners — existing consumers rely on the
		// current direct-type-assertion semantics.
		{name: "wrapped_via_fmt_Errorf_intentionally_false", err: fmt.Errorf("outer: %w", inner), want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsStaleInformerCacheErr(tc.err)
			assert.Equal(t, tc.want, got)
		})
	}

	// Contrast: prove that the wrap used in the row above IS a valid Go
	// errors-wrap and that errors.As DOES detect it. This pair of assertions
	// makes the intentional policy crystal clear: the wrap is valid; the
	// production helper simply chose not to unwrap. If a future contributor
	// believes IsStaleInformerCacheErr "should" detect wrapped errors, this
	// subtest shows exactly what errors.As would do instead — and why the
	// production behavior is a deliberate choice, not an oversight.
	t.Run("errors_As_does_unwrap_wrapped_error", func(t *testing.T) {
		wrapped := fmt.Errorf("outer: %w", inner)
		var target *StaleInformerCache
		assert.True(t, errors.As(wrapped, &target), "errors.As must successfully unwrap to *StaleInformerCache")
		require.NotNil(t, target)
		assert.Equal(t, "inner", target.Error())
	})
}
