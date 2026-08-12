# Appendix: Framework Conflict Register

**Kubernetes Monorepo Compliance Audit — `k8s.io/kubernetes`**
**Document Version:** 1.0
**Audit Scope:** NIST SP 800-53 Rev 5, NIST SP 800-190, NIST CSF, CIS Kubernetes Benchmark v1.12.0, CIS Controls v8 IG2/IG3
**Authority Hierarchy:** NIST SP 800-53 Rev 5 > NIST SP 800-190 > CIS Kubernetes Benchmark v1.12.0 > CIS Controls v8

---

## 1. Methodology

### 1.1 Conflict Identification Process

This register documents all instances where NIST SP 800-53 Rev 5 controls and CIS controls (Kubernetes Benchmark v1.12.0 and/or CIS Controls v8) prescribe different or conflicting requirements for the same security objective within the Kubernetes codebase. The conflict identification methodology follows a systematic, three-step process:

1. **Control Mapping:** Each NIST SP 800-53 Rev 5 control (AC, AU, CM, IA, SC, SI families) is mapped to its closest CIS Kubernetes Benchmark check(s) and CIS Controls v8 sub-control(s) that address the same security objective.

2. **Divergence Analysis:** For each mapped pair, the prescriptions are compared across three dimensions:
   - **Specificity:** Does one framework provide generic guidance while the other prescribes exact configurations?
   - **Restrictiveness:** Does one framework require a higher security posture or stricter enforcement?
   - **Approach:** Do the frameworks prescribe fundamentally different mechanisms to achieve the same objective?

3. **Resolution:** Every identified conflict is resolved to the **more restrictive** control. The resolution rationale documents why one prescription imposes a higher security bar than the other. Where both frameworks address the same objective but at different abstraction levels (e.g., principle-based vs. configuration-specific), the more prescriptive and operationally verifiable control is considered more restrictive.

### 1.2 Framework Authority Hierarchy

When NIST and CIS controls conflict, the following authority hierarchy determines precedence:

| Priority | Framework | Role |
|---|---|---|
| 1 (Highest) | NIST SP 800-53 Rev 5 | Primary control catalog — AC, AU, CM, IA, SC, SI families |
| 2 | NIST SP 800-190 | Container-specific risk guidance — image, registry, orchestrator, container, host OS |
| 3 | CIS Kubernetes Benchmark v1.12.0 | Kubernetes-specific hardening — Sections 1–5 |
| 4 (Lowest) | CIS Controls v8 IG2/IG3 | Enterprise security controls — Controls 1, 2, 4, 5, 6, 7, 8, 16, 18 |

**Resolution Rule:** Apply the more restrictive control regardless of hierarchy position. The hierarchy is used only to determine precedence when controls are equally restrictive but prescribe different approaches — in such cases, the higher-priority framework's approach is adopted.

### 1.3 Scope and Applicability

- This register covers only conflicts relevant to controls verified or verifiable in the Kubernetes codebase (`k8s.io/kubernetes`).
- No aspirational controls are included — every conflict documented can be traced to actual framework text and actual codebase components.
- All directive documents (D0–D7) reference this register when framework conflicts are encountered in their respective assessments.
- Conflict IDs follow the convention `CFR-{CONTROL_FAMILY}-{SEQUENCE}` (e.g., `CFR-AC-001`).

### 1.4 Affected Systems Reference

Affected systems are referenced using system_ids from the System Registry (Directive 0, `00-system-registry.md`). The system_id naming convention is `SYS-{VERTICAL_ABBREV}-{HORIZONTAL_ABBREV}` where verticals represent functional domains and horizontals represent architectural layers.

---

## 2. Master Conflict Table

The following table provides a consolidated view of all identified conflicts. Detailed analysis for each conflict is provided in subsequent sections.

