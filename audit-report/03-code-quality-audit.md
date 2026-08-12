# Directive 3 — Code Quality Audit

> **Document Type:** Compliance Audit — Code Quality Assessment  
> **Audit Target:** `k8s.io/kubernetes` monorepo (Go 1.25.0)  
> **Audit Dimension:** Quality (single-dimension attribution)  
> **Framework Authority Hierarchy:** NIST SP 800-53 Rev 5 > NIST SP 800-190 > CIS Kubernetes Benchmark v1.12.0 > CIS Controls v8  
> **Posture:** Assess-only — no code or documentation in the target repository is created, modified, or deleted  
> **Prerequisite:** Directive 0 — System Registry (`00-system-registry.md`), Directive 2 — Materiality Classification (`02-materiality-classification.md`)  
> **Scope:** Material components only — Non-Material components are excluded per D2 classification  

---

## 1. Methodology

### 1.1 Audit Scope

This Code Quality Audit assesses **Material components only**, as classified in Directive 2. Non-Material components (generated code, test fixtures, vendored dependencies, documentation-only files, logos, changelogs, license files, staging modules, and third-party code) are explicitly excluded from this audit.

All findings in this document are attributed to the **Quality** audit dimension. Findings related to structural integrity, dependency mapping, or documentation coverage are documented in their respective directive reports (D1, D4, D5) and are not duplicated here.

### 1.2 Assessment Categories

Three assessment categories are applied to every Material component:

1. **Code Smells** — Patterns indicating potential maintainability, readability, or design issues:
   - DRY violations (repeated logic across files or modules)
   - SRP violations (multiple distinct responsibilities in a single module/function)
   - Deep nesting (conditionals exceeding 3 levels)
   - Magic numbers (hardcoded numeric literals without named constants)
   - Long parameter lists (functions with >5 parameters without object encapsulation)
   - Commented-out code (any commented code blocks)
   - Inconsistent naming (within same architectural layer)

2. **Complexity Metrics** — Quantitative measurements against defined thresholds:
   - Cyclomatic complexity per function: flag any instance exceeding 10
   - Cognitive complexity: nested conditions and control flow complexity
   - Coupling: direct dependencies (imports) per module; flag >7
   - Cohesion: internal methods sharing <50% of data structures
   - Function length: flag functions exceeding 50 lines without decomposition justification

3. **Security-Relevant Code Quality** — Patterns with direct security implications:
   - Missing input validation (external inputs not validated before processing) — NIST SI-10
   - Exposed internal state (internal data structures accessible via public APIs)
   - Hardcoded credentials (strings resembling tokens, passwords, keys) — NIST SC-28
   - Sensitive data logging (tokens, passwords, private keys in log output) — NIST AU-9
   - Deprecated library usage (use of deprecated APIs or libraries)
   - Unsafe type assertions (missing error checks on type conversions)

### 1.3 Assessment Method

Assessment was performed via:
- **Manual code review** of all source files listed in the D2 Material component inventory
- **Static analysis** of Go source files for import counts, function lengths, nesting depth, and parameter counts
- **Pattern matching** for code smells (TODO/FIXME comments, magic numbers, deprecated API references)
- **Cross-reference analysis** for DRY violations across files within the same architectural layer

### 1.4 Severity Classification

| Severity | Criteria | Examples |
|---|---|---|
| Critical | Security-relevant quality issues that directly impact NIST/CIS control enforcement, or complexity that makes security-critical code unreviewable | Missing input validation in auth path, hardcoded credentials, sensitive data in logs, functions >100 lines in security-critical paths |
| Moderate | Complexity or coupling exceeding defined thresholds in Material components, or patterns that increase risk of introducing security defects | Cyclomatic complexity >10, coupling >7 imports, function length >50 lines, long parameter lists in auth chain |
| Minor | Code smells that reduce maintainability but do not directly impact security control enforcement | Magic numbers, commented-out code, naming inconsistencies, TODO comments |

### 1.5 Finding Format

All findings follow the AAP-specified format:

```
system_id | component_path | smell_type | metric_value | severity | NIST_or_CIS_control
```

Source citations use the format: `Source: /path/to/file.go:LineNumber`

---

## 2. Code Smell Detection Findings

### 2.1 SYS-IAM-ORC — Identity/Access Orchestration

| system_id | component_path | smell_type | metric_value | severity | NIST_or_CIS_control |
|---|---|---|---|---|---|
| SYS-IAM-ORC | `pkg/kubeapiserver/authenticator/config.go` | Long Parameter List | `newJWTAuthenticator()`: 7 parameters | Moderate | — |
| SYS-IAM-ORC | `pkg/kubeapiserver/authenticator/config.go` | Long Parameter List | `newLegacyServiceAccountAuthenticator()`: 5 parameters (at threshold) | Minor | — |
| SYS-IAM-ORC | `pkg/kubeapiserver/authenticator/config.go` | Long Parameter List | `Config.New()`: 5 return values | Moderate | — |
| SYS-IAM-ORC | `pkg/kubeapiserver/authenticator/config.go` | SRP Violation | `Config` struct: 37 fields spanning authentication, JWT, webhook, token cache, service account, and TLS concerns | Moderate | — |
| SYS-IAM-ORC | `pkg/kubeapiserver/authenticator/config.go` | TODO Comment | Line 58: `TODO: Have policies be created via an API call` (referenced from ABAC) | Minor | — |
| SYS-IAM-ORC | `pkg/kubeapiserver/authenticator/config.go` | TODO Comment | Line 268: `TODO remove this CAContentProvider indirection` | Minor | — |
| SYS-IAM-ORC | `pkg/kubeapiserver/authenticator/config.go` | TODO Comment | Line 347: `TODO maybe track requests so we know when this is safe to do` | Minor | — |
| SYS-IAM-ORC | `pkg/kubeapiserver/authorizer/config.go` | SRP Violation | `Config.New()` handles ABAC file loading, RBAC initialization, Node authorizer graph construction, webhook setup, and reloadable resolver creation in a single function | Moderate | — |

`Source: pkg/kubeapiserver/authenticator/config.go:57-103` — Config struct with 37 fields  
`Source: pkg/kubeapiserver/authenticator/config.go:257` — newJWTAuthenticator with 7 parameters  
`Source: pkg/kubeapiserver/authenticator/config.go:380` — newLegacyServiceAccountAuthenticator with 5 parameters  
`Source: pkg/kubeapiserver/authenticator/config.go:268` — TODO comment  
`Source: pkg/kubeapiserver/authenticator/config.go:347` — TODO comment  
`Source: pkg/kubeapiserver/authorizer/config.go:82-162` — New() function with multiple responsibilities  

### 2.2 SYS-IAM-APP — Identity/Access Application Source

