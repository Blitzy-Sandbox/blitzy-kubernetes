# Technical Specification

# 0. Agent Action Plan

## 0.1 Intent Clarification

### 0.1.1 Core Testing Objective

Based on the provided requirements, the Blitzy platform understands that the testing objective is to **add a baseline unit-test suite (with a small number of integration tests) for selected reconciliation logic, validation logic, and utility helpers in the Kubernetes monorepo at `k8s.io/kubernetes`**, focused on the in-tree controllers and API validation packages, with a 70% line-coverage target on the targeted packages and 100% on reconcile happy-path plus primary error branches.

Request categorization: **ADD NEW TESTS** (greenfield baseline coverage for packages that currently have zero `*_test.go` files at the target path), with a small number of **UPDATE** rows for incremental edge-case augmentation of existing tests.

The five testing objectives, with enhanced technical clarity:

- Add **unit tests for selected controller reconciliation logic** in `pkg/controller/*` using table-driven Go test patterns, focusing on packages that currently have no unit-test coverage of the top-level controller (`pkg/controller/namespace`, `pkg/controller/replication`)
- Add **unit tests for validation logic** in `pkg/apis/<group>/validation/` (in-tree built-in API types) and, where the user's "CRD validation" intent applies, in `staging/src/k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/validation/`
- Add **unit tests for utility/helper functions** in `pkg/controller/util/node/controller_utils.go` (11 untested helpers) and `pkg/controller/util/endpointslice/errors.go` (StaleInformerCache sentinel)
- Add **a small number of integration tests** under `test/integration/<subject>/` using the existing `test/integration/framework/` harness (in-process `kube-apiserver` + real `etcd`), only where unit tests cannot meaningfully exercise the behavior
- Achieve **70% line coverage on targeted packages** and **100% on reconcile happy path + primary error branches** for `syncNamespaceFromKey`, `NewReplicationManager`, `DeletePods`, `MarkPodsNotReady`, and `GetNodeCondition`

Implicit testing needs surfaced from the prompt:

- **Edge cases**: object-not-found, deletion-during-reconcile, owner-reference mismatches, label-selector edge cases, tombstone (`cache.DeletedFinalStateUnknown`) handling in event handlers
- **Error handling**: API server errors (`apierrors.IsNotFound`, `IsConflict`, `IsResourceExpired`), context cancellation, RateLimited responses, lister errors
- **Boundary conditions**: empty selectors, nil pod specs, zero/maximum replicas, namespace not yet propagated, empty error messages
- **Concurrency**: race detection enabled by default (`KUBE_RACE=-race`), informer cache mutation detection (`KUBE_CACHE_MUTATION_DETECTOR=true`), goroutine leak detection via `go.uber.org/goleak`
- **Async semantics**: `gomega.Eventually` with short poll intervals in integration tests; **no `time.Sleep`** anywhere
- **Finalizer lifecycle**: addition, removal, multi-pass reconcile
- **Status condition update logic**: condition transitions, `observedGeneration` tracking, `lastTransitionTime` semantics
- **Requeue semantics**: deterministic requeue intervals, exponential backoff for transient errors via `workqueue.TypedRateLimiter`

### 0.1.2 Special Instructions and Constraints

The user's prompt contains directives that the Blitzy platform preserves and applies to every test file and supporting artifact in this plan:

- **User Directive (verbatim)**: "Only add test files (`*test.go`). Do not modify any production `.go` files unless a trivial unexported-to-exported visibility change is strictly required for testability."
- **User Directive (verbatim)**: "Use controller-runtime's built-in `fake.NewClientBuilder()` for unit tests. No hand-rolled mocks — keep it idiomatic."
- **User Directive (verbatim)**: "No `time.Sleep` in tests. Every test must be independently runnable (no shared mutable state)."
- **User Directive (verbatim)**: "Validation: Intentionally break one reconcile function (e.g., comment out a status update), confirm the corresponding test fails — proving tests actually catch regressions."
- **User Directive (verbatim)**: "Add a `TESTING.md` that explains how to run each test tier, how to add new tests, and what envtest requires (e.g., `KUBEBUILDER_ASSETS`)."

Web-search requirements: research was performed on `sigs.k8s.io/controller-runtime/pkg/envtest` Go-1.25 compatibility (issue surfaced October 2025: `setup-envtest` latest requires Go ≥ 1.25.0). The repository's Go toolchain is `1.25.0`, so envtest would be technically compatible; however, the user's "test files only" directive precludes the production-file modifications required to vendor `sigs.k8s.io/controller-runtime` (go.mod and `vendor/`). The Blitzy platform therefore adapts the directive to the repository-native equivalents documented below.

### 0.1.3 Technical Interpretation

These testing requirements translate to the following technical test-implementation strategy adapted to the actual repository (`k8s.io/kubernetes` monorepo), which does **not** use `sigs.k8s.io/controller-runtime` (confirmed: zero references in `go.sum`):

- To **test reconciliation logic in `pkg/controller/namespace`**, the platform will **create `pkg/controller/namespace/namespace_controller_test.go`** that exercises `NewNamespaceController`, `nsControllerRateLimiter`, `enqueueNamespace`, `syncNamespaceFromKey`, and `worker` using `k8s.io/client-go/kubernetes/fake.NewSimpleClientset` (the idiomatic repo-native substitute for `controller-runtime fake.NewClientBuilder`)
- To **test reconciliation logic in `pkg/controller/replication`**, the platform will **create `pkg/controller/replication/replication_controller_test.go`** that exercises `NewReplicationManager` and the RC↔RS conversion adapters in `conversion.go`
- To **test utility helpers in `pkg/controller/util/node`**, the platform will **create `pkg/controller/util/node/controller_utils_test.go`** covering all 11 functions (`DeletePods`, `SetPodTerminationReason`, `MarkPodsNotReady`, `RecordNodeEvent`, `RecordNodeStatusChange`, `SwapNodeControllerTaint`, `AddOrUpdateLabelsOnNode`, `CreateAddNodeHandler`, `CreateUpdateNodeHandler`, `CreateDeleteNodeHandler`, `GetNodeCondition`)
- To **test the StaleInformerCache sentinel in `pkg/controller/util/endpointslice`**, the platform will **create `pkg/controller/util/endpointslice/errors_test.go`** covering `NewStaleInformerCache`, `Error()`, and `IsStaleInformerCacheErr`
- To **augment validation tests**, the platform will **update** `pkg/apis/apps/validation/validation_test.go`, `pkg/apis/core/validation/validation_test.go`, and `staging/src/k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/validation/validation_test.go` with new table-driven boundary cases without removing or restructuring existing tests
- To **document how to run each tier**, the platform will **create `TESTING.md`** at the repository root explaining `make test`, `make test-integration`, `KUBE_RACE`, `KUBE_COVER`, `KUBE_TIMEOUT`, and noting that the repository's `test/integration/framework/` substitutes for `controller-runtime envtest` (no `KUBEBUILDER_ASSETS` required because the apiserver is in-process)

### 0.1.4 Coverage Requirements Interpretation

Explicit coverage targets stated by the user:

- **70% line coverage on targeted packages**
- **100% on reconcile happy path + primary error branches**

Implicit coverage expectations derived from the repository's conventions (Tech Spec section 6.6 establishes that the repository has **no global coverage target**; race detection and cache-mutation detection are correctness gates instead, with `KUBE_COVER=y` as an opt-in measurement flag):

- To achieve comprehensive testing on the four primary CREATE targets, coverage must include:
  - For `pkg/controller/util/node`: all 11 exported functions plus the `nil`-status and missing-condition branches of `GetNodeCondition`
  - For `pkg/controller/util/endpointslice`: all 4 testable items (constructor, `Error()`, `IsStaleInformerCacheErr` true-branch, `IsStaleInformerCacheErr` false-branches including `nil` and unrelated error types)
  - For `pkg/controller/namespace`: `NewNamespaceController` field initialization, `syncNamespaceFromKey` NotFound branch, deleter-success branch, deleter-error branch, lister-transient-error branch, and `enqueueNamespace` tombstone handling
  - For `pkg/controller/replication`: `NewReplicationManager` constructor with `EnableStatusTerminatingReplicas=false` assertion, conversion round-trips via `convertRCtoRS`/`convertRStoRC`, event-handler `OnAdd`/`OnUpdate`/`OnDelete` with tombstone, and `clientsetAdapter` Create/Update/UpdateStatus/Get/List/Watch/Patch/Apply/ApplyStatus delegation
