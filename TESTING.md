# Testing in Kubernetes

This document explains the test tiers used in the `k8s.io/kubernetes` monorepo,
how to run each tier, how to add new tests, and—importantly for contributors
arriving from a kubebuilder / operator-SDK background—how this repository
substitutes the repo-native `test/integration/framework/` harness for
`sigs.k8s.io/controller-runtime/pkg/envtest`. Because that substitution boots an
**in-process** `kube-apiserver`, **no `KUBEBUILDER_ASSETS` is required** (see
[Section 5.6](#56-substitution-for-controller-runtime-envtest-important)).

## 1. Overview

Kubernetes ships several test tiers, each with its own runner and intent:

- **Unit** — fast, isolated, in-package tests (`make test`). No etcd, no apiserver.
- **Integration** — component-interaction tests against a real `etcd` and an
  in-process `kube-apiserver` (`make test-integration`).
- **Conformance** — a curated, portable subset of e2e tests used to certify a cluster.
- **e2e (cluster)** — end-to-end tests run with Ginkgo against a live cluster.
- **e2e_node** — kubelet-focused node end-to-end tests.
- **Fuzz** — Go native fuzzing of serializers and decoders.
- **Soak** — long-running workloads that exercise stability over time.
- **Kubemark** — large-scale simulation using hollow nodes.

This document focuses on the **Unit** and **Integration** tiers, which are the
tiers exercised by the baseline-coverage testing initiative and the tiers most
contributors interact with daily. The deeper tiers (e2e, conformance, fuzz,
soak, kubemark, and others) are governed by their own runners under
`test/<tier>/` and by the
[SIG-Testing contributor docs](https://git.k8s.io/community/contributors/devel/sig-testing/);
they are summarized only briefly in [Section 10](#10-other-test-tiers-brief).

## 2. Prerequisites

- **Go toolchain**: `1.25.0` (matches the `go` directive in `go.mod`). The
  `.go-version` file pins `1.25.6` for tools that honor it (such as `gimme` or
  `goenv`). The `.go-version` value is a hint for those tools; any Go `>= 1.25.0`
  works for building and testing.
- **`make`, `bash`, `git`** — the canonical entry points are `make` targets that
  delegate to `bash` scripts under `hack/make-rules/`.
- **`jq`** — required by `hack/make-rules/test.sh` (it calls
  `kube::util::require-jq`). Install via your package manager (for example,
  `apt-get install -y jq`).
- **For the integration tier: `etcd` `3.6.7` on `PATH`.** If it is missing,
  install the repo-pinned version (the version is determined by
  `hack/install-etcd.sh`, which is the source of truth):

  ```bash
  hack/install-etcd.sh   # installs etcd into third_party/etcd
  export PATH="$(pwd)/third_party/etcd:${PATH}"
  ```

- **`gotestsum`** is auto-installed by `hack/make-rules/test.sh` from
  `hack/tools` if it is not already on `PATH` (a one-time `go install`).
- **`prune-junit-xml`** is auto-installed from `cmd/prune-junit-xml` if it is not
  already on `PATH`.
- **A C toolchain (gcc/g++)** is required when the race detector is enabled
  (the default for unit tests), because `-race` builds with CGO.

## 3. Quick Start

```bash
# Run all unit tests (everything, slow):
make test

# Run unit tests for a single package:
make test WHAT=./pkg/controller/namespace/...

# Run unit tests for multiple packages:
make test WHAT="./pkg/controller/util/node/... ./pkg/controller/util/endpointslice/... ./pkg/controller/namespace/... ./pkg/controller/replication/..."

# Run unit tests with coverage:
KUBE_COVER=y make test WHAT=./pkg/controller/namespace/...

# Run integration tests for a single subject:
make test-integration WHAT=./test/integration/namespace/...

# Run the full presubmit verification suite:
make verify

# Run only fast presubmit verifications:
make quick-verify

# Refresh generated files / vendor / formatting:
make update
```

## 4. Unit Tests (`make test`)

### 4.1 Invocation

`make test` (and its alias `make check`) delegates to
`hack/make-rules/test.sh $(WHAT) $(TESTS)`. The two targets are identical—`check`
and `test` share a single rule in `build/root/Makefile`.

### 4.2 Arguments

- `WHAT` — a space-separated list of package patterns (for example,
  `./pkg/controller/namespace/...`). If unset, the script discovers all
  unit-test packages via `kube::test::find_go_packages`, which excludes the
  following non-unit paths so that tier separation is enforced automatically:
  `third_party/`, `cmd/kubeadm/test/`, `test/e2e`, `test/e2e_dra`,
  `test/e2e_node`, `test/e2e_kubeadm`, and any `*/test/integration/...`.
- `TESTS` — same semantics as `WHAT` (a legacy alias; both are passed through).
- `GOFLAGS` — extra flags forwarded to `go` when building/testing (for example,
  `GOFLAGS="-v -count=1"`).
- `GOLDFLAGS`, `GOGCFLAGS` — extra linker / compile flags forwarded to `go`.
- `PRINT_HELP=y` — print the in-tree help for the target instead of running it
  (for example, `make test PRINT_HELP=y`).

### 4.3 Environment Variables (with exact defaults)

These defaults are read directly from `hack/make-rules/test.sh`:

| Variable | Default | Purpose |
| --- | --- | --- |
| `KUBE_RACE` | `-race` | Race-detector flag for `go test`. Set to the **empty string** (`KUBE_RACE=""`) to disable it. Note: the script uses `${KUBE_RACE-"-race"}` (no colon), so an explicitly empty value is meaningful and is **not** replaced by the default. |
| `KUBE_CACHE_MUTATION_DETECTOR` | `true` | When `true`, mutating an object obtained from a shared informer cache panics at runtime, catching a common class of bugs early. |
| `KUBE_PANIC_WATCH_DECODE_ERROR` | `true` | When `true`, the server panics on watch decode errors (treated as coder mistakes in the unit tier). |
| `KUBE_TIMEOUT` | `-timeout=180s` | Per-package `go test` timeout. |
| `KUBE_COVER` | `n` | Set to `y` (or `Y`) to enable coverage collection (`-cover -covermode=<mode> -coverprofile=<dir>/combined-coverage.out`). |
| `KUBE_COVERMODE` | `atomic` | Go coverage mode. |
| `KUBE_COVER_REPORT_DIR` | (auto) | Directory for coverage artifacts. When unset, defaults to `/tmp/k8s_coverage/<sortable-date>`. |
| `KUBE_GOVERALLS_BIN` | (unset) | Path to the `goveralls` binary; when set (and executable) with coverage enabled, results are also reported to Coveralls.io. |
| `KUBE_JUNIT_REPORT_DIR` | (auto) | Directory for JUnit XML output. When unset and `ARTIFACTS` is set, it is matched to `${ARTIFACTS}`; otherwise no JUnit XML is emitted. |
| `KUBE_KEEP_VERBOSE_TEST_OUTPUT` | `n` | Set to `y` to retain the raw JSON `gotestsum` output alongside the JUnit XML (requires `KUBE_JUNIT_REPORT_DIR`). |
| `KUBE_PRUNE_JUNIT_TESTS` | `true` | When `true`, `prune-junit-xml` reduces the report to top-level tests. |
| `KUBE_TEST_ARGS` | (empty) | Extra arguments for the `go test` invocation. The value is `eval`'d, so embedded quoting is preserved. |
| `FULL_LOG` | (unset) | When set to any value, `gotestsum` uses the `standard-verbose` format instead of the default `pkgname-and-test-fails`. |
| `PARALLEL` | `-1` | The `-p` value forwarded to `go test`; values greater than `0` set the parallelism explicitly. |
| `GOTOOLCHAIN` | `local` | Forces the locally installed Go toolchain. Use `auto` to allow Go to fetch a matching toolchain on demand. |

Internally the script runs tests through
`gotestsum --raw-command -- go test -json`, which streams JSON for both
human-readable output and JUnit XML emission. Contributors generally do not need
to interact with this pipeline directly.

### 4.4 Running a Single Test Function or Subtest

```bash
# Whole TestXxx function:
make test WHAT=./pkg/controller/namespace/... KUBE_TEST_ARGS='-run TestSyncNamespaceFromKey'

# Specific table-driven subtest by name:
make test WHAT=./pkg/controller/namespace/... \
    KUBE_TEST_ARGS='-run TestSyncNamespaceFromKey/^exists_deleter_invoked$'

# Or invoke go test directly for fast iteration:
go test -run TestSyncNamespaceFromKey ./pkg/controller/namespace/...
go test -v -count=1 -run TestSyncNamespaceFromKey ./pkg/controller/namespace/...
```

### 4.5 Coverage Measurement

```bash
KUBE_COVER=y make test WHAT="./pkg/controller/util/node/... ./pkg/controller/util/endpointslice/... ./pkg/controller/namespace/... ./pkg/controller/replication/..."
```

After completion, the script prints a path similar to:

```text
Combined coverage report: /tmp/k8s_coverage/2026-01-02T12-34-56-0700/combined-coverage.html
```

Open the HTML report in a browser, or inspect the raw profile:

```bash
go tool cover -html=/tmp/k8s_coverage/DATE/combined-coverage.out
go tool cover -func=/tmp/k8s_coverage/DATE/combined-coverage.out | tail
```

(Substitute `DATE` with the sortable-date directory printed by the run, or set
`KUBE_COVER_REPORT_DIR` to a known path.)

The repository does **not** enforce a global coverage threshold; race detection
and cache-mutation detection serve as the primary correctness gates, with
`KUBE_COVER=y` as an opt-in measurement flag. The per-package targets for the
baseline-coverage initiative are:

- `pkg/controller/util/node`: `>= 70%` line coverage, plus 100% on the
  `DeletePods`, `MarkPodsNotReady`, and `GetNodeCondition` happy paths.
- `pkg/controller/util/endpointslice`: 100% line coverage (a small sentinel type).
- `pkg/controller/namespace`: `>= 70%` line coverage, plus 100% on the
  `syncNamespaceFromKey` happy path, NotFound branch, and deleter-error branch.
- `pkg/controller/replication`: `>= 70%` line coverage, plus 100% on the
  `NewReplicationManager` constructor and the conversion round-trips.

## 5. Integration Tests (`make test-integration`)

### 5.1 Invocation

`make test-integration` delegates to `hack/make-rules/test-integration.sh $(WHAT)`.
After starting a real `etcd` subprocess, the runner delegates the actual test
execution to the unit-tier `make test` machinery (via `make -C <repo-root> test`),
which is why several environment-variable defaults below intentionally differ
from the unit tier.

### 5.2 Arguments

- `WHAT` — a package pattern under `test/integration/<subject>/...` (for
  example, `./test/integration/deployment/...`). If unset, the script discovers
  all integration packages via `kube::test::find_integration_test_pkgs`.
- `KUBE_TEST_ARGS` — forwarded to the underlying `go test` invocation (for
  example, `-run` filters). The Makefile passes this through with single quotes
  to preserve embedded dollar signs.

### 5.3 Environment Variables (with exact defaults)

These defaults are read directly from `hack/make-rules/test-integration.sh`:

| Variable | Default | Purpose |
| --- | --- | --- |
| `KUBE_TIMEOUT` | `-timeout=600s` | Per-package timeout (10 minutes, versus 180s for the unit tier). |
| `KUBE_RACE` | `""` (empty) | Race detector is **off** by default for integration (the runner exports an empty value to avoid the slowdown when delegating to `make test`). Set `KUBE_RACE=-race` to enable it. |
| `KUBE_PANIC_WATCH_DECODE_ERROR` | `false` | Integration tests intentionally insert undecodable data; disabling the panic gate avoids spurious failures. |
| `KUBE_CACHE_MUTATION_DETECTOR` | `true` | Same as the unit tier; cache-mutation detection stays on. |
| `KUBE_INTEGRATION_TEST_MAX_CONCURRENCY` | `-1` | When greater than `0`, sets `GOMAXPROCS` to this value to cap parallelism. |
| `LOG_LEVEL` | `2` | klog verbosity for the in-process apiserver and controllers. |
| `KUBE_TEST_VMODULE` | (empty) | Per-module klog verbosity (for example, `KUBE_TEST_VMODULE="namespace_controller=4"`). |
| `SHORT` | `--short=true` | Forwarded as `-short` to `go test`. Set `SHORT=` (empty) to run long tests. |
| `KUBE_TEST_ARGS` | (empty) | Extra `go test` arguments. |

### 5.4 etcd Prerequisite

The runner calls `checkEtcdOnPath` and aborts if `etcd` is not on `PATH`. Install
the repo-pinned version with:

```bash
hack/install-etcd.sh
export PATH="$(pwd)/third_party/etcd:${PATH}"
```

The runner starts a real `etcd` subprocess (`kube::etcd::start`) and cleans it up
via `trap cleanup EXIT`. There is **no** persistent state carried between
integration runs.

### 5.5 Integration Harness — `test/integration/framework/`

Every integration suite under `test/integration/<subject>/` follows this
pattern. The `main_test.go` boots `etcd` for the lifetime of the test binary:

```go
// test/integration/<subject>/main_test.go
package subject

import (
	"testing"

	"k8s.io/kubernetes/test/integration/framework"
)

func TestMain(m *testing.M) {
	framework.EtcdMain(m.Run)
}
```

A test that needs a live apiserver obtains a typed `client.Interface` and a
`*rest.Config` via `StartTestServer`, and always tears it down with `defer`:

```go
import (
	"context"
	"testing"

	"k8s.io/kubernetes/test/integration/framework"
)

func TestSomething(t *testing.T) {
	ctx := context.Background()
	client, _, tearDown := framework.StartTestServer(ctx, t, framework.TestServerSetup{})
	defer tearDown()
	// ... call client.CoreV1().Namespaces().Create(ctx, ...) etc.
}
```

The exported entry points in `test/integration/framework/` include:

- `EtcdMain(tests func() int)` — boots a real `etcd` subprocess for the test
  binary lifecycle (in `etcd.go`).
- `StartTestServer(ctx context.Context, t testing.TB, setup TestServerSetup) (client.Interface, *rest.Config, TearDownFunc)`
  — boots an in-process `kube-apiserver` with ephemeral TLS (in `test_server.go`).
- `TestServerSetup` — a struct with optional `ModifyServerRunOptions` and
  `ModifyServerConfig` hooks.
- `TearDownFunc` — the `func()` returned by `StartTestServer`; call it via `defer`
  to stop the apiserver.
- `IgnoreBackgroundGoroutines() []goleak.Option` and
  `GoleakCheck(tb testing.TB, opts ...goleak.Option)` — goroutine-leak detection
  helpers (in `goleak.go`).

### 5.6 Substitution for `controller-runtime` envtest (IMPORTANT)

If you arrive here from a kubebuilder / operator-SDK project, you may expect
tests to use `sigs.k8s.io/controller-runtime/pkg/envtest` and to require
`KUBEBUILDER_ASSETS` pointing at `etcd` and `kube-apiserver` binaries. In
`k8s.io/kubernetes`:

- The repository does **NOT** vendor `sigs.k8s.io/controller-runtime` (verify with
  `grep -c controller-runtime go.sum`, which returns `0`).
- Instead, `test/integration/framework/` provides the equivalent capability
  natively: it boots a real `etcd` subprocess and an **in-process**
  `kube-apiserver` built from the repository's own source. The apiserver is
  compiled into the test binary itself.
- Because the apiserver is in-process, **no `KUBEBUILDER_ASSETS` environment
  variable is needed**. Only `etcd` on `PATH` is required.

| `controller-runtime` envtest pattern | `k8s.io/kubernetes` equivalent |
| --- | --- |
| `envtest.Environment{}.Start()` | `framework.StartTestServer(ctx, t, framework.TestServerSetup{})` |
| `envtest.Environment{}.Stop()` | `tearDown()` returned from `StartTestServer` (call via `defer`) |
| `KUBEBUILDER_ASSETS=/path/to/bin` | not required (apiserver is in-process; `etcd` must be on `PATH`) |
| `setup-envtest use <go-version>` | not required (no external binary download) |
| `go.uber.org/goleak` integration | `framework.GoleakCheck(t, framework.IgnoreBackgroundGoroutines()...)` |

The substitution is intentional and not a compromise: it removes the dependency
on a Go-version-pinned `setup-envtest` toolchain and eliminates the
`KUBEBUILDER_ASSETS` configuration surface entirely.

### 5.7 Async Assertions in Integration Tests

Use `k8s.io/apimachinery/pkg/util/wait.PollUntilContextTimeout` or the
`github.com/onsi/gomega` `Eventually` matcher for async assertions. **Never** use
`time.Sleep`. Example:

```go
import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

err := wait.PollUntilContextTimeout(ctx, 100*time.Millisecond, 30*time.Second, true, func(ctx context.Context) (bool, error) {
	list, err := client.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return false, err
	}
	return len(list.Items) == 3, nil
})
```


## 6. Adding a New Test

### 6.1 Unit Test

- **Location**: alongside the production source file, in the same Go package,
  named `<filename>_test.go` (for example,
  `pkg/controller/namespace/namespace_controller_test.go` for
  `namespace_controller.go`).
- **Package declaration**: match the production package (avoid the `_test`
  external-test variant unless it is strictly required).
- **License header**: Apache 2.0 with the current year.
- **Imports**: group `stdlib`, then external (`github.com/...`), then `k8s.io/...`,
  with a blank line between groups. Add `_ "k8s.io/kubernetes/pkg/apis/core/install"`
  if you need to decode typed core API objects.
- **Structure**: table-driven. Each row carries a unique `name`:

```go
func TestSyncNamespaceFromKey(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{name: "not_found", key: "ns-missing"},
		{name: "exists_deleter_invoked", key: "ns-1"},
		{name: "deleter_error", key: "ns-2", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ctx := ktesting.NewTestContext(t)
			client := fake.NewSimpleClientset(/* per-row objects */)
			_ = ctx
			_ = client
			// ... construct the controller, invoke, and assert
		})
	}
}
```

- **Context and logger**: always derive a context via
  `_, ctx := ktesting.NewTestContext(t)` (or
  `logger, ctx := ktesting.NewTestContext(t)` when you need the logger). Do not
  use `context.Background()` in unit tests.
- **Fake clientset**: use
  `k8s.io/client-go/kubernetes/fake.NewSimpleClientset(initialObjects...)`. Inject
  errors via
  `client.PrependReactor("verb", "resource", func(...) (handled bool, ret runtime.Object, err error) { ... })`.
- **Events**: use `record.NewFakeRecorder(capacity)`; emitted events arrive on the
  `recorder.Events` channel.
- **Informers**: `factory := informers.NewSharedInformerFactory(client, 0)`;
  pre-populate listers without starting the informer via
  `factory.<Group>().<Version>().<Resource>().Informer().GetIndexer().Add(...)`.
- **Independence**: construct fresh fakes and listers inside each `t.Run` so that
  every test is independently runnable with no shared mutable state.

### 6.2 Integration Test

- **Location**: under `test/integration/<subject>/<filename>_test.go`.
- **Bootstrap**: each subject folder needs a `main_test.go` calling
  `framework.EtcdMain(m.Run)`.
- **Server**: obtain `*rest.Config` and a `client.Interface` via
  `framework.StartTestServer(ctx, t, framework.TestServerSetup{})`; always
  `defer tearDown()`.
- **Async assertions**: use `wait.PollUntilContextTimeout` or `gomega.Eventually`.
  **No `time.Sleep`.**
- **Logger and context**: prefer `_, ctx := ktesting.NewTestContext(t)` and pass
  the context everywhere.

### 6.3 Test Helpers

Existing helpers SHOULD be reused; do not duplicate them:

- `pkg/controller/testutil/test_utils.go` — `NewNode(name)`, `NewPod(name, host)`,
  `NewFakeRecorder()`, the `FakeNodeHandler` clientset stub, `GetKey(obj, t)`,
  `GetZones`, and `CreateZoneID`.
- `test/integration/framework/` — see [Section 5.5](#55-integration-harness--testintegrationframework).
- `test/utils/` — broader cross-tier helpers (for example, `audit/`, `image/`, and
  polling helpers).

A new helper that is only useful to one package should live in an `*_test.go` file
in that package, **not** in a new top-level `internal/testutil/` (which does not
exist in this repository).

## 7. Mocking Conventions

Use **only** repo-native facilities — no hand-rolled mocks and no external mocking
libraries.

| Concern | Library / Helper | Example |
| --- | --- | --- |
| Kubernetes clientset | `k8s.io/client-go/kubernetes/fake.NewSimpleClientset(...)` | `client := fake.NewSimpleClientset(&v1.Namespace{...})` |
| Action assertions | `k8s.io/client-go/testing` typed actions | `for _, a := range client.Actions() { switch a.GetVerb() { /* ... */ } }` |
| Error injection | `client.PrependReactor(...)` | `client.PrependReactor("update", "nodes", func(a clienttesting.Action) (bool, runtime.Object, error) { return true, nil, errors.New("conflict") })` |
| Event capture | `k8s.io/client-go/tools/record.NewFakeRecorder(N)` | `recorder := record.NewFakeRecorder(16); evt := <-recorder.Events` |
| Logger / context | `k8s.io/klog/v2/ktesting.NewTestContext(t)` | `_, ctx := ktesting.NewTestContext(t)` |
| Informer factory | `k8s.io/client-go/informers.NewSharedInformerFactory(client, resync)` | `factory := informers.NewSharedInformerFactory(client, 0)` |
| Pre-populate lister | `Informer().GetIndexer().Add(obj)` | `factory.Core().V1().Namespaces().Informer().GetIndexer().Add(&v1.Namespace{...})` |
| Deep comparisons | `github.com/google/go-cmp/cmp.Diff` | `if d := cmp.Diff(want, got); d != "" { t.Errorf("mismatch (-want +got):\n%s", d) }` |
| Goroutine leaks | `go.uber.org/goleak` (and `framework.GoleakCheck`) | `goleak.VerifyTestMain(m)` |


## 8. Regression-Proof Drill

A test that does not fail when the code is broken is worse than no test at all.
To **prove** that a test catches regressions, perform this manual drill on your
local working copy:

1. Choose a reconcile function that has a covering test. The canonical target is
   `syncNamespaceFromKey` in `pkg/controller/namespace/namespace_controller.go`,
   covered by `TestSyncNamespaceFromKey/exists_deleter_invoked` in
   `pkg/controller/namespace/namespace_controller_test.go`.
2. Open the production file in your editor.
3. Temporarily break the behavior — for example, replace the deleter call
   `return nm.namespacedResourcesDeleter.Delete(ctx, namespace.Name)` with
   `return nil`.
4. Run the covering subtest:

   ```bash
   make test WHAT=./pkg/controller/namespace/... KUBE_TEST_ARGS='-run TestSyncNamespaceFromKey/exists_deleter_invoked'
   ```

5. The test **MUST** fail (typically because the expected deleter action no longer
   appears in `client.Actions()`).
6. **Revert** the production change immediately. The break is never committed; it
   is a one-time validation step on your local working copy.

If the test passes despite the intentional break, the test is **not** asserting
what it should — fix the assertion before considering the coverage complete. Repeat
this drill on any new reconcile function before declaring its tests adequate.

## 9. Verification (`make verify`, `make quick-verify`)

`make verify` runs the full presubmit suite (`hack/make-rules/verify.sh`): code
formatting (`gofmt`, `goimports`), generated-code freshness
(`zz_generated*.go`, OpenAPI, protobuf, deepcopy, conversion), linting, vendor
consistency, license headers, and more. It is what CI runs before merge.

```bash
make verify                         # full suite
make verify WHAT="gofmt typecheck"  # specific verifiers only
make quick-verify                   # only the fast checks (skips slow generated-code regeneration)
make update                         # apply auto-fixable updates (re-generate code, re-format, etc.)
```

## 10. Other Test Tiers (Brief)

These tiers are out of scope for `make test` / `make test-integration` and have
their own runners. They are summarized here for orientation only.

| Tier | Location | Runner |
| --- | --- | --- |
| e2e (cluster) | `test/e2e/` | `ginkgo ./test/e2e -- ...` against an external cluster |
| e2e_node | `test/e2e_node/` | `make test-e2e-node` (kubelet-focused) |
| e2e_dra | `test/e2e_dra/` | `test/e2e_dra/run.sh` |
| e2e_kubeadm | `test/e2e_kubeadm/` | `ginkgo ./test/e2e_kubeadm -- ...` |
| Conformance | `test/conformance/` | `test/conformance/walk.go` plus a ginkgo dry-run; verified against golden files under `test/conformance/testdata/` |
| Fuzz | `test/fuzz/` | `go test -fuzz=Fuzz<Name> ./test/fuzz/<format>/...` |
| Soak | `test/soak/` | `cd test/soak/serve_hostnames && make` (long-running workload) |
| Kubemark | `test/kubemark/` | provider-scripted bring-up; large-scale simulation |
| kubectl CLI | `test/cmd/` | `make test-cmd` (sources `test/cmd/legacy-script.sh`; bash-based) |
| Static type-check | `test/typecheck/` | `go test ./test/typecheck/...` |
| Test inventory | `test/list/` | `go run test/list/main.go` |
| Stable metrics | `test/instrumentation/` | included as a verifier in `make verify` |
| Fixtures | `test/fixtures/` | data-only; consumed via `//go:embed` |

For full details on these tiers, see the
[SIG-Testing contributor docs](https://git.k8s.io/community/contributors/devel/sig-testing/).

## 11. Troubleshooting

- **`etcd` not found** during `make test-integration` → run `hack/install-etcd.sh`
  and add `third_party/etcd` to `PATH`.
- **`ulimit -n` warning** during `make test` → the script warns when the open-file
  limit is below 1000; raise it with `ulimit -n 4096` (some unit tests open many
  sockets).
- **Stale generated code** → run `make update` (or `make verify` will report what
  needs regenerating).
- **Coverage report path** → printed at the end of `KUBE_COVER=y make test`; or set
  `KUBE_COVER_REPORT_DIR` (it otherwise defaults to `/tmp/k8s_coverage/<date>`).
- **JUnit report path** → controlled by `KUBE_JUNIT_REPORT_DIR` (defaults to
  `${ARTIFACTS}` when that is set; otherwise no JUnit XML is emitted).
- **Race-detector failure** in a new test → ensure any goroutine started inside a
  `t.Run` is joined or canceled before the test returns.
- **`KUBE_CACHE_MUTATION_DETECTOR` panic** → your code mutated an object retrieved
  from a shared informer cache; copy it via `obj.DeepCopy()` before mutating.

## 12. Conventions Checklist for New Tests

Before merging a new `*_test.go`, confirm:

- [ ] License header (Apache 2.0) is present with the current year.
- [ ] The `package` declaration matches the production package.
- [ ] Imports are grouped (stdlib, external, `k8s.io`) with blank lines between groups.
- [ ] The test is table-driven where multiple cases apply; each row has a unique `name`.
- [ ] Each `t.Run` constructs its own `fake.NewSimpleClientset()` and other fakes (no shared mutable state).
- [ ] Context and logger come from `ktesting.NewTestContext(t)`, not `context.Background()`.
- [ ] No `time.Sleep` anywhere. Use `wait.PollUntilContextTimeout` or `gomega.Eventually` if polling is required.
- [ ] The race detector and cache-mutation detector pass (`KUBE_RACE=-race KUBE_CACHE_MUTATION_DETECTOR=true make test WHAT=<pkg>`).
- [ ] No new dependencies were introduced (no `go.mod`, `go.sum`, or `vendor/` changes for test-only additions).
- [ ] The regression-proof drill ([Section 8](#8-regression-proof-drill)) has been performed if the test covers a reconcile loop.