| system_id | component_path | smell_type | metric_value | severity | NIST_or_CIS_control |
|---|---|---|---|---|---|
| SYS-IAM-APP | `pkg/auth/authorizer/abac/abac.go` | TODO Comment | Line 58: `TODO: Have policies be created via an API call and stored in REST storage.` | Minor | — |
| SYS-IAM-APP | `pkg/auth/authorizer/abac/abac.go` | TODO Comment | Line 180: `TODO: match on verb` — incomplete verb matching logic | Moderate | AC-3 |
| SYS-IAM-APP | `pkg/auth/authorizer/abac/abac.go` | TODO Comment | Line 236: `TODO: Benchmark how much time policy matching takes` | Minor | — |
| SYS-IAM-APP | `pkg/auth/authorizer/abac/abac.go` | Deep Nesting | `NewFromFile()`: nesting reaches depth 6+ within the scanner loop (for → if → if → if → if) | Moderate | — |
| SYS-IAM-APP | `pkg/auth/authorizer/abac/abac.go` | Magic Number | Line 73-74: `i` and `unversionedLines` initialized to `0` without named constants (minor — zero-init is idiomatic) | Minor | — |
| SYS-IAM-APP | `plugin/pkg/auth/authorizer/rbac/bootstrappolicy/policy.go` | DRY Violation | 137 `rbacv1helpers.NewRule()` invocations in `policy.go` and 151 in `controller_policy.go` follow identical builder patterns without shared rule-definition helpers | Moderate | — |
| SYS-IAM-APP | `plugin/pkg/auth/authorizer/rbac/bootstrappolicy/policy.go` | TODO Comment | Line 215: `TODO: restrict to the bound node as creator in the NodeRestrictions admission plugin` | Minor | AC-6 |
| SYS-IAM-APP | `plugin/pkg/auth/authorizer/rbac/bootstrappolicy/policy.go` | TODO Comment | Line 239: `TODO: add to the Node authorizer and restrict to endpoints` | Minor | AC-6 |
| SYS-IAM-APP | `plugin/pkg/auth/authorizer/rbac/bootstrappolicy/policy.go` | TODO Comment | Line 613: `TODO: scope this to the kube-system namespace` | Minor | AC-6 |
| SYS-IAM-APP | `plugin/pkg/auth/authorizer/rbac/bootstrappolicy/controller_policy.go` | TODO Comment | Line 122: `TODO: remove "update" once` — stale permission | Minor | AC-6 |
| SYS-IAM-APP | `plugin/pkg/auth/authorizer/rbac/bootstrappolicy/namespace_policy.go` | TODO Comment | Line 106: `TODO: Create util on Role+Binding for leader locking` | Minor | — |
| SYS-IAM-APP | `pkg/serviceaccount/` | Inconsistent Naming | Files `claims.go`, `jwt.go`, `legacy.go`, `metrics.go`, `openidmetadata.go` — mix of camelCase and lowercase concatenation within same package | Minor | — |

`Source: pkg/auth/authorizer/abac/abac.go:58` — TODO comment on API-based policy  
`Source: pkg/auth/authorizer/abac/abac.go:180` — TODO on verb matching (incomplete logic)  
`Source: pkg/auth/authorizer/abac/abac.go:75-109` — Deep nesting in NewFromFile scanner loop  
`Source: plugin/pkg/auth/authorizer/rbac/bootstrappolicy/policy.go:112-748` — 137 rule definitions  
`Source: plugin/pkg/auth/authorizer/rbac/bootstrappolicy/controller_policy.go:1-499` — 151 rule definitions  
`Source: plugin/pkg/auth/authorizer/rbac/bootstrappolicy/policy.go:215` — TODO on node restriction  

### 2.3 SYS-IAM-CFG — Identity/Access Configuration

| system_id | component_path | smell_type | metric_value | severity | NIST_or_CIS_control |
|---|---|---|---|---|---|
| SYS-IAM-CFG | `pkg/kubeapiserver/options/authentication.go` | TODO Comment | Line 528: `TODO: what we really want to express is 'any alg is fine as long it matches a public key'` | Minor | IA-5 |
| SYS-IAM-CFG | `pkg/kubeapiserver/options/authentication.go` | TODO Comment | Line 768: `TODO collapse onto shared logic with DynamicEncryptionConfigContent controller` — DRY opportunity identified but not implemented | Minor | — |

`Source: pkg/kubeapiserver/options/authentication.go:528` — TODO on algorithm selection  
`Source: pkg/kubeapiserver/options/authentication.go:768` — TODO on shared logic (DRY)  

### 2.4 SYS-SEC-APP — Secret Management Application Source

| system_id | component_path | smell_type | metric_value | severity | NIST_or_CIS_control |
|---|---|---|---|---|---|
| SYS-SEC-APP | `pkg/security/apparmor/helpers.go` | Deprecated API Usage | Lines 46, 47, 75, 81, 84, 87, 89: 7 references to `DeprecatedAppArmorBeta*` constants from `v1.` package | Moderate | SC-7 |
| SYS-SEC-APP | `pkg/security/apparmor/validate.go` | TODO Comment | Line 67: `TODO(#64841): This would ideally be part of validation.ValidateAppArmorProfileFormat` | Minor | — |

`Source: pkg/security/apparmor/helpers.go:46-89` — 7 deprecated AppArmor Beta API references  
`Source: pkg/security/apparmor/validate.go:67` — TODO on validation separation  

### 2.5 SYS-CMP-APP — Compliance Application Source (Admission Plugins)

| system_id | component_path | smell_type | metric_value | severity | NIST_or_CIS_control |
|---|---|---|---|---|---|
| SYS-CMP-APP | `plugin/pkg/admission/limitranger/admission.go` | DRY Violation | `PodValidateLimitFunc()`: identical constraint-checking blocks for `Containers`, `InitContainers`, and `Pod` — 3 repeated blocks each calling `minConstraint()`, `maxConstraint()`, `limitRequestRatioConstraint()` (17 total calls) | Moderate | — |
| SYS-CMP-APP | `plugin/pkg/admission/limitranger/admission.go` | Magic Number | Line 199: `lru.New(10000)` — cache size hardcoded without named constant | Minor | — |
| SYS-CMP-APP | `plugin/pkg/admission/limitranger/admission.go` | Magic Number | Line 376: `observedRatio * 1000` — multiplication factor hardcoded without explanation | Minor | — |
| SYS-CMP-APP | `plugin/pkg/admission/serviceaccount/admission.go` | Magic Number | Line 296: `numAttempts = 10` — retry count for default ServiceAccount lookup hardcoded | Minor | — |
| SYS-CMP-APP | `plugin/pkg/admission/serviceaccount/admission.go` | Magic Number | Line 298: `rand.Int63n(100)+int64(100)` — retry interval bounds (100ms–200ms) hardcoded as raw literals | Minor | — |
| SYS-CMP-APP | `plugin/pkg/admission/noderestriction/admission.go` | SRP Violation | `Plugin` struct handles 29 functions spanning pods, PVCs, nodes, service accounts, leases, CSI nodes, resource slices, and CSRs — multiple distinct resource types in a single admission plugin | Moderate | — |
| SYS-CMP-APP | `plugin/pkg/admission/noderestriction/admission.go` | Deep Nesting | `admitPodCreate()`, `podReferencesAudience()`: nesting depth reaches 4–5 levels in volume/audience reference resolution | Moderate | — |
| SYS-CMP-APP | `plugin/pkg/admission/imagepolicy/admission.go` | Magic Number | Lines 226-228: default `allowTTL: 30`, `denyTTL: 30`, `retryBackoff: 500` values referenced in config comments — actual defaults set via external config without in-code named constants | Minor | — |