| conflict_id | NIST_control | NIST_prescription | CIS_control | CIS_prescription | more_restrictive | resolution_rationale | affected_systems |
|---|---|---|---|---|---|---|---|
| CFR-AC-001 | AC-6 (Least Privilege) | Employ the principle of least privilege, allowing only authorized accesses for users and processes which are necessary to accomplish assigned organizational tasks | CIS K8s 5.1.1–5.1.9 | Specific RBAC checks: minimize cluster-admin usage (5.1.1), minimize access to secrets (5.1.2), minimize wildcard use in Roles/ClusterRoles (5.1.3), minimize access to create pods (5.1.4), ensure default SA is not actively used (5.1.5), ensure SA tokens are only mounted where necessary (5.1.6) | CIS K8s 5.1.x | CIS prescribes specific, operationally verifiable RBAC configurations that enforce least privilege at the Kubernetes API level, while NIST AC-6 states the principle without prescribing Kubernetes-specific enforcement mechanisms | SYS-IAM-APP, SYS-IAM-ORC, SYS-IAM-API |
| CFR-AC-002 | AC-3 (Access Enforcement) | Enforce approved authorizations for logical access to information and system resources in accordance with applicable access control policies | CIS Control 6 (Access Control Management) | Establish access granting process (6.1), establish access revoking process (6.2), require MFA for externally-exposed apps (6.3), require MFA for remote access (6.4), require MFA for admin access (6.5) | NIST AC-3 | NIST AC-3 requires system-enforced authorization at runtime — the system itself must enforce access decisions. CIS Control 6 focuses on administrative process (granting/revoking) rather than runtime enforcement. NIST demands the enforcement mechanism exists in code | SYS-IAM-APP, SYS-IAM-ORC |
| CFR-AC-003 | AC-6(1) (Authorize Access to Security Functions) | Authorize access to security-relevant functions for specifically designated personnel only | CIS K8s 5.1.6 | Ensure that Service Account tokens are only mounted where necessary — explicitly restrict auto-mounting of SA tokens | CIS K8s 5.1.6 | CIS K8s 5.1.6 prescribes a specific, testable restriction (disable auto-mounting of ServiceAccount tokens) that is more operationally restrictive than the broad NIST AC-6(1) requirement for designated-personnel-only access to security functions | SYS-IAM-APP, SYS-IAM-API |
| CFR-IA-001 | IA-2 (Identification and Authentication) | Uniquely identify and authenticate organizational users, processes, or devices. IA-2(1): Implement multi-factor authentication for privileged accounts. IA-2(2): Implement MFA for non-privileged accounts | CIS K8s 3.1.1 | Ensure client certificate authentication is configured for the API server (--client-ca-file argument is set) | NIST IA-2 | NIST IA-2 with enhancements (1) and (2) requires multi-factor authentication, which is a stronger authentication assurance than client certificate authentication alone as prescribed by CIS K8s 3.1.1. MFA requires two or more distinct factors; client certificates provide only one factor (something you have) | SYS-IAM-APP, SYS-IAM-ORC, SYS-IAM-CFG |
| CFR-IA-002 | IA-5 (Authenticator Management) | Manage system authenticators: verify identity before issuing, enforce complexity and lifetime, protect against unauthorized disclosure, require change on suspected compromise | CIS K8s 5.1.5, 5.1.6 | Ensure default ServiceAccount is not actively used (5.1.5); Ensure SA tokens are only mounted where necessary (5.1.6) | CIS K8s 5.1.5/5.1.6 | CIS prescribes specific, Kubernetes-native token management restrictions (disabling default SA usage, restricting token auto-mount) that are more prescriptive than NIST IA-5's general authenticator lifecycle requirements. For the Kubernetes SA token context, the CIS checks are directly testable and operationally more restrictive | SYS-IAM-APP, SYS-IAM-API, SYS-IAM-CFG |
| CFR-CM-001 | CM-6 (Configuration Settings) | Establish and document configuration settings for components using common secure configuration guidance; identify, document, and approve deviations | CIS K8s 1.1.x–4.2.x | Specific configuration checks: API server process file permissions (1.1.1–1.1.4, 600/644), scheduler config file permissions (1.1.5–1.1.8), controller-manager config (1.1.9–1.1.12), etcd file permissions (1.1.19–1.1.22), kubelet file permissions (4.1.1–4.1.10), specific API server flags (1.2.x), kubelet flags (4.2.x) | CIS K8s 1.1.x–4.2.x | CIS Kubernetes Benchmark prescribes exact file permission modes (e.g., 600 for scheduler.conf, 644 for API server pod spec), specific flag values (e.g., --anonymous-auth=false, --profiling=false), and testable configuration checks. NIST CM-6 requires configuration management but does not prescribe Kubernetes-specific settings | SYS-RUN-ORC, SYS-RUN-CFG, SYS-IAM-CFG |
| CFR-CM-002 | CM-3 (Configuration Change Control) | Determine types of changes under configuration control; review and approve proposed changes; document decisions; retain records of changes; audit and review activities | CIS Control 4 (Secure Configuration of Enterprise Assets and Software) | Establish and maintain secure configuration process (4.1), establish and maintain network infrastructure configuration (4.2), configure automatic session locking (4.3), implement and manage firewall (4.4–4.5) | NIST CM-3 | NIST CM-3 requires a formal, documented change control process with approval workflows, audit trails, and post-change review. CIS Control 4 focuses on establishing secure configurations but does not prescribe the same rigor for change approval and documentation | SYS-CCD-PIP, SYS-CCD-CFG, SYS-CCD-DEP |
| CFR-CM-003 | CM-7 (Least Functionality) | Configure the system to provide only mission-essential capabilities; prohibit or restrict use of unauthorized functions, ports, protocols, and services | CIS K8s 1.2.x (API Server Configuration) | Specific API server flag requirements: disable profiling (1.2.18), disable AlwaysAdmit (1.2.11), enable admission controllers (1.2.12–1.2.14), set audit log parameters (1.2.15–1.2.17), restrict authorization modes (1.2.7–1.2.8) | CIS K8s 1.2.x | CIS K8s 1.2.x prescribes specific flags and admission controller settings that operationally enforce least functionality for the Kubernetes API server. NIST CM-7 states the principle of least functionality without enumerating Kubernetes-specific flags | SYS-RUN-ORC, SYS-CMP-APP, SYS-CMP-ORC |
| CFR-AU-001 | AU-2 / AU-3 / AU-12 | AU-2: Define auditable events. AU-3: Audit records must contain what event occurred, when, where, source, outcome, and identity. AU-12: Generate audit records for defined auditable events with AU-3 content | CIS K8s 3.2.1–3.2.2 | 3.2.1: Ensure that a minimal audit policy is created. 3.2.2: Ensure that the audit policy covers key security concerns (e.g., access to Secrets, changes to ConfigMaps, authentication failures) | NIST AU-2/AU-3/AU-12 | NIST AU-3 requires specific content fields in every audit record (who, what, when, where, source, outcome) which is more prescriptive than CIS K8s 3.2.1's requirement for a "minimal audit policy." NIST AU-2 also requires documented event selection rationale, which CIS does not mandate | SYS-OBS-APP, SYS-OBS-CFG |
| CFR-AU-002 | AU-9 (Protection of Audit Information) | Protect audit information and audit logging tools from unauthorized access, modification, and deletion | CIS Control 8.1–8.5 | 8.1: Establish and maintain audit log management process. 8.2: Collect audit logs. 8.3: Ensure adequate audit log storage. 8.5: Collect detailed audit logs | NIST AU-9 | NIST AU-9 extends protection to the audit tools themselves (not just the logs) and requires protection against modification and deletion. CIS Control 8 focuses on collection, storage, and process management but does not explicitly require protection of the auditing tools from compromise | SYS-OBS-APP, SYS-OBS-CFG, SYS-OBS-ORC |
| CFR-SI-001 | SI-3 (Malicious Code Protection) | Implement malicious code protection mechanisms at system entry and exit points; update mechanisms when new releases are available; configure to perform periodic and real-time scans | CIS K8s 4.2.x (Worker Node Configuration) | Ensure container configuration file permissions (4.2.1–4.2.13): restrict kubelet, proxy configs, and restrict specific kubelet parameters (streaming connections, make-iptables-util-chains, hostname-override) | NIST SI-3 | NIST SI-3 requires active malicious code detection and protection mechanisms (scanning, real-time protection), which is a broader and more restrictive requirement than CIS K8s 4.2.x's focus on file permission hardening and configuration parameter restrictions. SI-3 requires runtime protection, not just secure configuration | SYS-CMP-APP, SYS-CMP-ORC, SYS-RUN-ORC |
| CFR-SI-002 | SI-10 (Information Input Validation) | Check the validity of information inputs: syntax, length, range, content, and authorized values | CIS Control 16 (Application Software Security) | Establish and maintain a secure application development process (16.1), perform root cause analysis on security vulnerabilities (16.2), perform code reviews (16.3), use up-to-date compiler features (16.9), apply secure design principles (16.10) | NIST SI-10 | NIST SI-10 requires runtime input validation at the point of processing — every external input must be checked for validity before use. CIS Control 16 focuses on the development lifecycle process (code reviews, secure design) rather than mandating specific runtime validation behavior. SI-10's runtime enforcement requirement is more restrictive than CIS 16's process-based approach | SYS-CMP-APP, SYS-CMP-ORC, SYS-IAM-APP |
| CFR-CS-001 | SP 800-190 (Image Risks) | Address image vulnerabilities (known CVEs), image configuration defects, embedded malware, embedded cleartext secrets, and use of untrusted images. Requires image scanning, provenance validation, and secret exclusion from image layers | CIS K8s 4.2.x (Container Configuration) | Ensure container-related configuration file ownership and permissions are correctly set (4.2.1–4.2.13); focused on file-level security of container runtime configuration | SP 800-190 | NIST SP 800-190 addresses the full image lifecycle (build, scan, sign, verify, runtime) including malware detection, secret exclusion, and provenance validation. CIS K8s 4.2.x focuses narrowly on container configuration file permissions after deployment. SP 800-190's comprehensive image risk coverage is substantially more restrictive | SYS-IMG-IAC, SYS-IMG-DEP, SYS-IMG-PIP |
| CFR-CS-002 | SP 800-190 (Orchestrator Risks) | Address unbounded administrative access to orchestrator, unauthorized access to inter-container network traffic, mixing of workload sensitivity levels, and orchestrator node trust | CIS K8s Section 1 (Control Plane Components) | Specific API server pod specification file permissions (1.1.1–1.1.4), API server configuration flags (1.2.1–1.2.25), controller manager arguments (1.3.1–1.3.7), scheduler arguments (1.4.1–1.4.2) | CIS K8s Section 1 | CIS K8s Section 1 prescribes specific, testable control plane configurations (file permissions, flag values, argument settings) for the Kubernetes orchestrator. While SP 800-190 identifies broader orchestrator risks, CIS K8s Section 1 provides more operationally restrictive and verifiable hardening checks for the Kubernetes control plane | SYS-RUN-ORC, SYS-RUN-CFG, SYS-IAM-ORC |
| CFR-CS-003 | SP 800-190 (Registry Risks) | Address insecure connections to registries, stale images in registries, and insufficient authentication/authorization to registries | CIS K8s 5.1.1–5.1.2 | Minimize use of cluster-admin role (5.1.1), minimize access to secrets (5.1.2) — indirectly governs registry credential access via RBAC | SP 800-190 | SP 800-190 directly addresses registry security (secure connections, image freshness, registry authentication) while CIS K8s 5.1 only indirectly addresses registry access through RBAC minimization. SP 800-190's direct registry risk coverage is more restrictive for container registry governance | SYS-IMG-IAC, SYS-IMG-DEP |
| CFR-SC-001 | SC-7 (Boundary Protection) | Monitor and control communications at the external managed interfaces; implement subnetworks for publicly accessible components; connect to external networks only through managed interfaces | CIS K8s 5.3.1–5.3.2 | 5.3.1: Ensure that the CNI in use supports NetworkPolicies. 5.3.2: Ensure that all Namespaces have NetworkPolicies defined — prescribes default-deny posture via NetworkPolicy | CIS K8s 5.3.x | CIS K8s 5.3.2 prescribes a default-deny NetworkPolicy posture for all namespaces, which is more operationally restrictive than NIST SC-7's general boundary protection requirement. The default-deny-all mandate is a specific, testable configuration that exceeds the general principle of managed interface communications | SYS-NET-APP, SYS-NET-API, SYS-NET-CFG |
| CFR-SC-002 | SC-8 (Transmission Confidentiality and Integrity) | Protect the confidentiality and integrity of transmitted information using cryptographic mechanisms | CIS K8s 1.2.x (API Server TLS) | Ensure TLS is configured for API server communication: --tls-cert-file and --tls-private-key-file are set (1.2.22–1.2.25); ensure etcd TLS is configured (2.1–2.6) | CIS K8s 1.2/2.x | CIS K8s prescribes specific TLS certificate configuration flags for the API server and etcd, including mutual TLS for etcd peer communication (2.4–2.6). These are more operationally restrictive and testable than NIST SC-8's general cryptographic protection requirement, which does not specify Kubernetes-specific TLS configurations | SYS-NET-CFG, SYS-RUN-ORC, SYS-RUN-CFG |