- Coverage is measured by opting in with `KUBE_COVER=y` (`KUBE_COVER=y make test WHAT=./pkg/controller/util/node/... ./pkg/controller/util/endpointslice/... ./pkg/controller/namespace/... ./pkg/controller/replication/...`)
- The user's regression-proof directive ("intentionally break one reconcile function, confirm the test fails") will be applied to `syncNamespaceFromKey` as the canonical sanity check


## 0.2 Test Discovery and Analysis

### 0.2.1 Existing Test Infrastructure Assessment

Repository analysis confirms the project is the upstream Kubernetes monorepo (`k8s.io/kubernetes`) with a documentation-only Blitzy audit overlay, **not** a kubebuilder/operator-SDK scaffolded project. The repository ships its own comprehensive testing infrastructure that the Blitzy platform will use rather than adding `sigs.k8s.io/controller-runtime`.

| Aspect | Value (from repository inspection) |
|--------|-------------------------------------|
| Current testing framework | Standard Go `testing` + `github.com/stretchr/testify` v1.11.1 (unit); `github.com/onsi/ginkgo/v2` v2.27.4 + `github.com/onsi/gomega` v1.39.0 (Ginkgo-style suites where used); repo-native `test/integration/framework` (integration) |
| Test runner configuration | `hack/make-rules/test.sh` (287 lines) invoked by `make test`; `hack/make-rules/test-integration.sh` invoked by `make test-integration`; `gotestsum --raw-command go test -json` for JSON streaming |
| Coverage tool | Standard Go `-coverprofile` plumbed through `KUBE_COVER=y` opt-in flag in `hack/make-rules/test.sh` |
| Mock/stub libraries detected | `k8s.io/client-go/kubernetes/fake.NewSimpleClientset` for clientset; `k8s.io/client-go/testing` for fake-action assertions and `PrependReactor` error injection; `k8s.io/client-go/tools/record.NewFakeRecorder` for events; `k8s.io/klog/v2/ktesting` for context-bound test logger |
| Test data fixtures or factories | Inline Go-struct factories (e.g., `cronJob()` helper in `pkg/controller/cronjob/cronjob_controllerv2_test.go`); shared helpers in `pkg/controller/testutil/test_utils.go` (`NewNode`, `NewPod`, `FakeRecorder`, `FakeNodeHandler`, `GetKey`); no YAML fixtures for unit tests |

Repository scan results that shaped the plan:

- **122 existing `*_test.go` files** under `pkg/controller/` and an extensive integration suite under `test/integration/` (60+ subpackages). This contradicts the prompt's "greenfield" framing; the Blitzy platform reinterprets it as "treat the four target packages as if coverage were minimal and add coverage with intent, without disturbing existing tests."
- **Confirmed uncovered targets** (zero existing `*_test.go` for the named source file):
  - `pkg/controller/util/node/controller_utils.go` — 11 functions, 0 tests
  - `pkg/controller/util/endpointslice/errors.go` — 4 testable items, 0 tests
  - `pkg/controller/namespace/namespace_controller.go` — controller has integration test in `test/integration/namespace/` but no unit test
  - `pkg/controller/replication/replication_controller.go` — controller has integration test in `test/integration/replicationcontroller/` and a `replication_controller_utils_test.go` for util helpers, but no unit test for the controller itself or for `conversion.go`
- **Integration framework signatures** (from `test/integration/framework/`):
  - `func EtcdMain(tests func() int)` — `TestMain` wrapper that boots a real `etcd` 3.6.7 subprocess (substitutes for `envtest.Environment.Start`/`Stop`)
  - `func StartTestServer(ctx context.Context, t testing.TB, setup TestServerSetup) (client.Interface, *rest.Config, TearDownFunc)` — boots an in-process `kube-apiserver` with ephemeral TLS (substitutes for `envtest.Environment.Start` returning `*rest.Config`)
- **Existing helper packages confirmed**:
  - `pkg/controller/testutil/test_utils.go` (existing `FakeRecorder`, `FakeNodeHandler`, `NewNode`, `NewPod`, `GetKey`, `GetZones`, `CreateZoneID`)
  - `test/integration/framework/{etcd.go, test_server.go, controlplane_utils.go, cbor.go, goleak.go, logger.go, serializer.go, flags.go, util.go}`
  - `test/utils/` (broader shared test helpers)

### 0.2.2 Web Search Research Conducted

Targeted web research was performed to validate testing-tool compatibility and to confirm the architectural decision to use repo-native test infrastructure:

- <cite index="1-1,1-3">Research on `setup-envtest` Go version requirement</cite>: <cite index="5-1">As of October 2025, `setup-envtest@latest` requires Go ≥ 1.25.0 (controller-runtime v0.22.3)</cite>, which is compatible with the repository's Go 1.25.0 toolchain. However, adding `sigs.k8s.io/controller-runtime` would require modifying `go.mod` and `vendor/` (production files), conflicting with the user's "test files only" directive.
- <cite index="2-5,2-6">Research on `envtest` operation</cite>: <cite index="3-1">controller-runtime/pkg/envtest helps write integration tests for controllers by setting up and starting an instance of etcd and the Kubernetes API server, without kubelet, controller-manager or other components</cite>. <cite index="2-6">Control plane binaries can be overridden by setting the `KUBEBUILDER_ASSETS` environment variable</cite>. The repository's `test/integration/framework` accomplishes the same outcome without requiring `KUBEBUILDER_ASSETS` because it builds and runs the in-tree `kube-apiserver` binary in-process.
- **Best practices for table-driven Go testing** (standard Go convention): use `tests := []struct{ name string; ...; wantErr bool }{...}`; iterate via `for _, tc := range tests { t.Run(tc.name, func(t *testing.T) { ... }) }`; capture local loop variable explicitly when running in parallel.
- **Recommended mocking strategies for Kubernetes clients**: use `k8s.io/client-go/kubernetes/fake.NewSimpleClientset(initialObjects...)`, then assert behavior via `clientset.Actions()` (typed action records from `k8s.io/client-go/testing`). For error injection, use `clientset.PrependReactor("verb", "resource", func(...) (bool, runtime.Object, error) { ... })`.
- **Test-organization conventions for `k8s.io/kubernetes`**: unit tests alongside production code at `pkg/controller/<name>/<name>_controller_test.go`; integration tests under `test/integration/<subject>/` with a `main_test.go` that calls `framework.EtcdMain(m.Run)`; no `internal/testutil/` package (the canonical location is `pkg/controller/testutil/`).
- **Common pitfalls to avoid**: never call `time.Sleep` in tests (use `wait.PollUntilContextTimeout` or `gomega.Eventually`); never share mutable clientset state across `t.Run` cases (construct fresh `fake.NewSimpleClientset()` per case); always pass a derived `context.Context` from `ktesting.NewContext(t)` rather than `context.Background()` so logs route to `t.Log`.


## 0.3 Testing Scope Analysis

### 0.3.1 Test Target Identification

**Primary code to be tested** (CREATE targets — currently no `*_test.go` exists at the target path):

- **Module**: `pkg/controller/util/node` at `pkg/controller/util/node/controller_utils.go` — requires **unit tests**
  - Functions: `DeletePods`, `SetPodTerminationReason`, `MarkPodsNotReady`, `RecordNodeEvent`, `RecordNodeStatusChange`, `SwapNodeControllerTaint`, `AddOrUpdateLabelsOnNode`, `CreateAddNodeHandler`, `CreateUpdateNodeHandler`, `CreateDeleteNodeHandler`, `GetNodeCondition`
  - Test categories needed: happy path, NotFound, DaemonSet-skip, mirror-pod-skip, tombstone handling, nil-status handling, error propagation
- **Module**: `pkg/controller/util/endpointslice` at `pkg/controller/util/endpointslice/errors.go` — requires **unit tests**
  - Functions: `NewStaleInformerCache`, `(e *StaleInformerCache).Error()`, `IsStaleInformerCacheErr`
  - Test categories needed: constructor invariants, error-string round-trip, type-guard truth table (nil, unrelated, direct, wrapped)
- **Module**: `pkg/controller/namespace` at `pkg/controller/namespace/namespace_controller.go` — requires **unit tests**
  - Functions: `NewNamespaceController`, `nsControllerRateLimiter`, `(nm *NamespaceController).enqueueNamespace`, `(nm *NamespaceController).worker`, `(nm *NamespaceController).syncNamespaceFromKey`
  - Test categories needed: constructor field initialization, queue enqueue with valid/tombstone/unexpected object, sync NotFound branch, sync deleter-success branch, sync deleter-error branch, sync lister-transient-error branch, worker drain semantics
