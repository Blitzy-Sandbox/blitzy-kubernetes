# Directive 2 — Materiality Classification

> **Document Type:** Compliance Audit — Materiality Classification  
> **Audit Target:** `k8s.io/kubernetes` monorepo (Go 1.25.0)  
> **Framework Authority Hierarchy:** NIST SP 800-53 Rev 5 > NIST SP 800-190 > CIS Kubernetes Benchmark v1.12.0 > CIS Controls v8  
> **Posture:** Assess-only — no code or documentation in the target repository is created, modified, or deleted  
> **Prerequisite:** Directive 0 — System Registry (`00-system-registry.md`)  
> **Consequence:** Material components proceed to Directives 3–6; Non-Material components are excluded from all subsequent audits  

---

## 1. Methodology

### 1.1 Classification Criteria

Each component registered in the Directive 0 System Registry is evaluated against two classification outcomes: **Material** or **Non-Material**.

A component is classified as **Material** if it governs ANY of the following control surfaces:

| Materiality Criterion | Primary NIST SP 800-53 Control | Primary CIS Control |
|---|---|---|
| Access control | AC-2, AC-3, AC-6 | CIS Control 5 (Account Management), CIS Control 6 (Access Control Management) |
| Audit logging | AU-2, AU-3, AU-6, AU-12 | CIS Control 8 (Audit Log Management) |
| Configuration state | CM-2, CM-3, CM-6, CM-7, CM-9 | CIS Control 4 (Secure Configuration) |
| Network segmentation | SC-7 (Boundary Protection), SC-8 | CIS K8s Section 5.3, CIS Control 4 |
| System integrity | SI-2, SI-3, SI-7, SI-10 | CIS Control 16 (Application Software Security) |
| Secret management | SC-12, SC-28 | CIS Control 18 (Penetration Testing / Application Software Security) |
| Deployment integrity | CM-2, SA-10, SI-7 (via NIST SP 800-190) | CIS K8s Section 4 (Worker Nodes / Pod Policies) |
| Cross-cutting concerns | Shared by 3+ systems with security impact | CIS Control 2 (Inventory of Software Assets) |

A component is classified as **Non-Material** if it meets ANY of the following exclusion criteria:

| Exclusion Criterion | Rationale |
|---|---|
| Generated code (vendored, auto-generated `zz_generated.*.go`, `generated.pb.go`) | Not directly authored; reflects upstream source |
| Test fixtures and scaffolding (`*_test.go`, `test/`, `testdata/`) | Does not execute in production; no control surface influence |
| Build artifacts with no security control surface influence | Intermediate outputs that do not govern policy or enforcement |
| Documentation-only files (existing repo `README.md`, `doc.go`, `CONTRIBUTING.md`) | Informational; does not implement or enforce controls |
| Logo/branding assets (`logo/`) | Visual assets with no security relevance |
| Changelog files (`CHANGELOG/`) | Historical records with no control enforcement |
| License compliance files (`LICENSES/`) | Legal artifacts with no runtime security function |
| Third-party vendored code (`vendor/`, `third_party/`) | External code not authored or maintained within this repository |
| Staging modules (`staging/`) | Referenced for context but maintained as separate repositories |

### 1.2 Evaluation Process

1. **System-level assessment:** For each system_id from D0, identify all component paths within its intersection_scope.
2. **Component-level evaluation:** For each component, determine if it governs any of the 8 materiality criteria by inspecting verified source code functionality.
3. **Framework mapping:** For each Material component, assign the governing NIST SP 800-53 control(s) and governing CIS control(s) based on verified codebase behavior.
4. **Rationale documentation:** Provide explicit, per-component rationale that references the specific materiality criterion met.
5. **No aspirational controls:** Only controls verified in the codebase are used for classification. Planned or theoretical controls are excluded.

### 1.3 Framework Authority Resolution

Where NIST SP 800-53 and CIS controls prescribe different requirements for the same component, the more restrictive control is applied. Conflicts are documented in `appendix-framework-conflict-register.md`.

**Authority precedence:** NIST SP 800-53 Rev 5 → NIST SP 800-190 → CIS Kubernetes Benchmark v1.12.0 → CIS Controls v8

---

## 2. Classified Component Inventory

The following table provides the comprehensive classification of all components across all 45 registered systems from D0. Components are organized by system_id.

### 2.1 Identity/Access Management (IAM) Systems