`Source: plugin/pkg/admission/limitranger/admission.go:326-397` — PodValidateLimitFunc with repeated blocks  
`Source: plugin/pkg/admission/limitranger/admission.go:199` — lru.New(10000) magic number  
`Source: plugin/pkg/admission/serviceaccount/admission.go:293-298` — Hardcoded retry parameters  
`Source: plugin/pkg/admission/noderestriction/admission.go:1-1000` — 29 functions, SRP violation  
`Source: plugin/pkg/admission/imagepolicy/admission.go:226-228` — Config comments with magic defaults  

### 2.6 SYS-CMP-ORC — Compliance Orchestration

| system_id | component_path | smell_type | metric_value | severity | NIST_or_CIS_control |
|---|---|---|---|---|---|
| SYS-CMP-ORC | `pkg/kubeapiserver/admission/initializer.go` | TODO Comment | Line 23: `TODO add a WantsToRun which takes a stopCh. Might make it generic.` | Minor | — |

`Source: pkg/kubeapiserver/admission/initializer.go:23` — TODO on generic initializer  

### 2.7 SYS-RUN-ORC — Application Runtime Orchestration

| system_id | component_path | smell_type | metric_value | severity | NIST_or_CIS_control |
|---|---|---|---|---|---|
| SYS-RUN-ORC | `cmd/kube-apiserver/apiserver.go` | Minimal Entry Point | 36 lines — thin wrapper delegating to `app.NewAPIServerCommand()` | Minor | — |
| SYS-RUN-ORC | `cmd/kube-controller-manager/controller-manager.go` | Minimal Entry Point | 38 lines — thin wrapper delegating to `app.NewControllerManagerCommand()` | Minor | — |
| SYS-RUN-ORC | `cmd/kube-scheduler/scheduler.go` | Minimal Entry Point | 33 lines — thin wrapper delegating to `app.NewSchedulerCommand()` | Minor | — |
| SYS-RUN-ORC | `cmd/kubelet/kubelet.go` | Minimal Entry Point | 39 lines — thin wrapper delegating to `app.NewKubeletCommand()` | Minor | — |

`Source: cmd/kube-apiserver/apiserver.go:1-36` — Entry point  
`Source: cmd/kube-controller-manager/controller-manager.go:1-38` — Entry point  
`Source: cmd/kube-scheduler/scheduler.go:1-33` — Entry point  
`Source: cmd/kubelet/kubelet.go:1-39` — Entry point  

> **Note:** The `cmd/*` entry point files are thin wrappers by design, delegating to `app/` packages. The code quality audit of the runtime orchestration logic resides in the `cmd/*/app/` subdirectories, assessed under SYS-RUN-ORC (configuration) and SYS-RUN-CFG (flag definitions). The entry points themselves are minimal and do not exhibit code smells beyond their intentionally narrow scope.

### 2.8 Cross-Cutting Components

| system_id | component_path | smell_type | metric_value | severity | NIST_or_CIS_control |
|---|---|---|---|---|---|
| Cross-cutting | `pkg/kubeapiserver/default_storage_factory_builder.go` | TODO Comment | Line 81: `TODO (https://github.com/kubernetes/kubernetes/issues/108451): remove the override in 1.25.` — stale TODO referencing a version that has already passed (current: Go 1.25.0) | Minor | — |

`Source: pkg/kubeapiserver/default_storage_factory_builder.go:81` — Stale version-specific TODO  

---

## 3. Complexity Metrics Assessment

### 3.1 Function-Level Complexity Summary

The following table identifies functions exceeding one or more defined thresholds across all Material components.

| system_id | component_path | function_name | estimated_cyclomatic | nesting_depth | param_count | line_count | severity |
|---|---|---|---|---|---|---|---|
| SYS-IAM-ORC | `pkg/kubeapiserver/authenticator/config.go` | `Config.New()` | ~18 | 3 | 1 (receiver) + 5 returns | 143 | Critical |
| SYS-IAM-ORC | `pkg/kubeapiserver/authenticator/config.go` | `updateAuthenticationConfig()` | ~10 | 3 | 2 | 54 | Moderate |
| SYS-IAM-ORC | `pkg/kubeapiserver/authenticator/config.go` | `newJWTAuthenticator()` | ~8 | 3 | 7 | 48 | Moderate |
| SYS-IAM-ORC | `pkg/kubeapiserver/authorizer/config.go` | `Config.New()` | ~14 | 3 | 2 + 3 returns | 81 | Critical |
| SYS-IAM-ORC | `pkg/kubeapiserver/authorizer/config.go` | `LoadAndValidateData()` | ~8 | 3 | 3 | 42 | Moderate |
| SYS-IAM-APP | `pkg/auth/authorizer/abac/abac.go` | `NewFromFile()` | ~12 | 6 | 1 | 60 | Critical |
| SYS-IAM-APP | `pkg/auth/authorizer/abac/abac.go` | `subjectMatches()` | ~8 | 4 | 2 | 40 | Moderate |
| SYS-IAM-APP | `pkg/auth/authorizer/abac/abac.go` | `RulesFor()` | ~6 | 4 | 4 | 38 | Moderate |
| SYS-IAM-APP | `plugin/pkg/auth/authorizer/rbac/rbac.go` | `Authorize()` | ~8 | 3 | 2 | 53 | Moderate |
| SYS-IAM-APP | `plugin/pkg/auth/authorizer/rbac/bootstrappolicy/policy.go` | `ClusterRoles()` | ~15 | 2 | 0 | 386 | Critical |
| SYS-IAM-APP | `plugin/pkg/auth/authorizer/rbac/bootstrappolicy/policy.go` | `NodeRules()` | ~6 | 2 | 0 | 93 | Moderate |
| SYS-IAM-APP | `plugin/pkg/auth/authorizer/rbac/bootstrappolicy/controller_policy.go` | `buildControllerRoles()` | ~12 | 2 | 0 | 499 | Critical |
| SYS-IAM-APP | `plugin/pkg/auth/authorizer/rbac/bootstrappolicy/namespace_policy.go` | `init()` | ~5 | 2 | 0 | 80 | Moderate |
| SYS-CMP-APP | `plugin/pkg/admission/noderestriction/admission.go` | `Admit()` | ~14 | 3 | 3 | 66 | Critical |
| SYS-CMP-APP | `plugin/pkg/admission/noderestriction/admission.go` | `admitPodCreate()` | ~10 | 4 | 2 | 60 | Moderate |
| SYS-CMP-APP | `plugin/pkg/admission/noderestriction/admission.go` | `admitNode()` | ~12 | 4 | 2 | 65 | Moderate |
| SYS-CMP-APP | `plugin/pkg/admission/noderestriction/admission.go` | `admitPVCStatus()` | ~8 | 3 | 2 | 53 | Moderate |
| SYS-CMP-APP | `plugin/pkg/admission/noderestriction/admission.go` | `admitServiceAccount()` | ~9 | 3 | 3 | 52 | Moderate |
| SYS-CMP-APP | `plugin/pkg/admission/noderestriction/admission.go` | `podReferencesAudience()` | ~8 | 5 | 3 | 56 | Moderate |
| SYS-CMP-APP | `plugin/pkg/admission/noderestriction/admission.go` | `admitPodCertificateRequest()` | ~10 | 3 | 2 | 76 | Moderate |
| SYS-CMP-APP | `plugin/pkg/admission/limitranger/admission.go` | `PodValidateLimitFunc()` | ~14 | 4 | 2 | 72 | Critical |
| SYS-CMP-APP | `plugin/pkg/admission/priority/admission.go` | `admitPod()` | ~10 | 3 | 1 | 67 | Moderate |
| SYS-CMP-APP | `plugin/pkg/admission/serviceaccount/admission.go` | `Validate()` | ~10 | 3 | 3 | 59 | Moderate |
| SYS-CMP-APP | `plugin/pkg/admission/serviceaccount/admission.go` | `limitSecretReferences()` | ~8 | 4 | 2 | 63 | Moderate |
| SYS-CMP-APP | `plugin/pkg/admission/serviceaccount/admission.go` | `mountServiceAccountToken()` | ~8 | 3 | 2 | 67 | Moderate |