---

## 3. Access Control Conflicts (NIST AC vs. CIS K8s 5.1 / CIS Control 5–6)

### 3.1 CFR-AC-001: Least Privilege — Broad Principle vs. Specific RBAC Checks

**NIST SP 800-53 Rev 5 — AC-6 (Least Privilege):**
NIST AC-6 requires organizations to employ the principle of least privilege, allowing only authorized accesses for users and processes necessary to accomplish assigned tasks. The control addresses privilege assignment broadly and includes enhancements for authorizing access to security functions (AC-6(1)), non-privileged access for non-security functions (AC-6(2)), and network access to privileged commands (AC-6(3)). AC-6 is principle-based and does not prescribe Kubernetes-specific enforcement mechanisms.

**CIS Kubernetes Benchmark v1.12.0 — Section 5.1 (RBAC and Service Accounts):**
CIS K8s 5.1 prescribes specific, testable RBAC configurations:
- 5.1.1: Ensure that the cluster-admin role is only used where required
- 5.1.2: Minimize access to secrets
- 5.1.3: Minimize wildcard use in Roles and ClusterRoles
- 5.1.4: Minimize access to create pods
- 5.1.5: Ensure that default ServiceAccounts are not actively used
- 5.1.6: Ensure that Service Account Tokens are only mounted where necessary
- 5.1.8: Limit use of the Bind, Impersonate, and Escalate permissions in the Kubernetes cluster

**Resolution:** CIS K8s 5.1.x is **more restrictive**.

**Rationale:** CIS K8s 5.1.1–5.1.8 translates the least privilege principle into specific, operationally verifiable checks for Kubernetes RBAC. Each check has a defined pass/fail criterion (e.g., no wildcard permissions in ClusterRoles, no default SA active use). NIST AC-6 states the principle without enumerating these Kubernetes-specific enforcement mechanisms. The CIS checks are directly auditable against the Kubernetes RBAC configuration observed in `plugin/pkg/auth/authorizer/rbac/bootstrappolicy/policy.go` and `pkg/apis/rbac/` types.

**Source:** `plugin/pkg/auth/authorizer/rbac/bootstrappolicy/policy.go` — Bootstrap RBAC policies defining default ClusterRoles
**Source:** `pkg/apis/rbac/` — RBAC API type definitions (Role, ClusterRole, RoleBinding, ClusterRoleBinding)

**Affected Systems:** SYS-IAM-APP, SYS-IAM-ORC, SYS-IAM-API

---

### 3.2 CFR-AC-002: Access Enforcement — Runtime Enforcement vs. Process Management

**NIST SP 800-53 Rev 5 — AC-3 (Access Enforcement):**
NIST AC-3 requires the information system to enforce approved authorizations for logical access to information and system resources in accordance with applicable access control policies. This is a system-level enforcement requirement — the system itself must make and enforce authorization decisions at runtime.

**CIS Controls v8 — Control 6 (Access Control Management):**
CIS Control 6 addresses access control management through organizational processes: establishing access granting processes (6.1), access revoking processes (6.2), requiring MFA for externally-exposed applications (6.3), for remote network access (6.4), and for administrative access (6.5). CIS Control 6 is process-oriented rather than system-enforcement-oriented.

**Resolution:** NIST AC-3 is **more restrictive**.

**Rationale:** NIST AC-3 mandates that the system itself — not just organizational processes — enforces access control decisions. In the Kubernetes context, this means the authorization chain (Node → RBAC → Webhook → ABAC → Default Deny) as implemented in `pkg/kubeapiserver/authorizer/config.go` must enforce access decisions at every API request. CIS Control 6 focuses on the administrative processes for granting and revoking access but does not mandate system-level runtime enforcement.