| system_id | component_path | classification | materiality_rationale | governing_NIST_control | governing_CIS_control |
|---|---|---|---|---|---|
| SYS-IAM-ORC | `pkg/kubeapiserver/authenticator/config.go` | Material | Configures the authentication chain (RequestHeader → x509 → StaticToken → ServiceAccount → BootstrapToken → OIDC → Webhook); governs how every API request is authenticated at runtime | IA-2, IA-5, IA-8 | CIS K8s 3.1; CIS Control 5 |
| SYS-IAM-ORC | `pkg/kubeapiserver/authorizer/config.go` | Material | Configures the authorization chain (Node → RBAC → Webhook → ABAC → default deny); governs how every authenticated request is authorized at runtime | AC-3, AC-6 | CIS K8s 5.1; CIS Control 6 |
| SYS-IAM-APP | `pkg/auth/authorizer/abac/abac.go` | Material | Implements the ABAC policy engine — parses policy files and evaluates subject/verb/resource/nonResource match rules for access control decisions | AC-3, AC-6 | CIS Control 6 |
| SYS-IAM-APP | `pkg/auth/nodeidentifier/interfaces.go` | Material | Defines the NodeIdentifier interface that determines whether a user.Info represents a node identity — critical for node-level access isolation | IA-4 | CIS Control 5 |
| SYS-IAM-APP | `pkg/auth/nodeidentifier/default.go` | Material | Default implementation of NodeIdentifier using `system:node:` username prefix and `system:nodes` group membership for node identity resolution | IA-4 | CIS Control 5 |
| SYS-IAM-APP | `plugin/pkg/auth/authorizer/rbac/rbac.go` | Material | Implements the RBACAuthorizer — evaluates Role/ClusterRole bindings via VisitRulesFor pattern to produce allow/deny decisions per request | AC-6 | CIS K8s 5.1; CIS Control 5, 6 |
| SYS-IAM-APP | `plugin/pkg/auth/authorizer/rbac/subject_locator.go` | Material | Resolves RBAC subjects to applicable policy rules; identifies which RoleBindings/ClusterRoleBindings apply to a given user+namespace | AC-6 | CIS K8s 5.1; CIS Control 6 |
| SYS-IAM-APP | `plugin/pkg/auth/authorizer/rbac/bootstrappolicy/policy.go` | Material | Defines bootstrap ClusterRoles and ClusterRoleBindings applied at cluster initialization (system:controller, system:node, etc.) | AC-6 | CIS K8s 5.1; CIS Control 5 |
| SYS-IAM-APP | `plugin/pkg/auth/authorizer/rbac/bootstrappolicy/controller_policy.go` | Material | Defines RBAC policies for built-in controllers (deployment, replicaset, service-account, etc.) — enforces least-privilege per controller | AC-6 | CIS K8s 5.1; CIS Control 5 |
| SYS-IAM-APP | `plugin/pkg/auth/authorizer/rbac/bootstrappolicy/namespace_policy.go` | Material | Defines default namespace-level RBAC policies (edit, view, admin roles) applied at namespace creation | AC-6 | CIS K8s 5.1; CIS Control 5 |
| SYS-IAM-APP | `plugin/pkg/auth/authorizer/node/node_authorizer.go` | Material | Implements the Node authorizer — restricts kubelet API access to only resources bound to the node's pods using a graph-based model | AC-6 | CIS Control 6 |
| SYS-IAM-APP | `plugin/pkg/auth/authorizer/node/graph.go` | Material | Maintains the runtime graph of node-to-pod-to-resource relationships used by the Node authorizer for access control decisions | AC-6 | CIS Control 6 |
| SYS-IAM-APP | `plugin/pkg/auth/authorizer/node/graph_populator.go` | Material | Populates the Node authorizer graph from informer events (pod create/update/delete) to track which resources each node may access | AC-6 | CIS Control 6 |
| SYS-IAM-APP | `plugin/pkg/auth/authenticator/token/bootstrap/bootstrap.go` | Material | Implements bootstrap token authentication — validates `bootstrap-token-*` secrets for cluster join operations | IA-5 | CIS K8s 5.1; CIS Control 5 |
| SYS-IAM-APP | `pkg/serviceaccount/claims.go` | Material | Implements ServiceAccount token claims validation — verifies bound object references (pod, node, secret) in JWT claims | IA-4, IA-5 | CIS K8s 5.1; CIS Control 5 |
| SYS-IAM-APP | `pkg/serviceaccount/jwt.go` | Material | Implements JWT token generation and signing for ServiceAccount tokens — produces bound tokens with audience and expiration | IA-4, IA-5 | CIS K8s 5.1; CIS Control 5 |
| SYS-IAM-APP | `pkg/serviceaccount/legacy.go` | Material | Implements legacy ServiceAccount token generation for backward compatibility with pre-bound-token ServiceAccount secrets | IA-5 | CIS K8s 5.1; CIS Control 5 |
| SYS-IAM-APP | `pkg/serviceaccount/metrics.go` | Material | Tracks ServiceAccount token generation metrics (stale token count, total issued) — supports audit logging of credential lifecycle | AU-12, IA-5 | CIS Control 8 |
| SYS-IAM-APP | `pkg/serviceaccount/openidmetadata.go` | Material | Serves OIDC discovery metadata (`.well-known/openid-configuration`) and JSON Web Key Sets for ServiceAccount token verification | IA-5, IA-8 | CIS Control 5 |
| SYS-IAM-APP | `pkg/serviceaccount/externaljwt/` | Material | Implements external JWT signer plugin interface for ServiceAccount tokens — enables external key management integration | IA-5, SC-12 | CIS Control 5, 18 |
| SYS-IAM-CFG | `cmd/kube-apiserver/app/options/` (authentication and authorization flags) | Material | Defines command-line flags controlling OIDC provider URLs, ABAC policy paths, webhook endpoints, authorization modes, and ServiceAccount issuers | AC-2, IA-5, IA-8 | CIS K8s 3.1; CIS Control 5 |
| SYS-IAM-API | `pkg/apis/rbac/` (types.go, helpers.go, v1/, v1alpha1/, v1beta1/, validation/) | Material | Defines RBAC API types (Role, ClusterRole, RoleBinding, ClusterRoleBinding, PolicyRule) — the contract for all RBAC-based access control | AC-3, AC-6 | CIS K8s 5.1; CIS Control 5, 6 |
| SYS-IAM-API | `pkg/apis/authentication/` (types.go, v1/, v1beta1/, validation/) | Material | Defines TokenReview API types — the contract for token authentication verification requests | IA-2 | CIS Control 5 |
| SYS-IAM-API | `pkg/apis/authorization/` (types.go, v1/, v1beta1/, validation/) | Material | Defines SubjectAccessReview, SelfSubjectAccessReview, LocalSubjectAccessReview API types — the contract for authorization decision queries | AC-3 | CIS Control 6 |
| SYS-IAM-API | `pkg/certauthorization/` | Material | Defines certificate-based authorization types for x509 client certificate authentication mapping | IA-2, IA-8 | CIS Control 5 |
| SYS-IAM-DTA | `pkg/registry/rbac/` (role/, clusterrole/, rolebinding/, clusterrolebinding/) | Material | Implements etcd storage for RBAC resources — persistence layer for all role bindings and permission definitions | AC-2, AC-3, AC-6 | CIS K8s 5.1; CIS Control 5, 6 |

### 2.2 Network Policy (NET) Systems

| system_id | component_path | classification | materiality_rationale | governing_NIST_control | governing_CIS_control |
|---|---|---|---|---|---|
| SYS-NET-ORC | `cmd/kube-proxy/app/` | Material | Orchestrates kube-proxy startup, service routing rule generation (iptables/IPVS/nftables), and endpoint change processing — enforces network segmentation at runtime | SC-7 | CIS K8s 4.2; CIS Control 4 |
| SYS-NET-APP | `pkg/proxy/` | Material | Implements service proxy logic including iptables, IPVS, and nftables backends — translates NetworkPolicy and Service definitions into kernel-level packet filtering rules | SC-7, SC-8 | CIS K8s 5.3; CIS Control 4 |
| SYS-NET-APP | `plugin/pkg/admission/network/` | Material | Implements network-related admission checks that validate NetworkPolicy definitions at creation time | SC-7 | CIS K8s 5.3; CIS Control 4 |
| SYS-NET-CFG | kube-proxy configuration flags (proxy mode, cluster CIDR, conntrack settings) | Material | Defines network proxy behavior including packet routing mode selection and CIDR boundaries — misconfiguration directly impacts network segmentation | SC-7 | CIS K8s 5.3; CIS Control 4 |
| SYS-NET-API | `pkg/apis/networking/` (types.go, v1/, v1alpha1/, v1beta1/, validation/) | Material | Defines NetworkPolicy, Ingress, IngressClass API types — the contract for all network segmentation policy definitions | SC-7 | CIS K8s 5.3; CIS Control 4 |