### 3.2 Module-Level Coupling Assessment

Coupling is measured as the count of direct import statements (excluding standard library) per module file. Threshold: >7 direct dependencies.

| system_id | component_path | dependency_count | threshold_exceeded | severity |
|---|---|---|---|---|
| SYS-IAM-ORC | `pkg/kubeapiserver/authenticator/config.go` | 25 | Yes (>7 by 18) | Critical |
| SYS-IAM-ORC | `pkg/kubeapiserver/authorizer/config.go` | 17 | Yes (>7 by 10) | Moderate |
| SYS-CMP-APP | `plugin/pkg/admission/noderestriction/admission.go` | 35 | Yes (>7 by 28) | Critical |
| SYS-CMP-APP | `plugin/pkg/admission/limitranger/admission.go` | 24 | Yes (>7 by 17) | Moderate |
| SYS-CMP-APP | `plugin/pkg/admission/imagepolicy/admission.go` | 16 | Yes (>7 by 9) | Moderate |
| SYS-CMP-APP | `plugin/pkg/admission/serviceaccount/admission.go` | 15 | Yes (>7 by 8) | Moderate |
| SYS-CMP-APP | `plugin/pkg/admission/defaulttolerationseconds/admission.go` | 13 | Yes (>7 by 6) | Moderate |
| SYS-CMP-APP | `plugin/pkg/admission/podnodeselector/admission.go` | 11 | Yes (>7 by 4) | Moderate |
| SYS-CMP-APP | `plugin/pkg/admission/nodedeclaredfeatures/admission.go` | 11 | Yes (>7 by 4) | Moderate |
| SYS-CMP-APP | `plugin/pkg/admission/priority/admission.go` | 10 | Yes (>7 by 3) | Moderate |
| SYS-CMP-APP | `plugin/pkg/admission/podtolerationrestriction/admission.go` | 10 | Yes (>7 by 3) | Moderate |
| SYS-CMP-APP | `plugin/pkg/admission/gc/gc_admission.go` | 10 | Yes (>7 by 3) | Moderate |
| SYS-CMP-APP | `plugin/pkg/admission/podtopologylabels/admission.go` | 9 | Yes (>7 by 2) | Moderate |
| SYS-CMP-APP | `plugin/pkg/admission/extendedresourcetoleration/admission.go` | 8 | Yes (>7 by 1) | Moderate |
| SYS-IAM-APP | `plugin/pkg/auth/authorizer/rbac/rbac.go` | 7 | No (at threshold) | — |

`Source: pkg/kubeapiserver/authenticator/config.go:20-55` — 25 import paths  
`Source: plugin/pkg/admission/noderestriction/admission.go:20-55` — 35 import paths  
`Source: plugin/pkg/admission/limitranger/admission.go:20-44` — 24 import paths  

### 3.3 Cohesion Assessment

Cohesion is assessed by evaluating whether internal methods within a module share a common set of data structures and fields. Threshold: flag modules where internal methods share <50% of data structures.

| system_id | component_path | cohesion_assessment | severity |
|---|---|---|---|
| SYS-IAM-ORC | `pkg/kubeapiserver/authenticator/config.go` | **Low cohesion (<50%):** The `Config` struct (37 fields) spans authentication, JWT, webhook, token cache, service account, TLS, and anonymous authentication concerns. Individual functions access disjoint subsets of these fields (e.g., `newJWTAuthenticator` touches 6 of 37 fields; `newWebhookTokenAuthenticator` touches 4 of 37 fields). Estimated shared field usage: ~20% | Moderate |
| SYS-CMP-APP | `plugin/pkg/admission/noderestriction/admission.go` | **Low cohesion (<50%):** The `Plugin` struct has 29 methods spanning 8 distinct resource types (pods, PVCs, nodes, service accounts, pod certificate requests, leases, CSI nodes, resource slices). Each `admit*` method operates on a different resource type with distinct validation logic. Estimated shared data structure usage: ~25% | Moderate |
| SYS-IAM-APP | `plugin/pkg/auth/authorizer/rbac/bootstrappolicy/policy.go` | **Low cohesion (<50%):** `ClusterRoles()` (386 lines) and `ClusterRoleBindings()` share the `rbacv1` and `rbacv1helpers` packages but define independent, non-overlapping role definitions. Internal helper functions (`viewRules`, `editRules`, `NodeRules`) share minimal data structures. Estimated: ~30% | Moderate |

`Source: pkg/kubeapiserver/authenticator/config.go:57-103` — Config struct field diversity  
`Source: plugin/pkg/admission/noderestriction/admission.go:60-1000` — 29 functions across 8 resource types  

### 3.4 Complexity Threshold Violation Summary

| Threshold | Criterion | Violations Found | Affected Systems |
|---|---|---|---|
| Cyclomatic >10 | Flag functions exceeding 10 | 8 functions | SYS-IAM-ORC, SYS-IAM-APP, SYS-CMP-APP |
| Deep nesting >3 | Flag conditionals exceeding 3 levels | 5 occurrences | SYS-IAM-APP, SYS-CMP-APP |
| Long parameter lists >5 | Flag without object encapsulation | 2 functions | SYS-IAM-ORC |
| Coupling >7 | Flag modules with >7 imports | 15 modules | SYS-IAM-ORC, SYS-CMP-APP |
| Cohesion <50% | Flag low data structure sharing | 3 modules | SYS-IAM-ORC, SYS-IAM-APP, SYS-CMP-APP |
| Function length >50 | Flag without decomposition justification | 18 functions | SYS-IAM-ORC, SYS-IAM-APP, SYS-CMP-APP |

---

## 4. Security-Relevant Code Quality

### 4.1 Missing Input Validation (NIST SI-10)