**Source:** `pkg/kubeapiserver/authorizer/config.go:82` — `func (config Config) New(...)` constructs the authorizer chain with Node, ABAC, RBAC, and Webhook authorizers
**Source:** `pkg/kubeapiserver/authorizer/config.go:103-139` — Authorization mode switch handling for Node, ABAC, RBAC authorizers

**Affected Systems:** SYS-IAM-APP, SYS-IAM-ORC

---

### 3.3 CFR-AC-003: Security Function Access — Broad Designation vs. Token Mount Restriction

**NIST SP 800-53 Rev 5 — AC-6(1) (Authorize Access to Security Functions):**
AC-6(1) requires organizations to authorize access to security-relevant functions and security-relevant information for specifically designated organizational personnel only.

**CIS Kubernetes Benchmark v1.12.0 — 5.1.6:**
CIS K8s 5.1.6 requires that Service Account tokens are only mounted where necessary, effectively restricting automatic credential injection into pods that do not require API access.

**Resolution:** CIS K8s 5.1.6 is **more restrictive**.

**Rationale:** CIS K8s 5.1.6 prescribes a specific, testable restriction — disable auto-mounting of ServiceAccount tokens by default and enable them only where explicitly required. This is a concrete, enforceable control that prevents credential sprawl in the Kubernetes pod environment. NIST AC-6(1) designates authorized personnel but does not prescribe a specific mechanism for restricting credential injection into workloads. In the Kubernetes ServiceAccount context, the CIS check produces a more restrictive security posture.

**Source:** `pkg/serviceaccount/` — ServiceAccount token generation and validation
**Source:** `pkg/kubeapiserver/authenticator/config.go:141-154` — ServiceAccount authentication configuration

**Affected Systems:** SYS-IAM-APP, SYS-IAM-API

---

## 4. Authentication Conflicts (NIST IA vs. CIS K8s 3.1)

### 4.1 CFR-IA-001: Authentication Strength — Multi-Factor vs. Client Certificates

**NIST SP 800-53 Rev 5 — IA-2 (Identification and Authentication):**
NIST IA-2 requires unique identification and authentication of organizational users, processes, and devices. Enhancement IA-2(1) requires multi-factor authentication for access to privileged accounts. Enhancement IA-2(2) requires multi-factor authentication for access to non-privileged accounts. Multi-factor authentication demands two or more distinct factors: something you know, something you have, or something you are.

**CIS Kubernetes Benchmark v1.12.0 — 3.1.1:**
CIS K8s 3.1.1 requires that client certificate authentication is configured for the API server (the `--client-ca-file` argument is set). Client certificate authentication provides a single authentication factor (something you have — the certificate and private key).

**Resolution:** NIST IA-2 is **more restrictive**.

**Rationale:** NIST IA-2 with enhancements (1) and (2) requires multi-factor authentication, which demands at least two independent authentication factors. Client certificate authentication as prescribed by CIS K8s 3.1.1 provides only a single factor (possession of the private key). The NIST MFA requirement imposes a higher authentication assurance level than the CIS single-factor certificate requirement. In the Kubernetes codebase, the authentication chain in `pkg/kubeapiserver/authenticator/config.go` supports multiple authenticators (x509, tokens, OIDC, webhooks), enabling MFA-capable configurations.

**Source:** `pkg/kubeapiserver/authenticator/config.go:128-131` — X509 client certificate authentication via `x509.NewDynamic()`
**Source:** `pkg/kubeapiserver/authenticator/config.go:107-248` — Full authentication chain supporting header, x509, token, JWT/OIDC, webhook authenticators

**Affected Systems:** SYS-IAM-APP, SYS-IAM-ORC, SYS-IAM-CFG

---

### 4.2 CFR-IA-002: Authenticator Management — Lifecycle Policy vs. Token Restrictions

**NIST SP 800-53 Rev 5 — IA-5 (Authenticator Management):**
NIST IA-5 requires organizations to manage information system authenticators by: verifying the identity of the individual receiving the authenticator, establishing initial authenticator content, ensuring authenticators have sufficient strength, implementing administrative procedures for initial distribution and for lost/compromised/damaged authenticators, changing default content, setting minimum/maximum lifetime restrictions, protecting authenticators from unauthorized disclosure and modification, and requiring individuals to take specific security safeguards for authenticators.

**CIS Kubernetes Benchmark v1.12.0 — 5.1.5, 5.1.6:**
CIS K8s 5.1.5 requires that default ServiceAccounts are not actively used. CIS K8s 5.1.6 requires that ServiceAccount tokens are only mounted where necessary. These checks target the specific Kubernetes ServiceAccount token lifecycle — preventing default credential usage and restricting automatic credential injection.

**Resolution:** CIS K8s 5.1.5/5.1.6 is **more restrictive** (for the Kubernetes ServiceAccount context).

**Rationale:** For the specific Kubernetes ServiceAccount token context, CIS K8s 5.1.5 and 5.1.6 are more prescriptive and restrictive than the general NIST IA-5 authenticator management requirements. CIS mandates disabling default SA usage and restricting token auto-mount — these are specific, testable Kubernetes-native restrictions. NIST IA-5's broader authenticator lifecycle requirements (strength, rotation, protection) are important but do not prescribe these Kubernetes-specific token restrictions. Note: NIST IA-5's lifecycle requirements remain applicable for non-ServiceAccount authenticators (x509 certificates, OIDC tokens, webhook tokens).

**Source:** `pkg/kubeapiserver/authenticator/config.go:141-154` — ServiceAccount authenticator configuration
**Source:** `pkg/serviceaccount/` — Token generation, validation, and lifecycle management

**Affected Systems:** SYS-IAM-APP, SYS-IAM-API, SYS-IAM-CFG

---

## 5. Configuration Management Conflicts (NIST CM vs. CIS Control 4 / CIS K8s 1–4)

### 5.1 CFR-CM-001: Configuration Settings — General Management vs. Specific Hardening Checks

**NIST SP 800-53 Rev 5 — CM-6 (Configuration Settings):**
NIST CM-6 requires organizations to establish and document configuration settings for components employed within the system using common secure configuration guidance as the basis. It requires identifying, documenting, and approving any deviations from established settings based on operational requirements.

**CIS Kubernetes Benchmark v1.12.0 — Sections 1.1.x through 4.2.x:**
CIS K8s prescribes exact configuration settings across all Kubernetes components:
- **1.1.1–1.1.22:** API server, scheduler, controller manager, and etcd pod specification and configuration file permissions (e.g., 600 for scheduler.conf, 644 for API server pod spec, specific ownership requirements)
- **1.2.1–1.2.25:** API server process arguments (e.g., `--anonymous-auth=false`, `--profiling=false`, `--audit-log-path` set, `--audit-log-maxage` ≥30, `--authorization-mode` includes RBAC)
- **1.3.1–1.3.7:** Controller manager arguments (e.g., `--terminated-pod-gc-threshold` set, `--profiling=false`, `--use-service-account-credentials=true`)
- **1.4.1–1.4.2:** Scheduler arguments (e.g., `--profiling=false`)
- **4.1.1–4.1.10:** Kubelet configuration file permissions (e.g., 600 for kubelet.conf)
- **4.2.1–4.2.13:** Kubelet configuration parameters