### 2.3 Secret Management (SEC) Systems

| system_id | component_path | classification | materiality_rationale | governing_NIST_control | governing_CIS_control |
|---|---|---|---|---|---|
| SYS-SEC-ORC | `pkg/controller/` (subset: secret/configmap controllers, serviceaccount token controller) | Material | Orchestrates secret creation, rotation, and distribution through controller reconciliation loops at runtime | SC-12, SC-28 | CIS K8s 5.4; CIS Control 18 |
| SYS-SEC-APP | `pkg/credentialprovider/` | Material | Implements external credential provider plugin interface for image pull secrets — governs how container registry credentials are sourced and managed | SC-28, IA-5 | CIS Control 18 |
| SYS-SEC-APP | `plugin/pkg/admission/serviceaccount/` | Material | Implements ServiceAccount admission plugin — automatically injects ServiceAccount token secrets into pods at creation time | IA-4, SC-28 | CIS K8s 5.1; CIS Control 18 |
| SYS-SEC-APP | `pkg/security/apparmor/` | Material | Implements AppArmor profile validation and enforcement for container security function isolation — governs mandatory access control profiles applied to pod workloads at runtime | SC-3, SC-7 | CIS K8s 4.1, 4.2; CIS Control 4 |
| SYS-SEC-CFG | Encryption configuration (EncryptionConfiguration resource paths, signing key paths) | Material | Defines encryption providers, key rotation configuration, and signing key paths for secrets at rest — misconfiguration exposes all cluster secrets | SC-12, SC-28 | CIS K8s 5.4; CIS Control 18 |
| SYS-SEC-API | `pkg/apis/core/` (subset: Secret, ConfigMap types in types.go, validation/) | Material | Defines Secret and ConfigMap API types including SecretType constants (Opaque, TLS, DockerConfigJson, ServiceAccountToken) — the contract for sensitive data storage | SC-28 | CIS K8s 5.4; CIS Control 18 |
| SYS-SEC-DTA | `pkg/registry/core/secret/`, `pkg/registry/core/configmap/` | Material | Implements etcd storage for Secrets and ConfigMaps — the persistence layer where encryption at rest is applied to sensitive data | SC-12, SC-28, IA-5 | CIS K8s 5.4; CIS Control 18 |

### 2.4 Image Supply Chain (IMG) Systems

| system_id | component_path | classification | materiality_rationale | governing_NIST_control | governing_CIS_control |
|---|---|---|---|---|---|
| SYS-IMG-IAC | `build/pause/Dockerfile` | Material | Defines the pause container image — the init container present in every Kubernetes pod; image provenance directly impacts deployment integrity | CM-2, SI-7 | CIS K8s 4.2; CIS Control 2, 7 |
| SYS-IMG-IAC | `build/server-image/Dockerfile` | Material | Defines the server container image for kube-apiserver, kube-controller-manager, kube-scheduler, kube-proxy — base image selection impacts supply chain integrity | CM-2, SI-7 | CIS K8s 4.2; CIS Control 2, 7 |
| SYS-IMG-IAC | `build/build-image/` | Material | Defines the build environment container image — governs the toolchain integrity of the compilation and release pipeline | CM-2, SI-7 | CIS K8s 4.2; CIS Control 2 |
| SYS-IMG-CFG | `build/dependencies.yaml` | Material | Pins external dependency versions (zeitgeist v0.5.4, CNI 1.9.0, CoreDNS 1.13.1, etcd 3.6.7, crictl 1.34.0, protoc 23.4) — controls supply chain integrity through explicit version governance | CM-2, CM-7 | CIS K8s 4.2; CIS Control 2, 7 |
| SYS-IMG-PIP | `build/release.sh`, `build/release-images.sh` | Material | Defines the release pipeline that produces signed container images — governs deployment artifact integrity from source to registry | SA-10, SI-7 | CIS K8s 4.2; CIS Control 2, 16 |
| SYS-IMG-PIP | `build/common.sh`, `build/run.sh` | Material | Shared build pipeline utilities that establish the build environment and execute compilation — governs build reproducibility | SA-10, SI-7 | CIS K8s 4.2; CIS Control 16 |
| SYS-IMG-DEP | `build/dependencies.yaml` (dependency tracking) | Material | Tracks external dependency versions with refPath verification — ensures dependency version consistency across all consuming scripts and manifests | CM-2, CM-7, SA-10 | CIS Control 2, 7 |

### 2.5 CI/CD (CCD) Systems

| system_id | component_path | classification | materiality_rationale | governing_NIST_control | governing_CIS_control |
|---|---|---|---|---|---|
| SYS-CCD-CFG | `.github/PULL_REQUEST_TEMPLATE.md` | Material | Defines PR review checklist enforcing documentation, testing, and release note requirements — governs change control process | CM-3, CM-9 | CIS Control 4, 16 |
| SYS-CCD-CFG | `.github/SECURITY.md` | Material | Defines security vulnerability disclosure process — governs incident response communication channel for the project | CM-9 | CIS Control 4 |
| SYS-CCD-CFG | `CONTRIBUTING.md` | Material | Defines contribution governance (redirects to external community guide) — governs the change control entry point | CM-3 | CIS Control 4 |
| SYS-CCD-CFG | `.github/ISSUE_TEMPLATE/` (bug-report.yaml, enhancement.yaml, failing-test.yaml, flaking-test.yaml) | Material | Defines structured issue reporting templates — governs the configuration change request process | CM-3, CM-9 | CIS Control 4 |
| SYS-CCD-PIP | `hack/verify-*.sh` (49 verification scripts) | Material | Implements automated quality and compliance verification gates including golangci-lint, import verification, generated file consistency, OpenAPI spec validation, boilerplate checks, feature gate verification, and more — enforces configuration change control | CM-3, SA-10 | CIS Control 4, 16 |
| SYS-CCD-PIP | `Makefile` | Material | Defines primary build system targets (all, build, test, clean, verify, update, release) — governs the build pipeline entry points | CM-3, SA-10 | CIS Control 4 |
| SYS-CCD-PIP | `hack/update-*.sh` (generation and update scripts) | Material | Implements code generation and dependency update scripts — governs automated artifact regeneration and consistency | CM-3, SA-10 | CIS Control 4 |
| SYS-CCD-DEP | `go.mod` | Material | Declares root module `k8s.io/kubernetes` with Go 1.25.0 and all direct dependencies — governs the complete dependency graph and version pinning for the project | CM-3, CM-7, SA-10 | CIS Control 2, 4 |
| SYS-CCD-DEP | `go.sum` | Material | Contains cryptographic checksums for all module dependencies — provides integrity verification for the entire dependency supply chain | CM-7, SI-7 | CIS Control 2 |