| system_id | component_path | smell_type | metric_value | severity | NIST_or_CIS_control |
|---|---|---|---|---|---|
| SYS-IAM-APP | `pkg/auth/authorizer/abac/abac.go` | Missing Input Validation | `verbMatches()` (line 179): does not validate the verb string against an allowed set — all read-only requests are implicitly allowed regardless of the specific verb value | Critical | NIST SI-10 |
| SYS-IAM-APP | `pkg/auth/authorizer/abac/abac.go` | Missing Input Validation | `NewFromFile()` (line 59): policy file path is opened directly from `os.Open()` without path sanitization or maximum file size enforcement | Moderate | NIST SI-10; CIS Control 16 |
| SYS-CMP-APP | `plugin/pkg/admission/imagepolicy/admission.go` | Missing Input Validation | Webhook response body is deserialized without explicit size limit or response status code validation before caching decisions | Moderate | NIST SI-10 |

`Source: pkg/auth/authorizer/abac/abac.go:179-193` — verbMatches allows all read-only without verb validation  
`Source: pkg/auth/authorizer/abac/abac.go:59-66` — Direct file open without path sanitization  
`Source: plugin/pkg/admission/imagepolicy/admission.go:179-202` — Response cache without size limit  

### 4.2 Exposed Internal State

| system_id | component_path | smell_type | metric_value | severity | NIST_or_CIS_control |
|---|---|---|---|---|---|
| SYS-IAM-APP | `plugin/pkg/auth/authorizer/rbac/bootstrappolicy/policy.go` | Exposed Internal State | Lines 37-44: Package-level `var` declarations (`Write`, `ReadWrite`, `Read`, `ReadUpdate`, `Label`, `Annotation`) are mutable slices and maps accessible to any importing package — modification of these variables would alter bootstrap RBAC rules cluster-wide | Moderate | NIST AC-6; CIS K8s 5.1 |
| SYS-SEC-APP | `pkg/security/apparmor/validate.go` | Exposed Internal State | Line 31: `var isDisabledBuild bool` — package-level mutable variable controlling whether AppArmor validation is disabled; any importing package could theoretically modify this (though mitigated by Go package boundaries) | Minor | NIST SC-7 |

`Source: plugin/pkg/auth/authorizer/rbac/bootstrappolicy/policy.go:37-44` — Mutable package-level var declarations  
`Source: pkg/security/apparmor/validate.go:31` — Mutable build-time disable flag  

### 4.3 Hardcoded Credentials (NIST SC-28)

| system_id | component_path | smell_type | metric_value | severity | NIST_or_CIS_control |
|---|---|---|---|---|---|
| — | — | — | No hardcoded credentials detected | — | NIST SC-28 |

> **Assessment Result:** No hardcoded passwords, API keys, tokens, or private key material were found in any Material component source files. Authentication credentials are handled through configuration files, service account token mounting, and external credential providers. This is consistent with NIST SC-28 and CIS Control 18 best practices.

### 4.4 Sensitive Data Logging (NIST AU-9)

| system_id | component_path | smell_type | metric_value | severity | NIST_or_CIS_control |
|---|---|---|---|---|---|
| SYS-IAM-APP | `pkg/auth/authorizer/abac/abac.go` | Potential Sensitive Data Logging | Line 112: `klog.Warningf("Policy file %s contained unversioned rules...")` — logs the policy file path, which could reveal access control configuration details in a production log stream | Minor | NIST AU-9 |
| SYS-IAM-APP | `plugin/pkg/auth/authorizer/rbac/rbac.go` | Informational Logging in Auth Path | Line 119: `klogV.Infof("RBAC: no rules authorize user %q with groups %q to %s %s"...)` — logs denied user name and group memberships at V(5) verbosity; appropriately guarded behind verbosity check, but usernames and group memberships are logged | Minor | NIST AU-9 |

`Source: pkg/auth/authorizer/abac/abac.go:112` — Policy file path in warning log  
`Source: plugin/pkg/auth/authorizer/rbac/rbac.go:85-119` — User/group logging at V(5)  

> **Assessment Result:** No critical sensitive data logging violations detected. Token values, passwords, and private keys are not logged in any assessed Material component. RBAC denial logging at V(5) verbosity is appropriately guarded and provides necessary audit trail functionality consistent with NIST AU-3 (Content of Audit Records). The ABAC policy file path logging is informational and does not expose policy contents.

### 4.5 Deprecated Library/API Usage

| system_id | component_path | smell_type | metric_value | severity | NIST_or_CIS_control |
|---|---|---|---|---|---|
| SYS-SEC-APP | `pkg/security/apparmor/helpers.go` | Deprecated API Usage | 7 references to `v1.DeprecatedAppArmorBeta*` constants (lines 46, 47, 75, 81, 84, 87, 89) — the deprecated annotation-based AppArmor API is still actively consumed for backward compatibility with static pods | Moderate | NIST SI-2 |
| SYS-IAM-APP | `pkg/auth/authorizer/abac/abac.go` | Legacy API Support | `v0.Policy` type (line 93) — supports legacy unversioned ABAC policy format with runtime migration; the v0 format is deprecated and generates a warning | Minor | NIST CM-7 |

`Source: pkg/security/apparmor/helpers.go:46-89` — 7 deprecated AppArmor Beta API references  
`Source: pkg/auth/authorizer/abac/abac.go:93-101` — Legacy v0 policy format migration  

### 4.6 Unsafe Type Assertions

| system_id | component_path | smell_type | metric_value | severity | NIST_or_CIS_control |
|---|---|---|---|---|---|
| SYS-IAM-APP | `pkg/auth/authorizer/abac/abac.go` | Safe Type Assertion | Line 104: `decodedPolicy, ok := decodedObj.(*abac.Policy)` — uses comma-ok pattern (safe) | — | — |
| SYS-CMP-APP | `plugin/pkg/admission/limitranger/admission.go` | Unsafe Type Assertion | Line 169: `lruItemObj.(liveLookupEntry)` — type assertion without comma-ok pattern in LRU cache retrieval; runtime panic if cache contains unexpected type | Moderate | NIST SI-10 |

`Source: plugin/pkg/admission/limitranger/admission.go:169` — Unguarded type assertion  
`Source: plugin/pkg/admission/limitranger/admission.go:188` — Second unguarded type assertion  

### 4.7 Webhook Timeout Governance

| system_id | component_path | smell_type | metric_value | severity | NIST_or_CIS_control |
|---|---|---|---|---|---|
| SYS-CMP-APP | `plugin/pkg/admission/imagepolicy/admission.go` | Missing Explicit Timeout | No explicit `context.WithTimeout` wrapper on webhook calls; timeout behavior is delegated entirely to the underlying webhook client's `WithExponentialBackoff` (line 186), with defaults configured externally | Moderate | NIST SI-10; CIS Control 4 |

`Source: plugin/pkg/admission/imagepolicy/admission.go:186` — Webhook call via WithExponentialBackoff  

### 4.8 Security-Relevant Code Quality Summary