- **Module**: `pkg/controller/replication` at `pkg/controller/replication/replication_controller.go` + `conversion.go` — requires **unit tests**
  - Functions: `NewReplicationManager`; in `conversion.go`: `informerAdapter.Informer/Lister`, `conversionInformer.AddEventHandler/AddEventHandlerWithResyncPeriod`, `conversionLister.List/ReplicaSets/GetPodReplicaSets`, `conversionNamespaceLister.List/Get`, `conversionEventHandler.OnAdd/OnUpdate/OnDelete`, `clientsetAdapter.AppsV1/Apps`, `conversionAppsV1Client.ReplicaSets`, `conversionClient.Create/Update/UpdateStatus/Get/List/Watch/Patch/Apply/ApplyStatus/GetScale/UpdateScale/ApplyScale`, `convertSlice`, `convertList`, `convertCall`, `convertRCtoRS`, `convertRStoRC`, `podControlAdapter.CreatePods/DeletePod`
  - Test categories needed: constructor field assertions (EnableStatusTerminatingReplicas=false, kind="ReplicationController", controller name), RC↔RS conversion round-trip including label/annotation preservation, event-handler tombstone handling, fake-clientset round-trip for each conversionClient method

**Existing test file mapping** (UPDATE candidates — incremental rows only):

| Source File | Existing Test File | Test Categories Present |
|-------------|--------------------|--------------------------|
| `pkg/controller/cronjob/cronjob_controllerv2.go` | `pkg/controller/cronjob/cronjob_controllerv2_test.go` | Controller sync, sort, list jobs, status update — add finalizer + requeue rows |
| `pkg/controller/deployment/deployment_controller.go` | `pkg/controller/deployment/deployment_controller_test.go` | Controller sync, recreate, rollback, scale — add status-condition transition rows |
| `pkg/apis/apps/validation/validation.go` | `pkg/apis/apps/validation/validation_test.go` | ValidateDeployment, ValidateStatefulSet, ValidateDaemonSet — add boundary cases |
| `pkg/apis/core/validation/validation.go` | `pkg/apis/core/validation/validation_test.go` | ValidatePod, ValidateService, ValidateNamespace — add boundary cases |
| `staging/.../apiextensions/validation/validation.go` | `staging/.../apiextensions/validation/validation_test.go` | ValidateCustomResourceDefinition, CEL validation — add edge cases |

**Dependencies requiring mocking** (achieved via repo-native facilities, not external mock libraries):

- External services to mock: none in unit scope (no HTTP clients, no etcd, no external APIs in the targeted packages)
- Database interactions to stub: none in unit scope (etcd is only contacted via `kube-apiserver` in integration tests, which use real `etcd` via `framework.EtcdMain`)
- Kubernetes API operations to fake: `k8s.io/client-go/kubernetes/fake.NewSimpleClientset(initialObjects...)` for clientset-level Create/Update/Delete/List/Get/Watch; `k8s.io/client-go/testing.Fake.PrependReactor` for error injection
- Event recording to capture: `k8s.io/client-go/tools/record.NewFakeRecorder(capacity)` to receive emitted events on a buffered channel
- Logger context: `k8s.io/klog/v2/ktesting.NewContext(t, ktesting.NewConfig())` to route logs to `t.Log`
- Informers: `k8s.io/client-go/informers.NewSharedInformerFactory(clientset, resyncPeriod)`; for pre-populating listers without starting informers, call `informerFactory.Core().V1().Namespaces().Informer().GetIndexer().Add(&v1.Namespace{...})` directly
- DaemonSetLister (for `DeletePods`): use `appsv1listers.NewDaemonSetLister(cache.NewIndexer(...))` with manually `Add`-ed `*appsv1.DaemonSet` items

### 0.3.2 Version Compatibility Research

Based on the repository's existing `go.mod` (Go 1.25.0 toolchain), the recommended testing stack is the **already-vendored** repo-native stack. No version conflicts exist because no new dependencies are introduced.

| Component | Name | Version (exact, from `go.mod`) | Rationale |
|-----------|------|--------------------------------|-----------|
| Testing framework | Standard Go `testing` | bundled with Go 1.25.0 | Idiomatic and repo-conventional |
| Assertion library | `github.com/stretchr/testify` | v1.11.1 | Already vendored; used by `pkg/controller/podautoscaler`, `pkg/controller/resourceclaim`, `pkg/controller/util/selectors` |
| Mocking library | `k8s.io/client-go/kubernetes/fake` | workspace replace → `staging/src/k8s.io/client-go` | Repo-native substitute for `controller-runtime fake.NewClientBuilder()` |
| Event recording | `k8s.io/client-go/tools/record` (`NewFakeRecorder`) | workspace replace | Repo-native fake recorder |
| Logger for tests | `k8s.io/klog/v2/ktesting` | v2.130.1 | Routes logs to `t.Log` |
| BDD framework (integration) | `github.com/onsi/ginkgo/v2` | v2.27.4 | Already vendored |
| Matchers (integration) | `github.com/onsi/gomega` | v1.39.0 | `Eventually` for async assertions in integration tests |
| Goroutine leak detection | `go.uber.org/goleak` | v1.3.0 | Already vendored; used by `test/integration/framework/goleak.go` |
| Deep comparison | `github.com/google/go-cmp` | v0.7.0 | Already vendored |
| Fuzz/randomized data | `sigs.k8s.io/randfill` | v1.0.0 | Already vendored (fuzz tier; not used by primary unit suite) |
| Coverage tool | Standard Go `-coverprofile` via `KUBE_COVER=y` | bundled | Opt-in coverage, no new tool |
| Integration harness | `k8s.io/kubernetes/test/integration/framework` | in-tree | Repo-native substitute for `controller-runtime envtest.Environment` |

**Version-conflict resolution**: <cite index="3-1">controller-runtime envtest sets up a local etcd and kube-apiserver</cite>, which is the same architecture as `framework.EtcdMain` + `framework.StartTestServer`. <cite index="2-6">Whereas envtest requires `KUBEBUILDER_ASSETS` to locate binaries</cite>, the repo-native framework builds and runs the in-tree apiserver in-process, so **no `KUBEBUILDER_ASSETS` configuration is required**. This sidesteps the Go-1.25 setup-envtest constraint entirely while delivering the same capability.


## 0.4 Test Implementation Design

### 0.4.1 Test Strategy Selection

A two-tier strategy is adopted, matching the user's stated tiers and the repository's existing make targets:

- **Unit tests**: Table-driven Go tests living alongside the production source file (`*_test.go` in the same package). Focus on **isolated components** — no etcd, no apiserver, no goroutines outside `t.Run`. Use `k8s.io/client-go/kubernetes/fake.NewSimpleClientset` as the standard fake; assert via typed action records and direct return-value comparison.
- **Integration tests** (small number, only where unit tests cannot meaningfully exercise behavior): Suites under `test/integration/<subject>/` that call `framework.EtcdMain(m.Run)` in `TestMain` and `framework.StartTestServer(ctx, t, framework.TestServerSetup{...})` to obtain a real `*rest.Config` against an in-process `kube-apiserver`. Focus on **component interactions** that require a live API server (resource versioning, watch semantics, status sub-resource).
- **Edge-case tests**: Boundary-condition rows added directly to the table-driven cases of the unit and validation tests (zero/empty/nil/max values, label-selector edge cases, owner-reference mismatches).
- **Error-handling tests**: Rows that exercise `apierrors.IsNotFound`, `IsConflict`, transient errors via `clientset.PrependReactor`, and context cancellation.

### 0.4.2 Test Case Blueprint

Component: pkg/controller/util/node (CREATE pkg/controller/util/node/controller_utils_test.go)
Test Categories:
- Happy path: DeletePods with mixed pods returns (true, nil) and emits NodeControllerEviction events; MarkPodsNotReady transitions Ready=True→False and emits NodeNotReady events; GetNodeCondition returns (index, &cond) for present type; SwapNodeControllerTaint adds taint when absent (true) and removes when present (true); AddOrUpdateLabelsOnNode applies new labels (true)
- Edge cases: empty pod slice (no-op, returns (false, nil)); pod with mirror-pod annotation skipped; pod owned by DaemonSet skipped via DaemonSetLister; GetNodeCondition with nil NodeStatus returns (-1, nil); GetNodeCondition with empty Conditions returns (-1, nil); duplicate condition type returns first index; CreateDeleteNodeHandler with cache.DeletedFinalStateUnknown tombstone unwraps to *v1.Node
- Error cases: API NotFound on Get is non-fatal; API Conflict on Update propagated; clientset.PrependReactor("update","nodes",...) returning error causes SwapNodeControllerTaint to return false; UpdateStatus error causes MarkPodsNotReady to return error
- Performance boundaries: not applicable for these helpers