### 2.6 Application Runtime (RUN) Systems

| system_id | component_path | classification | materiality_rationale | governing_NIST_control | governing_CIS_control |
|---|---|---|---|---|---|
| SYS-RUN-IAC | `build/server-image/Dockerfile` (shared with SYS-IMG-IAC) | Material | Defines the runtime container image for control plane binaries — image integrity directly impacts runtime security posture | CM-2, CM-6 | CIS K8s 1.1, 4.1; CIS Control 4 |
| SYS-RUN-IAC | `build/pause/Dockerfile` (shared with SYS-IMG-IAC) | Material | Pause container image present in every pod — runtime isolation foundation | CM-2, CM-6 | CIS K8s 4.1; CIS Control 4 |
| SYS-RUN-ORC | `cmd/kube-apiserver/app/` | Material | API server orchestration — startup, TLS configuration, authentication/authorization chain initialization, admission plugin loading, and request handling | CM-6, CM-7, SC-3 | CIS K8s 1.1, 1.2; CIS Control 4 |
| SYS-RUN-ORC | `cmd/kube-controller-manager/app/` | Material | Controller manager orchestration — initializes and runs all built-in controllers (deployment, replicaset, namespace, serviceaccount, node, etc.) | CM-6, CM-7 | CIS K8s 1.3; CIS Control 4 |
| SYS-RUN-ORC | `cmd/kube-scheduler/app/` | Material | Scheduler orchestration — initializes scheduling framework, processes pod scheduling queue, and binds pods to nodes | CM-6, CM-7 | CIS K8s 1.4; CIS Control 4 |
| SYS-RUN-ORC | `cmd/kubelet/app/` | Material | Kubelet orchestration — node agent startup, pod lifecycle management, container runtime interface coordination, volume mounting | CM-6, CM-7 | CIS K8s 4.1, 4.2; CIS Control 4 |
| SYS-RUN-ORC | `cmd/kube-proxy/app/` (shared with SYS-NET-ORC) | Material | kube-proxy orchestration — network rule management for service routing and network segmentation enforcement | CM-6, SC-7 | CIS K8s 4.2; CIS Control 4 |
| SYS-RUN-APP | `pkg/controlplane/` | Material | Implements the API server control plane logic — request routing, aggregated API server management, and storage initialization | CM-7, SI-2 | CIS K8s 1.2; CIS Control 4 |
| SYS-RUN-APP | `pkg/scheduler/` | Material | Implements the scheduling framework — filter, score, and bind extensions that determine pod placement decisions | CM-7 | CIS K8s 1.4; CIS Control 4 |
| SYS-RUN-APP | `pkg/kubelet/` | Material | Implements the node agent — container lifecycle management, pod status reporting, resource monitoring, volume management, and image pulling | CM-7 | CIS K8s 4.1, 4.2; CIS Control 4 |
| SYS-RUN-APP | `pkg/proxy/` (shared with SYS-NET-APP) | Material | Service proxy implementation — translates service/endpoint definitions into network routing rules | SC-7, CM-7 | CIS K8s 5.3; CIS Control 4 |
| SYS-RUN-APP | `pkg/quota/` | Material | Implements resource quota evaluation — enforces resource usage limits per namespace to prevent resource exhaustion | CM-7 | CIS Control 4 |
| SYS-RUN-CFG | `cmd/kube-apiserver/app/options/` | Material | API server flag definitions — security-relevant flags for TLS, authentication, authorization, admission, audit, and encryption configuration | CM-6, CM-7 | CIS K8s 1.1, 1.2; CIS Control 4 |
| SYS-RUN-CFG | `cmd/kube-controller-manager/app/options/` | Material | Controller manager flag definitions — service account private key, root CA, CIDR allocation, and controller-specific options | CM-6, CM-7 | CIS K8s 1.3; CIS Control 4 |
| SYS-RUN-CFG | `cmd/kube-scheduler/app/options/` | Material | Scheduler flag definitions — scheduling policy, bind address, leader election, and profile configuration | CM-6, CM-7 | CIS K8s 1.4; CIS Control 4 |
| SYS-RUN-CFG | `cmd/kubelet/app/options/` | Material | Kubelet flag definitions — TLS, authentication, authorization, cgroup, seccomp, AppArmor, and image credential configuration | CM-6, CM-7 | CIS K8s 4.1, 4.2; CIS Control 4 |
| SYS-RUN-CFG | `pkg/features/` | Material | Feature gate definitions — controls which experimental or beta features are enabled at runtime, directly impacting security posture | CM-6, CM-7 | CIS Control 4 |
| SYS-RUN-DEP | `go.mod` (runtime dependencies) | Material | Runtime Go module dependencies (cobra, ginkgo, cadvisor, cel-go, prometheus, etc.) — compromised runtime dependencies propagate to all components | CM-7, SI-2, SA-10 | CIS Control 2, 4 |
| SYS-RUN-API | `api/openapi-spec/swagger.json` | Material | Complete OpenAPI specification for the Kubernetes API — defines the programmatic contract for all API consumers | CM-6 | CIS K8s 1.2; CIS Control 4 |
| SYS-RUN-API | `pkg/generated/openapi/` | Material | Generated OpenAPI definitions compiled into the API server binary — ensures API contract accuracy at runtime | CM-6 | CIS Control 4 |

### 2.7 Observability (OBS) Systems