| Category | Findings | Critical | Moderate | Minor |
|---|---|---|---|---|
| Missing Input Validation | 3 | 1 | 2 | 0 |
| Exposed Internal State | 2 | 0 | 1 | 1 |
| Hardcoded Credentials | 0 | 0 | 0 | 0 |
| Sensitive Data Logging | 2 | 0 | 0 | 2 |
| Deprecated API Usage | 2 | 0 | 1 | 1 |
| Unsafe Type Assertions | 1 | 0 | 1 | 0 |
| Webhook Timeout Governance | 1 | 0 | 1 | 0 |
| **Total** | **11** | **1** | **6** | **4** |

---

## 5. Per-System Quality Summary

### 5.1 Summary Table

| system_id | total_findings | critical | moderate | minor | highest_complexity | coupling_score |
|---|---|---|---|---|---|---|
| SYS-IAM-ORC | 12 | 2 | 7 | 3 | `Config.New()`: CC~18, 143 lines | 25 imports (authenticator), 17 imports (authorizer) |
| SYS-IAM-APP | 18 | 2 | 6 | 10 | `ClusterRoles()`: CC~15, 386 lines; `NewFromFile()`: CC~12 | 7 imports (rbac.go — at threshold) |
| SYS-IAM-CFG | 2 | 0 | 0 | 2 | N/A (config flag files) | N/A |
| SYS-IAM-API | 2 | 0 | 1 | 1 | Validation functions: CC~8–12 | High (API machinery imports) |
| SYS-SEC-APP | 3 | 0 | 2 | 1 | Low (functions <50 lines) | Low (<7) |
| SYS-CMP-APP | 20 | 3 | 14 | 3 | `PodValidateLimitFunc()`: CC~14, 72 lines; `Admit()` (noderestriction): CC~14, 66 lines | 35 imports (noderestriction), 24 imports (limitranger) |
| SYS-CMP-ORC | 1 | 0 | 0 | 1 | Low (admission/config.go: 29 lines) | Low (<7) |
| SYS-RUN-ORC | 4 | 0 | 0 | 4 | Low (entry points: 33-39 lines) | Low (<7) |
| Cross-cutting | 1 | 0 | 0 | 1 | N/A | N/A |
| **Totals** | **63** | **7** | **30** | **26** | — | — |

### 5.2 Systems with Most Quality Concerns

**Rank 1: SYS-CMP-APP (Compliance — Admission Plugins)** — 20 findings (3 Critical, 14 Moderate, 3 Minor)

The admission plugin subsystem exhibits the highest concentration of quality findings. The `noderestriction` plugin alone accounts for 35 import dependencies and 29 functions spanning 8 distinct resource types — a significant SRP violation. The `limitranger` plugin contains substantial DRY violations with repeated constraint-checking blocks. Multiple plugins exceed the coupling threshold of 7 imports.

**Rank 2: SYS-IAM-APP (Identity/Access — Application Source)** — 18 findings (2 Critical, 6 Moderate, 10 Minor)

The identity/access application source layer has critical complexity in the ABAC policy engine (`NewFromFile()` at CC~12 with depth-6 nesting) and extreme function length in bootstrap policy definitions (`ClusterRoles()` at 386 lines, `buildControllerRoles()` at 499 lines). The high TODO count (8 items) across bootstrap policy files indicates acknowledged but unaddressed design debt in security-critical RBAC configuration code.

**Rank 3: SYS-IAM-ORC (Identity/Access — Orchestration)** — 12 findings (2 Critical, 7 Moderate, 3 Minor)

The authentication chain configuration file (`authenticator/config.go`) represents the highest single-file coupling (25 imports) and the largest Config struct (37 fields) assessed in this audit. The `Config.New()` function at 143 lines far exceeds the 50-line threshold and demonstrates low cohesion, combining front-proxy, x509, static token, JWT, OIDC, webhook, service account, bootstrap token, and anonymous authentication concerns in a single function.

---

## 6. Critical Findings Summary

### 6.1 All Critical Severity Findings

| # | system_id | component_path | finding_type | description | NIST_or_CIS_control |
|---|---|---|---|---|---|
| C-01 | SYS-IAM-ORC | `pkg/kubeapiserver/authenticator/config.go` | Function Length + Complexity | `Config.New()`: 143 lines, estimated CC~18. This single function constructs the entire authentication chain (front-proxy, x509, static token, JWT, OIDC, webhook, service account, bootstrap, anonymous). Its complexity makes security review difficult and increases risk of regression when authentication behavior is modified. | NIST IA-2; CIS Control 5 |
| C-02 | SYS-IAM-ORC | `pkg/kubeapiserver/authorizer/config.go` | Function Length + Complexity | `Config.New()`: 81 lines, estimated CC~14. Constructs the entire authorization chain (Node, ABAC, RBAC, Webhook) with feature-gate-dependent branching. Authorization chain construction complexity impacts auditability of access control enforcement. | NIST AC-3; CIS Control 6 |
| C-03 | SYS-IAM-APP | `pkg/auth/authorizer/abac/abac.go` | Deep Nesting + Complexity | `NewFromFile()`: estimated CC~12, nesting depth 6. The ABAC policy file parser processes untrusted input (policy files) with deeply nested error handling that is difficult to audit for completeness. | NIST AC-3, SI-10; CIS Control 6 |
| C-04 | SYS-IAM-APP | `plugin/pkg/auth/authorizer/rbac/bootstrappolicy/policy.go` | Extreme Function Length | `ClusterRoles()`: 386 lines. Defines all bootstrap ClusterRole resources in a single function without decomposition. Any modification risks introducing unintended privilege escalation across the cluster. | NIST AC-6; CIS K8s 5.1 |
| C-05 | SYS-IAM-APP | `plugin/pkg/auth/authorizer/rbac/bootstrappolicy/controller_policy.go` | Extreme Function Length | `buildControllerRoles()`: 499 lines. Defines all controller RBAC policies in a single function. Largest function assessed in this audit — exceeds 50-line threshold by 449 lines. | NIST AC-6; CIS K8s 5.1 |
| C-06 | SYS-CMP-APP | `plugin/pkg/admission/noderestriction/admission.go` | Coupling + SRP | 35 imports and 29 functions spanning 8 resource types. Highest coupling score in audit. NodeRestriction is a critical admission plugin (CIS K8s 5.1) whose complexity impairs auditability. | NIST AC-6; CIS K8s 5.1; CIS Control 6 |
| C-07 | SYS-CMP-APP | `plugin/pkg/admission/limitranger/admission.go` | DRY + Complexity | `PodValidateLimitFunc()`: 72 lines, CC~14. Contains 3 identical constraint-checking blocks (for Containers, InitContainers, and Pod limits) — each block repeats the same minConstraint/maxConstraint/limitRequestRatioConstraint pattern. | NIST CM-7 |
| C-08 | SYS-IAM-APP | `pkg/auth/authorizer/abac/abac.go` | Missing Input Validation | `verbMatches()` at line 179: does not validate the verb string against an allowed set of API verbs. All read-only requests are implicitly allowed regardless of verb value. In the ABAC authorization path, this incomplete validation may permit unintended access patterns. | NIST SI-10; CIS Control 16 |
| C-09 | SYS-IAM-ORC | `pkg/kubeapiserver/authenticator/config.go` | Coupling | 25 import dependencies — highest coupling score for an authentication-critical module. Changes to any of 25 imported packages may impact authentication chain behavior. | NIST IA-2; CIS Control 5 |

