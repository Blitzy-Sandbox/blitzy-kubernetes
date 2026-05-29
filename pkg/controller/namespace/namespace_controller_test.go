/*
Copyright The Kubernetes Authors.

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

// This file is the unit-test suite for the namespace controller defined in
// namespace_controller.go. It lives in package namespace (internal-test
// pattern) so that the tests can inject test doubles into the unexported
// fields of NamespaceController (lister, listerSynced, queue,
// namespacedResourcesDeleter) via struct-literal construction. This is
// necessary because NewNamespaceController hard-codes the construction of its
// namespacedResourcesDeleter and therefore cannot accept a fake one through
// its public signature.
//
// TestSyncNamespaceFromKey/exists_deleter_invoked is the canonical
// regression-proof drill anchor referenced by the testing plan: commenting
// out the line in syncNamespaceFromKey that calls
// namespacedResourcesDeleter.Delete must make that subtest fail loudly,
// proving the suite catches regressions. See TESTING.md for the drill.
//
// All tests are idiomatic to the k8s.io/kubernetes repository: they use
// k8s.io/client-go/kubernetes/fake for fake clients, build listers from a
// shared informer factory, route logs through k8s.io/klog/v2/ktesting, and
// never sleep the test goroutine (polling uses wait.PollUntilContextTimeout
// exclusively). Every
// subtest constructs its own fresh state so the cases remain independent and
// race-detector clean.
package namespace

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	corelisters "k8s.io/client-go/listers/core/v1"
	metadatafake "k8s.io/client-go/metadata/fake"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2/ktesting"

	_ "k8s.io/kubernetes/pkg/apis/core/install"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/controller/namespace/deletion"
)

// fakeDeleter is a test double for deletion.NamespacedResourcesDeleterInterface.
// It records every Delete invocation under mutex protection and returns the
// configured returnErr value (nil for success).
//
// Each test MUST construct a fresh fakeDeleter to avoid shared mutable state,
// keeping every case independently runnable.
//
// This type is the foundation of the regression-proof drill: if production
// code stops calling Delete (e.g., the Delete call in syncNamespaceFromKey is
// commented out), CallCount returns 0 and the corresponding test fails with a
// clear assertion.
type fakeDeleter struct {
	mu        sync.Mutex
	calls     []string // namespace names passed to Delete, in order of invocation
	returnErr error    // error to return from Delete; nil means success
}

// Delete records the invocation and returns the configured error. It
// implements deletion.NamespacedResourcesDeleterInterface.
func (f *fakeDeleter) Delete(ctx context.Context, nsName string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, nsName)
	return f.returnErr
}

// CallCount returns the number of times Delete was invoked.
func (f *fakeDeleter) CallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// CallArgs returns a copy of the namespace names passed to Delete (in order).
func (f *fakeDeleter) CallArgs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([]string, len(f.calls))
	copy(cp, f.calls)
	return cp
}

// Compile-time interface satisfaction check.
var _ deletion.NamespacedResourcesDeleterInterface = (*fakeDeleter)(nil)

// addAfterCall captures a single invocation of AddAfter for later assertion.
type addAfterCall struct {
	key   string
	delay time.Duration
}

// fakeQueue embeds a real TypedRateLimitingInterface[string] but overrides
// AddAfter to capture the delay and key for assertion. Every other method
// (Add, AddRateLimited, Get, Done, Forget, NumRequeues, ShutDown, Len, etc.)
// is inherited from the embedded queue and behaves normally, so a worker
// goroutine can pull items from it as usual.
//
// Use this type to verify that enqueueNamespace and worker invoke AddAfter
// with the expected delay (5s for an enqueued deleting namespace,
// (estimate/2+1)s for a ResourcesRemainingError requeue).
type fakeQueue struct {
	workqueue.TypedRateLimitingInterface[string]
	mu       sync.Mutex
	addAfter []addAfterCall
}

// AddAfter records the call and forwards to the embedded queue so that items
// scheduled by production code remain observable through Get.
func (f *fakeQueue) AddAfter(key string, delay time.Duration) {
	f.mu.Lock()
	f.addAfter = append(f.addAfter, addAfterCall{key: key, delay: delay})
	f.mu.Unlock()
	f.TypedRateLimitingInterface.AddAfter(key, delay)
}

// AddAfterCalls returns a copy of the recorded AddAfter calls (for assertion).
func (f *fakeQueue) AddAfterCalls() []addAfterCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([]addAfterCall, len(f.addAfter))
	copy(cp, f.addAfter)
	return cp
}

// errorLister is a corelisters.NamespaceLister implementation that returns a
// configurable deterministic error on Get and List. It is used to exercise the
// non-NotFound error branch of syncNamespaceFromKey, which is otherwise
// unreachable through a normal cache.Indexer-backed lister (which only ever
// returns NotFound for a missing key).
//
// The NamespaceListerExpansion interface is empty per the generated lister, so
// implementing only List and Get fully satisfies corelisters.NamespaceLister.
type errorLister struct {
	getErr  error
	listErr error
}

// List satisfies corelisters.NamespaceLister.
func (e *errorLister) List(selector labels.Selector) ([]*v1.Namespace, error) {
	return nil, e.listErr
}

// Get satisfies corelisters.NamespaceLister.
func (e *errorLister) Get(name string) (*v1.Namespace, error) {
	return nil, e.getErr
}

// Compile-time interface satisfaction check.
var _ corelisters.NamespaceLister = (*errorLister)(nil)

// testDiscoverResourcesFn returns a minimal, NON-EMPTY discovery result.
//
// It must be non-empty: NewNamespaceController constructs its deleter via
// deletion.NewNamespacedResourcesDeleter, whose initOpCache calls
// klog.FlushAndExit (which terminates the process) when the discovery result
// is empty. Returning a single namespaced resource keeps the constructor happy
// without performing any I/O.
func testDiscoverResourcesFn() ([]*metav1.APIResourceList, error) {
	return []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{
					Name:       "pods",
					Namespaced: true,
					Kind:       "Pod",
					Verbs:      []string{"get", "list", "delete", "deletecollection"},
				},
			},
		},
	}, nil
}

// TestNewNamespaceController verifies that NewNamespaceController returns a
// fully-initialized *NamespaceController with all four unexported fields set:
// lister, listerSynced, queue, and namespacedResourcesDeleter.
func TestNewNamespaceController(t *testing.T) {
	_, ctx := ktesting.NewTestContext(t)

	kubeClient := fake.NewSimpleClientset()
	metadataClient := metadatafake.NewSimpleMetadataClient(metadatafake.NewTestScheme())

	informerFactory := informers.NewSharedInformerFactory(kubeClient, controller.NoResyncPeriodFunc())
	nsInformer := informerFactory.Core().V1().Namespaces()

	nm := NewNamespaceController(ctx, kubeClient, metadataClient, testDiscoverResourcesFn, nsInformer, 0, v1.FinalizerKubernetes)

	require.NotNil(t, nm, "NewNamespaceController returned nil")
	assert.NotNil(t, nm.lister, "lister should be non-nil")
	assert.NotNil(t, nm.listerSynced, "listerSynced should be non-nil")
	assert.NotNil(t, nm.queue, "queue should be non-nil")
	assert.NotNil(t, nm.namespacedResourcesDeleter, "namespacedResourcesDeleter should be non-nil")
}

// TestSyncNamespaceFromKey exercises the four branches of syncNamespaceFromKey:
//
//	(1) namespace exists in lister and deleter succeeds -> returns nil and
//	    invokes deleter.Delete exactly once with the namespace's Name. This is
//	    the regression-proof drill anchor: commenting out the deleter.Delete
//	    call in syncNamespaceFromKey MUST cause subtest "exists_deleter_invoked"
//	    to fail (CallCount drops to 0).
//	(2) lister returns NotFound -> sync returns nil, deleter is NOT invoked.
//	(3) lister returns a non-NotFound error -> sync returns that error.
//	(4) deleter returns an error -> sync returns that same error.
func TestSyncNamespaceFromKey(t *testing.T) {
	// Subtest name MUST be exactly "exists_deleter_invoked" so the
	// regression-proof drill command
	// `go test -run 'TestSyncNamespaceFromKey/exists_deleter_invoked'`
	// matches this subtest.
	const regressionAnchorSubtest = "exists_deleter_invoked"

	deleterErr := errors.New("simulated deleter failure")
	listerErr := errors.New("simulated lister transient error")

	tests := map[string]struct {
		// useErrorLister, if true, overrides the informer-backed lister with an
		// errorLister that returns getErr from Get.
		useErrorLister bool
		getErr         error

		// namespaceInStore, if non-nil, is added to the informer indexer so the
		// real lister returns it on Get.
		namespaceInStore *v1.Namespace

		// syncKey is the key passed to syncNamespaceFromKey.
		syncKey string

		// deleterReturnErr is the error fakeDeleter.Delete returns.
		deleterReturnErr error

		// wantErr is the error syncNamespaceFromKey is expected to return.
		wantErr error
		// wantDeleterCallCount is the expected number of Delete invocations.
		wantDeleterCallCount int
		// wantDeleterCallArgs is the expected namespace names passed to Delete.
		wantDeleterCallArgs []string
	}{
		regressionAnchorSubtest: {
			// REGRESSION-PROOF DRILL ANCHOR.
			// If the deleter.Delete call in syncNamespaceFromKey is mutated to
			// `return nil`, wantDeleterCallCount=1 will not be satisfied (got 0)
			// and this subtest fails loudly, proving the suite catches
			// regressions.
			namespaceInStore:     &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test-ns"}},
			syncKey:              "test-ns",
			deleterReturnErr:     nil,
			wantErr:              nil,
			wantDeleterCallCount: 1,
			wantDeleterCallArgs:  []string{"test-ns"},
		},
		"not_found_from_lister": {
			// No namespace in store -> lister returns NotFound -> sync returns
			// nil. Deleter MUST NOT be invoked.
			syncKey:              "missing-ns",
			wantErr:              nil,
			wantDeleterCallCount: 0,
		},
		"lister_transient_error": {
			// Custom errorLister returns a non-NotFound error -> sync returns
			// that error. Deleter MUST NOT be invoked.
			useErrorLister:       true,
			getErr:               listerErr,
			syncKey:              "any-key",
			wantErr:              listerErr,
			wantDeleterCallCount: 0,
		},
		"deleter_returns_error": {
			// Namespace exists in store, but deleter returns an error. Sync must
			// propagate that error unchanged.
			namespaceInStore:     &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "broken-ns"}},
			syncKey:              "broken-ns",
			deleterReturnErr:     deleterErr,
			wantErr:              deleterErr,
			wantDeleterCallCount: 1,
			wantDeleterCallArgs:  []string{"broken-ns"},
		},
	}

	for name, tc := range tests {
		name, tc := name, tc
		t.Run(name, func(t *testing.T) {
			t.Parallel() // each subtest constructs fresh state, safe to run in parallel
			_, ctx := ktesting.NewTestContext(t)

			// Fresh fake clientset and informer-backed lister per subtest.
			kubeClient := fake.NewSimpleClientset()
			informerFactory := informers.NewSharedInformerFactory(kubeClient, controller.NoResyncPeriodFunc())
			nsInformer := informerFactory.Core().V1().Namespaces()

			// Pre-populate the lister only when a namespace is expected in the
			// store AND we're not overriding the lister with errorLister.
			if tc.namespaceInStore != nil && !tc.useErrorLister {
				require.NoError(t,
					nsInformer.Informer().GetIndexer().Add(tc.namespaceInStore),
					"failed to seed indexer")
			}

			fd := &fakeDeleter{returnErr: tc.deleterReturnErr}

			// Construct the controller via struct literal so the fake deleter
			// can be injected. (NewNamespaceController hard-codes its deleter.)
			nm := &NamespaceController{
				lister:       nsInformer.Lister(),
				listerSynced: func() bool { return true },
				queue: workqueue.NewTypedRateLimitingQueueWithConfig(
					nsControllerRateLimiter(),
					workqueue.TypedRateLimitingQueueConfig[string]{Name: "ns-test-" + name},
				),
				namespacedResourcesDeleter: fd,
			}
			// Override with errorLister when the case needs the non-NotFound
			// lister-error branch.
			if tc.useErrorLister {
				nm.lister = &errorLister{getErr: tc.getErr}
			}
			t.Cleanup(func() { nm.queue.ShutDown() })

			err := nm.syncNamespaceFromKey(ctx, tc.syncKey)

			if tc.wantErr != nil {
				require.Error(t, err, "expected an error from syncNamespaceFromKey")
				assert.Equal(t, tc.wantErr, err, "error does not match expected")
			} else {
				require.NoError(t, err, "syncNamespaceFromKey returned unexpected error")
			}

			assert.Equal(t, tc.wantDeleterCallCount, fd.CallCount(),
				"deleter Delete invocation count mismatch")
			if tc.wantDeleterCallArgs != nil {
				assert.Equal(t, tc.wantDeleterCallArgs, fd.CallArgs(),
					"deleter Delete arguments mismatch")
			}
		})
	}
}

// TestEnqueueNamespace verifies that enqueueNamespace only adds a namespace to
// the queue when it is being deleted (DeletionTimestamp is set and non-zero),
// and that it uses AddAfter with the namespaceDeletionGracePeriod (5 seconds).
//
// PRODUCTION LIMITATIONS (intentionally NOT tested here):
//   - Tombstone (cache.DeletedFinalStateUnknown): enqueueNamespace performs a
//     direct `obj.(*v1.Namespace)` type assertion, which would PANIC on a
//     tombstone. The production controller never receives tombstones in
//     practice because its informer event handlers register only AddFunc and
//     UpdateFunc (there is no DeleteFunc that would deliver a tombstone).
//   - Non-namespace object types: same panic-on-type-assertion situation.
//
// Both are documented limitations of the current production code. Exercising
// them would require modifying production code to add tombstone/type handling,
// which is out of scope for a tests-only change, so they are deliberately
// omitted rather than tested with a recover().
func TestEnqueueNamespace(t *testing.T) {
	now := metav1.NewTime(time.Now())

	tests := map[string]struct {
		obj          interface{}
		wantEnqueued bool          // whether AddAfter should have been called
		wantKey      string        // expected key (only checked when wantEnqueued)
		wantDelay    time.Duration // expected delay (only checked when wantEnqueued)
	}{
		"valid_namespace_with_deletion_timestamp_is_enqueued": {
			obj: &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "deleting-ns",
					DeletionTimestamp: &now,
				},
			},
			wantEnqueued: true,
			wantKey:      "deleting-ns",
			wantDelay:    namespaceDeletionGracePeriod, // 5 * time.Second
		},
		"namespace_without_deletion_timestamp_is_not_enqueued": {
			obj: &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: "alive-ns"},
			},
			wantEnqueued: false,
		},
		"namespace_with_zero_deletion_timestamp_is_not_enqueued": {
			obj: &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "alive-ns-zero",
					DeletionTimestamp: &metav1.Time{}, // zero Time -> IsZero() == true
				},
			},
			wantEnqueued: false,
		},
		"object_without_meta_is_not_enqueued": {
			// An object from which controller.KeyFunc cannot derive a key
			// (it does not implement metav1.Object) exercises the early-return
			// error branch of enqueueNamespace. KeyFunc fails BEFORE the
			// *v1.Namespace type assertion is reached, so this is safe to test
			// and does not panic.
			obj:          42,
			wantEnqueued: false,
		},
	}

	for name, tc := range tests {
		name, tc := name, tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, ctx := ktesting.NewTestContext(t)

			// Fresh queue per subtest so cases stay independent.
			base := workqueue.NewTypedRateLimitingQueueWithConfig(
				nsControllerRateLimiter(),
				workqueue.TypedRateLimitingQueueConfig[string]{Name: "ns-enqueue-" + name},
			)
			fq := &fakeQueue{TypedRateLimitingInterface: base}
			t.Cleanup(func() { fq.ShutDown() })

			nm := &NamespaceController{
				queue: fq,
			}

			nm.enqueueNamespace(ctx, tc.obj)

			calls := fq.AddAfterCalls()
			if tc.wantEnqueued {
				require.Len(t, calls, 1, "expected exactly one AddAfter call")
				assert.Equal(t, tc.wantKey, calls[0].key, "AddAfter key mismatch")
				assert.Equal(t, tc.wantDelay, calls[0].delay, "AddAfter delay mismatch")
			} else {
				assert.Empty(t, calls, "expected zero AddAfter calls (namespace not being deleted)")
			}
		})
	}
}

// TestWorkerDrainsQueueOnSuccess verifies the worker drain semantics: it pulls
// a key from the queue, invokes syncNamespaceFromKey, observes a nil return,
// and forgets the key so the queue length returns to zero.
func TestWorkerDrainsQueueOnSuccess(t *testing.T) {
	_, ctx := ktesting.NewTestContext(t)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	kubeClient := fake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(kubeClient, controller.NoResyncPeriodFunc())
	nsInformer := informerFactory.Core().V1().Namespaces()

	// Seed the lister with a namespace so syncNamespaceFromKey reaches the
	// deleter (which returns nil for the default fakeDeleter).
	require.NoError(t, nsInformer.Informer().GetIndexer().Add(
		&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "drain-ns"}}))

	fd := &fakeDeleter{}
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(
		nsControllerRateLimiter(),
		workqueue.TypedRateLimitingQueueConfig[string]{Name: "ns-worker-drain"},
	)

	nm := &NamespaceController{
		lister:                     nsInformer.Lister(),
		listerSynced:               func() bool { return true },
		queue:                      queue,
		namespacedResourcesDeleter: fd,
	}

	queue.Add("drain-ns")

	// Start one worker goroutine and ensure it is joined even if an assertion
	// below aborts the test (require.* triggers runtime.Goexit, which still
	// runs deferred functions).
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		nm.worker(ctx)
	}()
	defer func() {
		queue.ShutDown()
		wg.Wait()
	}()

	// Wait for the worker to fully process the key (poll only; do not sleep).
	err := wait.PollUntilContextTimeout(ctx, 10*time.Millisecond, 5*time.Second, true,
		func(_ context.Context) (bool, error) {
			return fd.CallCount() == 1 && queue.Len() == 0, nil
		})
	require.NoError(t, err, "worker did not drain queue within timeout")

	assert.Equal(t, 1, fd.CallCount(), "deleter should have been invoked exactly once")
	assert.Equal(t, []string{"drain-ns"}, fd.CallArgs(), "deleter received unexpected namespace name")
}

// TestWorkerRequeuesOnTransientError verifies that when syncNamespaceFromKey
// returns a non-ResourcesRemainingError error, the worker re-queues the key via
// AddRateLimited (so NumRequeues advances and the key is processed again).
func TestWorkerRequeuesOnTransientError(t *testing.T) {
	_, ctx := ktesting.NewTestContext(t)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	kubeClient := fake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(kubeClient, controller.NoResyncPeriodFunc())
	nsInformer := informerFactory.Core().V1().Namespaces()

	require.NoError(t, nsInformer.Informer().GetIndexer().Add(
		&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "broken-ns"}}))

	fd := &fakeDeleter{returnErr: errors.New("transient failure")}
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(
		nsControllerRateLimiter(),
		workqueue.TypedRateLimitingQueueConfig[string]{Name: "ns-worker-requeue"},
	)

	nm := &NamespaceController{
		lister:                     nsInformer.Lister(),
		listerSynced:               func() bool { return true },
		queue:                      queue,
		namespacedResourcesDeleter: fd,
	}
	queue.Add("broken-ns")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		nm.worker(ctx)
	}()
	defer func() {
		queue.ShutDown()
		wg.Wait()
	}()

	// Wait until the deleter is invoked at least twice: the first failure
	// re-enqueues the key via AddRateLimited and the worker processes it again.
	err := wait.PollUntilContextTimeout(ctx, 10*time.Millisecond, 5*time.Second, true,
		func(_ context.Context) (bool, error) {
			return fd.CallCount() >= 2, nil
		})
	require.NoError(t, err, "worker did not requeue after transient error within timeout")

	assert.GreaterOrEqual(t, queue.NumRequeues("broken-ns"), 1,
		"key should have been requeued at least once after a transient error")
}

// TestWorkerRequeuesOnResourcesRemainingError verifies that when the deleter
// returns a *deletion.ResourcesRemainingError, the worker re-queues the key via
// AddAfter(key, (estimate/2 + 1) seconds). For estimate=30: (30/2 + 1) = 16s.
func TestWorkerRequeuesOnResourcesRemainingError(t *testing.T) {
	_, ctx := ktesting.NewTestContext(t)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	kubeClient := fake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(kubeClient, controller.NoResyncPeriodFunc())
	nsInformer := informerFactory.Core().V1().Namespaces()

	require.NoError(t, nsInformer.Informer().GetIndexer().Add(
		&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "remaining-ns"}}))

	const estimate = int64(30)
	fd := &fakeDeleter{returnErr: &deletion.ResourcesRemainingError{Estimate: estimate}}

	base := workqueue.NewTypedRateLimitingQueueWithConfig(
		nsControllerRateLimiter(),
		workqueue.TypedRateLimitingQueueConfig[string]{Name: "ns-worker-resources"},
	)
	fq := &fakeQueue{TypedRateLimitingInterface: base}

	nm := &NamespaceController{
		lister:                     nsInformer.Lister(),
		listerSynced:               func() bool { return true },
		queue:                      fq,
		namespacedResourcesDeleter: fd,
	}
	fq.Add("remaining-ns")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		nm.worker(ctx)
	}()
	defer func() {
		fq.ShutDown()
		wg.Wait()
	}()

	expectedDelay := time.Duration(estimate/2+1) * time.Second // (30/2 + 1) = 16s

	err := wait.PollUntilContextTimeout(ctx, 10*time.Millisecond, 5*time.Second, true,
		func(_ context.Context) (bool, error) {
			return len(fq.AddAfterCalls()) >= 1, nil
		})
	require.NoError(t, err, "worker did not call AddAfter within timeout")

	calls := fq.AddAfterCalls()
	require.NotEmpty(t, calls, "expected at least one AddAfter call")
	// The first AddAfter call originates from the worker's
	// ResourcesRemainingError handling.
	assert.Equal(t, "remaining-ns", calls[0].key, "AddAfter key mismatch")
	assert.Equal(t, expectedDelay, calls[0].delay,
		"AddAfter delay should be (estimate/2 + 1) seconds = 16s for estimate=30")
}

// TestNSControllerRateLimiter verifies the invariants of nsControllerRateLimiter:
//   - the first attempt for an item yields at least the 5ms exponential floor;
//   - subsequent attempts grow but the per-item exponential limiter is bounded
//     by its 60s ceiling.
//
// nsControllerRateLimiter is a MaxOf of an exponential per-item limiter and a
// global token bucket (10qps, burst 100). MaxOf chooses the LARGER of the two
// delays, so the assertions use bounds that remain valid regardless of how the
// token bucket shapes the result for a single, low-contention key.
func TestNSControllerRateLimiter(t *testing.T) {
	limiter := nsControllerRateLimiter()

	// First attempt for a key starts at the 5ms exponential floor.
	first := limiter.When("test-key")
	assert.GreaterOrEqual(t, first, 5*time.Millisecond,
		"first When() should be at least 5ms (exponential floor)")

	// After many attempts the per-item exponential limiter caps at 60s. Assert a
	// generous upper bound that catches a runaway exponential without flaking on
	// token-bucket behavior.
	var last time.Duration
	for i := 0; i < 25; i++ {
		last = limiter.When("test-key")
	}
	assert.LessOrEqual(t, last, 2*time.Minute,
		"When() should not exceed the ~60s exponential ceiling (margin allowed for the token bucket)")
}

// TestRunStartsWorkersAndShutsDownOnContextCancel verifies the controller's
// top-level Run loop: it waits for the informer cache to sync, starts the
// requested number of workers (each of which drains the queue through worker),
// and returns cleanly once the context is canceled, shutting the queue down via
// its deferred cleanup. Covering Run here keeps the package above its line
// coverage target without modifying production code.
func TestRunStartsWorkersAndShutsDownOnContextCancel(t *testing.T) {
	_, ctx := ktesting.NewTestContext(t)
	ctx, cancel := context.WithCancel(ctx)

	kubeClient := fake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(kubeClient, controller.NoResyncPeriodFunc())
	nsInformer := informerFactory.Core().V1().Namespaces()

	// Seed the lister so the worker's syncNamespaceFromKey reaches the deleter.
	require.NoError(t, nsInformer.Informer().GetIndexer().Add(
		&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "run-ns"}}))

	fd := &fakeDeleter{}
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(
		nsControllerRateLimiter(),
		workqueue.TypedRateLimitingQueueConfig[string]{Name: "ns-run"},
	)

	nm := &NamespaceController{
		lister: nsInformer.Lister(),
		// Report the cache as synced immediately so Run proceeds to start
		// workers without blocking on a real informer.
		listerSynced:               func() bool { return true },
		queue:                      queue,
		namespacedResourcesDeleter: fd,
	}

	queue.Add("run-ns")

	// Run blocks until the context is canceled; launch it in a goroutine and
	// guarantee it is joined even if an assertion below aborts the test.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		nm.Run(ctx, 1)
	}()
	defer func() {
		cancel()
		wg.Wait()
	}()

	// Wait until a Run-managed worker has processed the seeded key.
	err := wait.PollUntilContextTimeout(ctx, 10*time.Millisecond, 5*time.Second, true,
		func(_ context.Context) (bool, error) {
			return fd.CallCount() >= 1, nil
		})
	require.NoError(t, err, "Run did not start a worker that drained the queue within timeout")

	assert.GreaterOrEqual(t, fd.CallCount(), 1,
		"expected the deleter to be invoked at least once by a Run-managed worker")
}