Component: pkg/controller/util/endpointslice (CREATE pkg/controller/util/endpointslice/errors_test.go)
Test Categories:
- Happy path: NewStaleInformerCache("msg") returns non-nil with Error()=="msg"; IsStaleInformerCacheErr on a *StaleInformerCache returns true
- Edge cases: NewStaleInformerCache("") returns non-nil with Error()==""; IsStaleInformerCacheErr(nil) returns false; IsStaleInformerCacheErr on errors.New("x") returns false
- Error cases: IsStaleInformerCacheErr on fmt.Errorf("%w", e) returns false (documents that the implementation uses type assertion, not errors.As — this is intentional per code review)
- Performance boundaries: not applicable

Component: pkg/controller/namespace (CREATE pkg/controller/namespace/namespace_controller_test.go)
Test Categories:
- Happy path: NewNamespaceController returns *NamespaceController with non-nil queue, lister, recorder, namespacedResourcesDeleter; syncNamespaceFromKey on existing namespace invokes deleter.Delete(ctx, name) exactly once and returns nil
- Edge cases: syncNamespaceFromKey with key not in lister (returns apierrors.IsNotFound from lister) returns nil with no deleter invocation; enqueueNamespace with cache.DeletedFinalStateUnknown unwraps and queues the wrapped key; enqueueNamespace with unrelated object type calls utilruntime.HandleError (verified by swapping runtime.ErrorHandlers via t.Cleanup)
- Error cases: syncNamespaceFromKey when lister returns a non-NotFound error returns the error; syncNamespaceFromKey when deleter.Delete returns an error returns that error; worker drains the queue and requeues on transient err, no requeue on terminal err
- Performance boundaries: nsControllerRateLimiter When() invariants — initial attempt yields a small backoff (within bounded range), subsequent attempts grow exponentially up to the documented maximum

Component: pkg/controller/replication (CREATE pkg/controller/replication/replication_controller_test.go)
Test Categories:
- Happy path: NewReplicationManager returns non-nil pointer; embedded ReplicaSetController has burstReplicas set, EnableStatusTerminatingReplicas=false, GroupVersionKind=v1.SchemeGroupVersion.WithKind("ReplicationController"), controller name "replication_controller"; conversionLister.List returns RC objects converted to *apps.ReplicaSet preserving labels, annotations, replicas, selector
- Edge cases: convertRCtoRS / convertRStoRC round-trip preserves UID, ResourceVersion, OwnerReferences, ObservedGeneration; conversionEventHandler.OnDelete with tombstone unwraps to *v1.ReplicationController and dispatches; conversionNamespaceLister.Get on missing name returns apierrors.IsNotFound; conversionLister.GetPodReplicaSets with no matching pod returns empty slice
- Error cases: conversionClient.Create when underlying RC client returns apierrors.IsAlreadyExists propagates; conversionClient.Update with stale ResourceVersion propagates Conflict; conversionClient.Watch returns wrapped watch.Interface that propagates events
- Performance boundaries: not applicable

### 0.4.3 Existing Test Extension Strategy

- **Tests to extend** — augment `pkg/controller/cronjob/cronjob_controllerv2_test.go` by adding cases for finalizer addition/removal and requeue-on-transient-error if not already present; augment `pkg/controller/deployment/deployment_controller_test.go` by adding cases for status condition transitions (Available, Progressing, ReplicaFailure) and `observedGeneration` advancement
- **Tests to refactor** — none; the existing tests follow current repo conventions and do not need modernization
- **Tests to fix** — none; no broken tests are in scope per the user's "add tests, do not modify production" directive; if a test breakage is incidentally discovered during execution it will be flagged in the implementation report rather than silently fixed

### 0.4.4 Test Data and Fixtures Design