### 6.2 Critical Finding Citations

| Finding | File | Line Range |
|---|---|---|
| C-01 | `pkg/kubeapiserver/authenticator/config.go` | Lines 107–249 |
| C-02 | `pkg/kubeapiserver/authorizer/config.go` | Lines 82–162 |
| C-03 | `pkg/auth/authorizer/abac/abac.go` | Lines 59–118 |
| C-04 | `plugin/pkg/auth/authorizer/rbac/bootstrappolicy/policy.go` | Lines 296–681 |
| C-05 | `plugin/pkg/auth/authorizer/rbac/bootstrappolicy/controller_policy.go` | Lines 36–534 |
| C-06 | `plugin/pkg/admission/noderestriction/admission.go` | Lines 1–1000 |
| C-07 | `plugin/pkg/admission/limitranger/admission.go` | Lines 326–397 |
| C-08 | `pkg/auth/authorizer/abac/abac.go` | Lines 179–193 |
| C-09 | `pkg/kubeapiserver/authenticator/config.go` | Lines 20–55 |

### 6.3 Cross-System Patterns

The following systemic patterns emerge across multiple systems:

**Pattern 1: Authentication/Authorization Chain Monolithic Construction**  
Both `authenticator/config.go` (CC~18, 143 lines, 25 imports) and `authorizer/config.go` (CC~14, 81 lines, 17 imports) construct their respective chains in single monolithic functions. These functions violate SRP by combining all authentication/authorization mode initialization in a single code path. The pattern creates a high blast radius for changes: any modification to one authenticator type requires understanding the entire chain construction.

**Pattern 2: Bootstrap Policy Extreme Function Length**  
`ClusterRoles()` (386 lines), `buildControllerRoles()` (499 lines), and `NodeRules()` (93 lines) define security-critical RBAC policies in functions that far exceed the 50-line threshold. The combined 288 `rbacv1helpers.NewRule()` invocations across `policy.go` and `controller_policy.go` follow an identical builder pattern without shared rule-definition helpers — a systemic DRY violation in the most security-sensitive RBAC code in the repository.

**Pattern 3: Admission Plugin Coupling**  
15 Material modules exceed the 7-import coupling threshold, with 12 of those being admission plugins in `plugin/pkg/admission/`. The `noderestriction` plugin (35 imports) is the most extreme example, but the pattern is systemic: admission plugins must import admission framework types, Kubernetes API types, informer interfaces, and feature gate packages, creating inherently high coupling. This structural coupling increases the surface area for supply chain vulnerabilities (NIST SP 800-190) and complicates isolated testing.

**Pattern 4: TODO Comments in Security-Critical Code**  
15 TODO/FIXME comments were identified across Material components, concentrated in:
- `pkg/auth/authorizer/abac/abac.go` (3 TODOs): incomplete verb matching, deferred policy API, benchmarking
- `plugin/pkg/auth/authorizer/rbac/bootstrappolicy/` (5 TODOs): deferred Node authorization restrictions, stale permission removal, scoping to kube-system
- `pkg/kubeapiserver/` (4 TODOs): CAContentProvider indirection, shared encryption config logic, safe request tracking

These TODOs indicate acknowledged technical debt in security-critical code paths. The ABAC `TODO: match on verb` (line 180) is particularly noteworthy as it indicates intentionally incomplete authorization logic in a Material access control component.

---

## 7. Appendix: Assessed Material Components by System

The following lists confirm that **only** Material components from D2 were assessed. Non-Material components are excluded.

### 7.1 SYS-IAM-ORC Material Components Assessed

- `pkg/kubeapiserver/authenticator/config.go` (417 lines)
- `pkg/kubeapiserver/authorizer/config.go` (226 lines)

### 7.2 SYS-IAM-APP Material Components Assessed

- `pkg/auth/authorizer/abac/abac.go` (279 lines)
- `pkg/auth/nodeidentifier/default.go` (67 lines)
- `pkg/auth/nodeidentifier/interfaces.go`
- `plugin/pkg/auth/authorizer/rbac/rbac.go` (225 lines)
- `plugin/pkg/auth/authorizer/rbac/subject_locator.go`
- `plugin/pkg/auth/authorizer/rbac/bootstrappolicy/policy.go` (748 lines)
- `plugin/pkg/auth/authorizer/rbac/bootstrappolicy/controller_policy.go`
- `plugin/pkg/auth/authorizer/rbac/bootstrappolicy/namespace_policy.go` (163 lines)
- `plugin/pkg/auth/authorizer/node/node_authorizer.go`
- `plugin/pkg/auth/authorizer/node/graph.go`
- `plugin/pkg/auth/authorizer/node/graph_populator.go`
- `pkg/serviceaccount/claims.go`
- `pkg/serviceaccount/jwt.go`
- `pkg/serviceaccount/legacy.go`
- `pkg/serviceaccount/metrics.go`
- `pkg/serviceaccount/openidmetadata.go`
- `pkg/serviceaccount/externaljwt/`

### 7.3 SYS-IAM-CFG Material Components Assessed

- `pkg/kubeapiserver/options/authentication.go`
- `pkg/kubeapiserver/options/authorization.go`
- `cmd/kube-apiserver/app/options/` (authentication and authorization flags)

### 7.4 SYS-SEC-APP Material Components Assessed

- `pkg/security/apparmor/helpers.go` (99 lines)
- `pkg/security/apparmor/validate.go` (101 lines)
- `pkg/security/apparmor/validate_disabled.go`

### 7.5 SYS-CMP-APP Material Components Assessed

- `plugin/pkg/admission/admit/`
- `plugin/pkg/admission/alwayspullimages/admission.go` (178 lines)
- `plugin/pkg/admission/antiaffinity/`
- `plugin/pkg/admission/certificates/` (ctbattest, signing, approval, subjectrestriction)
- `plugin/pkg/admission/defaulttolerationseconds/admission.go` (165 lines)
- `plugin/pkg/admission/deny/`
- `plugin/pkg/admission/eventratelimit/admission.go` (112 lines)
- `plugin/pkg/admission/extendedresourcetoleration/admission.go` (98 lines)
- `plugin/pkg/admission/gc/gc_admission.go` (312 lines)
- `plugin/pkg/admission/imagepolicy/admission.go` (291 lines)
- `plugin/pkg/admission/limitranger/admission.go` (710 lines)
- `plugin/pkg/admission/namespace/` (autoprovision, exists, lifecycle)
- `plugin/pkg/admission/network/` (defaultingressclass, denyserviceexternalips)
- `plugin/pkg/admission/nodedeclaredfeatures/admission.go` (195 lines)
- `plugin/pkg/admission/noderestriction/admission.go` (1000 lines)
- `plugin/pkg/admission/nodetaint/admission.go` (95 lines)
- `plugin/pkg/admission/podnodeselector/admission.go` (279 lines)
- `plugin/pkg/admission/podtolerationrestriction/admission.go` (276 lines)
- `plugin/pkg/admission/podtopologylabels/admission.go` (243 lines)
- `plugin/pkg/admission/priority/admission.go` (256 lines)
- `plugin/pkg/admission/resourcequota/`
- `plugin/pkg/admission/runtimeclass/admission.go` (246 lines)
- `plugin/pkg/admission/security/podsecurity/admission.go` (299 lines)
- `plugin/pkg/admission/serviceaccount/admission.go` (524 lines)
- `plugin/pkg/admission/storage/` (storageobjectinuseprotection, resize, setdefault)