**Resolution:** CIS K8s 1.1.x–4.2.x is **more restrictive**.

**Rationale:** CIS K8s Benchmark prescribes exact file permission modes, specific flag values, and testable configuration parameters for every major Kubernetes component. Each check has a defined pass/fail criterion that can be automated (e.g., via kube-bench). NIST CM-6 requires configuration management with deviation documentation but does not enumerate Kubernetes-specific settings. In the Kubernetes deployment context, CIS provides the more operationally restrictive and verifiable configuration baseline.

**Source:** `cmd/kube-apiserver/` — API server binary entry point and configuration
**Source:** `cmd/kubelet/` — Kubelet binary configuration
**Source:** `cmd/kube-controller-manager/` — Controller manager configuration
**Source:** `cmd/kube-scheduler/` — Scheduler configuration

**Affected Systems:** SYS-RUN-ORC, SYS-RUN-CFG, SYS-IAM-CFG

---

### 5.2 CFR-CM-002: Configuration Change Control — Formal Process vs. Secure Configuration Establishment

**NIST SP 800-53 Rev 5 — CM-3 (Configuration Change Control):**
NIST CM-3 requires organizations to: determine types of changes to the system that are configuration-controlled, review proposed configuration-controlled changes and approve or disapprove with explicit consideration for security and privacy impact, document configuration change decisions, implement approved changes, retain records of changes, and audit and review configuration change control activities.

**CIS Controls v8 — Control 4 (Secure Configuration of Enterprise Assets and Software):**
CIS Control 4 requires establishing and maintaining a secure configuration process (4.1), establishing and maintaining network infrastructure management (4.2), configuring automatic session locking (4.3), implementing and managing host-based firewalls (4.4–4.5), and securely managing enterprise assets and software (4.6–4.12). CIS Control 4 focuses on establishing and maintaining secure configurations but does not prescribe formal change approval workflows.

**Resolution:** NIST CM-3 is **more restrictive**.

**Rationale:** NIST CM-3 requires a formal, documented change control process with explicit approval workflows, security impact analysis, decision documentation, and audit trail. CIS Control 4 requires establishing secure configuration processes but does not mandate the same rigor for change approval and post-change review. In the Kubernetes codebase context, the 51 `hack/verify-*.sh` scripts and the PR review process documented in `.github/PULL_REQUEST_TEMPLATE.md` implement aspects of both, but NIST CM-3's explicit approval and audit requirements are more restrictive.

**Source:** `hack/verify-*.sh` — 51 verification scripts implementing configuration verification gates
**Source:** `build/dependencies.yaml` — External dependency version pins (zeitgeist v0.5.4, CNI 1.9.0, CoreDNS 1.13.1)
**Source:** `.github/PULL_REQUEST_TEMPLATE.md` — PR review template

**Affected Systems:** SYS-CCD-PIP, SYS-CCD-CFG, SYS-CCD-DEP

---

### 5.3 CFR-CM-003: Least Functionality — Principle vs. Specific Flag Requirements

**NIST SP 800-53 Rev 5 — CM-7 (Least Functionality):**
NIST CM-7 requires organizations to configure the system to provide only mission-essential capabilities and to prohibit or restrict the use of unauthorized functions, ports, protocols, connectivity, and services.

**CIS Kubernetes Benchmark v1.12.0 — Section 1.2 (API Server Configuration):**
CIS K8s 1.2 prescribes specific API server flag requirements that enforce least functionality:
- 1.2.7–1.2.8: Restrict authorization modes (do not include AlwaysAllow)
- 1.2.11: Ensure AlwaysAdmit admission controller is not enabled
- 1.2.12–1.2.14: Ensure specific admission controllers are enabled (ServiceAccount, NamespaceLifecycle, NodeRestriction)
- 1.2.18: Ensure `--profiling` is set to false
- 1.2.19: Ensure `--audit-log-path` is set

**Resolution:** CIS K8s 1.2.x is **more restrictive**.

**Rationale:** CIS K8s 1.2.x enumerates specific flags, admission controllers, and configuration parameters that must be set to enforce least functionality for the Kubernetes API server. Each check produces a binary pass/fail result. NIST CM-7 states the principle of least functionality without prescribing Kubernetes-specific flags or settings. For the Kubernetes API server context, the CIS checks translate the NIST principle into operationally verifiable requirements.

**Source:** `cmd/kube-apiserver/` — API server binary and flag configuration
**Source:** `pkg/kubeapiserver/admission/` — Admission chain configuration

**Affected Systems:** SYS-RUN-ORC, SYS-CMP-APP, SYS-CMP-ORC

---

## 6. Audit and Accountability Conflicts (NIST AU vs. CIS K8s 3.2 / CIS Control 8)

### 6.1 CFR-AU-001: Audit Record Requirements — Detailed Content vs. Minimal Policy

**NIST SP 800-53 Rev 5 — AU-2 / AU-3 / AU-12:**
- AU-2 requires organizations to identify the types of events that the system is capable of logging and define the subset to be audited, with documented rationale.
- AU-3 requires that each audit record contains: what type of event occurred, when the event occurred, where the event occurred, the source of the event, the outcome (success or failure), and the identity of individuals, subjects, or objects involved.
- AU-12 requires the system to generate audit records for events identified in AU-2, with content defined by AU-3, at system components where audit capability is deployed.

**CIS Kubernetes Benchmark v1.12.0 — Section 3.2:**
- 3.2.1: Ensure that a minimal audit policy is created — requires that the `--audit-policy-file` flag is set and a policy file exists.
- 3.2.2: Ensure that the audit policy covers key security concerns — requires the audit policy to cover access to Secrets, modification of ConfigMaps, authentication failures, and RBAC authorization failures.

**Resolution:** NIST AU-2/AU-3/AU-12 is **more restrictive**.

**Rationale:** NIST AU-3 specifies mandatory content fields for every audit record (event type, timestamp, location, source, outcome, identity), which is more prescriptive than CIS K8s 3.2.1's requirement for a "minimal audit policy." Additionally, NIST AU-2 requires documented rationale for event selection, and AU-12 requires systematic audit record generation. CIS K8s 3.2.2 identifies key security concerns to audit but does not mandate the per-record content requirements that NIST AU-3 specifies. The Kubernetes audit backend (referenced in staging/src/k8s.io/apiserver/pkg/audit/) supports configurable audit levels (None, Metadata, Request, RequestResponse), enabling compliance with both frameworks.

**Source:** Kubernetes audit policy configuration supports NIST AU-3 content fields through RequestResponse audit level
**Source:** `cmd/kube-apiserver/` — API server audit log configuration flags (--audit-log-path, --audit-log-maxage, --audit-log-maxbackup, --audit-log-maxsize)

**Affected Systems:** SYS-OBS-APP, SYS-OBS-CFG

---

### 6.2 CFR-AU-002: Audit Protection — Tool Protection vs. Log Management Process