- **Required test data structures**: inline Go-struct literals constructed per `t.Run` (e.g., `&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test-ns"}}`); for nodes and pods, reuse existing helpers `testutil.NewNode(name)` and `testutil.NewPod(name, host)` from `pkg/controller/testutil/test_utils.go`
- **Fixture organization strategy**: no `testdata/` YAML files for unit tests (matches the convention of `pkg/controller/cronjob/cronjob_controllerv2_test.go` and the rest of `pkg/controller/`); for integration tests added under `test/integration/`, fixtures are constructed via `clientset.CoreV1().<Resource>().Create(...)` rather than loaded from disk
- **Mock object specifications**: fake clientset via `fake.NewSimpleClientset(initialObjects...)`; fake recorder via `record.NewFakeRecorder(capacity)`; for namespace lister, pre-populate via `informerFactory.Core().V1().Namespaces().Informer().GetIndexer().Add(...)` without starting the informer; for DaemonSetLister, construct via `appsv1listers.NewDaemonSetLister(cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}))` and `Add` items directly
- **Test database/state management approach**: no test database is used at the unit tier; each `t.Run` constructs fresh fakes and listers (no shared mutable state, per the user's directive); integration tests inherit `framework.EtcdMain` etcd lifecycle and `framework.StartTestServer` apiserver lifecycle, both of which clean up via `TearDownFunc` returned from `StartTestServer`

```go
// Canonical fake-clientset + ktesting + recorder pattern (illustrative, ~5 lines)
_, ctx := ktesting.NewTestContext(t)
client := fake.NewSimpleClientset(initialObjects...)
recorder := record.NewFakeRecorder(16)
factory := informers.NewSharedInformerFactory(client, 0)
// ... pre-populate listers via Informer().GetIndexer().Add(...) and invoke unit under test
```

### 0.4.5 Helper Package Layout

The user's prompt referenced an `internal/testutil/` package typical of kubebuilder scaffolds. In this repository the idiomatic equivalents already exist and are reused rather than re-created:

- **`pkg/controller/testutil/test_utils.go`** (existing) — provides `FakeRecorder`, `FakeLegacyHandler`, `FakeNodeHandler` (with Create/Get/List/Update/Delete/Watch/Patch/Apply), `NewNode`, `NewPod`, `GetKey(obj, t)`, `GetZones`, `CreateZoneID`, `NewFakeRecorder`
- **`test/integration/framework/`** (existing) — provides `EtcdMain`, `StartTestServer`, `TestServerSetup`, `TearDownFunc`, `IgnoreBackgroundGoroutines`, `GoleakCheck`, `RedirectKlog`, `NewTBWriter`, `SharedEtcd`, `DefaultOpenAPIConfig`, `UnprivilegedUserToken`, `AssertRequestResponseAsCBOR`
- **`test/utils/`** (existing) — broader cross-tier test helpers
- **Package-local helpers** — if a new helper is needed and only relevant to one package, place it within an `*_test.go` file in the same package (Go build tag automatically excludes from non-test builds). Do **not** create a new top-level `internal/testutil/`.

### 0.4.6 TESTING.md Outline

The new `TESTING.md` at the repository root will document:

- **Test Tiers**: unit (`make test`), integration (`make test-integration`), conformance (`make WHAT=...`), e2e (separate runners), fuzz (separate runners)
- **Running Unit Tests**: `make test WHAT=./pkg/controller/util/node/...`; flags `KUBE_RACE` (default `-race`), `KUBE_COVER=y`, `KUBE_TIMEOUT=180s`, `KUBE_VERBOSE`, `KUBE_CACHE_MUTATION_DETECTOR=true`
- **Running Integration Tests**: `make test-integration WHAT=./test/integration/<subject>/...`; flags `KUBE_INTEGRATION_TEST_MAX_CONCURRENCY`, `FOCUS`, `SKIP`
- **Coverage**: `KUBE_COVER=y` opt-in; output written under `${KUBE_TIMESTAMP}/coverage.out`; opens `go tool cover -html=coverage.out`
- **Adding a New Test**: location convention (alongside production for unit; under `test/integration/<subject>/` for integration); naming (`TestXxx` + table-driven); use `ktesting.NewTestContext(t)` for logger/context; use `record.NewFakeRecorder` for events; no `time.Sleep`; `gomega.Eventually` permitted in integration but discouraged in unit
- **Mocking Dependencies**: `k8s.io/client-go/kubernetes/fake.NewSimpleClientset` as standard; `informers.NewSharedInformerFactory` for informers; `record.NewFakeRecorder(capacity)` for events
- **envtest Substitution Note**: explains that this repo uses `test/integration/framework` instead of `controller-runtime envtest`, and that **`KUBEBUILDER_ASSETS` is not required** because the repo builds and runs its own `kube-apiserver` in-process
- **Regression-Proof Drill**: documents the user's directive — temporarily comment out a status update in `syncNamespaceFromKey`, run the corresponding test, observe a failure, restore the change


## 0.5 Test File Transformation Mapping

### 0.5.1 File-by-File Test Plan

The complete file transformation table. Target test file is listed **first** in each row, per format requirement. Transformation modes: **CREATE** = new file; **UPDATE** = augment existing file with additional rows/cases; **DELETE** = remove obsolete file (none in scope); **REFERENCE** = use as pattern guide (do not modify).

| Target Test File | Transformation | Source File / Test | Purpose / Changes |
|------------------|----------------|--------------------|-------------------|
| `pkg/controller/util/node/controller_utils_test.go` | CREATE | `pkg/controller/util/node/controller_utils.go` | Add comprehensive unit tests for all 11 helpers (`DeletePods`, `SetPodTerminationReason`, `MarkPodsNotReady`, `RecordNodeEvent`, `RecordNodeStatusChange`, `SwapNodeControllerTaint`, `AddOrUpdateLabelsOnNode`, `CreateAddNodeHandler`, `CreateUpdateNodeHandler`, `CreateDeleteNodeHandler`, `GetNodeCondition`) including happy path, NotFound, DaemonSet-skip, mirror-pod-skip, tombstone, nil-status, and Conflict/Update-error branches |
| `pkg/controller/util/endpointslice/errors_test.go` | CREATE | `pkg/controller/util/endpointslice/errors.go` | Add unit tests for the StaleInformerCache error sentinel: `NewStaleInformerCache` constructor (including empty msg), `(e *StaleInformerCache).Error()`, `IsStaleInformerCacheErr` truth table (nil, unrelated, direct, wrapped) |
| `pkg/controller/namespace/namespace_controller_test.go` | CREATE | `pkg/controller/namespace/namespace_controller.go` | Add unit tests for `NewNamespaceController`, `nsControllerRateLimiter`, `enqueueNamespace` (valid/tombstone/unexpected-type), `syncNamespaceFromKey` (NotFound, exists+deleter-success, exists+deleter-error, lister-error), `worker` drain semantics |
| `pkg/controller/replication/replication_controller_test.go` | CREATE | `pkg/controller/replication/replication_controller.go`, `pkg/controller/replication/conversion.go` | Add unit tests for `NewReplicationManager` (field assertions, EnableStatusTerminatingReplicas=false), `informerAdapter`/`conversionLister`/`conversionEventHandler` (OnAdd/OnUpdate/OnDelete with tombstone), `clientsetAdapter`/`conversionClient` round-trip via `convertRCtoRS`/`convertRStoRC` for Create/Update/UpdateStatus/Get/List/Watch/Patch/Apply/ApplyStatus/GetScale/UpdateScale, and `podControlAdapter.CreatePods`/`DeletePod` |
| `TESTING.md` | CREATE | (root) | New repository-root document explaining how to run each test tier (unit, integration, conformance), how to add new tests, KUBE_* flags, the regression-proof drill, and the `test/integration/framework` substitution for `controller-runtime envtest` (no `KUBEBUILDER_ASSETS` required) |
| `pkg/controller/cronjob/cronjob_controllerv2_test.go` | UPDATE | `pkg/controller/cronjob/cronjob_controllerv2_test.go` | Add incremental table rows for finalizer add/remove and requeue-on-transient-error scenarios; preserve every existing test |
| `pkg/controller/deployment/deployment_controller_test.go` | UPDATE | `pkg/controller/deployment/deployment_controller_test.go` | Add incremental table rows for status-condition transitions (Available/Progressing/ReplicaFailure) and `observedGeneration` advancement; preserve every existing test |
| `pkg/apis/apps/validation/validation_test.go` | UPDATE | `pkg/apis/apps/validation/validation_test.go` | Add boundary-value rows for Deployment/StatefulSet replica counts (0, 1, MaxInt32), selector edge cases, and template-validation interactions; preserve every existing test |
| `pkg/apis/core/validation/validation_test.go` | UPDATE | `pkg/apis/core/validation/validation_test.go` | Add boundary-value rows for Pod resource quantities, container name length, namespace name length; preserve every existing test |
| `staging/src/k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/validation/validation_test.go` | UPDATE | `staging/src/k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/validation/validation_test.go` | Add edge-case rows for CEL validation expressions (empty, very long, recursive self-reference) and schema ratcheting boundary cases; preserve every existing test |
| `pkg/controller/testutil/test_utils.go` | REFERENCE | (existing helpers) | Use the existing `FakeRecorder`, `FakeNodeHandler`, `NewNode`, `NewPod`, `GetKey` helpers in new tests; do not duplicate or modify |
| `test/integration/framework/etcd.go`, `test/integration/framework/test_server.go` | REFERENCE | (existing harness) | Reuse `framework.EtcdMain` and `framework.StartTestServer` in any integration tests added; do not modify |
| `test/integration/deployment/main_test.go` | REFERENCE | (existing pattern) | Copy-paste template for `TestMain(m *testing.M){ framework.EtcdMain(m.Run) }` in any new integration suite |
| `pkg/controller/cronjob/cronjob_controllerv2_test.go` | REFERENCE | (existing pattern) | Canonical table-driven unit-test pattern using `fake.NewSimpleClientset` + `informers` + `ktesting` + `record.NewFakeRecorder`; also a UPDATE target |

All test files expected to be in scope based on the user's instructions are enumerated above. No file is left as "pending" or "to be discovered."

### 0.5.2 New Test Files Detail

- **`pkg/controller/util/node/controller_utils_test.go`** — unit-test coverage for 11 helper functions
  - Test categories: happy path (taint swap success, label update success, condition lookup), edge cases (empty pods, nil status, tombstone, DaemonSet-skip, mirror-pod-skip), errors (NotFound, Conflict, Update-error injected via `clientset.PrependReactor`)
  - Mock dependencies: `fake.NewSimpleClientset()`, `record.NewFakeRecorder(N)`, `appsv1listers.NewDaemonSetLister(cache.NewIndexer(...))`
  - Assertions focus: return values; `clientset.Actions()` typed action records; events received on `FakeRecorder.Events` channel
- **`pkg/controller/util/endpointslice/errors_test.go`** — unit-test coverage for `StaleInformerCache`
  - Test categories: happy path (constructor + Error()), edge cases (empty message), type-guard truth table (nil, unrelated error, direct *StaleInformerCache, wrapped via `fmt.Errorf("%w", e)`)
  - Mock dependencies: none (pure value/error type testing)
  - Assertions focus: error-message string equality, boolean return of `IsStaleInformerCacheErr`
- **`pkg/controller/namespace/namespace_controller_test.go`** — unit-test coverage for `NamespaceController`
  - Test categories: constructor field assertions, queue-enqueue valid/tombstone/unexpected-type, sync NotFound, sync deleter-success, sync deleter-error, sync lister-transient-error, worker drain
  - Mock dependencies: `fake.NewSimpleClientset(initialNamespaces...)`, `informers.NewSharedInformerFactory`, prepopulated lister via `Informer().GetIndexer().Add`, custom in-test type implementing `deletion.NamespacedResourcesDeleterInterface`
  - Assertions focus: deleter invocation count and arguments, queue length after enqueue, error propagation
- **`pkg/controller/replication/replication_controller_test.go`** — unit-test coverage for `ReplicationManager` and its conversion adapters
  - Test categories: constructor field assertions (kind="ReplicationController", controller name, EnableStatusTerminatingReplicas=false), conversion round-trips (RC↔RS preserving labels, annotations, replicas, owner refs), event-handler dispatch with tombstone, conversionClient delegation for every method (Create/Update/UpdateStatus/Get/List/Watch/Patch/Apply/ApplyStatus/GetScale/UpdateScale/ApplyScale)
  - Mock dependencies: `fake.NewSimpleClientset(initialReplicationControllers...)`, `informers.NewSharedInformerFactory`
  - Assertions focus: reflect-deep-equal on converted objects, `clientset.Actions()` for delegation verification
- **`TESTING.md`** — repository-root test-runner documentation (no Go content)

### 0.5.3 Test Files to Modify Detail

- **`pkg/controller/cronjob/cronjob_controllerv2_test.go`** — add finalizer-lifecycle and requeue-on-transient-error rows
  - New test methods or table rows: finalizer-add row, finalizer-remove row, requeue-on-API-error row (using `clientset.PrependReactor`)
  - Updated fixtures: extend the inline `cronJob()` helper as needed without breaking existing callers
  - Assertions to add: post-sync ResourceVersion advancement on finalizer change, requeue delay falls within expected range
- **`pkg/controller/deployment/deployment_controller_test.go`** — add status-condition transition rows
  - New test methods or table rows: Available True→False, Progressing condition emission, ReplicaFailure condition on quota error, observedGeneration tracking
  - Updated fixtures: no changes
  - Assertions to add: condition list contains expected type+status; lastTransitionTime is monotonically non-decreasing
- **`pkg/apis/apps/validation/validation_test.go`** — boundary-value rows
  - New table rows for `ValidateDeployment`, `ValidateStatefulSet`, `ValidateDaemonSet`: replicas=0 (allowed), replicas=1, replicas=MaxInt32 (forbidden), empty selector (forbidden), label-key with leading dash (forbidden)
- **`pkg/apis/core/validation/validation_test.go`** — boundary-value rows
  - New table rows for `ValidatePod`, `ValidateService`, `ValidateNamespace`: zero CPU/memory request, max-length container name (63 chars), max-length namespace name (63 chars), invalid DNS-1123 names
- **`staging/.../apiextensions/validation/validation_test.go`** — CEL edge-case rows
  - New table rows for `ValidateCustomResourceDefinition`: empty CEL rule (forbidden), very long CEL expression (boundary), recursive self-reference (forbidden), ratcheting boundary cases

### 0.5.4 Test Configuration Updates

- **`Makefile`**: no change required; existing `test`, `test-integration`, and `verify` targets already cover the new test files (they are discovered by `kube::test::find_go_packages` in `hack/make-rules/test.sh`)
- **`hack/make-rules/test.sh`**: no change required
- **`hack/make-rules/test-integration.sh`**: no change required
- **Coverage configuration**: no static file change; coverage is gated by the `KUBE_COVER=y` environment variable at invocation time
- **`.gitignore`**: no change required (coverage artifacts already ignored)

### 0.5.5 Cross-File Test Dependencies

- **Shared fixtures**: reuse `testutil.NewNode(name)` and `testutil.NewPod(name, host)` from `pkg/controller/testutil/test_utils.go` in the new node-utility test; no new shared fixtures created
- **Mock objects**: each new test file uses its own per-`t.Run` `fake.NewSimpleClientset()` instance to satisfy the user's "no shared mutable state" directive
- **Test utilities**: reuse `testutil.GetKey(obj, t)` for namespace-key generation; reuse `testutil.NewFakeRecorder()` for the event-recording assertion pattern in the node-utility test
- **Import updates required across test files**: none; all new test files import existing, already-vendored packages


## 0.6 Dependency Inventory

### 0.6.1 Testing Dependencies

All testing dependencies required by this plan are **already present in `go.mod` and vendored** in `vendor/`. No new packages, no version bumps, and no `go.mod` modifications are required — which honors the user's "test files only" directive.

| Registry | Package Name | Version (exact, from `go.mod`) | Purpose |
|----------|--------------|-------------------------------|---------|
| go modules | `github.com/stretchr/testify` | v1.11.1 | Assertion library (`assert`, `require`); used by new unit tests for concise checks alongside the standard `testing` package |
| go modules | `github.com/onsi/ginkgo/v2` | v2.27.4 | BDD-style runner; available for Ginkgo suites in integration tests where applicable |
| go modules | `github.com/onsi/gomega` | v1.39.0 | Matcher library; provides `Eventually` for async assertions in integration tests (not used in unit tier) |
| go modules | `go.uber.org/goleak` | v1.3.0 | Goroutine leak detection; used by `test/integration/framework/goleak.go` and available to new tests via `goleak.VerifyTestMain` |
| go modules | `github.com/google/go-cmp` | v0.7.0 | Deep object comparison via `cmp.Diff` for clearer diff output than `reflect.DeepEqual` |
| go modules | `sigs.k8s.io/randfill` | v1.0.0 | Random-data filler (used by repo's fuzz tier; available if a property-based row is needed) |
| go modules | `k8s.io/client-go` (workspace replace → `./staging/src/k8s.io/client-go`) | v0.0.0 (workspace) | Provides `kubernetes/fake.NewSimpleClientset`, `informers.NewSharedInformerFactory`, `tools/record.NewFakeRecorder`, `testing.Fake` (PrependReactor) |
| go modules | `k8s.io/klog/v2` | v2.130.1 | Provides `ktesting.NewTestContext(t)` for context-bound test logger that routes to `t.Log` |
| go modules | `k8s.io/kubernetes/test/integration/framework` | in-tree | Provides `EtcdMain`, `StartTestServer`, `TestServerSetup`, `TearDownFunc` for integration tests |
| go modules | `k8s.io/kubernetes/pkg/controller/testutil` | in-tree | Provides existing `FakeRecorder`, `FakeNodeHandler`, `NewNode`, `NewPod`, `GetKey` helpers |
| binary | `go` | 1.25.0 (per `go.mod`); 1.25.6 (per `.go-version`) | Toolchain |
| binary | `etcd` | 3.6.7 (managed by `framework.EtcdMain`) | Real etcd subprocess for integration tests |

**Explicitly NOT added** (with rationale):

- `sigs.k8s.io/controller-runtime` and `sigs.k8s.io/controller-runtime/pkg/envtest` — would require modifying `go.mod` and adding to `vendor/` (production-file changes), which conflicts with the user's "test files only" directive. <cite index="3-1">controller-runtime/pkg/envtest provides etcd and kube-apiserver setup for controller integration tests</cite>; the repo-native `test/integration/framework` provides equivalent capability without the new dependency. <cite index="5-1">setup-envtest@latest currently requires Go ≥ 1.25.0</cite>, which the repository satisfies, but the dependency is still not required for this plan.

### 0.6.2 Import Updates

No import updates to existing production or test code are required. New test files introduce **new imports only within themselves**, drawing exclusively from already-vendored packages:

- Test files requiring import updates: **none**
- Import transformation rules: **not applicable** (no rewrites)

Canonical import block expected in each new unit test file:

```go
import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    v1 "k8s.io/api/core/v1"
    apierrors "k8s.io/apimachinery/pkg/api/errors"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/informers"
    "k8s.io/client-go/kubernetes/fake"
    core "k8s.io/client-go/testing"
    "k8s.io/client-go/tools/record"
    "k8s.io/klog/v2/ktesting"
    _ "k8s.io/kubernetes/pkg/apis/core/install"
    "k8s.io/kubernetes/pkg/controller/testutil"
)
```

Integration test files (if any are added) replace the unit-test imports with:

```go
import (
    "context"
    "testing"

    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/util/wait"
    "k8s.io/client-go/kubernetes"
    "k8s.io/kubernetes/test/integration/framework"
)
```


## 0.7 Coverage and Quality Targets

### 0.7.1 Coverage Metrics

The repository does **not** set a global coverage target; race detection and cache-mutation detection are the primary correctness gates (per Tech Spec section 6.6), with `KUBE_COVER=y` as an opt-in flag for measuring coverage on demand. Within the scope of this plan, the following per-package targets apply:

| Package | Current Coverage | Target Coverage | Rationale |
|---------|------------------|------------------|-----------|
| `pkg/controller/util/node` | 0% (no `*_test.go` exists) | 70% line + 100% on `DeletePods`, `MarkPodsNotReady`, `GetNodeCondition` happy paths | User-specified target; primary CREATE package |
| `pkg/controller/util/endpointslice` | 0% (no `*_test.go` exists) | 100% line | Trivial sentinel-error type with 4 testable items; full coverage is feasible and warranted |
| `pkg/controller/namespace` | 0% on `namespace_controller.go` (deletion sub-package has tests) | 70% line + 100% on `syncNamespaceFromKey` happy path + NotFound + deleter-error branches | User-specified target; primary CREATE package |
| `pkg/controller/replication` | Partial (only `replication_controller_utils.go` has tests) | 70% line + 100% on `NewReplicationManager` constructor + conversion round-trips | User-specified target; primary CREATE package |
| `pkg/controller/cronjob` (UPDATE) | Existing coverage retained | Net increase; no regression | Incremental UPDATE only |
| `pkg/controller/deployment` (UPDATE) | Existing coverage retained | Net increase; no regression | Incremental UPDATE only |
| `pkg/apis/apps/validation` (UPDATE) | Existing coverage retained | Net increase; no regression | Incremental UPDATE only |
| `pkg/apis/core/validation` (UPDATE) | Existing coverage retained | Net increase; no regression | Incremental UPDATE only |
| `staging/.../apiextensions/validation` (UPDATE) | Existing coverage retained | Net increase; no regression | Incremental UPDATE only |

**Coverage gaps to address** in the four CREATE targets:

- `pkg/controller/util/node`: every exported function has at least one happy-path row; in addition, `DeletePods` covers DaemonSet-skip, mirror-pod-skip, NotFound, and Conflict-on-update paths; `GetNodeCondition` covers nil-status, missing-condition, and multi-condition paths; `SwapNodeControllerTaint`/`AddOrUpdateLabelsOnNode` cover both true and false returns and the API-error branch via `clientset.PrependReactor`
- `pkg/controller/util/endpointslice`: 100% achievable — constructor, `Error()`, `IsStaleInformerCacheErr` true and false branches (including `nil` and unrelated error type)
- `pkg/controller/namespace`: `syncNamespaceFromKey` exercises lister NotFound → return nil, lister transient error → return error, deleter success → return nil, deleter error → return error; `enqueueNamespace` exercises valid object, `cache.DeletedFinalStateUnknown` tombstone, and unrelated-type error; constructor field initialization is asserted by reflection on the returned controller
- `pkg/controller/replication`: `NewReplicationManager` asserts the embedded controller's fields (kind="ReplicationController", controller name, `EnableStatusTerminatingReplicas=false`); each conversionClient method asserts that the underlying RC client received the converted RC and that the result was converted back to RS

**Coverage measurement command** (opt-in):

```bash
KUBE_COVER=y make test WHAT=./pkg/controller/util/node/... ./pkg/controller/util/endpointslice/... ./pkg/controller/namespace/... ./pkg/controller/replication/...
```

### 0.7.2 Test Quality Criteria

- **Assertion density**: each `t.Run` row contains at least one `require.NoError` or `require.Error` plus a behavior assertion (return value, action sequence, or event content); no row should be a tautology
- **Test isolation**: every `t.Run` constructs a fresh `fake.NewSimpleClientset()` and fresh informer factory; no test mutates package-level variables; the user's "Every test must be independently runnable (no shared mutable state)" directive is enforced by this construction pattern
- **Performance constraints**: each unit test must complete well within the default `KUBE_TIMEOUT=180s`; the four CREATE tests are expected to complete in **under 5 seconds combined** because they perform no I/O and no goroutine startup beyond the workqueue exercised in the namespace `worker` test
- **Maintainability standards**: table-driven format matches existing repo convention (e.g., `pkg/controller/cronjob/cronjob_controllerv2_test.go`); error messages from `assert`/`require` include the row's `name` field; `cmp.Diff` is used in place of `reflect.DeepEqual` for non-trivial structural comparisons; the `_test.go` file's `package` declaration matches the production package (no `_test` external-test pattern unless mandatory)
- **Race correctness**: tests pass with `KUBE_RACE=-race` (default); no use of `time.Sleep`; no use of shared global state; goroutines started inside a test must be joined or canceled before the test returns to avoid `goleak` complaints in integration suites
- **Following repository conventions**: import groupings (std, third-party, k8s.io); copyright header; package-level test helpers in same `_test.go` file or sibling `*_test.go` files within the same package; logger via `ktesting.NewTestContext(t)` rather than `context.Background()`


## 0.8 Scope Boundaries

### 0.8.1 Exhaustively In Scope

- **New test files**:
  - `pkg/controller/util/node/controller_utils_test.go` (CREATE) — unit tests for 11 helper functions
  - `pkg/controller/util/endpointslice/errors_test.go` (CREATE) — unit tests for the `StaleInformerCache` sentinel
  - `pkg/controller/namespace/namespace_controller_test.go` (CREATE) — unit tests for the `NamespaceController`
  - `pkg/controller/replication/replication_controller_test.go` (CREATE) — unit tests for `ReplicationManager` and `conversion.go`
- **Test file updates** (incremental rows only; preserve every existing test):
  - `pkg/controller/cronjob/cronjob_controllerv2_test.go` (UPDATE) — finalizer + requeue cases
  - `pkg/controller/deployment/deployment_controller_test.go` (UPDATE) — status condition transition cases
  - `pkg/apis/apps/validation/validation_test.go` (UPDATE) — boundary values
  - `pkg/apis/core/validation/validation_test.go` (UPDATE) — boundary values
  - `staging/src/k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/validation/validation_test.go` (UPDATE) — CEL validation edge cases
- **Test configuration**: none — no changes required to `Makefile`, `hack/make-rules/test.sh`, `hack/make-rules/test-integration.sh`, or coverage configuration (the existing `KUBE_COVER` toggle is sufficient)
- **Test utilities and helpers**: reuse existing helpers only (no new helper package); applicable paths:
  - `pkg/controller/testutil/test_utils.go` (REFERENCE — reuse, do not modify)
  - `test/integration/framework/etcd.go`, `test/integration/framework/test_server.go` (REFERENCE — reuse, do not modify)
  - `test/utils/**/*` (REFERENCE — reuse, do not modify)
- **Documentation updates**:
  - `TESTING.md` (CREATE at repository root) — documents test tiers, `make` targets, `KUBE_*` flags, regression-proof drill, and the envtest substitution note (no `KUBEBUILDER_ASSETS` required)

### 0.8.2 Explicitly Out of Scope

- **Source code modifications**: any production `.go` file in `pkg/controller/`, `pkg/apis/`, `cmd/`, or `staging/src/k8s.io/apiextensions-apiserver/` is out of scope unless a trivial unexported-to-exported visibility change is strictly required for testability and explicitly specified by the user
- **`go.mod` and `go.sum`**: out of scope (no new dependencies are needed; modifying these counts as production-file changes)
- **`vendor/` tree**: out of scope for the same reason
- **`main.go`, `doc.go`, generated files** (`zz_generated*.go`, `*.pb.go`, `*conversion.go`, `*deepcopy.go`): out of scope — these are produced by code generators
- **Refactoring beyond what's needed for testing**: out of scope
- **Feature additions or behavior changes while adding tests**: out of scope
- **Unrelated test files not specified by user**: out of scope
- **Performance optimizations not related to test coverage**: out of scope
- **`sigs.k8s.io/controller-runtime` / `envtest`**: out of scope — would require `go.mod` and `vendor/` changes; the repo-native `test/integration/framework` provides equivalent capability
- **Out-of-scope source paths** (also out of scope per Tech Spec section 1.3): `vendor/`, `third_party/`, `staging/` build artifacts, `CHANGELOG/`, `logo/`
- **Other test tiers**: tests under `test/e2e/`, `test/e2e_node/`, `test/e2e_dra/`, `test/conformance/`, `test/fuzz/`, `test/soak/`, `test/kubemark/`, `test/typecheck/`, `test/list/`, `test/images/`, `test/fixtures/`, `test/compatibility_lifecycle/` — different tiers, governed separately, not in user's testing scope
- **All items explicitly excluded by user instructions**


## 0.9 Execution Parameters

### 0.9.1 Testing-Specific Instructions

The repository's canonical entry points are `make` targets. The Blitzy platform uses these targets (not custom `go test` invocations) to ensure that test runs honor the repository's defaults — race detection, cache-mutation detection, panic-on-watch-decode-error, timeout, and JUnit XML emission.

- **Run all unit tests on the new packages**:
  ```
  make test WHAT="./pkg/controller/util/node/... ./pkg/controller/util/endpointslice/... ./pkg/controller/namespace/... ./pkg/controller/replication/..."
  ```
- **Run unit tests on a single new test file's package** (fast iteration):
  ```
  make test WHAT=./pkg/controller/namespace/...
  ```
- **Coverage measurement command** (opt-in):
  ```
  KUBE_COVER=y make test WHAT="./pkg/controller/util/node/... ./pkg/controller/util/endpointslice/... ./pkg/controller/namespace/... ./pkg/controller/replication/..."
  ```
  Coverage artifacts are written under `${KUBE_TIMESTAMP}/coverage.out`; view with `go tool cover -html=coverage.out`.
- **Watch mode**: not applicable to this repository (Go does not ship a watch runner and the repo does not provide one); rapid iteration is achieved by re-invoking `make test WHAT=<pkg>` after edits.
- **Single test execution pattern** (run a specific table case):
  ```
  go test -run 'TestSyncNamespaceFromKey/^not_found$' ./pkg/controller/namespace/...
  ```
  Subtests are matched by `t.Run` name, and the `make test` script forwards `-run` via `KUBE_TEST_ARGS` if needed: `make test WHAT=./pkg/controller/namespace/... KUBE_TEST_ARGS='-run TestSyncNamespaceFromKey'`.
- **Debug mode execution** (verbose, no parallelism, no cache):
  ```
  go test -v -count=1 -run TestSyncNamespaceFromKey ./pkg/controller/namespace/...
  ```
- **Run integration tests**:
  ```
  make test-integration WHAT=./test/integration/<subject>/...
  ```
- **Verify (formatting, lint, generated-code freshness)**:
  ```
  make verify
  ```

**Repository-default environment variables** honored by every invocation through `hack/make-rules/test.sh`:

| Variable | Default | Purpose |
|----------|---------|---------|
| `KUBE_RACE` | `-race` | Enable Go race detector |
| `KUBE_CACHE_MUTATION_DETECTOR` | `true` | Detect informer-cache mutations at runtime |
| `KUBE_PANIC_WATCH_DECODE_ERROR` | `true` (unit) / `false` (integration) | Treat watch decode errors as panics |
| `KUBE_TIMEOUT` | `180s` (unit) / `600s` (integration) | Per-package test timeout |
| `KUBE_COVER` | unset (opt-in) | When `y`, enable `-coverprofile=` and emit coverage |
| `KUBE_VERBOSE` | unset | When set, pass `-v` to `go test` |
| `KUBE_JUNIT_REPORT_DIR` | derived from `ARTIFACTS` | Directory for JUnit XML output |
| `KUBE_TEST_ARGS` | empty | Extra `go test` args |
| `GOTOOLCHAIN` | `local` | Use installed Go 1.25.0; `.go-version` pins to 1.25.6 |

**Test patterns to follow in this repository**:

- Tests use `package <name>` (internal-test) by default, matching existing files like `pkg/controller/cronjob/cronjob_controllerv2_test.go`
- Logger and context are obtained via `_, ctx := ktesting.NewTestContext(t)`
- Fake clientset is `client := fake.NewSimpleClientset(initialObjects...)`; action assertions use `client.Actions()` which returns `[]core.Action` (where `core "k8s.io/client-go/testing"`)
- Informers are `factory := informers.NewSharedInformerFactory(client, 0)`; pre-populate via `factory.<group>().<version>().<resource>().Informer().GetIndexer().Add(...)`
- Event capture is `recorder := record.NewFakeRecorder(16)`; events arrive on `recorder.Events` channel
- Scheme registration (when needed for typed conversions): `import _ "k8s.io/kubernetes/pkg/apis/core/install"` and equivalent per-group install packages

**Excluded test categories per user instruction**: none excluded explicitly by the user beyond the production-file modification prohibition. Tests for e2e/conformance/fuzz/soak/kubemark/images/typecheck/list tiers are out of scope per the testing-scope boundaries.

**Environment setup requirements for tests**: Go 1.25.0 toolchain (matches `go.mod`; `.go-version` pins to 1.25.6). For integration tests, `etcd` 3.6.7 must be available on `PATH` or built from the in-tree vendored version; the `framework.EtcdMain` wrapper handles startup and teardown. No `KUBEBUILDER_ASSETS` is needed because the apiserver runs in-process.


## 0.10 Special Instructions for Testing

### 0.10.1 User-Specified Testing Directives (Verbatim)

The following directives from the user's prompt are preserved verbatim and applied to every file in this plan. The right-hand "Applied as" column documents the repository-native adaptation where the user's framework choice is not present in `k8s.io/kubernetes`.

| User Directive (verbatim) | Applied as |
|---------------------------|------------|
| "Only add test files (`*test.go`). Do not modify any production `.go` files unless a trivial unexported-to-exported visibility change is strictly required for testability." | Hard constraint. All new files are `*_test.go`. No production file is modified. `go.mod`, `go.sum`, and `vendor/` are treated as production files and are not modified. No exported-visibility change is required by this plan. |
| "Use controller-runtime's built-in `fake.NewClientBuilder()` for unit tests. No hand-rolled mocks — keep it idiomatic." | Adapted to repo-native equivalent: `k8s.io/client-go/kubernetes/fake.NewSimpleClientset(initialObjects...)`. The repository does not vendor `sigs.k8s.io/controller-runtime` (confirmed via `grep -c controller-runtime go.sum` → 0); the `client-go` fake is the idiomatic substitute used by every existing `pkg/controller/*` unit test. No hand-rolled mocks introduced. |
| "No `time.Sleep` in tests. Every test must be independently runnable (no shared mutable state)." | Hard constraint. New tests use `k8s.io/apimachinery/pkg/util/wait.PollUntilContextTimeout` for any required polling and `gomega.Eventually` in integration tests; never `time.Sleep`. Each `t.Run` constructs fresh fakes; no package-level mutable state introduced. |
| "Validation: Intentionally break one reconcile function (e.g., comment out a status update), confirm the corresponding test fails — proving tests actually catch regressions." | Applied to `syncNamespaceFromKey` as the canonical sanity check: temporarily replacing `return nm.namespacedResourcesDeleter.Delete(ctx, namespace.Name)` with `return nil` must cause `TestSyncNamespaceFromKey/exists_deleter_invoked` to fail. The drill is documented in `TESTING.md` so future contributors can repeat it on any reconcile loop. The break is **never committed**; it is a one-time validation step on the developer's local working copy. |
| "Add a `TESTING.md` that explains how to run each test tier, how to add new tests, and what envtest requires (e.g., `KUBEBUILDER_ASSETS`)." | A `TESTING.md` is created at the repository root. It documents `make test`, `make test-integration`, `make verify`, the `KUBE_*` flags, the regression-proof drill, **and explicitly notes the envtest substitution**: the repo uses `test/integration/framework` (real `etcd` + in-process `kube-apiserver`) rather than `controller-runtime envtest`, so `KUBEBUILDER_ASSETS` is **not** required. The `KUBEBUILDER_ASSETS` mechanism is mentioned for reader awareness when transferring patterns from a kubebuilder project. |

### 0.10.2 Additional Constraints Applied

- **Maintain backward compatibility in test utilities**: existing `pkg/controller/testutil/test_utils.go` API is used as-is; no signature changes
- **Match existing code style and naming conventions in tests**: import grouping, copyright header (Apache 2.0, year 2026), package declaration matches production package, table fields are `name`, `args`/`input`, `want`/`expected`, `wantErr`
- **Ensure all tests can run independently and in parallel**: where row-level parallelism is safe (no shared workqueue, no shared informer factory), `t.Parallel()` is called inside `t.Run`; otherwise tests remain serial to avoid contention on the shared fake clientset
- **Race-detector clean**: tests pass with `KUBE_RACE=-race` (default); no goroutine started inside a test outlives the test
- **Cache-mutation-detector clean**: tests pass with `KUBE_CACHE_MUTATION_DETECTOR=true` (default); test code never mutates objects retrieved from an informer indexer
- **Goroutine leak clean**: tests are compatible with `go.uber.org/goleak`-based assertions used by `test/integration/framework/goleak.go` if invoked at integration tier

### 0.10.3 Implementation Validation

After the test files are written, the following validation steps confirm conformance with the user's directives:

- `make test WHAT=./pkg/controller/util/node/... ./pkg/controller/util/endpointslice/... ./pkg/controller/namespace/... ./pkg/controller/replication/...` exits 0
- `git diff --stat` shows changes only to `*_test.go` files and `TESTING.md` (no production `.go` file changes; no `go.mod`/`go.sum`/`vendor/` changes)
- `grep -nR "time.Sleep" pkg/controller/util/node pkg/controller/util/endpointslice pkg/controller/namespace pkg/controller/replication --include='*_test.go'` returns empty
- `KUBE_COVER=y make test WHAT=./pkg/controller/util/node/...` reports ≥ 70% line coverage for `pkg/controller/util/node`
- The regression-proof drill described in 0.10.1 is verified by toggling a single line in `syncNamespaceFromKey` and observing that `TestSyncNamespaceFromKey/exists_deleter_invoked` fails; the toggle is reverted before commit