| system_id | component_path | classification | materiality_rationale | governing_NIST_control | governing_CIS_control |
|---|---|---|---|---|---|
| SYS-OBS-ORC | `staging/src/k8s.io/apiserver/pkg/audit/` (staging reference) | Material | Audit event generation — processes every API request to produce audit records per configured policy; governs audit logging at the orchestration layer | AU-2, AU-3, AU-12 | CIS K8s 3.2; CIS Control 8 |
| SYS-OBS-APP | `pkg/routes/` | Material | Registers metrics, profiling, and health check endpoints — provides the runtime observability surface area for monitoring and alerting | AU-6, AU-12 | CIS K8s 3.2; CIS Control 8 |
| SYS-OBS-APP | `pkg/probe/` | Material | Implements liveness, readiness, and startup probe logic — governs health-based traffic routing and pod restart decisions | AU-12 | CIS Control 8 |
| SYS-OBS-CFG | Audit policy configuration, metrics bind address flags, logging verbosity | Material | Defines which API events are logged, at what level, and to which backends — audit policy misconfiguration directly impacts compliance visibility | AU-2, AU-3 | CIS K8s 3.2; CIS Control 8 |
| SYS-OBS-API | Audit API types, metrics API surface, health endpoint contracts | Material | Defines the audit event schema and metrics API contracts — the interface through which observability data is produced and consumed | AU-2, AU-3 | CIS K8s 3.2; CIS Control 8 |

### 2.8 Compliance (CMP) Systems

| system_id | component_path | classification | materiality_rationale | governing_NIST_control | governing_CIS_control |
|---|---|---|---|---|---|
| SYS-CMP-ORC | `pkg/kubeapiserver/admission/config.go` | Material | Configures the admission chain — initializes plugin initializers that connect admission plugins to API server informers and service resolution | CM-7, SI-10 | CIS K8s 5.2; CIS Control 4 |
| SYS-CMP-ORC | `pkg/kubeapiserver/admission/initializer.go` | Material | Implements the admission plugin initializer — injects shared dependencies (informers, authorizer, cloud config) into admission plugins | CM-7, SI-10 | CIS K8s 5.2; CIS Control 4 |
| SYS-CMP-APP | `plugin/pkg/admission/admit/` | Material | Implements the AlwaysAdmit admission plugin — baseline system integrity validation that allows all requests (used for testing/default chains) | SI-10 | CIS Control 4 |
| SYS-CMP-APP | `plugin/pkg/admission/alwayspullimages/` | Material | Implements the AlwaysPullImages admission plugin — forces image pull on every pod creation to prevent use of stale or tampered local images | SI-7 | CIS K8s 4.2; CIS Control 7 |
| SYS-CMP-APP | `plugin/pkg/admission/antiaffinity/` | Material | Implements anti-affinity admission validation — ensures scheduling constraints are properly formed, impacting workload isolation | CM-7 | CIS Control 4 |
| SYS-CMP-APP | `plugin/pkg/admission/certificates/` | Material | Implements certificate signing request admission — validates and approves CSR resources for TLS certificate lifecycle management | SC-8 | CIS Control 4 |
| SYS-CMP-APP | `plugin/pkg/admission/defaulttolerationseconds/` | Material | Implements default toleration seconds injection — configures pod eviction behavior on node NotReady/Unreachable conditions | CM-7 | CIS Control 4 |
| SYS-CMP-APP | `plugin/pkg/admission/deny/` | Material | Implements the AlwaysDeny admission plugin — baseline deny for system integrity testing and fail-closed chain validation | SI-10 | CIS Control 4 |
| SYS-CMP-APP | `plugin/pkg/admission/eventratelimit/` | Material | Implements event rate limiting — prevents audit event flooding that could degrade cluster observability or storage capacity | AU-2 | CIS Control 8 |
| SYS-CMP-APP | `plugin/pkg/admission/extendedresourcetoleration/` | Material | Implements extended resource toleration injection — automatically adds tolerations for pods requesting extended resources | CM-7 | CIS Control 4 |
| SYS-CMP-APP | `plugin/pkg/admission/gc/` | Material | Implements garbage collection admission — prevents unauthorized deletion of ownerReference-protected resources, enforcing resource lifecycle governance | CM-7 | CIS Control 4 |
| SYS-CMP-APP | `plugin/pkg/admission/imagepolicy/` | Material | Implements the ImagePolicyWebhook admission plugin — delegates image admission decisions to an external webhook for supply chain validation | SI-7 | CIS K8s 4.2; CIS Control 7 |
| SYS-CMP-APP | `plugin/pkg/admission/limitranger/` | Material | Implements LimitRange admission — enforces resource request/limit constraints per namespace to prevent resource exhaustion | SC-5 | CIS Control 4 |
| SYS-CMP-APP | `plugin/pkg/admission/namespace/` (autoprovision, exists, lifecycle) | Material | Implements namespace admission plugins — enforces namespace existence checks and prevents operations on terminating namespaces for access isolation | AC-6 | CIS Control 4 |
| SYS-CMP-APP | `plugin/pkg/admission/network/` | Material | Implements network admission validation — validates NetworkPolicy resources at creation and update time for correct policy specification | SC-7 | CIS K8s 5.3; CIS Control 4 |
| SYS-CMP-APP | `plugin/pkg/admission/nodedeclaredfeatures/` | Material | Implements node declared features admission — validates node feature declarations for consistency with configured feature gates | CM-7 | CIS Control 4 |
| SYS-CMP-APP | `plugin/pkg/admission/noderestriction/` | Material | Implements NodeRestriction admission — prevents kubelets from modifying resources outside their own node scope, enforcing node-level access control | AC-6 | CIS K8s 5.1; CIS Control 6 |
| SYS-CMP-APP | `plugin/pkg/admission/nodetaint/` | Material | Implements node taint admission — automatically taints nodes at registration to control workload scheduling during initialization | CM-7 | CIS Control 4 |
| SYS-CMP-APP | `plugin/pkg/admission/podnodeselector/` | Material | Implements PodNodeSelector admission — restricts pod scheduling to specific nodes based on namespace annotations, enforcing scheduling-based access control | AC-6 | CIS Control 4 |
| SYS-CMP-APP | `plugin/pkg/admission/podtolerationrestriction/` | Material | Implements pod toleration restriction — limits which tolerations pods can specify per namespace to prevent privilege escalation through scheduling | CM-7 | CIS Control 4 |
| SYS-CMP-APP | `plugin/pkg/admission/podtopologylabels/` | Material | Implements pod topology label admission — validates and injects topology spread constraints for workload distribution governance | CM-7 | CIS Control 4 |
| SYS-CMP-APP | `plugin/pkg/admission/priority/` | Material | Implements priority class admission — validates PriorityClass references and prevents unauthorized use of system-critical priority classes | CM-7 | CIS Control 4 |
| SYS-CMP-APP | `plugin/pkg/admission/resourcequota/` | Material | Implements ResourceQuota admission — enforces per-namespace resource consumption limits to prevent resource exhaustion across tenants | SC-5 | CIS Control 4 |
| SYS-CMP-APP | `plugin/pkg/admission/runtimeclass/` | Material | Implements RuntimeClass admission — validates runtime class references and injects overhead/scheduling requirements for container runtime selection | CM-7 | CIS K8s 4.2; CIS Control 4 |
| SYS-CMP-APP | `plugin/pkg/admission/security/` | Material | Implements pod security admission — enforces Pod Security Standards (Privileged/Baseline/Restricted) at the namespace level | SC-7 | CIS K8s 5.2; CIS Control 4 |
| SYS-CMP-APP | `plugin/pkg/admission/serviceaccount/` | Material | Implements ServiceAccount admission — auto-mounts service account tokens, validates referenced service accounts exist, and enforces pull secret injection | IA-4, SC-28 | CIS K8s 5.1; CIS Control 5, 18 |
| SYS-CMP-APP | `plugin/pkg/admission/storage/` | Material | Implements storage admission plugins — validates PersistentVolumeClaim bindings, StorageClass references, and CSI driver configurations for storage governance | SC-28 | CIS K8s 5.4; CIS Control 4 |
| SYS-CMP-CFG | Admission webhook configurations, enabled/disabled plugin lists, Pod Security Standards namespace labels | Material | Defines which admission plugins are active and their configuration — misconfiguration can disable critical policy enforcement | CM-7, SI-10 | CIS K8s 5.2; CIS Control 4 |
| SYS-CMP-API | `pkg/apis/admission/` (types.go, v1/, v1beta1/) | Material | Defines Admission Review API types — the contract for webhook-based admission processing | CM-7, SI-10 | CIS K8s 5.2; CIS Control 4 |
| SYS-CMP-API | `pkg/apis/admissionregistration/` (types.go, v1/, v1alpha1/, v1beta1/, validation/) | Material | Defines MutatingWebhookConfiguration and ValidatingWebhookConfiguration API types — governs dynamic admission control registration | CM-7, SI-10 | CIS K8s 5.2; CIS Control 4 |
| SYS-CMP-API | `pkg/apis/imagepolicy/` | Material | Defines ImageReview API types — the contract for image policy webhook admission decisions | SI-7 | CIS K8s 4.2; CIS Control 7 |