**NIST SP 800-53 Rev 5 — AU-9 (Protection of Audit Information):**
NIST AU-9 requires the system to protect audit information and audit logging tools from unauthorized access, modification, and deletion. This extends to both the audit records themselves and the mechanisms that generate them.

**CIS Controls v8 — Control 8 (Audit Log Management):**
CIS Control 8 sub-controls address: establishing audit log management process (8.1), collecting audit logs (8.2), ensuring adequate storage (8.3), standardizing time synchronization (8.4), collecting detailed audit logs (8.5), collecting DNS query audit logs (8.6), collecting URL request audit logs (8.7), collecting command-line audit logs (8.8), centralizing audit logs (8.9), retaining audit logs (8.10), conducting audit log reviews (8.11), and collecting service provider audit logs (8.12).

**Resolution:** NIST AU-9 is **more restrictive**.

**Rationale:** NIST AU-9's requirement extends to protecting the audit tools themselves — not just the log data — from unauthorized access, modification, and deletion. CIS Control 8 provides a comprehensive log management lifecycle (collection, storage, review, retention) but does not explicitly require protection of the auditing infrastructure itself against compromise. In the Kubernetes context, this means protecting the audit webhook backend, audit policy configuration, and audit log aggregation pipeline from tampering, which goes beyond the CIS Control 8 scope.

**Source:** Kubernetes audit backends (staging/src/k8s.io/apiserver/pkg/audit/) — audit event generation, policy evaluation, and backend delivery
**Source:** `cmd/kube-apiserver/` — Audit configuration flags controlling audit policy and log destinations

**Affected Systems:** SYS-OBS-APP, SYS-OBS-CFG, SYS-OBS-ORC

---

## 7. System Integrity Conflicts (NIST SI vs. CIS K8s 4.2 / CIS Control 16)

### 7.1 CFR-SI-001: Malicious Code Protection — Active Detection vs. File Hardening

**NIST SP 800-53 Rev 5 — SI-3 (Malicious Code Protection):**
NIST SI-3 requires organizations to implement malicious code protection mechanisms at system entry and exit points to detect and eradicate malicious code. This includes automatic updates of protection mechanisms when new releases are available and configuration for periodic scans and real-time scans of files from external sources at endpoint, network entry/exit points.

**CIS Kubernetes Benchmark v1.12.0 — Section 4.2 (Worker Node Configuration):**
CIS K8s 4.2 prescribes specific worker node configuration parameters:
- 4.2.1–4.2.13: Kubelet configuration parameters including streaming connection timeouts, authentication/authorization settings, TLS configuration, event rate limiting, and make-iptables-util-chains
- Focus on hardening the kubelet configuration rather than detecting malicious code at runtime

**Resolution:** NIST SI-3 is **more restrictive**.

**Rationale:** NIST SI-3 requires active malicious code detection and eradication mechanisms with real-time scanning capability at system entry and exit points. This is fundamentally different from and more restrictive than CIS K8s 4.2's configuration hardening approach, which focuses on securing worker node parameters but does not mandate runtime malicious code scanning. In the Kubernetes context, SI-3 would require image scanning, runtime behavior monitoring, and admission-time image validation — capabilities addressed by admission controllers like `plugin/pkg/admission/imagepolicy/` and `plugin/pkg/admission/alwayspullimages/`.

**Source:** `plugin/pkg/admission/imagepolicy/` — ImagePolicyWebhook admission controller for image validation
**Source:** `plugin/pkg/admission/alwayspullimages/` — AlwaysPullImages admission controller ensuring fresh image pulls

**Affected Systems:** SYS-CMP-APP, SYS-CMP-ORC, SYS-RUN-ORC

---

### 7.2 CFR-SI-002: Input Validation — Runtime Enforcement vs. Development Process

**NIST SP 800-53 Rev 5 — SI-10 (Information Input Validation):**
NIST SI-10 requires the information system to check the validity of information inputs. This includes checking syntax, length, range, content, and authorized values for all inputs before processing. This is a runtime enforcement requirement — the system must validate inputs at the point of processing.

**CIS Controls v8 — Control 16 (Application Software Security):**
CIS Control 16 requires: establishing secure development processes (16.1), performing root cause analysis on vulnerabilities (16.2), performing code reviews (16.3), establishing and managing software component inventories (16.4), using up-to-date compilation features (16.9), and applying secure design principles (16.10). CIS Control 16 is a development lifecycle control, not a runtime enforcement control.

**Resolution:** NIST SI-10 is **more restrictive**.

**Rationale:** NIST SI-10 mandates runtime input validation at the point of processing — every external input must be checked for validity (syntax, length, range, content) before use. This is a system behavior requirement. CIS Control 16 addresses application security through development process controls (code reviews, secure design, root cause analysis) but does not mandate that the deployed system perform specific input validation at runtime. In the Kubernetes codebase, admission controllers in `plugin/pkg/admission/` (e.g., LimitRanger, ResourceQuota, NodeRestriction) implement SI-10's runtime validation requirement.

**Source:** `plugin/pkg/admission/limitranger/` — LimitRanger admission controller validating resource limits
**Source:** `plugin/pkg/admission/noderestriction/` — NodeRestriction admission controller validating node-level requests
**Source:** `plugin/pkg/admission/resourcequota/` — ResourceQuota admission controller validating resource consumption

**Affected Systems:** SYS-CMP-APP, SYS-CMP-ORC, SYS-IAM-APP

---

## 8. Container Security Conflicts (NIST SP 800-190 vs. CIS K8s)

### 8.1 CFR-CS-001: Image Security — Lifecycle Risk Coverage vs. File Permission Hardening

**NIST SP 800-190 (Image Risks):**
SP 800-190 Section 3.1 identifies five image risk categories requiring mitigation:
1. Image vulnerabilities (known CVEs in embedded software)
2. Image configuration defects (insecure default configurations)
3. Embedded malware
4. Embedded cleartext secrets
5. Use of untrusted images

SP 800-190 recommends: using image vulnerability scanners, enforcing image provenance and integrity (signing/verification), preventing cleartext secrets in image layers, maintaining image currency (rebuilding from known-good base images), and using trusted registries.

**CIS Kubernetes Benchmark v1.12.0 — Section 4.2 (Container Configuration):**
CIS K8s 4.2 focuses on worker node container runtime configuration:
- 4.2.1–4.2.13: Kubelet configuration parameters including authentication, authorization, TLS, streaming connections, event rate limiting
- Focus is on the container runtime configuration after deployment, not on image lifecycle governance

**Resolution:** SP 800-190 is **more restrictive**.

**Rationale:** NIST SP 800-190 addresses the entire container image lifecycle — from build-time vulnerability scanning and secret exclusion through registry governance to runtime image integrity verification. CIS K8s 4.2 focuses narrowly on the kubelet and container runtime configuration parameters after deployment. SP 800-190's comprehensive image risk coverage is substantially more restrictive because it governs the image before it reaches the runtime, addressing risks that CIS K8s 4.2 does not (embedded malware, cleartext secrets in layers, untrusted image sources).