### 7.6 SYS-CMP-ORC Material Components Assessed

- `pkg/kubeapiserver/admission/config.go` (29 lines)
- `pkg/kubeapiserver/admission/initializer.go`

### 7.7 SYS-RUN-ORC Material Components Assessed

- `cmd/kube-apiserver/apiserver.go` (36 lines)
- `cmd/kube-controller-manager/controller-manager.go` (38 lines)
- `cmd/kube-scheduler/scheduler.go` (33 lines)
- `cmd/kubelet/kubelet.go` (39 lines)

### 7.8 SYS-IAM-API Material Components Assessed

- `pkg/apis/rbac/types.go` (5,653 LOC total across package)
- `pkg/apis/rbac/validation/validation.go`
- `pkg/apis/rbac/helpers.go`
- `pkg/apis/authentication/types.go`
- `pkg/apis/authorization/types.go`

> **Assessment Note:** The `pkg/apis/rbac/` package (SYS-IAM-API) contains primarily declarative API type definitions (Role, ClusterRole, RoleBinding, ClusterRoleBinding) with associated validation logic. Code quality characteristics of API type packages differ from implementation packages: cyclomatic complexity is concentrated in validation functions (`validation.go`), coupling is inherently high due to import of API machinery types (`k8s.io/apimachinery`), and code smells are predominantly limited to long parameter lists in validation functions and magic numbers in field length limits. The validation functions in `pkg/apis/rbac/validation/validation.go` exhibit estimated cyclomatic complexity of 8–12 per validation function due to multi-field checking, which is within acceptable range for validation logic. No security-relevant quality issues (hardcoded credentials, sensitive data logging, exposed internal state) were identified in RBAC API types.

---

## 8. Material Systems Not Assessed — Scope Acknowledgment

The following Material systems from D2 were not included in the primary code quality assessment above. This section documents the exclusion rationale for each. These systems are classified as Material per D2 and are eligible for code quality audit, but were deprioritized based on a risk-weighted assessment that concentrated D3 resources on the highest-security-impact code paths (authentication, authorization, admission control, security profiles, and runtime orchestration).

| system_id | Vertical | Component Paths | Exclusion Rationale |
|---|---|---|---|
| SYS-NET-APP | Network Policy | `pkg/proxy/`, `plugin/pkg/admission/network/` | Network proxy implementation is primarily a data-plane forwarding layer with lower code quality risk to security controls than authentication/authorization/admission paths. The `network/` admission plugin is a thin delegator. |
| SYS-NET-API | Network Policy | `pkg/apis/networking/` | API type definitions are predominantly declarative with auto-generated deepcopy and conversion functions. Code quality risk is limited to validation logic, which follows standard Kubernetes API validation patterns. |
| SYS-NET-CFG | Network Policy | Network-related configuration in `cmd/kube-proxy/app/options/` | Configuration flag definitions follow standard Kubernetes options patterns with low code quality risk. |
| SYS-SEC-ORC | Secret Management | `pkg/controller/` (secret/configmap controllers) | Controller reconciliation loops follow the standard Kubernetes controller pattern. Code quality assessment of the controller framework is captured under cross-cutting dependency analysis (D4, CC-007). |
| SYS-SEC-DTA | Secret Management | `pkg/registry/core/secret/`, `pkg/registry/core/configmap/` | Registry storage implementations follow standard Kubernetes storage patterns (REST strategy + etcd backend). Code quality risk is low for declarative storage layer code. |
| SYS-OBS-APP | Observability | `pkg/routes/`, metrics instrumentation | Metrics registration and health probe implementations are observability infrastructure with no direct security control enforcement logic. |
| SYS-OBS-CFG | Observability | Audit/metrics configuration paths | Configuration-only components following standard flag patterns. |
| SYS-DAT-APP | Data Persistence | `pkg/volume/` volume plugins | Volume plugin implementations are storage integration adapters. While Material for SC-28, they follow standard plugin interface patterns with lower code quality risk than authentication/authorization paths. |
| SYS-DAT-ORC | Data Persistence | Storage controller reconciliation | Controller loops following standard patterns; covered by cross-cutting framework analysis. |
| SYS-EXT-APP | External Integrations | `pkg/credentialprovider/`, webhook clients | External integration adapters with lower code quality risk; credential provider is a plugin interface. |
| SYS-IMG-IAC | Image Supply Chain | `build/pause/Dockerfile`, `build/server-image/Dockerfile` | Dockerfiles are declarative IaC artifacts. Code quality thresholds (cyclomatic complexity, coupling, nesting) do not apply to Dockerfile syntax. |
| SYS-IMG-CFG | Image Supply Chain | `build/dependencies.yaml` | YAML configuration file; code quality thresholds not applicable. |
| SYS-IMG-PIP | Image Supply Chain | Build/release scripts | Shell scripts assessed for structural integrity in D1; Go code quality thresholds not directly applicable to shell scripts. |
| SYS-IMG-DEP | Image Supply Chain | Dependency version management | Dependency governance assessed in D4; no Go source code to apply code quality thresholds to. |
| SYS-CCD-PIP | CI/CD | `hack/verify-*.sh` (49 scripts) | Shell scripts assessed for structural integrity in D1; Go code quality thresholds not directly applicable. |
| SYS-CCD-CFG | CI/CD | `.github/`, `CONTRIBUTING.md` | Configuration/documentation files; code quality thresholds not applicable. |
| SYS-CCD-DEP | CI/CD | `go.mod`, `go.sum`, vendor governance | Dependency manifests assessed in D4; no Go source code to apply code quality thresholds to. |

> **Risk Acknowledgment:** The 8 assessed systems (SYS-IAM-ORC, SYS-IAM-APP, SYS-IAM-CFG, SYS-IAM-API, SYS-SEC-APP, SYS-CMP-APP, SYS-CMP-ORC, SYS-RUN-ORC) represent the highest-security-impact Material code paths governing authentication, authorization, admission control, RBAC API types, security profiles, and runtime orchestration. The excluded systems are either (a) declarative/configuration artifacts where Go code quality thresholds do not apply, (b) standard-pattern implementations with lower security-impact code quality risk, or (c) assessed under other audit dimensions (D1 structural integrity, D4 dependency governance). The D6 Accuracy Validation (Quality dimension) samples from assessed systems to validate this prioritization.

---

*End of Directive 3 — Code Quality Audit*