### 2.9 Data Persistence (DAT) Systems

| system_id | component_path | classification | materiality_rationale | governing_NIST_control | governing_CIS_control |
|---|---|---|---|---|---|
| SYS-DAT-ORC | `pkg/controller/volume/` | Material | Orchestrates PersistentVolume lifecycle — provisions, binds, and reclaims persistent storage at runtime based on workload claims | SC-28, CP-9 | CIS K8s 2; CIS Control 4 |
| SYS-DAT-APP | `pkg/volume/` (configmap, csi, csimigration, downwardapi, emptydir, fc, flexvolume, hostpath, iscsi, local, and more) | Material | Implements volume plugin mount/unmount operations — directly handles data access paths at runtime during pod lifecycle events | SC-28 | CIS K8s 5.4; CIS Control 4 |
| SYS-DAT-CFG | StorageClass definitions, PV reclaim policies, CSI driver configuration | Material | Defines data persistence policies including reclaim behavior (Retain/Delete/Recycle) and CSI driver parameters — governs data lifecycle governance | SC-28 | CIS K8s 2, 5.4; CIS Control 4 |
| SYS-DAT-API | `pkg/apis/storage/` (types.go, v1/, v1beta1/, validation/) | Material | Defines StorageClass, VolumeAttachment, CSIDriver, CSINode API types — the contract for all storage resource management | SC-28 | CIS K8s 2; CIS Control 4 |
| SYS-DAT-DTA | `pkg/registry/storage/`, `pkg/registry/core/persistentvolume/`, `pkg/registry/core/persistentvolumeclaim/` | Material | Implements etcd storage for PV, PVC, StorageClass resources — the persistence layer for all storage state and configuration | SC-28, CP-9 | CIS K8s 2; CIS Control 4 |

### 2.10 External Integrations (EXT) Systems

| system_id | component_path | classification | materiality_rationale | governing_NIST_control | governing_CIS_control |
|---|---|---|---|---|---|
| SYS-EXT-ORC | `cmd/cloud-controller-manager/` | Material | Orchestrates cloud controller manager — communicates with external cloud provider APIs for node management, route management, and service load balancer provisioning | IA-8, SA-9 | CIS K8s 1.2; CIS Control 1, 6 |
| SYS-EXT-APP | `pkg/credentialprovider/` (shared with SYS-SEC-APP) | Material | Implements external credential provider interface — makes outbound calls to external credential management services at runtime for image pull authentication | IA-8, SC-8 | CIS K8s 4.2; CIS Control 1, 6 |
| SYS-EXT-CFG | Cloud provider configuration, webhook endpoint URLs, CRI/CNI endpoint settings | Material | Defines external integration connection parameters — misonfiguration can expose authentication credentials or enable unauthorized external access | IA-8, SA-9 | CIS K8s 1.2; CIS Control 1 |
| SYS-EXT-DEP | External integration dependencies in `go.mod` (cloud-provider libraries, CRI client, CNI plugins) | Material | Pins external integration library versions — compromised external integration dependencies directly impact authentication and communication integrity | SA-9, CM-7 | CIS Control 1, 2 |
| SYS-EXT-API | Cloud provider API contracts, webhook API interfaces, CRI/CNI interface definitions | Material | Defines external communication interface contracts — structural integrity ensures correct external system interaction | IA-8, SC-8 | CIS K8s 1.2; CIS Control 1, 6 |

### 2.11 Cross-Cutting Components

The following components span multiple systems and are classified as Material based on their cross-cutting security impact (shared by 3+ systems).