**Source:** `build/pause/Dockerfile` — Pause container image build definition
**Source:** `build/server-image/Dockerfile` — Server image build definition
**Source:** `build/dependencies.yaml` — External dependency version pins (supply chain governance)

**Affected Systems:** SYS-IMG-IAC, SYS-IMG-DEP, SYS-IMG-PIP

---

### 8.2 CFR-CS-002: Orchestrator Security — Risk Categories vs. Prescriptive Hardening

**NIST SP 800-190 (Orchestrator Risks):**
SP 800-190 Section 3.4 identifies orchestrator-specific risks:
1. Unbounded administrative access — excessive privileges for orchestrator administrators
2. Unauthorized access to inter-container network traffic — lack of network segmentation between containers
3. Mixing of workload sensitivity levels — containers with different trust levels sharing resources
4. Orchestrator node trust — compromised nodes accepted into the cluster

SP 800-190 recommends: limiting administrative access, segmenting container networks, segregating sensitive workloads, and implementing node trust verification.

**CIS Kubernetes Benchmark v1.12.0 — Section 1 (Control Plane Components):**
CIS K8s Section 1 prescribes specific, testable control plane hardening:
- 1.1.1–1.1.22: File ownership and permission checks for control plane pod specifications and configuration files
- 1.2.1–1.2.25: API server process arguments with specific flag values
- 1.3.1–1.3.7: Controller manager arguments
- 1.4.1–1.4.2: Scheduler arguments

**Resolution:** CIS K8s Section 1 is **more restrictive** (for control plane hardening).

**Rationale:** While SP 800-190 identifies broader orchestrator risk categories (unbounded admin access, inter-container network access, workload mixing, node trust), CIS K8s Section 1 translates these risks into specific, testable control plane configurations with binary pass/fail criteria. Each CIS check specifies an exact flag value, file permission, or ownership requirement that can be automated via kube-bench. For the Kubernetes control plane context, the CIS checks are more operationally restrictive and verifiable. Note: SP 800-190's broader risk categories remain applicable for orchestrator risks not covered by CIS K8s Section 1 (e.g., workload sensitivity segregation).

**Source:** `cmd/kube-apiserver/` — API server binary and configuration
**Source:** `cmd/kube-controller-manager/` — Controller manager binary
**Source:** `cmd/kube-scheduler/` — Scheduler binary
**Source:** `pkg/kubeapiserver/authorizer/config.go:82-161` — Authorization chain configuration addressing admin access restriction

**Affected Systems:** SYS-RUN-ORC, SYS-RUN-CFG, SYS-IAM-ORC

---

### 8.3 CFR-CS-003: Registry Security — Direct Coverage vs. Indirect RBAC Protection

**NIST SP 800-190 (Registry Risks):**
SP 800-190 Section 3.2 identifies registry-specific risks:
1. Insecure connections to registries — unencrypted communication with registries
2. Stale images in registries — outdated images with known vulnerabilities
3. Insufficient authentication and authorization to registries — weak access controls

SP 800-190 recommends: encrypting all registry connections, implementing image freshness policies, and enforcing strong authentication/authorization for registry access.

**CIS Kubernetes Benchmark v1.12.0 — Section 5.1.1–5.1.2:**
CIS K8s 5.1.1 requires minimizing use of the cluster-admin role. CIS K8s 5.1.2 requires minimizing access to secrets. These checks indirectly govern registry credential access through RBAC minimization (registry credentials are typically stored as Kubernetes Secrets).

**Resolution:** SP 800-190 is **more restrictive**.

**Rationale:** SP 800-190 directly addresses container registry security with specific requirements for encrypted connections, image freshness policies, and registry-specific authentication. CIS K8s 5.1 only indirectly addresses registry security through RBAC minimization of secret access. SP 800-190's targeted registry risk mitigation provides more comprehensive and restrictive coverage than CIS K8s's indirect approach through general RBAC controls.

**Source:** `build/dependencies.yaml` — External dependency version pins tracking image dependencies
**Source:** `plugin/pkg/admission/alwayspullimages/` — AlwaysPullImages admission controller addressing image freshness

**Affected Systems:** SYS-IMG-IAC, SYS-IMG-DEP

---

## 9. Network and Communications Conflicts (NIST SC vs. CIS K8s 5.3)

### 9.1 CFR-SC-001: Boundary Protection — General Monitoring vs. Default-Deny NetworkPolicy

**NIST SP 800-53 Rev 5 — SC-7 (Boundary Protection):**
NIST SC-7 requires the system to: monitor and control communications at the external managed interfaces, implement subnetworks for publicly accessible system components, and connect to external networks or systems only through managed interfaces. SC-7 enhancements include: access points (SC-7(3)), external telecommunications services (SC-7(4)), deny by default / allow by exception (SC-7(5)), and boundary protection for all communications (SC-7(8)).

**CIS Kubernetes Benchmark v1.12.0 — Section 5.3 (Network Policies):**
- 5.3.1: Ensure that the CNI in use supports NetworkPolicies
- 5.3.2: Ensure that all Namespaces have NetworkPolicies defined — prescribes that every namespace has at least one NetworkPolicy enforcing a default-deny posture for both ingress and egress traffic

**Resolution:** CIS K8s 5.3.x is **more restrictive**.

**Rationale:** CIS K8s 5.3.2 prescribes a mandatory default-deny NetworkPolicy for all namespaces, requiring explicit allow rules for all permitted traffic. While NIST SC-7(5) prescribes "deny by default / allow by exception" at managed interfaces, CIS K8s 5.3.2 applies this principle at the namespace level for every namespace in the cluster — a more granular and operationally restrictive posture. Additionally, CIS K8s 5.3.1 requires that the CNI plugin supports NetworkPolicy enforcement, adding a testable infrastructure prerequisite that NIST SC-7 does not specify.

**Source:** `pkg/apis/networking/` — NetworkPolicy API type definitions
**Source:** `build/dependencies.yaml` — CNI version 1.9.0 tracked as external dependency

**Affected Systems:** SYS-NET-APP, SYS-NET-API, SYS-NET-CFG

---

### 9.2 CFR-SC-002: Transmission Protection — General Encryption vs. Specific TLS Configuration

**NIST SP 800-53 Rev 5 — SC-8 (Transmission Confidentiality and Integrity):**
NIST SC-8 requires the system to protect the confidentiality and integrity of transmitted information. Enhancement SC-8(1) requires implementing cryptographic mechanisms to prevent unauthorized disclosure and detect changes to information during transmission.

**CIS Kubernetes Benchmark v1.12.0 — Section 1.2 (API Server) / Section 2 (etcd):**
CIS K8s prescribes specific TLS configurations:
- 1.2.22–1.2.25: Ensure `--tls-cert-file` and `--tls-private-key-file` arguments are set for the API server, and that appropriate TLS cipher suites and minimum TLS version are configured
- 2.1: Ensure `--cert-file` and `--key-file` arguments are set for etcd
- 2.4–2.6: Ensure etcd peer communication uses TLS with `--peer-cert-file`, `--peer-key-file`, and `--peer-client-cert-auth` set to true

**Resolution:** CIS K8s 1.2/2.x is **more restrictive**.

**Rationale:** CIS K8s prescribes specific TLS certificate flags for the API server and etcd, including mutual TLS for etcd peer communication. These are operationally restrictive and testable requirements that go beyond NIST SC-8's general "use cryptographic mechanisms" prescription. CIS specifies exactly which flags must be set and verifies mutual TLS between etcd peers, which is a more concrete and restrictive requirement than NIST SC-8's principle of cryptographic transmission protection.

**Source:** `cmd/kube-apiserver/` — API server TLS configuration
**Source:** `pkg/apis/certificates/` — Certificate signing request API types

**Affected Systems:** SYS-NET-CFG, SYS-RUN-ORC, SYS-RUN-CFG

---

## 10. Conflict Resolution Summary

### 10.1 Summary Statistics

| conflict_area | total_conflicts | NIST_more_restrictive | CIS_more_restrictive | equal_restrictiveness |
|---|---|---|---|---|
| Access Control (AC) | 3 | 1 | 2 | 0 |
| Identification/Authentication (IA) | 2 | 1 | 1 | 0 |
| Configuration Management (CM) | 3 | 1 | 2 | 0 |
| Audit and Accountability (AU) | 2 | 2 | 0 | 0 |
| System Integrity (SI) | 2 | 2 | 0 | 0 |
| Container Security (SP 800-190 vs CIS K8s) | 3 | 2 | 1 | 0 |
| Network/Communications (SC) | 2 | 0 | 2 | 0 |
| **TOTAL** | **17** | **9** | **8** | **0** |

### 10.2 Conflict Density Analysis

The control areas with the **highest conflict density** are:

1. **Access Control (AC) and Configuration Management (CM):** 3 conflicts each — reflecting the fundamental tension between NIST's principle-based controls and CIS's Kubernetes-specific operational checks. AC conflicts arise from the gap between broad least-privilege principles (AC-6) and specific RBAC configurations (CIS K8s 5.1). CM conflicts arise from general configuration management requirements (CM-3, CM-6, CM-7) versus detailed Kubernetes flag and permission checks (CIS K8s 1.1–4.2).

2. **Container Security (SP 800-190 vs. CIS K8s):** 3 conflicts — reflecting the different abstraction levels between NIST's container risk categorization and CIS's prescriptive Kubernetes hardening. SP 800-190 addresses broader risk categories (image lifecycle, registry governance) while CIS K8s provides more specific control plane and worker node configurations.

### 10.3 Resolution Pattern Analysis

| Pattern | Count | Description |
|---|---|---|
| CIS more restrictive due to Kubernetes-specific specificity | 8 | CIS K8s Benchmark prescribes exact, testable configurations (flag values, file permissions, default-deny policies) that operationally exceed NIST's principle-based requirements for the Kubernetes context |
| NIST more restrictive due to broader scope | 5 | NIST controls cover broader security objectives (MFA, runtime input validation, active malware detection, audit content requirements, audit tool protection) that CIS does not fully address |
| NIST SP 800-190 more restrictive due to container lifecycle coverage | 2 | SP 800-190 addresses container image and registry lifecycle risks that CIS K8s does not fully cover through its configuration-focused checks |
| NIST more restrictive due to process rigor | 2 | NIST CM-3 and AU-9 require formal processes and protections that exceed CIS's operational focus |

### 10.4 Guidance for Directive Documents

When applying this conflict register across directive documents (D0–D7):

1. **For each system in the System Registry (D0):** Identify which conflicts apply based on the system's vertical domain and framework control mapping. Apply the resolved (more restrictive) control as the assessment baseline.

2. **For Structural Integrity findings (D1):** When mapping findings to CIS Benchmark check IDs, note any conflicts where the NIST prescription is more restrictive than the CIS check. Apply the NIST standard.

3. **For Materiality Classification (D2):** Material classification should reference both NIST and CIS controls. Where conflicts exist, the materiality determination uses the more restrictive control's scope.

4. **For Code Quality (D3) and Documentation Coverage (D5):** Security-relevant quality findings and documentation requirements should be assessed against the more restrictive resolved control from this register.

5. **For Dependency Audit (D4):** Dependency governance requirements follow NIST CM-3 (CFR-CM-002 resolution) for change control rigor, complemented by CIS Control 2 for asset inventory.

6. **For Accuracy Validation (D6):** When validating audit findings, the resolved control from this register determines the accuracy baseline — a finding is accurate if it correctly represents the system state relative to the more restrictive control.

7. **General Rule:** If a conflict exists for a given control area, always apply the resolved (more restrictive) control. Reference this register by conflict_id (e.g., "per CFR-AC-001, CIS K8s 5.1.x applied as more restrictive for RBAC assessment").

---

## 11. Conflict ID Quick Reference

| conflict_id | Short Description | More Restrictive |
|---|---|---|
| CFR-AC-001 | Least privilege: broad vs. specific RBAC checks | CIS K8s 5.1.x |
| CFR-AC-002 | Access enforcement: runtime enforcement vs. process management | NIST AC-3 |
| CFR-AC-003 | Security function access: broad designation vs. SA token mount restriction | CIS K8s 5.1.6 |
| CFR-IA-001 | Authentication: multi-factor vs. client certificates | NIST IA-2 |
| CFR-IA-002 | Authenticator management: lifecycle policy vs. SA token restrictions | CIS K8s 5.1.5/5.1.6 |
| CFR-CM-001 | Configuration settings: general management vs. specific hardening checks | CIS K8s 1.1.x–4.2.x |
| CFR-CM-002 | Change control: formal process vs. secure config establishment | NIST CM-3 |
| CFR-CM-003 | Least functionality: principle vs. specific API server flags | CIS K8s 1.2.x |
| CFR-AU-001 | Audit records: detailed content requirements vs. minimal policy | NIST AU-2/AU-3/AU-12 |
| CFR-AU-002 | Audit protection: tool protection vs. log management process | NIST AU-9 |
| CFR-SI-001 | Malicious code: active detection vs. file hardening | NIST SI-3 |
| CFR-SI-002 | Input validation: runtime enforcement vs. development process | NIST SI-10 |
| CFR-CS-001 | Image security: lifecycle risk vs. file permission hardening | SP 800-190 |
| CFR-CS-002 | Orchestrator security: risk categories vs. prescriptive hardening | CIS K8s Section 1 |
| CFR-CS-003 | Registry security: direct coverage vs. indirect RBAC protection | SP 800-190 |
| CFR-SC-001 | Boundary protection: general monitoring vs. default-deny NetworkPolicy | CIS K8s 5.3.x |
| CFR-SC-002 | Transmission protection: general encryption vs. specific TLS config | CIS K8s 1.2/2.x |

---

*This document is referenced by all directive documents (D0–D7) in the Kubernetes monorepo compliance audit. For full traceability across directives, see `appendix-cross-reference-index.md`.*