| system_id | component_path | classification | materiality_rationale | governing_NIST_control | governing_CIS_control |
|---|---|---|---|---|---|
| Cross-cutting | `pkg/controller/` | Material | Controller framework shared by IAM (ServiceAccount controller), SEC (secret controller), DAT (volume controller), RUN (deployment, replicaset, namespace, node controllers), and CMP (garbage collection) — cross-cutting concern impacting 5+ verticals | CM-7 | CIS Control 4 |
| Cross-cutting | `pkg/util/` | Material | Shared utilities consumed across all 10 verticals — includes IP validation, string helpers, mount utilities, certificate helpers, and system resource management | CM-7 | CIS Control 2 |
| Cross-cutting | `pkg/registry/` | Material | Resource storage framework consumed by IAM (RBAC storage), SEC (secret storage), DAT (PV/PVC storage), RUN (deployment/pod storage), and CMP (admission review storage) — cross-cutting data access layer | SC-28 | CIS Control 4 |
| Cross-cutting | `pkg/api/` | Material | Internal API helpers and conversion utilities consumed by IAM, SEC, CMP, DAT, and RUN systems — cross-cutting API layer | CM-7 | CIS Control 4 |

---

## 3. Non-Material Components

The following component categories are classified as **Non-Material** and are excluded from Directives 3–6. Each exclusion category includes the rationale for non-materiality.

### 3.1 Non-Material Component Inventory

| component_category | component_path_pattern | classification | exclusion_rationale |
|---|---|---|---|
| Vendored dependencies | `vendor/` | Non-Material | Auto-generated mirror of upstream Go module dependencies; not authored within this repository; changes are managed through `hack/update-vendor.sh` |
| Third-party code | `third_party/` | Non-Material | External code maintained by other projects; not subject to Kubernetes project governance |
| Staging modules | `staging/src/k8s.io/` | Non-Material | Maintained as separate repositories (e.g., k8s.io/apiserver, k8s.io/client-go); referenced for context in this audit but audited independently |
| Test files | `**/*_test.go`, `test/`, `testdata/` | Non-Material | Test fixtures and scaffolding that do not execute in production and have no influence over security control surfaces |
| Changelog files | `CHANGELOG/` | Non-Material | Historical release notes with no runtime security function |
| Logo assets | `logo/` | Non-Material | Visual branding assets with no security relevance |
| License files | `LICENSES/` | Non-Material | Legal compliance artifacts with no runtime security function |
| Generated code | `**/zz_generated.*.go` | Non-Material | Auto-generated deepcopy, conversion, and defaults code produced by code-gen tooling; reflects upstream type definitions |
| Generated protobuf | `**/generated.pb.go` | Non-Material | Auto-generated Protocol Buffer serialization code; reflects proto definitions |
| Documentation generators | `cmd/gendocs/` | Non-Material | Produces kubectl command reference documentation; no runtime security control surface |
| Documentation generators | `cmd/genkubedocs/` | Non-Material | Produces kube-apiserver/kube-controller-manager/kube-scheduler/kubelet documentation; no runtime security function |
| Documentation generators | `cmd/genman/` | Non-Material | Produces man pages; no runtime security function |
| Documentation generators | `cmd/genyaml/` | Non-Material | Produces YAML reference documentation; no runtime security function |
| Documentation generators | `cmd/genfeaturegates/` | Non-Material | Produces feature gate documentation; no runtime security function (feature gates themselves are Material under SYS-RUN-CFG) |
| Documentation generators | `cmd/genswaggertypedocs/` | Non-Material | Produces Swagger type documentation annotations; no runtime security function |
| Utility commands | `cmd/gotemplate/` | Non-Material | Go template rendering utility; build tooling with no production security function |
| Utility commands | `cmd/prune-junit-xml/` | Non-Material | JUnit XML test result pruning; CI helper with no production security function |
| Analysis commands | `cmd/clicheck/` | Non-Material | CLI compliance checking tool; no runtime security function |
| Analysis commands | `cmd/dependencycheck/` | Non-Material | Dependency verification tool; build-time utility (dependency governance itself is Material under SYS-CCD-DEP) |
| Analysis commands | `cmd/dependencyverifier/` | Non-Material | Dependency version verification; build-time utility |
| Analysis commands | `cmd/fieldnamedocscheck/` | Non-Material | Field name documentation checker; build-time analysis tool |
| Analysis commands | `cmd/import-boss/` | Non-Material | Import restriction enforcement; build-time analysis tool |
| Analysis commands | `cmd/importverifier/` | Non-Material | Import verification tool; build-time analysis tool |
| Analysis commands | `cmd/preferredimports/` | Non-Material | Preferred imports enforcement; build-time analysis tool |
| Utility packages | `pkg/printers/` | Non-Material | Output formatting utilities for kubectl; display-only, no security control surface |
| Utility packages | `pkg/fieldpath/` | Non-Material | Field path resolution for downward API; utility with no direct security governance |
| Utility packages | `pkg/capabilities/` | Non-Material | Deprecated capabilities management; minimal surface area, no active security enforcement |
| Utility packages | `pkg/windows/` | Non-Material | Windows-specific utility functions; platform compatibility helpers with no security governance |
| Kubemark | `cmd/kubemark/`, `pkg/kubemark/` | Non-Material | Kubemark simulation tool for performance testing; not a production component |
| kubectl-convert | `cmd/kubectl-convert/` | Non-Material | API version conversion utility; no runtime security function |
| Kubeadm | `cmd/kubeadm/` | Material | (Exception — see below) |

> **Note on kubeadm:** `cmd/kubeadm/` is classified as **Material** under SYS-RUN-ORC because it performs cluster bootstrap operations including certificate generation, kubelet configuration, etcd initialization, and control plane component deployment. These operations directly govern the security posture of new clusters (NIST IA-5, CM-2; CIS K8s Section 1).

### 3.2 Boundary Cases

The following components were evaluated and required explicit determination:

| component_path | classification | determination_rationale |
|---|---|---|
| `cmd/kubeadm/` | **Material** | Cluster bootstrap tool generates CA certificates, configures authentication, and initializes RBAC — directly governs initial security posture (IA-5, CM-2) |
| `cmd/kubectl/` | **Material** | CLI tool that authenticates to the API server, processes kubeconfig credentials, and executes RBAC-governed operations — governs access control at the user interface layer (AC-3, IA-2) |
| `pkg/securitycontext/` | **Material** | Security context accessor utilities used by admission plugins and kubelet to evaluate pod/container security context settings — cross-cutting security concern (SC-7) |
| `pkg/client/` | **Material** | Internal client libraries used by controllers and components to authenticate and communicate with the API server — governs internal service-to-service authentication (IA-2, SC-8) |
| `pkg/cluster/` | **Material** | Cluster-level configuration utilities consumed by multiple systems — cross-cutting configuration concern (CM-6) |
| `pkg/generated/openapi/` | **Material** | Generated OpenAPI definitions compiled into the API server; while generated, they define the runtime API contract and directly influence request validation (CM-6) |
| `cmd/genutils/` | Non-Material | Shared utility functions for documentation generators; no runtime security function |

---

## 4. Materiality Summary Statistics

### 4.1 Overall Classification

| metric | value |
|---|---|
| **Total systems in D0 registry** | **45** |
| Systems with Material components | 45 (100%) |
| Total Material component entries (vertical sections 2.1–2.10) | 119 |
| Total cross-cutting Material entries (section 2.11) | 4 |
| Total boundary case Material entries (section 3.2) | 5 |
| **Grand total Material component groups** | **128** |
| Total Non-Material component categories | 31 |

> **Note:** All 45 registered systems contain at least one Material component. This is expected because the D0 system registry excludes intersections with no substantive artifacts, and every registered intersection contains components that govern at least one of the 8 materiality criteria.

### 4.2 Material Components by System Vertical

| vertical | total_material_components | percentage_of_vertical_total |
|---|---|---|
| Identity/Access (IAM) | 26 | 21.8% |
| Compliance (CMP) | 31 | 26.1% |
| Application Runtime (RUN) | 20 | 16.8% |
| CI/CD (CCD) | 9 | 7.6% |
| Image Supply Chain (IMG) | 7 | 5.9% |
| Secret Management (SEC) | 6 | 5.0% |
| Network Policy (NET) | 5 | 4.2% |
| Observability (OBS) | 5 | 4.2% |
| Data Persistence (DAT) | 5 | 4.2% |
| External Integrations (EXT) | 5 | 4.2% |
| **Vertical subtotal** | **119** | **100%** |
| Cross-cutting (Section 2.11) | 4 | — |
| Boundary cases Material (Section 3.2) | 5 | — |
| **Grand total** | **128** | — |

### 4.3 Material Components by NIST SP 800-53 Control Family

| NIST_control_family | material_components_governed | primary_controls_referenced |
|---|---|---|
| AC (Access Control) | 26 | AC-2, AC-3, AC-6 |
| AU (Audit and Accountability) | 7 | AU-2, AU-3, AU-6, AU-12 |
| CM (Configuration Management) | 52 | CM-2, CM-3, CM-6, CM-7, CM-9 |
| CP (Contingency Planning) | 2 | CP-9 |
| IA (Identification and Authentication) | 20 | IA-2, IA-4, IA-5, IA-8 |
| SA (System and Services Acquisition) | 12 | SA-9, SA-10 |
| SC (System and Communications Protection) | 24 | SC-3, SC-5, SC-7, SC-8, SC-12, SC-28 |
| SI (System and Information Integrity) | 14 | SI-2, SI-3, SI-7, SI-10 |

> **Note:** Components may be governed by multiple control families. Totals exceed the number of unique Material components because a single component may satisfy multiple control objectives (e.g., `plugin/pkg/admission/serviceaccount/` governs both IA-4 and SC-28).

### 4.4 Material Components by CIS Framework

| CIS_framework | material_components_governed | primary_sections_controls |
|---|---|---|
| CIS K8s Benchmark — Section 1 (Control Plane) | 12 | 1.1, 1.2, 1.3, 1.4 |
| CIS K8s Benchmark — Section 2 (etcd) | 3 | 2 |
| CIS K8s Benchmark — Section 3 (Control Plane Config) | 5 | 3.1, 3.2 |
| CIS K8s Benchmark — Section 4 (Worker Nodes) | 14 | 4.1, 4.2 |
| CIS K8s Benchmark — Section 5 (Policies) | 22 | 5.1, 5.2, 5.3, 5.4 |
| CIS Controls v8 — Control 1 (Enterprise Assets) | 5 | Inventory and Control |
| CIS Controls v8 — Control 2 (Software Assets) | 8 | Inventory and Control |
| CIS Controls v8 — Control 4 (Secure Configuration) | 68 | Secure Configuration |
| CIS Controls v8 — Control 5 (Account Management) | 17 | Account Management |
| CIS Controls v8 — Control 6 (Access Control) | 14 | Access Control Management |
| CIS Controls v8 — Control 7 (Vulnerability Management) | 6 | Continuous Vulnerability Management |
| CIS Controls v8 — Control 8 (Audit Log Management) | 7 | Audit Log Management |
| CIS Controls v8 — Control 16 (Application Software Security) | 5 | Application Security |
| CIS Controls v8 — Control 18 (Application Software Security) | 9 | Secret/Penetration Testing |

### 4.5 Static vs. Dynamic Material Distribution

| system_classification | systems_count | total_material_components | avg_material_per_system |
|---|---|---|---|
| Static | 26 | 46 | 1.8 |
| Dynamic | 19 | 73 | 3.8 |
| **Vertical subtotal** | **45** | **119** | **2.6** |
| Cross-cutting | — | 4 | — |
| Boundary cases | — | 5 | — |
| **Grand total** | **45** | **128** | — |

> **Observation:** Dynamic systems average nearly twice the Material component density of Static systems. This reflects the nature of runtime enforcement — Dynamic systems implement active policy evaluation, request processing, and state reconciliation logic that require multiple cooperating components. Static systems typically define single configuration artifacts or type definitions.

---

## 5. Materiality Gating Declaration

Based on the classification above:

- **128 Material component groups** (119 vertical + 4 cross-cutting + 5 boundary cases) proceed to Directive 3 (Code Quality Audit), Directive 4 (Dependency Audit), Directive 5 (Documentation Coverage Audit), and Directive 6 (Accuracy Validation).
- **31 Non-Material component categories** are excluded from Directives 3–6.
- All Material components reference a `system_id` from the D0 registry (`00-system-registry.md`).
- All Material classifications are based on verified codebase functionality — no aspirational controls are documented.
- Where NIST SP 800-53 and CIS controls prescribe different requirements, the more restrictive control has been applied and conflicts are flagged in `appendix-framework-conflict-register.md`.

---

*Document generated as part of the Kubernetes codebase compliance audit. This is Directive 2 output — the materiality classification that gates all subsequent audit directives (D3–D6). See `00-system-registry.md` for system_id definitions and `appendix-cross-reference-index.md` for full traceability linkage.*
