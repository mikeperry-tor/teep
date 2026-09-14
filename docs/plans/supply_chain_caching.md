# Plan: Attestation Cache (`teep cache`)

## 1. Goals and runtime baseline

`teep cache` has two goals:

1. **Reduce or eliminate additional requests before inference starts.** Prepare
   reusable evidence and verification results, distribute them to replicas, and
   reevaluate retained evidence locally after a teep update where possible. Specify
   the remaining requests for discovery, fresh endpoint admission, report-bound
   services, and missing or ineligible collateral. Measure request reduction and
   time until inference can begin; do not equate HTTP/2 connection reuse with fewer
   HTTP requests. See [request accounting](#4b-request-reduction-and-elimination).
2. **Provide an optional operator decision path to pin observed evidence despite
   policy violations.** `teep cache --update-whitelist` records exact TOFU pins or
   policy exceptions for selected failures. It does not relabel a failed check as
   cryptographic success. Define ordinary, elevated-risk, and unsupported decisions
   and the checks and requests each decision replaces. See the
   [exception inventory](#5a-whitelist-inventory).

### Command and flag synopsis

The following is the complete proposed command surface for this plan, in addition
to existing configuration and service flags. Brackets indicate optional arguments;
`TARGETS` means either `--all-models` or one or more `--model provider:model`
arguments. These are proposed interfaces, not commands available before implementation.

```text
teep cache TARGETS [--cache-file PATH]
teep cache TARGETS --update-whitelist [--reason TEXT] [--cache-file PATH]
teep cache TARGETS --update-whitelist --proposal-out PATH [--reason TEXT] [--cache-file PATH]
teep cache --update-whitelist --apply-proposal PATH [--cache-file PATH]
teep serve [--cache-file PATH] [--autocache]
teep verify [existing target/options] [--cache-file PATH | --no-cache]
```

| Command or flag | User interaction |
| --- | --- |
| `teep cache` | Collect and verify evidence, then merge complete successful targets into the cache file. Ordinary caching does not change policy. |
| `--all-models` | Discover models from active providers. Available for ordinary caching, interactive whitelist updates, and proposal generation; subject to the provider support restrictions below. |
| `--model provider:model` | Select specific targets instead of all models. Repeat the flag or use comma-separated fully qualified names. Mutually exclusive with `--all-models`; no provider positional argument. |
| `--cache-file PATH` | Select the cache input/output for `cache`, the prefill file and optional autocache destination for `serve`, or the read-only evidence/decision input for `verify`. |
| `--update-whitelist` | Review concrete eligible policy changes, select them, provide a reason, and confirm before writing. Supports bulk selection of ordinary changes; elevated changes require separate acknowledgement. Never accepts failures automatically. |
| `--reason TEXT` | Supply the operator explanation for interactive review or proposal generation. Interactive review can collect it when omitted; apply uses the reviewed proposal's explanations. |
| `--proposal-out PATH` | With `--update-whitelist`, write a reviewable proposal for automation instead of changing cache policy. Selections and risk acknowledgements start unset. |
| `--apply-proposal PATH` | With `--update-whitelist`, validate and apply the exact reviewed selections noninteractively. Targets, explanations, and acknowledgements come from the proposal; no new model discovery or silent substitution. Mutually exclusive with proposal generation and target flags. |
| `teep verify --cache-file PATH` | Verify fresh endpoint evidence against the candidate cache and effective policy without modifying the file. The flag is optional: default cache resolution is identical across all three commands. |
| `teep verify --no-cache` | Run baseline live verification without cache evidence or operator decisions. Mutually exclusive with `--cache-file`; explicitly overrides environment/config/default cache selection. Reports that cache policy was not tested. |
| `teep serve --autocache` | Automatically persist eligible evidence/results after successful admission through the shared runtime path. Portable writes are asynchronous; the flag alone creates no whitelist decisions or persisted endpoint authorizations. Endpoint persistence additionally requires the service configuration below. Without the flag, `serve` reads portable cache material without writing it. |

Service configuration additions are `cache_endpoint_persistence` (default `false`)
and `cache_state_dir` (required only when persistence is enabled). Enabling it also
requires `--autocache`; the mandatory durable writer is separate from asynchronous
portable writes. `policy_state` is part of the cache artifact, not another whitelist
input. Policy revisions and removal records are managed by explicit whitelist edits.

Cache path precedence is `--cache-file`, `$TEEP_CACHE_FILE`, configured `cache_file`,
then `~/.config/teep/cache.yaml`, identically for `cache`, `serve`, and `verify`.
All three load an existing default without requiring `--cache-file`; a missing
implicit default starts empty. `cache` and `serve --autocache` can create their
output file. An explicitly selected missing file is an error for `verify` and for
`serve` without `--autocache`.
Autocaching requires a writable cache destination. Noninteractive whitelist
updates require proposal generation or explicit apply; there is no implicit consent.

`teep verify` imports eligible portable evidence and operator decisions, performs
fresh endpoint admission, and never exports or updates cache state. It does not
restore persisted endpoint authorizations. There is no separate `--whitelist` input.
Replace `--update-config` and `--config-out` with the operator decision workflow in Phase 10.
Existing `--force` is not a whitelist-selection or trusted-cache-generation option.
Optional endpoint persistence is enabled only for `serve` by
`cache_endpoint_persistence = true` plus `cache_state_dir`, with `--autocache` required.
It defaults off and uses a separate mandatory durable writer. No additional CLI flag
is assigned; ordinary `cache` and `verify` do not persist endpoint authorizations. See [commands and deployment](#5-commands-and-deployment)
and [operator decisions](#5b-operator-decision-command-and-reporting) for detailed
validation, partial-failure, proposal, and acknowledgement rules.

### Evidence and runtime baseline

An image must satisfy its effective provenance policy and compose/attestation
binding. Where release authentication is required, it must use an authenticated
release unless an explicit supported operator decision replaces that requirement.
Existing compose-only requirements and `allow_fail` controls retain their distinct
guarantees; they do not become signature-verification successes. It does not have to use the latest release. Ordinary
caching never expands policy automatically.

This document specifies proposed cache behavior. The maintained runtime contracts
are in the [transport reference](../transport/README.md),
[retry rules](../transport/retries.md),
[NEAR reference](../providers/near/near_attestation.md), and
[Tinfoil reference](../providers/tinfoil/tinfoil_support.md).

For NearDirect, NearCloud, Tinfoil direct, and Tinfoil cloud, the current proxy
publishes the report, authenticated public encryption key, and transport identity
as one authorization. It reuses that authorization while the identity and required
keys remain usable, until invalidation, eviction, or process exit. Evidence expiry
and loss of all connections do not cause renewal. HTTP/2 connections and
attestation authorizations have independent lifetimes. Other providers retain
their runtime behavior until explicitly migrated.

The proposed disk cache supports portable evidence and verified subjects across
replicas, and operator decisions distributed through the deployment's trusted path.
Optional persistence of a complete endpoint authorization extends the process-exit
boundary only where durable invalidation is available. It must not be implemented
as separate pin, report, and encryption-key files.

### Provider scope and migration prerequisites

Initial cache support covers NearCloud, NearDirect, Tinfoil cloud, and Tinfoil
direct. Chutes and Venice are planned extensions, **blocked until each provider
migrates to the shared HTTP/2 attestation authorization machinery used by Near and
Tinfoil**. This prerequisite applies to portable prefill, `--update-whitelist`,
and endpoint persistence. HTTP/2 negotiation alone does not satisfy it.

Do not add adapters to legacy report/key caches, parallel admission paths, or
provider-specific cache lifetimes to enable either provider early. Their migrations
are separate prerequisite work and must not delay the initial cache implementation.
After migration, enable only capabilities supported by the common interfaces and
provider evidence. Until then, reject cache targets for these providers with a clear
migration-required diagnostic; do not silently omit targets from a multi-provider run.

PhalaCloud (`phalacloud`) and NanoGPT (`nanogpt`) are not planned for cache support
or migration under this work. Reject these cache targets as unsupported. This scope
does not remove or change their existing provider implementations.

### Tinfoil direct live-validation prerequisite

`tinfoil_v3_direct` remains in initial implementation scope through its existing
shared transport and authorization machinery. Its live inference validation is
blocked by [cvmimage issue 337](https://github.com/tinfoilsh/cvmimage/issues/337),
which concerns billing authenticated requests sent directly to model endpoints.
Defer direct live inference tests and live request-reduction measurements until the
upstream fix is deployed to the endpoints under test. Issue closure or a merged
change alone does not establish deployment readiness.

Continue direct-provider implementation, unit tests, captured-evidence tests, and
deterministic TLS/HTTP/2 request-count tests. Record live validation as blocked,
not passed or replaced by offline coverage. This is an external live-validation
prerequisite, not a missing provider migration or a reason to add alternate cache
machinery. `tinfoil_v3_cloud` is unaffected by this issue and retains its live
validation requirements. Cloud results do not establish direct-provider coverage.

### Implementation goal: prefill shared attestation state

Implement both goals by sharing as much machinery as possible with the attestation
data management used by the HTTP/2 transport. `teep cache` is a preparation and
export path; loading its output is a prefill path into that same data management.
Disk and network are sources of material, not separate definitions of trust.

Reuse the production verification, policy evaluation, immutable authorization
construction/publication, scope matching, shared-work coordination, and conditional
invalidation paths. Extract shared interfaces where necessary; do not add a second
verifier, a cache-only request handler, or independently maintained key/pin lifetimes.
The existing [authorization implementation](../../internal/proxy/authorization.go)
and [transport contract](../transport/README.md) define the integration boundary.

## 2. Data model and trust boundaries

The serialized classes describe persistence and operator-visible meaning. They do
not require a parallel hierarchy of runtime stores. Adapt them to shared admission
inputs and the existing unified authorization store through validated interfaces.
Keep separate storage only where scope or semantics require it, such as portable
artifact evidence, deployment policy decisions, and durable invalidation records.

### 2a. Evidence and verification records

The evidence class contains original signed artifacts, certificate chains,
transparency proofs, compose documents, and authenticated reference material.
Store complete bytes needed for local verification, not just a source URL or a
provider-asserted success field. Deduplicate bytes by a cryptographic content digest.
Source URLs are not trust roots. Retrieval time is diagnostic for signed immutable
artifacts. For material whose existing eligibility depends on authenticated retrieval
time, retain that observation as trusted deployment evidence and apply the current
refresh policy; copying or importing the file must not reset it.

Verification records also belong to the evidence class. Each records:

- Exact evidence references and authenticated subject identifiers.
- The teep verifier/build identity that performed the checks.
- Effective verification policy identity, including trust roots, signer rules,
  applicable factor requirements, and explicit exemptions.
- Verification time, checks performed, results, and any admission-time validity
  information needed to explain or reevaluate the result.

Build identity must identify the actual verifier implementation, dependencies,
security-relevant build options, and local modifications. A human version string
alone is insufficient. Use the executable identity and canonical policy descriptors
specified in Section 2e. Never include API keys in those encodings.

### 2a-i. Stapled evidence and independent subjects

Design collection around a response carrying evidence for one or more attested
subjects, including a gateway and selected backend. Delivery through a gateway does
not make backend evidence a gateway assertion: verify each supported child quote,
nonce, REPORTDATA, key, compose, and applicable collateral independently. Treat
stapling as the preferred extension pattern for future providers, not a claim that
all existing providers supply or cryptographically bind the same evidence.

NEAR already follows this pattern. [NearCloud parsing](../../internal/provider/nearcloud/parser.go)
selects a model from `model_attestations`; [NearDirect parsing](../../internal/provider/neardirect/parser.go)
validates its direct/repeated representation. Both use
[nearparse.Model.Raw](../../internal/provider/nearparse/model.go) to construct
`FormatNear` model evidence. NearCloud additionally supplies separate gateway quote,
compose, nonce, event-log, and transport fields. Preserve their strict envelope
validation and model selection; do not accept a direct/gateway envelope interchange
merely because the extracted model representation is shared.

Separate delivery provenance, intrinsic evidence identity, consumer policy evaluation,
and endpoint authorization. Retain original response envelopes only when eligible
for portable storage. Exclude envelopes containing consumable request tokens, secrets,
or inference content; do not retain prohibited values inside base64 payloads.
Supported
extractors may retain original child quotes, compose strings, signed artifacts, and
collateral as independently content-addressed evidence, alongside their containing
envelope digests. Byte extraction must preserve the exact cryptographic inputs;
never reserialize signed content or equate a normalized compose with the attested
bytes. Validate every retained containment relationship through the production parser.
When an envelope is excluded, retain independently verifiable signed child bytes
and the safe verification context required by the supported extractor, without a
reference to an absent parent. Do not redact or reserialize signed inputs. If safe
extraction cannot retain every cryptographic prerequisite, that evidence is not
portable; use fresh collection. Complete HTTP capture is separate from this cache.
Mere
containment proves no signature, key ownership, or gateway/backend relationship.

The same child bytes can be shared whether fetched directly, stapled by a gateway,
or loaded from disk. Different envelopes do not prevent sharing an identical child;
different child bytes must not be merged because their fields look similar. Matching
component repository/digest pairs permit artifact evidence sharing even when complete
compose subjects differ. Reuse consumer policy results only through explicit shared
checks with equivalent trust roots, identity requirements, tier applicability, and
exemptions; matching provider lineage or signing keys alone is insufficient.

Fresh gateway/backend admission remains bound to the caller's nonce and selected
model/key/route. A saved stapled quote cannot answer a later nonce. A shared backend
key or software subject does not merge NearCloud and NearDirect authorizations:
NearCloud authenticates its gateway TLS peer and backend model key, while NearDirect
authenticates the selected backend TLS peer. Gateway-only evidence, including current
Tinfoil cloud and Venice ACI/1, must remain gateway-only. Cache generality must not
manufacture backend CPU or software coverage that a provider does not supply.

### 2b. Verified subjects

A software subject identifies an artifact or complete configuration independently
of its delivery path. Its evaluations record permitted use under each exact
consumer scope/policy and the verification that established it. A verified subject
means that identity together with an eligible evaluation, not the identity alone. The verifier/build
identity is not duplicated in the verified subject: the loader validates the associated verification context to enforce the upgrade
boundary.

Verified software subjects are portable across hosts and replicas. For example, two NEAR
endpoints using the same compose and image digests can share verification work.
Evidence bytes may also be shared between providers, but a verification result under one
provider's signer or provenance policy does not satisfy another policy. Model and
gateway tiers remain distinct when their requirements differ.

Verifying an image does not establish which endpoint runs it. Live attestation
must bind the image digest, compose hash, or release measurements to that endpoint.
A compose verification result covers the exact compose hash and its complete required image
and signer checks. It must not convert `compose_binding_only` into a verified image
signature or prove an unobserved image digest.

### 2b-i. Complete component coverage

One CVM authorization can depend on several component repositories and artifact
versions. Model and gateway tiers each need an explicit complete component set;
neither the primary application repository nor a successful first component stands
for the whole environment. Keep per-component subjects/results independently
reusable and include their identities/results within a compose or release-set record. An endpoint
references the complete sets required by its admission, not one representative image.

Component repositories are record values, never predefined YAML field names or
parser branches. Store arbitrary-length component collections within the schema's
bounds. Nest component identities and their verification details under readable
software records. Reference shared original evidence by content digest, and software
by explicit scope/subject/policy selectors. Validate digest integrity and selector
uniqueness. No local record names or list positions carry identity or trust. Section 6
defines the common serialization; runtime stores need not mirror that hierarchy.

Provider replacement, addition, or removal of components changes data and set
identity, not schema structure. Trust policy still applies: NEAR/Venice repository
entries declare required provenance and signer checks, while Tinfoil also supports
a constrained organization-signer rule for eligible release repositories. Neither
policy structure mandates a fixed set of components per CVM. A structurally valid
new repository can still fail policy; accepting arbitrary component records does
not grant arbitrary repository trust.

Identify a component by canonical repository, immutable digest or authenticated
release subject, role/tier, and applicable policy. Preserve multiple digests of the
same repository and multiple repositories sharing a digest. Deduplicating content
bytes must not merge signer, repository, tier, or decision requirements. The current
compose helper uses a digest-to-repository map; the cache coverage model must retain
the full repository/digest relations instead of copying that lossy representation.
Resolve ambiguous input or fail explicitly; do not silently choose the first policy.

Derive required membership from the exact bound compose or supported authenticated
release/measurement relationships, not every repository in a provider allowlist.
Policy lists include alternatives, not necessarily co-resident components. Check
set membership, each component's required provenance, and aggregate coverage before
publishing a reusable complete-set result. A compose-only component has a policy
result, not a fabricated image signature. A valid signature without its required
measurement/compose relationship does not establish deployment coverage.

Keep raw supplied collateral, independently verified artifacts, and complete
admission coverage distinct. Tinfoil V3 can supply code, platform, and freshness
collateral. Its [parser](../../internal/provider/tinfoil/attestation.go) currently
validates the envelope without reading entry data. The existing
[verifier](../../internal/verify/attest.go) fetches the selected code release and,
for TDX, `tinfoilsh/hardware-measurements`, collecting separate component results.
The current route's single `SupplyChainRepo` selects the primary release; it is not
a complete component inventory. Do not interpret a supplied
`tinfoilsh/platform-endorsements` bundle as the existing hardware-registry result.
Adding its independent signature/subject/measurement verification is separate shared
verifier work, required before that evidence can contribute a verified subject.
Do not require latest-release freshness as a substitute for authenticated binding.

HTTP/2 authorization scope remains provider/route/attested identity/key epoch. Do
not create a connection pool or endpoint authorization per component repository.
Complete component coverage is an admission dependency of the atomic report/key/
identity publication. Reusing that authorization follows the existing transport
lifetime; a background metadata change alone must not invent a new renewal rule.

### 2c. Endpoint authorizations

An endpoint authorization references its full admission evidence and verified subjects,
report, immutable route scope, attested TLS identity, and required public E2EE key.
All references must resolve and all required checks must be covered before atomic
publication. No partially loaded authorization may become visible to requests.
Both live admission and eligible disk restoration use the shared authorization
constructor and publication path. Serialization must not expose a way to insert
unchecked reports, pins, or keys directly into the runtime store.

Scope follows the existing provider authorization key, not a universal
`(provider, model)` tuple:

| Provider | Endpoint authorization scope and bindings |
| --- | --- |
| `neardirect` | Provider, model, selected indexed or static authority, attested TLS SPKI, and the required model encryption key. The selected index is not a permanent machine identity. |
| `nearcloud` | Provider, model, fixed `cloud-api.near.ai` authority, gateway TLS SPKI, separately attested backend identity, and the model Ed25519 key used for the gateway hint and E2EE conversion. |
| `tinfoil_v3_direct` | Provider, model, resolved backend authority, attested TLS SPKI, and HPKE public key. Different backend selections need separate authorizations. |
| `tinfoil_v3_cloud` | Provider and router authority, router TLS SPKI and HPKE public key. The router authorization is model-independent; E2EE outcomes remain model-specific. It is not independent attestation of the router's backend models. |

Do not persist TLS connections, session tickets, client ephemeral private keys,
shared session secrets, inference data, or a successful E2EE probe as a promise of
future usability. Historical nonce-bound evidence explains an existing admission;
it cannot satisfy a new fresh-nonce challenge.

### 2d. Who may supply the cache

A generated YAML file is not self-authenticating. A digest detects changed content
only when its expected value is trusted; a `verifier_build` field is not proof that
that verifier ran. Reuse of same-build verification results is permitted only for a cache supplied
through the operator's trusted deployment path, with validated ownership and access
controls. Treat this as distribution of trust data, including in an image layer.

Untrusted imported bytes may become evidence only through normal independent
verification. Never accept imported result flags as proof of verification. Hand editing a
verification record does not create a verified result. Explicit policy changes
belong to operator decision input and produce a different effective policy identity.

Ordinary `teep cache` authenticates under existing policy. It never creates an
operator decision. Values independently authenticated under an already trusted
authority may satisfy that policy without TOFU. `--update-whitelist` is the explicit
path for changing policy when current evidence does not satisfy it.

### 2e. Operator decisions

Store operator decisions separately from verified subjects and verification records.
Each decision records its kind, provider and tier, exact subject/value, selected
failure code, supporting observed evidence, source policy identity, decision time,
operator-supplied reason, and any additional risk acknowledgement. Pinning a signer
must identify the key or OIDC issuer and exact identity, not only an organization
name. Pinning measurements must identify the register/platform and its relationship
to the complete attested state. Never turn a set of observations into wildcards.

TOFU means the operator elects to trust the currently observed value without an
existing trusted reference establishing that value as permitted. It still requires
the independent authentication and binding prerequisites in the inventory. Updating
a reference from a source already authorized by policy is not itself TOFU.

A decision can substitute an operator-selected expected value or explicitly exempt
a supported requirement for an exact subject. It cannot claim that the original
policy passed. Keep the underlying outcome and show `operator_decision_applied`
with the decision reference in diagnostics. Existing `allow_fail` remains a separate
factor-wide policy control; a pin is a narrower subject-specific decision.

A verified subject records only the properties actually verified. Where a decision
was used, reference it through `decisions` and retain the failed base
check in the verification record. A subject admitted solely by a content pin belongs
to `operator_decisions`, not to a fabricated signature-verification result. Endpoint
authorizations can reference both verified subjects and applicable operator decisions.

Effective policy identity is scoped to the operation being evaluated. Hash the
canonical applicable base rules, trust roots, required checks, exemptions, and exact
active decisions relevant to that subject/provider/tier. Software subchecks and
endpoint admission have separate policy identities. Unrelated decisions, evidence
additions, and evaluation timestamps do not change these identities. A shared
rule change invalidates every dependent evaluation, even if the rule is stored
elsewhere. Each evaluator defines and tests its complete dependency projection;
callers cannot omit a rule to obtain a cache hit. The rollout report additionally
identifies the whole artifact digest and policy revision. A build change invalidates derived
verification results, but does not silently erase the operator's intent or make it
an unconditional override. The current verifier checks whether the decision kind,
scope, risk acknowledgement, and base-policy compatibility remain permitted. New
hard restrictions or unsupported decisions block affected reuse with a clear error;
do not reinterpret them as broader exemptions. Revalidating a decision is local
unless its documented evidence prerequisites require retrieval.

Canonical policy descriptors use RFC 8785 JSON, with an explicit domain and version
prefix before hashing. Sort set-valued rules/decisions by canonical identity; preserve
order only where evaluation semantics depend on it. Exclude source-file formatting,
paths, diagnostic strings, retrieval times, and unrelated provider rules. Hash the
immutable deployed executable at startup, before constructing shared services, and
retain that identity for the process lifetime. Deployment must not modify the
executable in place. Use that digest for build identity; version/commit strings alone miss
local modifications, dependencies, build tags, toolchain, and linker options. Refuse
derived-result reuse if executable identity cannot be established; retained raw
evidence may still be evaluated. Different executable bytes require reevaluation,
even when their printed version matches. This conservative rule avoids a second
hand-maintained list of code dependencies.

The policy projection is explicit by operation:

| Operation | Included policy dependencies |
| --- | --- |
| Component provenance | Provider/tier/format applicability; complete matching `ImageProvenance` rules (provenance mode, source repositories, OIDC issuer/identity alternatives, key fingerprint, `NoDSSE`, provider-signer and workflow constraints); applicable organization-signer rule; log/root identities; checks/exemptions/decisions actually used. |
| Compose/release coverage | Exact required membership and binding semantics plus each referenced component's evaluated policy identity; no first-component scalar can substitute for the set. |
| Intel/AMD material | Build-owned trust anchors/chains, allowed retrieval authority, platform/CA/product/TCB matching rules, and actual certificate/CRL validity and revocation requirements. Fresh quote measurements are inputs, not policy. |
| NVIDIA/CT/TUF material | Accepted authority and trust/bootstrap roots, refresh and key-rotation rules, signature/time/version requirements, and current withdrawal restrictions. Metadata contents/version/expiry are evidence dependencies, not arbitrary user policy. |
| Endpoint admission | Provider/route and required TLS/E2EE binding rules, model and gateway measurement policies, factor applicability, merged default/configured `allow_fail`, all applicable exact decisions, and dependent software/material policy identities. |

Define descriptors beside the relevant evaluators and test mutation of every field.
Read `ImageProvenance`, `OrgSignerPolicy`, and measurement rules from the validated
policy objects; never maintain a separate incomplete policy copy in cache code.
Cryptographic equality checks retain constant-time comparison. Canonical decision
digests include the full record; aggregate policy identity includes only applicable
active decisions. Material evidence updates need not invalidate unrelated policy.

Decisions may be portable when their subjects are portable (image digest, signer,
measurement set). Endpoint-specific key/identity facts retain endpoint scope.
Removing a decision changes affected effective policies and requires a deployment
update and restart to withdraw its runtime effect. `--update-whitelist` also presents
existing decisions for explicit removal, including in proposals; removal does not
require current provider evidence or successful admission. A restrictive withdrawal
must remain possible during an outage. Its report identifies affected targets even
when they no longer pass. Additions still require the admission rules below.

The artifact contains `policy_state` with a stable deployment-policy authority,
monotonic revision, and canonical removed-decision digests. New empty policy state
starts at revision zero. Only explicit policy operations advance the revision;
ordinary cache collection and autocache preserve it. Proposals record the authority,
revision, and policy-state digest they reviewed. Under the cross-process lock, apply
compares these against current state; a mismatch requires renewed review, not a merge
of stale intent. Selected removals and eligible additions form one atomic policy
transaction. Explicit reintroduction requires a newly reviewed decision and revision.

An evidence writer rereads current policy under the lock, preserves its decisions
and removal records, and commits only evaluations compatible with that current policy.
It may discard stale optional evaluations, reporting the omission, but cannot revive
a decision or leave dangling decision dependencies. This does not reload policy in
the running service. The trusted deployment system must deliver the authoritative
revision and prevent whole-artifact rollback; a self-declared revision cannot detect
replacement of both the artifact and its history. Restart affected instances after
policy rollout. This has the same deployment trust boundary as package withdrawal.

### 2f. Provider and format capabilities

Cache support is a set of capabilities, not a provider-wide boolean. Select the
format from strictly parsed evidence, not a model-name list or cached assumption.
Include provider, evidence format, principal/tier, authenticated subject, and
applicable policy in verification-result reuse scope. Content-addressed original
bytes may be deduplicated without sharing policy conclusions. A format change
requires fresh evaluation of its evidence coverage and enforcement policy.

| Provider / format | Portable input prefill | Operator decisions | Unified runtime authorization / restoration |
| --- | --- | --- | --- |
| NearDirect / near | Model compose/images, eligible collateral | Applicable exact model software/measurement decisions | Existing shared runtime path; restoration requires the planned durable-state support. |
| NearCloud / gateway plus model evidence | Separate gateway/model subjects and eligible shared collateral | Tier-specific decisions | Existing shared runtime path; restoration requires the planned durable-state support. |
| Tinfoil direct / V3 | Model release predicates, hardware references, eligible CPU collateral | Applicable model software/measurement decisions | Existing shared runtime path; each authority has its own scope. Live inference validation awaits the upstream billing fix (see Section 1). |
| Tinfoil cloud / V3 | Router release and applicable CPU collateral; no independent backend software result | Gateway-scoped decisions | Existing router-scoped runtime path; do not infer backend attestation. |
| Venice / dstack | Model compose/image evidence actually supplied and eligible Intel collateral | Applicable model-tier inventory classes | Blocked on shared authorization migration; all listed cache capabilities are post-migration candidates. |
| Venice / ACI/1 | Gateway compose/digests, eligible Intel collateral, original keyset/custody material for scoped local verification | Gateway software/measurement classes plus separately scoped KMS-root and application-ID candidates | Blocked on shared authorization migration. No inferred model CPU/software evidence or automatic router-wide sharing; persistence requires separate eligibility. |
| Chutes / sek8s | Eligible Intel collateral and original authenticated measurement evidence; no client-verifiable image provenance assumed | Exact MRTD/MRSEAM decisions and other independently eligible measurement classes | Blocked on shared authorization migration, including instance/key scope and single-use nonce separation. |
| PhalaCloud / NanoGPT | Not planned | Not planned | No migration planned under this work. |

Chutes and Venice entries describe post-migration candidates, not initial support.
Their separate migrations must satisfy the prerequisites below. Model-catalog
maintenance is not part of cache implementation.
Implement capabilities through the shared admission interfaces; do not duplicate
verification per provider or force an E2EE-only provider into a TLS-SPKI contract
whose live-peer binding it does not establish. Cache-aware CLI acceptance tests run
after the runtime migration is complete; they are cache-enablement tests, not a
circular prerequisite for completing the transport migration.

## 3. Reuse, upgrades, and lifetime

### 3a. Portable verification-result reuse

Prefill has two levels:

| Level | Material loaded | Shared destination and effect |
| --- | --- | --- |
| Verification inputs | Eligible evidence, verified subjects, and validated operator decisions | Populate the inputs/stores consulted by shared admission. They reduce retrievals but do not alone authorize an endpoint. |
| Complete endpoint authorization | Eligible persisted report, authenticated public keys, transport identity, and admission references | Construct and atomically publish through the existing authorization store after restoration checks. Requests acquire it through the normal route/scope lookup. |

The loader first performs strict parsing, provenance checks, build/policy and decision
compatibility checks, and reference validation. For endpoint restoration it also
checks deployment scope and durable invalidation. These are persistence-boundary
checks, not replacements for production verification. Once material is published,
request handling uses the normal acquisition path; it does not branch on disk versus
network origin. Admission checks that need fresh evidence still run on new admission.

The portable verification-result lookup then proceeds as follows:

1. Resolve the exact subject required by the attested compose or measurement.
2. Validate the cache structure, evidence references, and trusted provenance.
3. Match the verification result's subject, policy, required checks, and exemptions.
4. Follow its verification record and compare the verifier/build identity.
5. Reuse an eligible verification result, or reevaluate retained evidence with the current
   verifier before creating a replacement verification record and verified subject.

On a build or effective-policy change, old conclusions are ineligible for direct
reuse. Retain original evidence and verify it locally where current admission rules
permit. Fetch additional evidence only when it is missing or ineligible. Do not
interpret a new evaluation as having happened at the old record's time. Historical
signatures may use authenticated signing-time semantics where the production
verifier supports them; other validity checks apply at the new admission time.

This is the upgrade boundary even though build identity is stored in evidence.
A retained cache cannot restore trust withdrawn by the new package or policy.
No schema or internal API backward compatibility is required.

### 3b. Runtime authorization lifetime

After live admission or eligible restoration, use the same runtime store and
retain the existing TLS/E2EE key-use lifetime. Do not add a `max_cache_age` deadline to authorizations, require the latest image release, or
refresh attestation merely because collateral, certificates, or an NRAS JWT expire.
On new admission, recheck NRAS time eligibility immediately before initial
publication, then discard transient admission deadlines from the runtime
authorization. Eligible restoration preserves that admission; it does not introduce
a new NRAS expiry check solely because a new process publishes the restored record.

Each new inference connection still requires TLS 1.3, WebPKI, CT, and applicable
attested-SPKI validation before request bytes. Each request acquires authorization
for its immutable route and required E2EE key. Persisting a verification result does not persist
successful TLS validation. Preserve TLS-SPKI session-resumption restrictions.

Identity/key changes and classified trust failures require new admission according
to the [retry contract](../transport/retries.md). A new admission requires fresh
client-nonce evidence and all enforced factors; portable verified subjects and
eligible collateral may satisfy their respective subchecks. A cache miss never
authorizes transmission by itself. Offline admission keeps its explicit factor
policy and does not manufacture successful online results from cached booleans.

### 3c. Endpoint restoration and invalidation

Endpoint restoration prefills the unified authorization store; it does not create
a second store that requests can consult after invalidation. It is restricted to
the same deployment, build, effective policy, and exact authorization scope. Deployments share portable software
verified subjects within their configured policy; they do not distribute endpoint
authorization as though it were an image-level fact. Resolve routes through the current
provider contract. A saved NearDirect route must not override a newly selected
route or current discovery validation.

Durable invalidation and clean-owner state are prerequisites for endpoint restoration.
Each `cache_state_dir` belongs to one service owner and is exclusively locked for
that process lifetime; multiple owners may share the portable file but not this state.
Read the predecessor's clean state, then durably mark this owner unclean before any
restored or live authorization can be published. Only a drained shutdown with all
required state and invalidation writes committed may mark the owner clean. If the
predecessor was unclean, reject all its endpoint restoration and perform fresh
admission using eligible portable evidence. An active ledger alone cannot close the
crash interval between observing a trust failure and persisting its invalidation.
See [the planning issue](supply_chain_caching_issues.md#endpoint-restoration-after-an-unclean-shutdown).

Durable invalidation is a prerequisite for endpoint restoration:

- Keep a deployment-owned writable record of invalidated persisted authorization
  identities. Use a stable persisted identity distinct from process-local generation
  counters. Assign a new runtime generation on restoration.
- A failed request can invalidate only the persisted authorization it used and its
  corresponding runtime generation. It cannot remove a replacement authorization.
- Record invalidation before allowing future restoration. If recording fails,
  fail closed for affected reuse and report the storage error. A restart must not
  erase the failure and restore the same authorization.
- Validate durable state before enabling restoration. Missing, malformed, or
  unavailable invalidation state disables restoration; perform new full admission
  instead. Define initialization and crash recovery so missing state is never
  interpreted as proof that nothing was invalidated.
- Runtime eviction must not immediately reload the evicted authorization from disk.
  Require new admission after eviction, and preserve this exclusion across restart
  when the evicted record remains in the file.

A deployment that cannot provide durable invalidation, including a fully read-only
replica, may load portable evidence and verified software subjects but must obtain fresh
endpoint admission after each restart. Do not enable endpoint restoration there.
An authenticated new admission may use the same keys again if current policy
permits; invalidation does not permanently blacklist an otherwise valid key.

Trust withdrawal still requires updated package/policy delivery and restart of
every affected instance, as described in the
[transport reference](../transport/README.md#approval-withdrawal). Preserve its
limitation: retaining keys can retain an already admitted authorization until an
explicit withdrawal reaches that instance. This plan adds no advisory feed or live
policy reload. Endpoint persistence must not weaken the documented withdrawal path.

## 4. Portable material

| Material | Identity and portable use | New-admission requirements |
| --- | --- | --- |
| Sigstore bundles, signatures, provenance, and Rekor proofs | Exact artifact digest, authenticated signer, provenance requirements, and policy. Share evidence bytes across providers; scope verification results to policy. | Verify all required signature, identity, transparency, and binding checks. A log entry's presence alone is not signature verification. |
| Compose-policy result | Exact compose hash, complete image subjects, tier, and policy. | Bind the compose to fresh endpoint attestation; reject incomplete image coverage. Reevaluate after policy/build change. |
| Signed hardware measurement registry | Exact signed registry artifact, platform, signer policy, and authenticated predicate. | Compare actual attested measurements; do not equate a platform label with a verified measurement. |
| AMD VCEK and signing chains | Product, chip HWID, TCB extensions, certificate digest, and issuer. Share among replicas contacting applicable hardware. | Match HWID/TCB to the fresh report; check chain, signature, validity, and applicable revocation policy. No arbitrary short TTL on immutable certificate bytes. |
| Intel PCS collateral and CRLs | Exact issuer/platform scope, signed collateral version, and content digest. | Respect signed validity and revocation requirements for new admissions; do not cache a timeless `tcb_current: true`. |
| NVIDIA certificates and reference material | Exact supported hardware/firmware and issuer scope. | Independently verify the material with the production pathway. A cached reference is not a verdict on a new GPU report. |
| NRAS response | Exact submitted GPU evidence and client-nonce context, signed JWT, and authenticated response binding. | Not a reusable verification result for another report, GPU, or nonce. A new submission needs its own eligible result and publication-time check. |
| Release metadata | Repository, authenticated release artifact, digest, and measurement predicate. | Match attested content. No latest-release requirement or release-age check. A remembered tag alone does not authenticate deployed bytes. |
| Proof of Cloud evidence | Exact hardware identity, authenticated authority, and response scope. | Specify the authority's verification and withdrawal contract before portable reuse; do not assume registrations are append-only or forever valid. |

Retain evidence validity metadata for admission, not as runtime authorization
expiry. Successful requests must not extend evidence validity or rewrite retrieval
times. Missing or expired mutable collateral triggers the existing retrieval and
verification path when needed for admission. Failure remains a factor failure
under its applicable policy; do not substitute stale evidence.

Transient network errors and malformed responses are not portable negative
verified subjects. Keep bounded negative-cache behavior under the existing retry contract;
never persist an outage as a permanent rejection of an image.

Tinfoil already uses embedded AMD signing chains and its VCEK proxy. Add caching to
that verified retrieval path. Preserve its configured origin and fail-closed
retrieval behavior. See [AMD certificate retrieval](../providers/tinfoil/tinfoil_support.md#amd-certificate-retrieval).

### Material adapters established by source research

The following boundaries use the versions pinned in `go.mod`, rather than assumed
upstream APIs. They are implementation choices, not completed adapters.

| Material | Existing owner and concrete integration |
| --- | --- |
| Intel | `attestation.NewCollateralGetter` already implements `trust.HTTPSGetter.GetContext`, returning both headers and bytes. Wrap it with validated typed lookup and acquisition/export; keep `VerifyTDXQuoteOnline` and go-tdx-guest signature, revocation, and current-time checks. The verifier invokes both quote verification and TCB-level extraction with the same options; deduplicate by authenticated scope, not just invocation count. |
| AMD | `tinfoil.NewSEVCertGetter` returns embedded product signing chains and rewrites validated VCEK requests to Tinfoil's proxy. Insert typed VCEK lookup at this boundary; preserve product/HWID/TCB and certificate verification. Current `sevverify.Options{Getter: ...}` does not enable CRL retrieval. `evalSEVTCBNotRevoked` explicitly says AMD CRL is not checked. The cache must not manufacture a revocation success or add an unbudgeted CRL guarantee. |
| Rekor | `RekorClient` retrieves UUIDs/entries and computes parsed provenance, SET, inclusion proof, and DSSE results. Split byte acquisition from the existing entry verifier, retaining the complete response fields needed for SET/proof verification. Current certificate parsing extracts OIDC extensions; it is not a general independent Fulcio-chain verifier. Retain that actual assurance distinction. |
| Sigstore/TUF | `SigstoreVerifier.fetchAndVerifyAttestation` creates its own root client and calls `FetchTrustedRootWithOptions`. Inject the verified trust-material resolver and use the local metadata sequence in Section 6. The pinned lower-level API supports in-memory verification; the higher-level updater also owns disk cache and refresh behavior and is not a transparent YAML adapter. |
| NVIDIA | `NVIDIAVerifier` owns JWKS entries, singleflight retrieval, creation times, capacity, and rotation retry. Add immutable import/export on that owner, preserving its one-hour eligibility and refresh throttling. Do not copy JWT result caches into a new report. |
| CT | `tlsct.DefaultChecker` currently refers to a process-wide checker; normal and pinned clients attach it. Its log list has a 24-hour lifetime and a separate bootstrap HTTP client. Introduce checker dependency injection per service/command and pass it through normal, pinned, proxy, and dependency-client construction. Sharing one configured checker within that owner is sufficient; mutating the process global from cache import is not. Preserve the bootstrap client's existing WebPKI behavior and every live SCT check. |

This includes two explicit ownership changes: shared evidence orchestration and
per-owner CT injection. None of these dependencies currently supplies a complete
portable artifact snapshot/export operation. Existing getters and verifiers provide
the integration points; wrappers still need immutable bytes, eligibility, scoped
singleflight, and bounded retention. No reusable verification result may be formed
by parsing a display report or trusting an asserted `pass` field.

### Chutes support after transport migration

Chutes is a planned extension for measurement decisions and portable verification
inputs. It is blocked until report, authenticated ML-KEM key, immutable route, and
instance identity use the shared authorization publication/acquisition/invalidation
path. Keep relay TLS reuse independent of backend authorization; never invent an
attested relay SPKI from backend evidence. Scope authorization to provider, chute,
model, instance, and authenticated key epoch, including explicit rules for any
sharing. A new unauthenticated identity requires full admission.

The existing nonce pool is request-material management, not an independent trust
source. Consume each provider-issued request nonce atomically against the matching
authorization. These request nonces are distinct from client-generated attestation
challenges. Exclude consumable nonces and ephemeral encryption secrets from portable
files and endpoint snapshots. Nonce exhaustion or expiry requires replenishment;
it does not alone invalidate an authenticated backend key. Prove key/instance
matching and concurrent consumption before enabling reuse through the shared path.

Exact MRTD and MRSEAM membership decisions are a concrete `--update-whitelist` use
case. Record the authenticated observed configuration and selected exceptions,
without broadening all failures of `tee_measurement`. Such decisions do not assert
build reproducibility, repair invalid GPU signatures or nonce binding, or waive
missing CPU/GPU binding. Retain existing allowed failures in reports, separately
from supported operator decisions. Collect fresh authenticated evidence when
creating decisions; a previous test log is not admission evidence.

Cache eligible Intel collateral through common services. Chutes currently exposes
no compose/component supply-chain surface for client image verification; do not
invent image-signature results or promise image retrieval savings. No integration
with its legacy report/key caches is permitted for early cache support.

### Venice support after transport migration

Venice portable input prefill is planned after migration, separately for dstack and ACI/1.
Its [parser](../../internal/provider/venice/aci.go),
[policy](../../internal/provider/venice/policy.go),
[key-custody verifier](../../internal/provider/venice/keyset.go), and
[attestation-gap reference](../attestation_gaps/venice_aci_gateway.md) define the
available evidence. Do not infer deployment coverage from historical model counts.

For ACI/1, the quote, event log, and measured compose describe the gateway. Cache
its exact compose bytes, digest-pinned component subjects, policy results, and
eligible Intel collateral. The gateway images have `ComposeBindingOnly` provenance
in current policy. Reuse may establish compose coverage and repository recognition;
it must not create a Sigstore-signature result when no such verification exists.
For dstack responses, cache model-tier material only when supplied and verified.
Evidence bytes shared with NEAR still require Venice's own policy evaluation.

Retain original ACI workload keysets and custody signature chains for local checks,
but never trust `workload_keyset_digest` alone: it is a self-asserted consistency
check. Preserve the production chain through the quote-bound E2EE signing key,
keyset membership, custody signatures, accepted KMS root, and event-log-authenticated
application ID. Recheck the keyset's applicable `not_after` condition on admission.
A stored custody result cannot supply a fresh quote/nonce or authenticate arbitrary
keyset fields. Define exact signed inputs and policy scope before reusing a local
custody subcheck; do not add a separately authoritative encryption-key cache.

`source_provenance.repo_url` and `repo_commit` are provider assertions, not
quote-bound release provenance. Retain them only as diagnostic observations.
`downstream_tls_binding` and keyset TLS entries describe the gateway's downstream
hop, not teep's TLS peer. Do not use them to populate teep's attested transport
identity, or assume keyset consistency authenticates that entire downstream claim.

ACI/1 does not provide model-host CPU attestation or its compose manifest. Cache
loading must preserve visible missing-model failures and the exact format-specific
exemptions. It must not turn those gaps into successful or inapplicable factors.
Relayed GPU signatures and nonce checks do not establish CPU/GPU colocation, backend
software identity, or encryption to an attested model host. Keep NRAS and other
report-bound operations under their actual admission rules.

A separate Venice migration must define authorization scope, E2EE identity/custody
lifetime, keyset admission conditions, conditional invalidation, and connection
requirements before enabling any cache capability. Use the shared runtime path;
do not preserve a parallel generic-cache integration for portable prefill. Similar
captured gateway keys across models are not sufficient to enable router-wide reuse.
Endpoint persistence additionally requires the durable-state rules in this plan.

### Required checks and diagnostic retrieval

The current factor implementation establishes the classification. In
`evalSigstoreVerification`, missing compose-only digest lookups are tolerated, but an
empty result list becomes Skip (or an explicit ACI/1 missing-backend failure).
`buildTransparencyNoRekor` returns NotApplicable when no signed components are
required. Provider/component signer evaluators also exclude compose-only components.
Consequently removing HTTP queries without adjusting the typed report inputs can
change enforcement or hide an ACI/1 backend gap.

Represent each compose-only component explicitly as a locally bound component with
no required transparency query. Keep all expected signed components in the required
work set and preserve ACI/1's separate absence of model evidence. Evaluate factors
from typed component coverage, not a fabricated successful Rekor index response.
Regression tests must compare enforced outcomes and preserved gaps before/after this
change. Signed components reuse complete verified Rekor entry bytes and proofs;
index search results are retrieval hints, not the cryptographic proof to cache.

Classify each supply-chain retrieval by the production check it serves. The current
merged-digest path can perform Rekor index/provenance requests even for compose-only
components. A YAML `not_required` value alone must not suppress an enforced factor
or claim those calls were eliminated. Route all required checks through shared
prefill-aware services, retaining cached evidence for any query-dependent required
result. When a query serves only an optional diagnostic, explicitly separate it from
admission and avoid a new pre-inference request; report that the diagnostic was not
refreshed. Verify this distinction against current factor aggregation, not just the
component's signer policy. Never turn skipped diagnostics into successful checks.
If an enforced diagnostic factor still requires a query, retain that request and
revise the budget until its complete reusable result has a defined contract.
Apply the same behavior to cache preparation, serving, and verification, with their
specified completion differences. Venice's budget is conditional on this work and
its shared-runtime migration; the compose-only example promises no image signatures.

### 4a. Release and image binding

For digest-pinned compose images, match the full canonical repository and digest.
For attestation-bound release measurements, match the signed release predicate to
the live measurements through the existing verifier. Retain release tags for
operator readability, not as evidence that a release is current or immutable.

A tag-only compose reference does not prove which image digest was pulled. Resolving
the tag during caching does not close that gap. Such entries must retain their
actual weaker binding classification and diagnostics; they cannot consume an
verification result that claims digest-bound image authentication. The existing factor policy
controls the resulting failure or explicit exemption. Do not add an automatic
`allow_any_version` exemption or label an unbound image as authenticated.

Provider-controlled artifact delivery is acceptable only where independent checks
establish the required authenticity and binding. Choosing an older authenticated
release is not itself a failure. Caching does not require a switch to direct GitHub
or routine `/releases/latest` requests. On a miss, resolve sufficient evidence for
the attested release through the supported retrieval path.

### Resolving authenticated Tinfoil releases on a miss

Use a local index of verified release predicates keyed by repository and measured
subject first. The permitted GitHub proxy supports latest-release metadata,
`/<repo>/releases/download/<known-tag>/tinfoil.hash`, and digest-addressed attestation
bundles. Planning probes of the router repository established that latest metadata
and a known older release download work, but release enumeration (with or without
pagination parameters) and `/releases/tags/<tag>` return HTTP 400 with an unsupported
path response. Do not implement speculative pagination or numeric tag guessing.

On a miss, candidate hints may come from independently verified retained bundles,
a tag delivered by the existing model-to-repository discovery response, and the
supported latest-release response. A discovered tag is only a retrieval hint;
verify the fetched bundle, repository/signer policy, and exact attested measurements.
Retain that hint when resolving direct routes instead of discarding it. Supplied V3
collateral is a candidate only after its format has an implemented verifier; merely
parsing its envelope cannot establish trust. Deduplicate candidates before retrieval.

Bound the known-candidate search to 32 candidates, 32 MiB aggregate response bytes,
and the existing bounded admission deadline, with the existing per-response limits.
Stop on an authenticated exact match. Exhaustion, a missing bundle, or no matching
known candidate fails the affected admission loudly. A latest mismatch never becomes
acceptable because another release could not be located. Record which candidate
source and dependency required each request, separately from verification costs.

A previously prepared matching release remains usable without asking whether it is
latest. However, cold preparation cannot guarantee discovery of every older deployed
release through the currently permitted proxy. This limitation is recorded in
[the planning issues](supply_chain_caching_issues.md#older-tinfoil-release-discovery).
No new tag flag, direct-GitHub bypass, or alternate whitelist input is introduced.
A broader discovery contract or a reviewed reference-hint interface is separate work.
Fixture coverage must include supported known-tag retrieval, unsupported enumeration,
matching older retained evidence, and failure when no known candidate matches.

### 4b. Request reduction and elimination

Count outbound HTTP attempts initiated by teep, including retries and redirects
where supported. Count HTTP/2 streams as requests, not connections. Report TLS
handshakes separately; DNS, TCP, and TLS exchanges are not additional HTTP requests.
Include library-owned clients, Sigstore TUF bootstrap/refresh, NVIDIA JWKS, and CT
metadata retrieval if triggered. Do not infer zero network activity from a cache hit
in only the application-level verifier.

The following is a source-derived request inventory, not a measured benchmark.
Counts describe successful first-attempt paths; dependencies, input size, cache
state, early failures, and retries can change totals. Chutes and Venice targets
apply only after their migration prerequisites are met.

| Request group | Current source-derived work | Prepared-cache target |
| --- | --- | --- |
| NearDirect discovery | Default initial selection retrieves `/endpoints` and `/backends/count`; explicit-index selection needs membership metadata but not a count; configured static routes may need neither. | Preserve route selection rules. No recurring discovery for established selections; a portable image cache does not eliminate cold discovery. |
| Tinfoil discovery | Model/backend mapping through `/.well-known/tinfoil-proxy` where required by the route. | Retain required discovery and its freshness policy. Do not use cached software to authorize stale route mappings. |
| Endpoint attestation | One response per full admission on the normal first-attempt path. NearCloud's response includes gateway and selected model evidence. | Zero on eligible runtime reuse or endpoint restoration; fresh client-nonce request on new admission. Count gateway/backend verification separately from HTTP fetch count. |
| NEAR and applicable Venice image transparency/provenance | For each queried digest: one Rekor index search; for each successful digest, another index search and one or more entry retrievals. Model/gateway digest sets are deduplicated before this pass. | Zero retrievals for complete matching cached evidence/results. On a miss, share lookup work and retain all material needed for verification, rather than repeating the index search. |
| Tinfoil release evidence | Three explicit requests per repository: release tag, `tinfoil.hash`, and digest-addressed attestation bundle. A TDX admission also fetches the hardware-measurement repository. Sigstore TUF work is additional. | Zero release requests when retained signed predicates match the attested measurements under current policy. No latest-release request just to test freshness. |
| Venice ACI/1 | A model-selected attestation request supplies gateway quote/compose/keyset; generic verification may also query gateway digest evidence, Intel collateral, PoC, and relayed GPU services. Custody signature verification itself is local. | Eliminate eligible repeated artifact/collateral retrievals only. Preserve fresh gateway admission and report-bound work; do not promise that absent model provenance becomes verified or that every gateway image has Sigstore material to cache. |
| Chutes discovery, attestation, and request nonces | Model resolution when needed; fresh attestation fetches instances and nonce-bound evidence. Runtime nonce-pool replenishment fetches another instance/nonce batch. Count collateral and report-bound services separately. | After migration, reuse matching instance/key authorization and eligible collateral. Count nonce replenishment separately; zero extra requests requires both a valid authorization and an available matching nonce. Portable files never supply consumable nonces. Measurement decisions alone eliminate no quote or collateral requests. |
| Sigstore trust-root material | Tinfoil's verifier invokes the Sigstore TUF client. Exact downloads depend on dependency bootstrap and local metadata. | Retain authenticated root/metadata dependencies needed for local verification. Zero downloads only when their current verification requirements are satisfied; measure bootstrap and refresh separately. |
| AMD VCEK | Per-chip certificate retrieval through Tinfoil's VCEK proxy; signing chains are embedded. | Zero retrievals when an eligible VCEK matches product, HWID, and TCB. A changed chip/TCB can require a new certificate. |
| Intel PCS / revocation | Dependency-driven retrievals for applicable quote/collateral scope; exact count depends on quote and existing caches. | Zero when retained material satisfies new-admission validity requirements. Fetch only missing/ineligible objects, sharing applicable collateral between gateway/model and replicas. |
| NVIDIA NRAS / JWKS | One NRAS submission per applicable payload on the normal path, plus JWKS retrieval on a key-cache miss or eligible refresh. | Fresh report-bound NRAS submission remains on new admission. Persisting eligible verification-key material may remove JWKS retrieval, subject to an explicit issuer/key-refresh contract. |
| Proof of Cloud | Quote-bound protocol: stage 1 contacts configured peers; stage 2 chains responses through collected quorum peers. Current default has three peers and quorum three; successful full fan-out normally starts six requests. Cancellation and early results can change attempts observed. | Do not reuse a prior quote's JWT for a new quote. Count these calls as live until an independently verified portable registration contract exists or an explicit supported operator exception replaces that requirement. |
| Inference / E2EE | Normal inference sends a request and establishes actual encryption usability. `verify` performs its required live probe; portable `cache` preparation does not add a probe. | Do not add a preliminary inference probe to `serve` merely to consume the cache. Count `verify` probes separately from portable preparation and pre-inference metadata overhead. |

Source references: [NearDirect selection](../providers/near/near_attestation.md#neardirect-backend-selection),
[NearCloud fetch](../../internal/provider/nearcloud/nearcloud.go),
[proxy supply-chain verification](../../internal/proxy/proxy.go),
[Rekor index checking](../../internal/attestation/sigstore.go),
[Rekor provenance retrieval](../../internal/attestation/rekor.go),
[Tinfoil release verification](../../internal/provider/tinfoil/sigstore.go),
[NVIDIA verification](../../internal/attestation/nvidia.go), and
[Proof of Cloud protocol](../../internal/attestation/poc.go).

Let `D` be distinct NEAR digests queried, `S` the subset with index matches, and
`E` the total Rekor entries retrieved. The current successful image path costs
`D + S + E` requests before retry overhead. If every digest succeeds and one entry
per digest suffices, this is `3D`; complete reusable material reduces this group to
zero. For Tinfoil, `R` repositories cost `3R` explicit release requests plus TUF
requests. A one-repository SEV path can eliminate three explicit requests; a
TDX release plus hardware registry can eliminate six. These are conditional
source-derived counts, not claims about total provider startup traffic.

For `N` replicas with identical software subjects, let `M` be the measured portable
metadata retrieval cost and `L_i` each replica's necessary live work. Without shared
preparation, the comparable cold runs cost `N*M + sum(L_i)`. With one preparation,
they cost `M + sum(L_i)`, plus artifact distribution and preparation-specific live
work. Report both preparation and serving costs: moving requests earlier eliminates
them from the inference critical path, but not necessarily from total network use.
Avoid double-counting shared objects or already effective dependency caches.

### 4c. Scenario budgets and acceptance measurements

| Scenario | Locally reused or reevaluated | Requests still permitted before inference |
| --- | --- | --- |
| Cold start without prepared material | None assumed | Required discovery, fresh attestation, software evidence, collateral, report-bound services, and dependency metadata. Establish the baseline. |
| Prepared portable file, new replica | Matching verified subjects; locally eligible evidence | Discovery, fresh endpoint admission, NRAS/PoC where applicable, and only missing/ineligible dependency material. Cached image groups must issue zero requests. |
| Same-build restart with eligible endpoint restoration | Complete persisted authorization and applicable durable state | Required route resolution and current TLS/CT operations; zero full-attestation, software, PCS, VCEK, NRAS, or PoC requests for the restored scope. |
| Read-only replica restart | Portable file only | New endpoint admission and its report-bound checks. No promise of zero admission requests. |
| Teep build update, evidence sufficient for local reevaluation | Recompute software verification and validate decision compatibility locally | Zero software retrievals; fresh endpoint admission remains required after build mismatch. Admission-time expiry may require collateral retrieval. |
| Policy change or missing/new verifier dependency | Recompute only eligible facts under current policy | Fetch precisely the missing/ineligible dependencies. Report which requirement caused each request. |
| Endpoint key rotation / new authority | Reuse unchanged software subjects | Fresh endpoint admission, route work as required, and new hardware collateral if needed. No retrieval of unchanged software merely because a key changed. |
| Operator decision applied | Match exact decision and evaluate its prerequisites locally | Only the request group explicitly replaced by that decision can be omitted. All other admission and trust checks remain. |

Define a deterministic request-count suite before implementing reuse. Use production
verifiers, TLS test servers, valid captured/signed evidence, and counters around
all outbound clients. For `nearcloud`, `neardirect`, `tinfoil_v3_cloud`,
`tinfoil_v3_direct`, record the baseline and proposed counts for every scenario.
Add Chutes and both Venice formats only when their migrations permit cache support.
Include both CPU evidence types where supported and multi-model/router sharing.
Keep deterministic Tinfoil direct scenarios in this suite. Defer its live
measurements under the Section 1 prerequisite, then run them against fixed
endpoints; continue Tinfoil cloud live measurements independently.
Mark all pre-migration Chutes/Venice cache scenarios blocked, not zero-call successes.
After migration, mark endpoint restoration unsupported until separately enabled.
For Chutes, cover nonce exhaustion/expiry, concurrent nonce consumption, same-key
reuse, new instance/key admission, and independent model activity. Exercise dstack and ACI/1 concurrently at the same Venice
origin, including a model changing response format; no cached model result may
satisfy a gateway-only response. Measure Venice discovery separately when model
listing is requested, and retain its format-specific factor outcomes.
Count network envelopes separately from attested subjects and component checks.
NearCloud can deliver gateway and selected backend evidence in one HTTP response;
do not budget another backend fetch when complete valid stapled evidence suffices.
Count independently required collateral/provenance/NRAS requests, and distinguish
material present in a response from material the shared verifier actually consumes.
Compare direct-first/cloud-second and cloud-first/direct-second preparation with
identical component evidence: fetch shared eligible objects once while evaluating
each consumer's policy. Repeat for identical versus different compose bytes, fresh
nonces, changed keys, and missing or ineligible stapled material. No cached report
or matching key can replace fresh required admission or missing backend evidence.

Measure component counts independently of CVM, model, and authorization counts.
Include shared components across tiers, multiple repositories per compose/release
set, one changed component, and a cache containing only the first required member.
A newly added component incurs only its required retrievals, but complete-set
coverage must be reevaluated. Supplied Tinfoil collateral saves no retrieval until
the shared verifier independently accepts it. Inspect actual parser/verifier use,
not just metadata presence, when claiming request elimination.
Do not weaken enforcement or replace cryptography with success mocks for benchmarks.

Measure request attempts by destination and purpose, bytes fetched, cache hits,
local reevaluations, handshakes, and time from inbound inference request to the first
upstream inference request. Report total first-response latency separately. For
repeatable timing runs report sample count and median/p95, separating cold and warm
dependency caches. Keep fixture counts as assertions; live counts are observations
with provider/model, build/policy, date, and dependency-cache conditions recorded.
Use the existing live-test opt-in. Store measurements in test artifacts, not a
running validation log in this plan.

Acceptance requires zero calls to cached software groups, zero metadata/admission
calls for eligible already-acquired authorizations (with Chutes nonce replenishment
counted separately), local software reevaluation on
upgrade without retrieval when evidence suffices, and precise remaining-call counts.
Test network denial to cached groups while necessary live groups remain available.
Run these scenarios through the normal request handler and authorization
acquisition path with prefilled shared state, not a separate cache-only path.
Unknown counts must be measured, not reported as zero. A result that skips required
checks does not count as a cache saving.

### 4d. Budgets for the complete YAML examples

These are conditional, source-derived successful first-attempt budgets for one
selected route, not measured cache-implementation results. Assume matching software,
hardware-scoped collateral, policy, eligible trust metadata, and the GPU evidence
shown. Count all teep-owned and library-owned HTTP requests, excluding the inference
request itself. The complete portable examples target zero software, collateral,
JWKS, TUF, and CT metadata retrievals. TLS handshakes and local checks still occur.
`verify` adds its required live probe and always performs fresh admission; it cannot
use the restoration column. "Complete validation" means every applicable check is
evaluated under effective policy, retaining permitted failures and provider limits;
it does not imply that unavailable backend evidence becomes authenticated.

| Example and route assumptions | Software-only prefill, other dependency stores cold | Complete portable prefill, same build | Build update, retained dependencies sufficient and eligible | Eligible same-build endpoint restoration |
| --- | --- | --- | --- | --- |
| 6a NearCloud: one response, gateway and backend TDX, backend GPU | 23 + CT requests | 14 | 14 | 0 |
| 6b NearDirect: default selection with two discovery requests, TDX and GPU | 15 + CT requests | 10 | 10 | 2 discovery requests |
| 6c Tinfoil direct: one route-discovery request, TDX and GPU | 14 + CT requests | 9 | 9 | 1 discovery request |
| 6h Tinfoil cloud: fixed SEV router, no backend GPU evidence | 2 + CT requests | 1 | 1 | 0 |
| 6e Venice ACI/1: selected model, gateway TDX and relayed GPU evidence, after migration | 13 + CT requests + any diagnostic image retrievals | 8, after diagnostic retrieval behavior is resolved | 8 under the same condition | Unsupported until separate restoration eligibility exists |

All columns that assume software reuse also require the required-versus-diagnostic
classification above, including compose-only NEAR components. Remaining required
image queries add to the budget; no `not_required` example field can waive them.

The software-only column assumes same-build software results can bypass release
verification; after a build update, missing Sigstore trust material adds TUF traffic.
Do not count TUF as inherently necessary when complete matching trust material permits
local verification. Every complete-prefill number requires the asserted dependency
coverage, including CT. If that contract is not implemented or material is ineligible,
report the added requests and the unmet requirement; do not advertise the smaller
number as achieved. Current captures omit some library/bootstrap traffic and cannot
establish these end-to-end totals by themselves.

The decompositions are:

- NearCloud: 1 attestation + 8 Intel collateral + 1 NRAS + 1 JWKS + 12 Proof of Cloud
  = 23 before CT. Full portable dependencies remove 9, leaving 14.
- NearDirect: 2 discovery + 1 attestation + 4 Intel + 1 NRAS + 1 JWKS + 6 Proof of Cloud
  = 15 before CT. Full dependencies remove 5, leaving 10.
- Tinfoil direct: 1 discovery + 1 attestation + 4 Intel + 1 NRAS + 1 JWKS + 6 Proof of
  Cloud = 14 before CT. Full dependencies remove 5, leaving 9. This remains source-only
  until the direct live-validation prerequisite is satisfied.
- Tinfoil cloud: 1 attestation + 1 VCEK = 2 before CT; a matching VCEK leaves 1.
  AMD signing chains are embedded. No backend validation is inferred.
- Venice ACI/1: 1 attestation + 4 Intel + 1 NRAS + 1 JWKS + 6 Proof of Cloud = 13
  before CT and diagnostic image lookups; eligible dependencies leave 8.

The NEAR captures used for the examples are
`nearcloud_z-ai_glm-5.3-flash_20260910_153519` and
`neardirect_z-ai_glm-5.3-flash_20260909_201111` under
[provider replay data](../../internal/integration/testdata/). NearCloud contains eight
Intel requests with five distinct URLs: sharing retrievals can reduce that group
to five before persistence, while complete eligible prefill reduces it to zero.
The captures contain early Proof of Cloud 403 responses; fewer recorded responses
are not successful quorum budgets or proof that no cancelled attempts started.
Count six requests per successful default three-peer quote verification, separately
for gateway and backend. Do not persist failure shortcuts as positive evidence.

For 6d, a measurement decision replaces a local expected-value comparison and
eliminates no quote/collateral requests by itself. Combine it with complete portable
material. Example 6g illustrates relationships, not a complete provider scenario.
Example 6f combines with 6a and its deployment-owned durable state to achieve its
restoration budget; a read-only replica or build change instead uses fresh admission.
Restoration can still need route discovery, new TLS handshakes, and ineligible CT
metadata. A currently acquired runtime authorization needs no renewed admission;
HTTP/2 reuse does not itself authenticate a new scope.

Each primary example must have executable fixture coverage for all applicable
columns. Assert zero network calls to every prepared dependency group with network
access to those groups denied, and exact counts for live groups. Repeat with expired
collateral, stale JWKS/CT metadata, unknown key IDs, changed hardware, missing TUF
transitions, and build/policy changes. Verify the precise necessary retrieval or
failure rather than filling missing data from ambient developer caches. Publish
preparation costs, cold-start costs, and per-request reuse separately.

### 4e. Captured baseline and populated-evidence experiment

The planning baseline below counts recorded HTTP responses in the named fixtures,
not all attempts or current live-provider traffic. Library-owned bootstrap requests
and cancelled requests may be absent. Fixture verification uses capture-time rules;
these old bytes cannot be imported as currently eligible collateral without current
eligibility checks. The successful NEAR fixture tests confirm existing production
verification behavior under their configured factor policy, not every factor passing.

| Fixture under `internal/integration/testdata/` | Recorded request groups | Total responses |
| --- | --- | --- |
| `nearcloud_z-ai_glm-5.3-flash_20260910_153519` | Attestation 1; Intel 8; NRAS 1; JWKS 1; PoC 2 (403); Rekor 33 | 46 |
| `neardirect_z-ai_glm-5.3-flash_20260909_201111` | Discovery 2; attestation 1; Intel 4; NRAS 1; JWKS 1; PoC 1 (403); Rekor 10 | 20 |
| `tinfoil_v3_cloud_glm-5-2_20260817_003424` | Attestation 1; AMD chain/VCEK 2; release 3 | 6 |
| `venice_aci_e2ee-glm-5-2-p_20260826_010725` | Attestation 1; Intel 4; NRAS 1; JWKS 1; PoC 6 (200); Rekor 10 | 23 |

The Tinfoil capture predates embedded signing-chain retrieval, so one recorded AMD
request is no longer necessary in the current source. Neither these response totals
nor a 403 shortcut replaces the successful-service budgets in Section 4d.

A planning experiment populated content-addressed evidence lists from original
response bodies and issuer-chain header values, deduplicated by SHA-256, and checked
base64 round-trip/digest integrity. Serialized size below is compact JSON (valid YAML)
with digest/kind/payload fields. It includes retained historical service/discovery
responses for coverage analysis, not a prescription to export all those responses.
No inference exchanges, credentials, consumable nonces, or private keys were imported.

| Fixture | Distinct retained objects | Decoded bytes | Serialized bytes |
| --- | --- | --- | --- |
| NearCloud | 38 | 568,000 | 762,303 |
| NearDirect | 20 | 466,552 | 624,705 |
| Tinfoil cloud | 6 | 95,635 | 128,331 |
| Venice ACI/1 | 23 | 313,948 | 421,614 |

These establish representative byte costs and the need for header preservation;
they are evidence-only research artifacts, not completed cache-loader fixtures or
benchmarks of startup latency. TUF and CT material are absent from these captures
and must come from their authenticated acquisition paths. Build the executable core
examples by extracting the actual original compose strings, complete Rekor responses,
and typed CPU collateral from these fixtures; never stamp current-build successful
evaluations on historical bytes without verification. Keep live elapsed-time reports
separate from deterministic request-count acceptance.

A separate local prototype used the pinned go-tuf/Sigstore libraries on authenticated
public repository metadata: bootstrap root 14, signed transition to root 15,
timestamp 782, snapshot 165, targets 14, and the hash-verified `trusted_root.json`
target. The `trustedmetadata` update sequence and Sigstore root parser completed
locally in approximately 5 ms in one run without constructing a network client.
This demonstrates API feasibility, not a performance guarantee, proof of latest
metadata, or full offline Tinfoil admission. Retain root transitions in the examples:
the pinned bootstrap root alone was expired at the research date, while the verified
successor was eligible. Production uses current time and actual metadata versions.

## 5. Commands and deployment

```sh
teep cache --all-models
teep cache --model nearcloud:example-model
teep cache --model neardirect:example-model --model tinfoil_v3_direct:example-model
teep cache --model nearcloud:example-model,tinfoil_v3_cloud:example-model
```

All names above are illustrative. `--model` requires fully qualified
`provider:model` names and is mutually exclusive with `--all-models`. Resolve active
providers as `teep serve` does; reject unknown/inactive providers, ambiguous models,
invalid names, and an empty active set. There is no provider positional argument.

`teep cache` constructs the shared admission services with command-owned bounded
lifecycle and dependencies, resolves each target, obtains fresh client-nonce
attestation, and runs normal online admission using eligible portable verified subjects and evidence.
In `--update-whitelist` mode, apply the selected decisions as specified below and
require successful evaluation under the resulting effective policy. It uses
the production verification and binding pathways; it does not create success
results by copying a stored report. Offline and debug-force operation must not
produce portable verified subjects. Record explicit `allow_fail` outcomes accurately;
never reuse a waived check as a passed check under stricter policy. Export immutable
snapshots of the persistable material produced by those shared services. Do not
reconstruct trust from display reports or reimplement verification in the command.

Write complete successful targets only. For a multi-target run, preserve unrelated
entries, retain valid dependencies shared with them, collect target failures, and
exit nonzero if any target failed. Failed targets gain no successful cached evaluation
or endpoint authorization. Decision scope can cover several models: an explicitly
reviewed decision admitted through a successful target may affect a failed target's
future policy, but must not claim that target passed. Show known affected models and
any wider provider/tier scope before confirmation; do not imply complete fleet coverage.
Commit an addition only if at least one selected successful target establishes all
its prerequisites. Reevaluate successful targets against the exact committed subset.
Withdrawals are independent restrictive policy operations and may commit even when
all live targets fail, with nonzero target-validation status reported separately. A portable cache target is successful after all non-deferred admission checks under
its effective policy are satisfied, as defined below; deferred live usability is
reported separately. Validate all target syntax before network activity or writes. Do not
claim that `--all-models` discovered every physical backend: it covers discovered
models and the routes actually verified. Tinfoil cloud may share router evidence
while retaining separate per-model probe diagnostics.

Use `--cache-file`, then `$TEEP_CACHE_FILE`, then configured `cache_file`, then
`~/.config/teep/cache.yaml` as identical location precedence for `cache`, `serve`,
and `verify`. Use one shared resolver and loader; all three automatically read an
existing default file. A missing implicit default starts empty. An explicitly
selected path (flag, environment, or configuration) that is missing is an error in
`verify` and in `serve` unless `--autocache` is selected. `teep cache` and `serve --autocache` may create their
output file after validating its destination. A malformed or insecure file always fails loudly.

`teep serve` constructs the shared admission services and authorization store
whether or not a disk file is configured. Startup loads eligible material through
the prefill interfaces before it becomes available to request acquisition. Cache
misses join the same bounded live verification used without prefill. Newly verified
material populates the same stores and can be exported to a configured writable
file only when `--autocache` is selected, using the same snapshot/export path as
`teep cache`. Without this option, `serve` does not write portable evidence. A read-only cache destination
supports image-layer deployment without attempted writes. Failed optional evidence write-back
leaves a separately completed in-memory verification valid, emits an error, and
must not claim persistence. This is distinct from mandatory durable invalidation
for restored endpoint authorization, whose failure blocks affected reuse.

### Configuration and persistence controls

Use `cache_file` for the shared artifact path. Ordinary `serve` is a read-only
portable consumer unless `--autocache` is selected; filesystem permissions provide
the deployment's read-only restriction. No separate undefined read-only declaration
is required. An autocache writer validates destination writability at startup.

Optional endpoint persistence uses explicit service configuration:
`cache_endpoint_persistence = false` by default, and `cache_state_dir` for a
restricted deployment-owned writable state directory. Reject a missing state path
when enabled and reject an unused state path when disabled. Require `--autocache`
when enabling endpoint persistence so cache-file writing is explicit. This setting
adds a separate mandatory durable writer; the optional evidence writer never becomes
responsible for invalidation durability. A service may restore eligible endpoints
and persist its own completed live authorizations only when this setting and its
state directory pass startup validation. `cache` and `verify` never create or restore
endpoint authorizations; these service-only settings have no effect on them.

Endpoint persistence records only a complete successfully admitted authorization;
any deferred required transport/E2EE usability check must first complete. Fsync the
authorization and its durable-state relationship before claiming persistence.
Classified invalidation/eviction uses the synchronous mandatory state path even if
optional evidence writes are backed up. Failure follows Section 3c, not the optional
writer's continue-serving rule. Missing initialization state requires fresh admission;
explicit provisioning initializes a new state directory without trusting old endpoint
records. Publish these config fields with Phase 13, not as usable earlier options.

### Admission and command completion

Portable evidence becomes export-eligible when all non-deferred admission checks
satisfy effective policy. A deferred `e2ee_usable` result is recorded as deferred,
not passed; independently verified software can be exported without an inference
probe. `serve --autocache` uses this boundary before the inference outcome is known.
`cache` is preparation, so it performs no inference probe solely to populate portable
software and reports deferred usability separately. `verify` retains its live probe
behavior and cannot report complete live verification when its required probe fails.
An inference 429 may therefore leave valid cached software while making live `verify`
fail. Shared checks must agree across commands; their completion criteria differ
explicitly. Endpoint persistence has the stricter completion requirement above.

### Read-only policy validation with `teep verify`

`teep verify` loads eligible portable evidence and explicit operator decisions from
the same resolved cache path as `cache` and `serve`. It obtains fresh endpoint
attestation and evaluates it through the same shared admission and effective-policy
services. It may reuse eligible software evidence and collateral to avoid repeated
retrievals, but must not restore persisted endpoint authorizations or substitute a
previous endpoint report for current admission. Retain required online checks and
verification probes; apply the Tinfoil direct live-validation prerequisite.

Report original factor failures, matching decisions, remaining enforced failures,
and unused decisions separately. An unused decision is diagnostic, not by itself a
verification failure. Success requires every selected target to satisfy effective
policy and required verification steps. An enforced cryptographic failure still
blocks; an existing explicit `allow_fail` remains visible as a permitted failure.
The unsupported-decision inventory prohibits creating new exceptions through this
command, not application of existing factor policy. Report selected models and observed endpoints without implying coverage of
unobserved backends. The purpose is to show that effective policy accounts for all
enforced failures, not to require whitelist entries for factors that already pass.

For automated rollout, identify the exact loaded cache content digest, effective
policy identity, and teep build in the report. Compute the digest from the bytes
actually loaded, not a later reread of a concurrently replaced file. Report an absent
implicit default explicitly. Evaluate one immutable loaded snapshot per invocation,
so automation can associate results with the tested artifact.

Verification is read-only: do not create a missing cache, persist retrieved evidence,
create decisions, or run the autocache writer. Offer no separate whitelist input;
operator decisions and their supporting evidence use the single cache artifact.
Refactor CLI orchestration to call shared admission services where necessary; proxy
unification alone is not proof that the CLI uses them. Test equivalent effective-policy
outcomes across `cache`, `serve`, and `verify`, accounting for fresh admission versus
permitted runtime reuse.

Cache-aware verification is a post-migration cache-enablement requirement for Chutes and Venice.
Until migration, reject cache-backed verification of those providers explicitly;
ordinary live verification retains its existing behavior. For an implicit default
file containing only unrelated provider data, validate the file but report that no
cache capabilities apply to the unsupported target and proceed with ordinary live
verification. If relevant records exist, or a candidate file was explicitly selected
by flag/environment/configuration, reject unsupported cache-backed verification.
`verify --no-cache` provides an explicit baseline path even when a default exists;
it reads no cache, applies no cache decisions, and must not claim rollout validation.
PhalaCloud and NanoGPT remain outside cache scope. Apply the same relevance rules to
ordinary `serve` routes; do not silently discard matching policy or add legacy adapters.
Explicit `cache` targets remain rejected, with multi-target failure reporting.

Remove `--update-config` and `--config-out` when delivering the operator decision workflow in Phase 10. Move
operator measurement-policy input to explicit operator decisions in the same
migration; only `--update-whitelist` may create TOFU pins from observations. Reject
old config fields rather than silently ignoring them. Update CLI help, configuration
examples, and provider documentation together.

Operators can generate one trusted cache file and distribute it to replicas through
their trusted deployment system. Replicas can reuse matching verified software subjects
without independent GitHub/Sigstore retrievals. Different builds can reuse retained
evidence only after current verification. Read-only replicas perform fresh endpoint
admission and keep runtime authorization in memory.

Prompt-cache secret generation/persistence from issue #134 is separate optional
work. Such secrets are not portable public evidence and must not be distributed in
this shared artifact by default. Never include API credentials in the artifact.

### Automatic persistence with `teep serve --autocache`

`--autocache` is an opt-in writer to the cache location selected above. It persists
reusable evidence and verification results encountered during normal service; it
does not add a discovery loop, poll for newer releases, or introduce a second
verification path. Explicit `teep cache` prepares material before traffic arrives.
Autocaching pays the normal retrieval cost on the first encounter and reduces
later retrievals across restarts or replicas that receive the file.

Export eligible material only after successful admission under the current policy,
through an immutable snapshot of the shared verification/admission state. Inference
response success is not the publication trigger: an upstream rate limit does not
undo independently completed verification. Do not export failed or incomplete
admissions as reusable successful results. Apply the existing per-object portability
rules even when the containing authorization was admitted successfully.

For example, a newly encountered compose with changed image digests requires normal
compose binding and verification of those image subjects. Newly encountered Tinfoil
release metadata must authenticate a release matching the attested measurements.
Only then can eligible objects and results be persisted. A mutable tag, provider
assertion, or latest-release lookup alone cannot establish a verified subject.

Autocaching does not create, expand, or revive operator decisions. New evidence must
pass current policy or match an existing explicit decision. Preserve base failures,
`allow_fail` exemptions, decision references, and their exact policy dependencies;
a permitted failure does not become cryptographic success in the file. Import under
a different policy must reevaluate compatibility. Keep decision creation exclusive
to `teep cache --update-whitelist`; `serve` does not offer automatic TOFU.

Use a server-owned asynchronous writer with bounded pending work, deduplication,
and coalescing by exact subject/policy identity. Request handlers enqueue or mark
eligible shared state for export without waiting for filesystem I/O. If capacity
is exhausted, retain a bounded dirty-state indication for later snapshot work and
emit a diagnostic; do not create an unbounded queue or delay authorized inference.
Use the same locked read-merge-write transaction as `teep cache`, preserving
unrelated targets and operator decisions on disk. Reconcile current deletion and
invalidation state before merging so delayed snapshots cannot restore withdrawn
trust. Do not hold runtime store mutexes during disk I/O.

Reject `--autocache` with a read-only cache destination or an unusable destination at
startup, before accepting requests. Validate existing files strictly; the option
must not overwrite malformed or insecure input. A later write failure leaves an
independently verified in-memory authorization usable, emits a non-secret error,
and records that persistence failed. Use bounded retry with backoff, coalescing
repeated work; expose pending writes, failures, and the last successful write.
On orderly shutdown, attempt a bounded flush and report unfinished persistence.
A crash may lose pending optional evidence writes; atomic replacement must preserve
a valid committed file. Never claim that asynchronous enqueueing guarantees durability.

The option alone covers portable evidence/results, not complete endpoint authorizations.
Explicit endpoint-persistence configuration adds its separately enabled durable-invalidation contract;
its mandatory writes must never use the optional writer's failure semantics.
Exclude private keys, ephemeral encryption secrets, inference payloads, and
consumable Chutes request nonces. Chutes and Venice remain blocked until their shared
runtime migrations; PhalaCloud and NanoGPT remain outside scope. Apply the same
provider eligibility checks to automatic export as to explicit cache targets.

### 5a. Whitelist inventory

This is an inventory of proposed decisions, mapped to the current factor families
in [report.go](../../internal/attestation/report.go) and the provider
[NearDirect](../../internal/provider/neardirect/policy.go),
[NearCloud](../../internal/provider/nearcloud/policy.go), and
[Tinfoil](../../internal/provider/tinfoil/policy.go) policies. Gateway equivalents
follow the same rule but have separate scope. One report factor can combine
multiple failures: implementation must expose typed reasons before allowing a
specific reason to be overridden. Whitelisting `tee_hardware_config` as a whole
would also waive unrelated hardware checks and is not an exact pin.

**Ordinary** means eligible for explicit selection with `--update-whitelist`.
**Elevated** means a proposed candidate that additionally needs a concrete per-change
risk acknowledgement. Elevated support must remain disabled until its typed checks,
prerequisites, and tests exist; the generic flag does not enable it. **Unsupported**
means this command cannot turn the failure into a reusable decision. These are
requirements for the new decision path, not changes to existing `allow_fail` or
release/debug enforcement semantics.

| Observed condition / factor family | Exact decision and prerequisites | Class | Request effect on later admission |
| --- | --- | --- | --- |
| Unlisted repository: `component_recognition` | Add exact canonical repository within provider/tier; retain signature, signer, and attested-content checks. Repository recognition alone grants no signature trust. | Ordinary | Local list update; independently verified image evidence is still required. |
| New or changed component/provider signer: `provider_signer_recognition`, `component_signature_recognition` | Pin exact key fingerprint or OIDC issuer plus workflow identity, repository, and tier. Verify possession/signature and existing certificate chain independently; do not pin a mere name or arbitrary root. | Ordinary TOFU | Identity comparison becomes local; cached signature/transparency material eliminates retrieval only when otherwise sufficient. |
| Unlisted TDX MR_SEAM/MRTD/RTMR or SEV launch measurement: `tee_measurement`, allowlist portions of `tee_hardware_config` / `tee_boot_config` | Pin the explicitly selected measurement fields as one correlated match condition, with platform/provider/tier scope, from an authenticated fresh quote. Retain the full observation as evidence; unselected registers are not added to the decision match. Keep inseparable security conditions required by the decision class. Retain debug, TCB, revocation, quote-signature, event-log, and REPORTDATA checks; never combine independently observed fields into an unobserved allowed tuple. | Ordinary TOFU | Local expected-value comparison. Does not eliminate PCS, NRAS, or fresh quote requests. |
| Valid signed code/boot reference does not cover current attested measurements: `sigstore_code_verified`, signed-registry match in boot/gateway measurement factors | Pin exact current authenticated measurement tuple as an operator reference, preserving the mismatch with the publisher's reference. Do not claim the provider signed these measurements. | Elevated: `unpublished_measurement` | May replace reference lookup for this exact tuple only; no removal of hardware authentication or unrelated image checks. |
| New compose or image digest with valid signature/provenance | Pin exact attested compose hash or repository/digest; compute complete dependency coverage. If existing policy already permits it, use ordinary caching without an operator decision. | Ordinary when only expected-value policy fails | Matching retained software evidence eliminates repeated artifact queries; live compose binding remains. |
| Missing image signature or transparency evidence: `sigstore_verification`, `build_transparency_log`, provenance requirements | Pin exact digest-bound content or authenticated compose/measurement subject, explicitly waiving only the named missing provenance requirement. Require a definite absence result or an explicit operator-selected provenance waiver; a timeout alone must not automatically create one. | Elevated: `unverified_provenance` | Can eliminate the specifically waived provenance query for the pinned subject; records operator trust, never signature/transparency success. |
| Tag-only image reference / mutable tag | A tag cannot identify deployed bytes. An exact compose pin may trust the observed configuration, with an explicit weaker image-binding classification; it cannot invent a digest binding. | Elevated: `unbound_image_version` for configuration-only pin; unsupported as an image-digest pin | No claim of authenticated release caching. Other required evidence and live compose binding remain. |
| Authenticated outdated TCB or advisory: `tee_tcb_current` and gateway equivalent | Pin exact platform/TCB/measurement and authenticated status/advisory identifiers, after all cryptographic checks. Do not generalize to all future firmware on the machine. | Elevated: `outdated_tcb` | Status-policy comparison is local. Valid collateral retrieval and revocation checks remain unless separately and explicitly excepted. |
| Explicitly revoked content, platform, or certificate: `tee_tcb_not_revoked` and applicable authenticated revocation decisions | Candidate exact subject, issuer/revocation record, TCB or certificate serial, and acknowledged reason. Require a verifiable chain/signature apart from the selected revocation failure. Never apply an image exception to a revoked hardware key. | Elevated: `revoked_subject`; disabled until the precise supported revocation classes are defined | A decision about one observed revocation does not waive new revocations or stop retrieval needed for other checks. Zero-request behavior requires a separately specified scoped status pin. |
| Expired collateral or certificates used for new admission | Candidate exact signed object/issuer/subject with explicit time-policy exception and intact signatures. Separate certificate validity, CRL next-update, and collateral expiry; do not reset timestamps. | Elevated: `expired_material`; disabled until per-type semantics are defined | May remove only the refresh required by the excepted time check. It cannot replace fresh nonce evidence or treat an expired NRAS/PoC JWT as a new report result. |
| Hardware registration absent: `cpu_id_registry`, gateway equivalent | Pin authenticated hardware identity and observed registration failure; describe this as waiving registration, not proving cloud membership. Define a stable authenticated hardware selector before support. | Elevated: `unregistered_hardware` | Can omit the exact registration requirement when supported. A quote-bound PoC JWT cannot itself serve as a portable registration pin. |
| Venice ACI/1 accepted KMS root mismatch: typed `aci_key_custody` reason | Pin an exact recovered KMS root for Venice ACI/1 gateway custody, with retained valid signature chains, derivation purpose, quote/key binding, and authenticated app ID. This changes the key-releasing authority, not an image signer. Record independent corroboration when available; otherwise identify the decision as TOFU. | Elevated: `kms_authority`; disabled until isolated typed checks and scope are implemented | Root membership is local; all custody, nonce, key, and quote checks remain. No general removal of attestation or collateral requests. |
| Venice ACI/1 gateway application ID mismatch: typed `aci_key_custody` reason | Pin exact event-log-authenticated app ID under a specified accepted KMS root and gateway scope. Retain event-log replay and custody signatures; do not permit arbitrary applications of that KMS. | Elevated: `gateway_application`; disabled until isolated typed checks and scope are implemented | Application membership is local; fresh gateway evidence and all other admission checks remain. |
| Debug enabled, insecure guest policy, insufficient CPU/GPU/key-custody binding | These can remove confidentiality guarantees despite correct signatures. Do not conflate them with a changed register allowlist. | Unsupported in this cache decision path | No request saving or reusable pin from the failure. |
| Invalid quote/image/JWT signature, untrusted hardware root, nonce mismatch, REPORTDATA/compose/event-log mismatch, key substitution, malformed evidence, unusable encryption, TLS/WebPKI/CT failure | No new decision may waive this failure. Existing explicit factor policy is evaluated separately; never label a permitted failure as successful cryptographic verification. | Unsupported | No new bypass or successful result for the failed check. |

For missing provenance, distinguish absence from an invalid supplied signature.
The absence decision must not absorb a definitive cryptographic failure. For
measurement changes, separate authentication of the quote from trust in its measured
software: TOFU supplies the latter only. An authenticated but revoked subject is
not the same condition as a forged signature; inventory and diagnostics must retain
that distinction even where elevated support is not yet implemented.

### Ordinary decision implementation boundaries

Source research identifies two different expected-value mechanisms. Model/gateway
`MeasurementPolicy` contains MRTD, MRSEAM, and indexed RTMR allowlists; a selected
unlisted-field tuple can become an ordinary measurement decision while preserving
all other quote and configuration checks. In contrast, mismatch with a signed
Tinfoil code/hardware predicate remains `unpublished_measurement`, an elevated
candidate. Do not classify it as an ordinary register-list edit merely because
both compare measurements.

Repository recognition and signer recognition are separate evaluators. An ordinary
repository decision may extend the recognized repository/tier while retaining a
fully specified existing applicable signer/provenance rule. If no such rule exists,
repository recognition alone cannot create it. Organization-signer rules apply only
when their repository/workflow/issuer predicates independently match. New image or
compose versions already accepted by current policy require ordinary caching, not
an artificial expected-digest decision: the current `ImageProvenance` policy does
not define a universal expected-image-digest whitelist.

Use typed verification failures and observations emitted before factor formatting;
never identify an exception by matching error-message text. A signer candidate needs
independently verified key possession and its required trust prerequisites even when
the expected identity comparison fails. In the Tinfoil path, separate that comparison
from bundle authentication instead of treating `FetchAndVerify`'s combined error as
an authenticated observation. In the NEAR path, parsing certificate OIDC extensions
alone does not independently validate the issuer chain. Ordinary raw-key fingerprint
TOFU may use verified possession/provenance; an OIDC-identity exception requiring an
independent chain is unavailable until that prerequisite is implemented. Do not claim
additional assurance for existing `NoDSSE` or compose-only components.

Before authoring candidates, classify each as implemented ordinary, existing-policy
accepted, elevated/deferred, or unsupported. Preserve all base failures, prerequisite
outcomes, applicable exemptions, and exact selected fields. These capability limits
are recorded in [the planning issues](supply_chain_caching_issues.md#ordinary-decision-coverage)
and apply equally to interactive and proposal workflows. They introduce no class flag
or separate policy input. Phase 9 tests each supported typed reason and rejects the rest.

### 5b. Operator decision command and reporting

The normal workflow selects concrete proposed changes, not decision-class names.
Keep the typed inventory internal to validation, serialization, and diagnostics;
operators do not need to learn it to select a firmware or repository change.

```sh
teep cache --model neardirect:example-model --update-whitelist
teep cache --all-models --update-whitelist
```

For additions, fetch and verify current evidence before presenting choices. Offer
existing-decision removal without requiring that collection. Show each eligible
change in plain language, such as “Accept this firmware measurement for Chutes” or
“Accept this TDX module measurement for Chutes,” with the exact value, provider,
target/tier, scope, supporting evidence, retained checks, remaining failures, and
requests eliminated or retained. Chutes examples apply after its required migration.
Show unsupported failures with explanations, but never make them selectable.
Already permitted subjects need ordinary caching, not a new operator decision.

Require explicit selection of concrete changes, a nonempty operator explanation,
and confirmation of the complete policy delta before writing. Accept the explanation
interactively or through `--reason`. Nothing is selected implicitly; an empty
selection or cancellation writes no decisions. Support both explicit model targets
and `--all-models` in interactive and proposal-generation workflows. All-model
selection controls discovery scope; it does not automatically accept failures. Also
present existing in-scope decisions for removal even if discovery fails. A removal-only
transaction does not require discovery or provider connectivity; report any separately
requested live validation failure without blocking the withdrawal.
Group proposed changes by provider and authenticated subject, show all affected
models, and deduplicate identical decisions only when their trust scope permits it.
Allow explicit bulk selection of the displayed ordinary changes, followed by review
and confirmation of the complete delta. Elevated changes retain their individual
acknowledgements; unsupported failures remain unselectable. Freeze the discovered
targets and exact proposals for review so later discovery cannot silently expand
what is applied. Use the same per-target success and partial-failure rules as other
multi-target cache runs. Do not print inference data or credentials.

For each supported elevated-risk change, require a separate explicit acknowledgement
that describes its concrete consequence and is bound to that selected decision.
Do not require the operator to type internal risk-class names. Reject missing,
unused, or mismatched acknowledgements. Inventory categories whose prerequisites
are not implemented remain unselectable. Do not reuse `--force`: the existing
[debug-only flag](../../cmd/teep/force_debug.go) bypasses enforced factors broadly
and cannot select failures or generate trusted cache state.

For automation, provide a reviewable proposal and explicit apply workflow:

```sh
teep cache --all-models --update-whitelist --proposal-out decisions.yaml
teep cache --update-whitelist --apply-proposal decisions.yaml
```

Proposal generation performs evidence collection and verification but does not write
operator decisions into the cache or change service policy. The proposal contains
exact candidate changes, stable identifiers, evidence references and content digests,
provider/target/tier scope, base-policy/build identities, prerequisites, remaining
failures, and request effects. Include explicit selection, explanation, and per-change
risk acknowledgement fields for review; leave selections and acknowledgements unset.
Retain or package the referenced original evidence so apply can validate it. A
proposal is untrusted input, not an authorization or a loadable cache file.

Apply is an explicit noninteractive operation on the reviewed selections. Use strict,
bounded parsing and the shared evaluator. Validate evidence integrity, scope, current
policy/build compatibility, supported decision kinds, and all admission prerequisites.
Fetch fresh evidence where required; if a subject or relevant failure differs, reject
that selection and require a new proposal. Never substitute newly observed values,
expand selection to additional failures, or accept a stale verification-result flag.
The reviewed proposal supplies the resolved targets, including those collected by
`--all-models`; apply does not rediscover or add models. Reject conflicting target flags.
Noninteractive invocation without proposal generation or explicit apply fails with
instructions for this workflow, rather than assuming consent. Proposal generation
may report unresolved failures; successful apply still requires effective-policy
success for each target receiving an addition or a successful evaluation. Selected
withdrawals may commit independently under the policy transaction rules.

Run ordinary checks first, then construct decisions only for selected eligible
failures. Rerun policy evaluation with the exact decisions without suppressing
unrelated failures. If any remaining enforced failure exists, write no decisions,
verified subjects, or endpoint authorization justified solely by that failed target;
shared decisions and withdrawals follow Section 5 transaction rules. Return nonzero with
the unresolved conditions. An operator can therefore TOFU-pin evidence rejected
by the base policy, but cannot export an apparently verified endpoint that still
fails the effective policy. Do not mutate service policy during collection.
Evaluate the selected decisions through the shared policy evaluator with explicit command-local inputs, then export
them. The command must not patch factor results after verification to simulate a
successful effective-policy evaluation.

Store the original failed-check evidence and decision in the artifact. Reuse may
perform the replacement value comparison without repeating the specifically waived
lookup; retain explicit diagnostics instead of fabricating `Pass` for the original
check. If a later live operation observes a different failure, value, signer, or
revocation, it needs its own decision. Service operation never auto-expands pins.

For each implemented decision kind, document whether reuse needs zero retrievals,
current status retrieval, or fresh endpoint evidence. Test this using request
counters as well as factor results. A build update rechecks decision compatibility
locally and reevaluates retained evidence where possible. It must identify any new
retrieval requirement rather than unconditionally refetching everything.

## 6. YAML structure and examples

Use a document of typed lists, with components and their verification details
nested under the software configuration they describe. No collection is keyed by
a repository nickname, generated ordinal, or user-selected record name. Repository
names, roles, platforms, and provider scopes are data fields. The schema supports
new components through additional records, without new field names.

| Collection | Contents and relationship |
| --- | --- |
| `evidence` | Original bytes, kind, and content digest. Shared by content digest; optional source metadata is diagnostic. `source_envelopes` records validated containment without granting trust; omit ineligible parent envelopes. |
| `software` | Provider-independent compose or release-set identities, with consumer-scoped evaluations, complete component membership, and verification results. Each component states its artifact identity and the checks performed. |
| `verification_material` | Typed collateral, certificates, issuer keys, and trust metadata, with lookup subjects, original inputs, verification dependencies, and admission eligibility. Shared independently of software and endpoint identities. |
| `policy_state` | Deployment-policy authority, monotonic revision, and removed-decision digests. Required with decisions or removal history; absence denotes empty revision-zero policy. |
| `operator_decisions` | Exact decision scope and subject, original failed-check evidence, explanation, and risk acknowledgements. These records never inherit authority from software results. |
| `endpoint_authorizations` | Optional complete runtime admissions with explicit endpoint identity, report/evidence, software dependency selectors, and durable-state requirements. Omit for ordinary portable files. |

`software` is the serialized form of the verified subjects described in Section 2.
Each software record has a provider-independent `subject` and an `evaluations` list.
An evaluation contains explicit consumer scope, verification context, and complete
component results. The same exact subject may have several consumer evaluations;
a different compose digest always needs a different subject record. Nested
`verification` records still belong logically to the evidence class: nesting is for
readability, not a new trust boundary. No independent success flag or second
software subject table is needed. Runtime adapters normalize records into the shared stores.
The parallel `verification_material` collection represents prerequisites that are not
software configurations; do not invent repositories or container roles for them.

Each evaluation's `verification.context` applies to its own checks and nested
component results: verifier build and effective policy are fixed for that evaluation.
The evaluation's `scope` supplies consumer provider, tier, and attestation evidence
format. The intrinsic `subject.encoding` identifies artifact representation
(`app_compose_json` or `canonical_release_set_v1`), independently of the delivery or
attestation format. Identical compose bytes can share a subject across `near`,
`dstack`, or `aci/1` evaluations without merging their trust rules.
Evaluation timestamps remain on individual results. A result from another scope
requires an explicit evaluation; there is no hidden file-wide policy default.
A component result's reusable identity includes artifact, scope, context, and required
checks, independently of the containing configuration or response envelope. An importer
may share verified subchecks only after proving the equivalence described in Section 2a-i.

Use `sha256:<hex>` content digests instead of local names for original evidence references. Resolve
each against exactly one byte sequence and validated evidence kind; duplicate or
conflicting evidence records in a file are errors. A merge may deduplicate identical
bytes. Optional `source_envelopes` lists content digests of retained original responses
containing the extracted bytes. It can name multiple delivery paths for identical
child evidence. Support the `attestation_envelope` kind and extraction relationships
explicitly; these links do not replace the quote's cryptographic subject bindings.
A digest is an integrity check, not a trust root. Do not use YAML anchors,
relative file paths, display names, or list positions as references.

Software dependencies use structured selectors containing consumer `scope`, intrinsic
`subject`, and effective `policy`; selectors resolve a subject plus its eligible
evaluation. For compose, the subject digest covers the exact original `app_compose`
bytes. For a release set, it covers canonical complete component identities, roles,
and required binding relationships, not one primary release. Exclude consumer policy,
provider, timestamps, and evaluation outcomes from this intrinsic identity. For
`canonical_release_set_v1`, hash the ASCII domain `teep:release-set:v1\n` followed by
RFC 8785 JSON containing the complete `components` list of `role`, `artifact`, and
`required_binding` values. Sort set members by their canonical bytes and reject
duplicate identities before hashing. Do not include the resulting digest itself.
For a typed material set, use the domain `teep:material-set:v1\n` and canonical
kind/applicability fields plus complete named input references, excluding verification
outcomes. Single-object evidence/subjects continue to hash their exact original bytes. Component order is not identity; distinct roles,
repositories, digests, and binding requirements are. Match selectors exactly and
reject missing or ambiguous results. Writers keep one active complete evaluation per
subject/consumer-scope/effective-policy/build key. Under the lock, identical evaluations
coalesce; a newly completed equivalent evaluation may replace the old one with its
actual timestamps and evidence dependencies, without changing subject identity.
Keep old raw evidence only while referenced or within storage limits. Conflicting
check outcomes for the same claimed input/context require shared reevaluation or
rejection, never newest-pass selection. Foreign-build evaluations may coexist but
cannot be used directly. Duplicate active keys in imported files fail validation;
routine refresh/merge must resolve them before export. Never select by list position.

Writers present policy state, software, verification material, and decisions first, optional endpoint records next, and
encoded evidence last. Use deterministic sorting for reviewable diffs, but do not
interpret list order as semantic.

Authors can read repository and digest next to the applicable result without
following arbitrary verification IDs. Repeating an unchanged component in two
configurations is permitted; original bytes remain deduplicated and the runtime
may share equivalent verification work. Human labels, if later introduced, must be
optional diagnostics and cannot participate in references or trust decisions.

These are illustrative fragments, not deployable cache files. Angle-bracket values
stand for real bytes, digests, keys, and complete check sets. Loaders reject literal
placeholders or incomplete required coverage. No fixed repository roster, timestamp,
component count, or provider-specific field name is prescribed by the examples.

### Material selection and dependency coverage

`verification_material` is a typed list, not a generic HTTP response cache. Each
record has `subject`, `inputs`, and `verification`. Its kind defines the required
subject fields, input roles, checks, and eligibility rules. A collateral set's digest
covers its canonical complete input membership with a versioned, kind-specific
encoding; a single-object subject uses the original content digest. FMSPC, CA,
product/HWID/TCB, issuer, or authority are applicability fields, not user-defined
record names. Recompute them from authenticated material. Fresh quote/token/TLS
inputs select eligible records through the shared production verifier. A matching
lookup key alone never authenticates a new report, key, or connection.

`inputs` names each original evidence object by digest, including certificate chains
returned in HTTP headers. Preserve each original header name and value and its
percent-encoded-PEM representation. A typed decoder reconstructs the expected
getter headers; do not replace them with a generic merged certificate blob.
The `intel_issuer_chain_header` evidence kind carries `header_name` and
`encoding: percent_encoded_pem`; its digest covers the original header-value bytes.
The material input role must agree with that header name, and validation checks the
actual certificate chain rather than trusting the name. No payload placeholder implicitly includes other evidence
objects. Required signed bundles retain their complete original representation;
additional roots, metadata, and chains use explicit references. Each typed adapter
must reject incomplete inputs. `verification.dependencies` uses `source: cache`
with an exact material subject, or `source: embedded` / `source: configured` with
the required build-owned or configuration-owned identity. A configured origin must
match current policy and be independently authenticated during acquisition; a YAML
URL cannot install a new trust root. Resolve cached dependencies under the caller's
current applicable material policy and build; do not inherit a software policy as
a collateral or issuer-key policy. Reject missing, ambiguous, cyclic, or substituted
dependencies. Reuse equivalent material across provider/tier consumers only after
checking these requirements. Embedded trust dependencies require no HTTP request.

The examples use one `verification` per material record under an explicit material
policy. Separate policy/build evaluations may use separate records for the same
subject; uniqueness and merge rules use subject/policy/build, not digest alone.
Original evidence remains deduplicated. This presentation does not require a second
runtime cache: each adapter prefills the existing verifier-owned dependency store.
Freshly fetched eligible material uses the same export path as prefetched material.

`eligibility` describes a typed verifier obligation, not an operator override or
cached verdict for a future report. Signed validity, versions, and revocation data
come from the original inputs. Recheck them at new admission with the real clock.
For authenticated-retrieval material such as JWKS and the CT log list, preserve the
original retrieval time through trusted export/import and apply the current issuer
or log-list refresh rules. Preserve NVIDIA's key-rotation refresh behavior. Import
must not extend the current time-based eligibility window or supply a timeless
key authorization. Incompatible new policy, expired metadata, or an unknown key
requires the existing retrieval or rejection path. These constraints do not add
expiry to an already published endpoint authorization.

CT material must prefill every relevant CT checker, including dependency-owned
clients; loading it into only the inference client cannot establish zero CT HTTP
requests. Continue live WebPKI, TLS identity, and SCT validation. Sigstore material
must cover the root transition chain from the current build's bootstrap root,
timestamp/snapshot/targets and any delegated metadata needed for the selected trust
target. The illustrated root chain has one member and no delegations; deployments
retain all required transitions and delegated metadata as additional typed inputs.
Run existing TUF signature, expiry, version, target-hash, and rollback checks locally;
a root update may require additional evidence. Neither TUF nor CT prefill may weaken
bootstrap authentication to avoid a request.

Local Sigstore metadata evaluation uses the pinned go-tuf `trustedmetadata` API:
initialize from the build's bootstrap root, apply each consecutive `UpdateRoot`,
then `UpdateTimestamp`, `UpdateSnapshot(..., false)`, and
`UpdateDelegatedTargets` for targets and each required delegation. Verify target
length/hashes before `root.NewTrustedRootFromJSON` and normal bundle verification.
Use the current time, never the capture time, in production. Compare incoming
versions with the trusted deployment's retained high-water state; a fresh verifier
instance alone does not provide historical rollback protection.

This evaluates a retained signed metadata snapshot within its validity bounds; it
does not establish that no newer root or timestamp exists. Do not synthesize a 404
for an absent next-root file. The regular updater probes for newer roots and the
Sigstore wrapper refreshes by default; `DisableLocalCache` does not disable retrieval.
Neither `ForceCache` nor `UnsafeLocalMode` is the cache adapter. On expired, missing,
incompatible, or revoked-by-current-policy material, run bounded authenticated live
refresh through an injected fetcher and retain the complete returned graph. Keep
that fetcher isolated from ambient `$HOME/.sigstore` state and from other deployments.
The bounded-snapshot update semantics and withdrawal responsibility are recorded in
[the planning issues](supply_chain_caching_issues.md#sigstore-trust-metadata-reuse).

Examples 6a, 6b, 6c, 6e, and 6h are portable admission-prefill examples for their stated
hardware and evidence. Their numeric budgets are in Section 4d. Populate real bytes
and complete required checks before turning them into fixtures. The Intel examples
assume processor-CA collateral; actual quote-derived CA and platform scope control
selection. NearCloud has two TCB-information objects and shares eligible QE/CRL
objects; NearDirect can reuse the backend set. No saved attestation response, NRAS
JWT, or Proof of Cloud response answers a new challenge. Current embedded Rekor verification keys and NVIDIA device-identity roots add no
retrievals; record the build-owned dependencies rather than fabricating downloaded
trust objects. No independently portable
NVIDIA RIM verifier is assumed: the NRAS submission remains live.

### 6a. Near cloud: separate model and gateway configurations

This example shows four model components and seven gateway components. Membership
comes from each actual compose. Shared OpenTelemetry provenance has one evidence
record and distinct model/gateway policy evaluations. Current NEAR `NoDSSE` entries
show `dsse_signature: not_required`, not a signature success. The model and gateway
compose bindings themselves must be established by fresh endpoint admission or an
eligible restored authorization; portable software checks alone do not authenticate
a new endpoint.

```yaml
schema_version: 1
software:
- subject:
    kind: compose
    digest: sha256:<model compose>
    encoding: app_compose_json
  evaluations:
  - scope:
      provider: nearcloud
      tier: model
      evidence_format: near
    verification:
      context:
        verifier_build: sha256:<teep build>
        policy: sha256:<nearcloud model effective policy>
      evidence:
      - sha256:<model compose>
      checks:
        required_membership: pass
        component_policy_coverage: pass
      exemptions: []
      evaluated_at: '2026-09-13T00:00:00Z'
    components:
    - role: container_image
      artifact:
        repository: nearaidev/compose-manager
        digest: sha256:<nearaidev/compose-manager image>
      verification:
        evidence:
        - sha256:<nearaidev/compose-manager provenance>
        provenance: fulcio_signed
        checks:
          repository_policy: pass
          transparency: pass
          fulcio_identity: pass
          source_repository: pass
          dsse_signature: not_required
        exemptions: []
        evaluated_at: '2026-09-13T00:00:00Z'
        dependencies:
        - source: embedded
          kind: rekor_log_key
          identity: sha256:<Rekor verification key in this build>
    - role: container_image
      artifact:
        repository: nearaidev/compose-manager-launcher
        digest: sha256:<nearaidev/compose-manager-launcher image>
      verification:
        evidence:
        - sha256:<nearaidev/compose-manager-launcher provenance>
        provenance: fulcio_signed
        checks:
          repository_policy: pass
          transparency: pass
          fulcio_identity: pass
          source_repository: pass
          dsse_signature: not_required
        exemptions: []
        evaluated_at: '2026-09-13T00:00:00Z'
        dependencies:
        - source: embedded
          kind: rekor_log_key
          identity: sha256:<Rekor verification key in this build>
    - role: container_image
      artifact:
        repository: certbot/dns-cloudflare
        digest: sha256:<certbot/dns-cloudflare image>
      verification:
        evidence: []
        provenance: compose_binding_only
        checks:
          repository_policy: pass
          image_signature: not_required
          transparency: not_required
        exemptions: []
        evaluated_at: '2026-09-13T00:00:00Z'
    - role: container_image
      artifact:
        repository: otel/opentelemetry-collector-contrib
        digest: sha256:<otel/opentelemetry-collector-contrib image>
      verification:
        evidence:
        - sha256:<otel/opentelemetry-collector-contrib provenance>
        provenance: sigstore_present
        checks:
          repository_policy: pass
          transparency: pass
          signer_fingerprint: pass
        exemptions: []
        evaluated_at: '2026-09-13T00:00:00Z'
        dependencies:
        - source: embedded
          kind: rekor_log_key
          identity: sha256:<Rekor verification key in this build>
- subject:
    kind: compose
    digest: sha256:<gateway compose>
    encoding: app_compose_json
  evaluations:
  - scope:
      provider: nearcloud
      tier: gateway
      evidence_format: dstack
    verification:
      context:
        verifier_build: sha256:<teep build>
        policy: sha256:<nearcloud gateway effective policy>
      evidence:
      - sha256:<gateway compose>
      checks:
        required_membership: pass
        component_policy_coverage: pass
      exemptions: []
      evaluated_at: '2026-09-13T00:00:00Z'
    components:
    - role: container_image
      artifact:
        repository: nearaidev/cloud-api
        digest: sha256:<nearaidev/cloud-api image>
      verification:
        evidence:
        - sha256:<nearaidev/cloud-api provenance>
        provenance: fulcio_signed
        checks:
          repository_policy: pass
          transparency: pass
          fulcio_identity: pass
          source_repository: pass
          dsse_signature: not_required
        exemptions: []
        evaluated_at: '2026-09-13T00:00:00Z'
        dependencies:
        - source: embedded
          kind: rekor_log_key
          identity: sha256:<Rekor verification key in this build>
    - role: container_image
      artifact:
        repository: nearaidev/cvm-ingress
        digest: sha256:<nearaidev/cvm-ingress image>
      verification:
        evidence:
        - sha256:<nearaidev/cvm-ingress provenance>
        provenance: fulcio_signed
        checks:
          repository_policy: pass
          transparency: pass
          fulcio_identity: pass
          source_repository: pass
          dsse_signature: not_required
        exemptions: []
        evaluated_at: '2026-09-13T00:00:00Z'
        dependencies:
        - source: embedded
          kind: rekor_log_key
          identity: sha256:<Rekor verification key in this build>
    - role: container_image
      artifact:
        repository: nearaidev/dstack-vpc
        digest: sha256:<nearaidev/dstack-vpc image>
      verification:
        evidence:
        - sha256:<nearaidev/dstack-vpc provenance>
        provenance: fulcio_signed
        checks:
          repository_policy: pass
          transparency: pass
          fulcio_identity: pass
          source_repository: pass
          dsse_signature: not_required
        exemptions: []
        evaluated_at: '2026-09-13T00:00:00Z'
        dependencies:
        - source: embedded
          kind: rekor_log_key
          identity: sha256:<Rekor verification key in this build>
    - role: container_image
      artifact:
        repository: nearaidev/dstack-vpc-client
        digest: sha256:<nearaidev/dstack-vpc-client image>
      verification:
        evidence:
        - sha256:<nearaidev/dstack-vpc-client provenance>
        provenance: fulcio_signed
        checks:
          repository_policy: pass
          transparency: pass
          fulcio_identity: pass
          source_repository: pass
          dsse_signature: not_required
        exemptions: []
        evaluated_at: '2026-09-13T00:00:00Z'
        dependencies:
        - source: embedded
          kind: rekor_log_key
          identity: sha256:<Rekor verification key in this build>
    - role: container_image
      artifact:
        repository: datadog/agent
        digest: sha256:<datadog/agent image>
      verification:
        evidence:
        - sha256:<datadog/agent provenance>
        provenance: sigstore_present
        checks:
          repository_policy: pass
          transparency: pass
          signer_fingerprint: pass
        exemptions: []
        evaluated_at: '2026-09-13T00:00:00Z'
        dependencies:
        - source: embedded
          kind: rekor_log_key
          identity: sha256:<Rekor verification key in this build>
    - role: container_image
      artifact:
        repository: alpine
        digest: sha256:<alpine image>
      verification:
        evidence:
        - sha256:<alpine provenance>
        provenance: fulcio_signed
        checks:
          repository_policy: pass
          transparency: pass
          fulcio_identity: pass
          source_repository: pass
          dsse_signature: not_required
        exemptions: []
        evaluated_at: '2026-09-13T00:00:00Z'
        dependencies:
        - source: embedded
          kind: rekor_log_key
          identity: sha256:<Rekor verification key in this build>
    - role: container_image
      artifact:
        repository: otel/opentelemetry-collector-contrib
        digest: sha256:<otel/opentelemetry-collector-contrib image>
      verification:
        evidence:
        - sha256:<otel/opentelemetry-collector-contrib provenance>
        provenance: sigstore_present
        checks:
          repository_policy: pass
          transparency: pass
          signer_fingerprint: pass
        exemptions: []
        evaluated_at: '2026-09-13T00:00:00Z'
        dependencies:
        - source: embedded
          kind: rekor_log_key
          identity: sha256:<Rekor verification key in this build>
verification_material:
- subject:
    kind: intel_tdx_collateral
    digest: sha256:<model canonical collateral set>
    fmspc: "<model FMSPC>"
    pck_ca: processor
    api_version: 4
  inputs:
    tcb_info: sha256:<model TCB information>
    qe_identity: sha256:<Intel QE identity>
    pck_crl: sha256:<Intel PCK CRL>
    root_ca_crl: sha256:<Intel root CA CRL>
    issuer_chains:
    - sha256:<model TCB issuer chain>
    - sha256:<QE identity issuer chain>
    - sha256:<PCK CRL issuer chain>
  verification:
    context:
      verifier_build: sha256:<teep build>
      policy: sha256:<intel_tdx_collateral applicable material policy>
    dependencies:
    - source: embedded
      kind: trust_anchor
      identity: sha256:<Intel root in this build>
    checks:
      signature_chains: pass
      collateral_scope: pass
    eligibility:
      basis: signed_evidence
      recheck:
      - validity
      - revocation
      - fresh_quote_platform_and_tcb
    evaluated_at: '2026-09-13T00:00:00Z'
- subject:
    kind: intel_tdx_collateral
    digest: sha256:<gateway canonical collateral set>
    fmspc: "<gateway FMSPC>"
    pck_ca: processor
    api_version: 4
  inputs:
    tcb_info: sha256:<gateway TCB information>
    qe_identity: sha256:<Intel QE identity>
    pck_crl: sha256:<Intel PCK CRL>
    root_ca_crl: sha256:<Intel root CA CRL>
    issuer_chains:
    - sha256:<gateway TCB issuer chain>
    - sha256:<QE identity issuer chain>
    - sha256:<PCK CRL issuer chain>
  verification:
    context:
      verifier_build: sha256:<teep build>
      policy: sha256:<intel_tdx_collateral applicable material policy>
    dependencies:
    - source: embedded
      kind: trust_anchor
      identity: sha256:<Intel root in this build>
    checks:
      signature_chains: pass
      collateral_scope: pass
    eligibility:
      basis: signed_evidence
      recheck:
      - validity
      - revocation
      - fresh_quote_platform_and_tcb
    evaluated_at: '2026-09-13T00:00:00Z'
- subject:
    kind: nvidia_jwks
    digest: sha256:<NVIDIA JWKS>
    authority: https://nras.attestation.nvidia.com/.well-known/jwks.json
  inputs:
    jwks: sha256:<NVIDIA JWKS>
  verification:
    context:
      verifier_build: sha256:<teep build>
      policy: sha256:<nvidia_jwks applicable material policy>
    dependencies:
    - source: configured
      kind: authenticated_origin
      identity: https://nras.attestation.nvidia.com/.well-known/jwks.json
    checks:
      origin_authentication: pass
      keyset_structure: pass
    eligibility:
      basis: authenticated_retrieval
      retrieved_at: '2026-09-13T00:00:00Z'
      recheck:
      - issuer_key_refresh_policy
      - new_token_signature_and_claims
    evaluated_at: '2026-09-13T00:00:00Z'
- subject:
    kind: ct_log_list
    digest: sha256:<CT log list>
    authority: chrome_ct_log_list
  inputs:
    log_list: sha256:<CT log list>
  verification:
    context:
      verifier_build: sha256:<teep build>
      policy: sha256:<ct_log_list applicable material policy>
    dependencies:
    - source: configured
      kind: authenticated_origin
      identity: "<configured CT log-list origin>"
    checks:
      origin_authentication: pass
      log_list_structure: pass
    eligibility:
      basis: authenticated_retrieval
      retrieved_at: '2026-09-13T00:00:00Z'
      recheck:
      - log_list_refresh_policy
      - live_peer_certificate_and_scts
    evaluated_at: '2026-09-13T00:00:00Z'
operator_decisions: []
evidence:
- digest: sha256:<model compose>
  kind: compose
  payload_base64: "<complete original bytes of this evidence object>"
  source_envelopes:
  - sha256:<nearcloud original response envelope>
- digest: sha256:<nearaidev/compose-manager provenance>
  kind: rekor_provenance
  payload_base64: "<complete original bytes of this evidence object>"
- digest: sha256:<nearaidev/compose-manager-launcher provenance>
  kind: rekor_provenance
  payload_base64: "<complete original bytes of this evidence object>"
- digest: sha256:<otel/opentelemetry-collector-contrib provenance>
  kind: rekor_provenance
  payload_base64: "<complete original bytes of this evidence object>"
- digest: sha256:<gateway compose>
  kind: compose
  payload_base64: "<complete original bytes of this evidence object>"
  source_envelopes:
  - sha256:<nearcloud original response envelope>
- digest: sha256:<nearaidev/cloud-api provenance>
  kind: rekor_provenance
  payload_base64: "<complete original bytes of this evidence object>"
- digest: sha256:<nearaidev/cvm-ingress provenance>
  kind: rekor_provenance
  payload_base64: "<complete original bytes of this evidence object>"
- digest: sha256:<nearaidev/dstack-vpc provenance>
  kind: rekor_provenance
  payload_base64: "<complete original bytes of this evidence object>"
- digest: sha256:<nearaidev/dstack-vpc-client provenance>
  kind: rekor_provenance
  payload_base64: "<complete original bytes of this evidence object>"
- digest: sha256:<datadog/agent provenance>
  kind: rekor_provenance
  payload_base64: "<complete original bytes of this evidence object>"
- digest: sha256:<alpine provenance>
  kind: rekor_provenance
  payload_base64: "<complete original bytes of this evidence object>"
- digest: sha256:<nearcloud original response envelope>
  kind: attestation_envelope
  payload_base64: "<complete original response; includes nonce context and all delivered subjects>"
- digest: sha256:<model TCB information>
  kind: intel_tcb_info
  payload_base64: "<original signed TCB information JSON>"
- digest: sha256:<Intel QE identity>
  kind: intel_qe_identity
  payload_base64: "<original signed QE identity JSON>"
- digest: sha256:<Intel PCK CRL>
  kind: x509_crl
  payload_base64: "<original DER CRL for the applicable PCK CA>"
- digest: sha256:<Intel root CA CRL>
  kind: x509_crl
  payload_base64: "<original DER CRL>"
- digest: sha256:<gateway TCB information>
  kind: intel_tcb_info
  payload_base64: "<original signed TCB information JSON>"
- digest: sha256:<NVIDIA JWKS>
  kind: jwk_set
  payload_base64: "<original NVIDIA JWKS JSON>"
- digest: sha256:<CT log list>
  kind: ct_log_list
  payload_base64: "<original authenticated CT log-list JSON>"
- digest: sha256:<model TCB issuer chain>
  kind: intel_issuer_chain_header
  payload_base64: "<original header value bytes, without decoding or normalization>"
  header_name: Tcb-Info-Issuer-Chain
  encoding: percent_encoded_pem
- digest: sha256:<QE identity issuer chain>
  kind: intel_issuer_chain_header
  payload_base64: "<original header value bytes, without decoding or normalization>"
  header_name: Sgx-Enclave-Identity-Issuer-Chain
  encoding: percent_encoded_pem
- digest: sha256:<PCK CRL issuer chain>
  kind: intel_issuer_chain_header
  payload_base64: "<original header value bytes, without decoding or normalization>"
  header_name: Sgx-Pck-Crl-Issuer-Chain
  encoding: percent_encoded_pem
- digest: sha256:<gateway TCB issuer chain>
  kind: intel_issuer_chain_header
  payload_base64: "<original header value bytes, without decoding or normalization>"
  header_name: Tcb-Info-Issuer-Chain
  encoding: percent_encoded_pem
```

### 6b. Near direct: the same structure under its own policy

NearDirect uses the same model evidence representation and four-component shape
without a gateway configuration. The same original provenance bytes can be reused,
but its evaluation names NearDirect's effective policy. The example deliberately
uses a different complete compose digest: equal component images do not imply equal
attested compose bytes. If complete compose bytes are identical, store one subject
with separate NearCloud and NearDirect evaluations instead of duplicating the subject. Repetition here makes the example independently
readable; it does not require repeated downloads. Route selection and TLS/E2EE
identity remain endpoint admission facts under the [NEAR route contract](../providers/near/near_attestation.md#neardirect-backend-selection).

```yaml
schema_version: 1
software:
- subject:
    kind: compose
    digest: sha256:<direct model compose>
    encoding: app_compose_json
  evaluations:
  - scope:
      provider: neardirect
      tier: model
      evidence_format: near
    verification:
      context:
        verifier_build: sha256:<teep build>
        policy: sha256:<neardirect model effective policy>
      evidence:
      - sha256:<direct model compose>
      checks:
        required_membership: pass
        component_policy_coverage: pass
      exemptions: []
      evaluated_at: '2026-09-13T00:00:00Z'
    components:
    - role: container_image
      artifact:
        repository: nearaidev/compose-manager
        digest: sha256:<nearaidev/compose-manager image>
      verification:
        evidence:
        - sha256:<nearaidev/compose-manager provenance>
        provenance: fulcio_signed
        checks:
          repository_policy: pass
          transparency: pass
          fulcio_identity: pass
          source_repository: pass
          dsse_signature: not_required
        exemptions: []
        evaluated_at: '2026-09-13T00:00:00Z'
        dependencies:
        - source: embedded
          kind: rekor_log_key
          identity: sha256:<Rekor verification key in this build>
    - role: container_image
      artifact:
        repository: nearaidev/compose-manager-launcher
        digest: sha256:<nearaidev/compose-manager-launcher image>
      verification:
        evidence:
        - sha256:<nearaidev/compose-manager-launcher provenance>
        provenance: fulcio_signed
        checks:
          repository_policy: pass
          transparency: pass
          fulcio_identity: pass
          source_repository: pass
          dsse_signature: not_required
        exemptions: []
        evaluated_at: '2026-09-13T00:00:00Z'
        dependencies:
        - source: embedded
          kind: rekor_log_key
          identity: sha256:<Rekor verification key in this build>
    - role: container_image
      artifact:
        repository: certbot/dns-cloudflare
        digest: sha256:<certbot/dns-cloudflare image>
      verification:
        evidence: []
        provenance: compose_binding_only
        checks:
          repository_policy: pass
          image_signature: not_required
          transparency: not_required
        exemptions: []
        evaluated_at: '2026-09-13T00:00:00Z'
    - role: container_image
      artifact:
        repository: otel/opentelemetry-collector-contrib
        digest: sha256:<otel/opentelemetry-collector-contrib image>
      verification:
        evidence:
        - sha256:<otel/opentelemetry-collector-contrib provenance>
        provenance: sigstore_present
        checks:
          repository_policy: pass
          transparency: pass
          signer_fingerprint: pass
        exemptions: []
        evaluated_at: '2026-09-13T00:00:00Z'
        dependencies:
        - source: embedded
          kind: rekor_log_key
          identity: sha256:<Rekor verification key in this build>
verification_material:
- subject:
    kind: intel_tdx_collateral
    digest: sha256:<model canonical collateral set>
    fmspc: "<model FMSPC>"
    pck_ca: processor
    api_version: 4
  inputs:
    tcb_info: sha256:<model TCB information>
    qe_identity: sha256:<Intel QE identity>
    pck_crl: sha256:<Intel PCK CRL>
    root_ca_crl: sha256:<Intel root CA CRL>
    issuer_chains:
    - sha256:<model TCB issuer chain>
    - sha256:<QE identity issuer chain>
    - sha256:<PCK CRL issuer chain>
  verification:
    context:
      verifier_build: sha256:<teep build>
      policy: sha256:<intel_tdx_collateral applicable material policy>
    dependencies:
    - source: embedded
      kind: trust_anchor
      identity: sha256:<Intel root in this build>
    checks:
      signature_chains: pass
      collateral_scope: pass
    eligibility:
      basis: signed_evidence
      recheck:
      - validity
      - revocation
      - fresh_quote_platform_and_tcb
    evaluated_at: '2026-09-13T00:00:00Z'
- subject:
    kind: nvidia_jwks
    digest: sha256:<NVIDIA JWKS>
    authority: https://nras.attestation.nvidia.com/.well-known/jwks.json
  inputs:
    jwks: sha256:<NVIDIA JWKS>
  verification:
    context:
      verifier_build: sha256:<teep build>
      policy: sha256:<nvidia_jwks applicable material policy>
    dependencies:
    - source: configured
      kind: authenticated_origin
      identity: https://nras.attestation.nvidia.com/.well-known/jwks.json
    checks:
      origin_authentication: pass
      keyset_structure: pass
    eligibility:
      basis: authenticated_retrieval
      retrieved_at: '2026-09-13T00:00:00Z'
      recheck:
      - issuer_key_refresh_policy
      - new_token_signature_and_claims
    evaluated_at: '2026-09-13T00:00:00Z'
- subject:
    kind: ct_log_list
    digest: sha256:<CT log list>
    authority: chrome_ct_log_list
  inputs:
    log_list: sha256:<CT log list>
  verification:
    context:
      verifier_build: sha256:<teep build>
      policy: sha256:<ct_log_list applicable material policy>
    dependencies:
    - source: configured
      kind: authenticated_origin
      identity: "<configured CT log-list origin>"
    checks:
      origin_authentication: pass
      log_list_structure: pass
    eligibility:
      basis: authenticated_retrieval
      retrieved_at: '2026-09-13T00:00:00Z'
      recheck:
      - log_list_refresh_policy
      - live_peer_certificate_and_scts
    evaluated_at: '2026-09-13T00:00:00Z'
operator_decisions: []
evidence:
- digest: sha256:<direct model compose>
  kind: compose
  payload_base64: "<complete original bytes of this evidence object>"
  source_envelopes:
  - sha256:<neardirect original response envelope>
- digest: sha256:<nearaidev/compose-manager provenance>
  kind: rekor_provenance
  payload_base64: "<complete original bytes of this evidence object>"
- digest: sha256:<nearaidev/compose-manager-launcher provenance>
  kind: rekor_provenance
  payload_base64: "<complete original bytes of this evidence object>"
- digest: sha256:<otel/opentelemetry-collector-contrib provenance>
  kind: rekor_provenance
  payload_base64: "<complete original bytes of this evidence object>"
- digest: sha256:<neardirect original response envelope>
  kind: attestation_envelope
  payload_base64: "<complete original response; includes nonce context and all delivered subjects>"
- digest: sha256:<model TCB information>
  kind: intel_tcb_info
  payload_base64: "<original signed TCB information JSON>"
- digest: sha256:<Intel QE identity>
  kind: intel_qe_identity
  payload_base64: "<original signed QE identity JSON>"
- digest: sha256:<Intel PCK CRL>
  kind: x509_crl
  payload_base64: "<original DER CRL for the applicable PCK CA>"
- digest: sha256:<Intel root CA CRL>
  kind: x509_crl
  payload_base64: "<original DER CRL>"
- digest: sha256:<NVIDIA JWKS>
  kind: jwk_set
  payload_base64: "<original NVIDIA JWKS JSON>"
- digest: sha256:<CT log list>
  kind: ct_log_list
  payload_base64: "<original authenticated CT log-list JSON>"
- digest: sha256:<model TCB issuer chain>
  kind: intel_issuer_chain_header
  payload_base64: "<original header value bytes, without decoding or normalization>"
  header_name: Tcb-Info-Issuer-Chain
  encoding: percent_encoded_pem
- digest: sha256:<QE identity issuer chain>
  kind: intel_issuer_chain_header
  payload_base64: "<original header value bytes, without decoding or normalization>"
  header_name: Sgx-Enclave-Identity-Issuer-Chain
  encoding: percent_encoded_pem
- digest: sha256:<PCK CRL issuer chain>
  kind: intel_issuer_chain_header
  payload_base64: "<original header value bytes, without decoding or normalization>"
  header_name: Sgx-Pck-Crl-Issuer-Chain
  encoding: percent_encoded_pem
```

### 6c. Tinfoil: code and platform references in a release set

This source-derived direct TDX example has a code release and a hardware-reference
release. Their roles differ, but their record structure is shared. The release-set
check validates component coverage; fresh admission must also compare the signed
code and hardware measurements with the actual CPU evidence. A valid first component
cannot conceal failure of the other. Direct live validation remains blocked by the
upstream issue in Section 1.

```yaml
schema_version: 1
software:
- subject:
    kind: release_set
    digest: sha256:<canonical complete TDX component set>
    encoding: canonical_release_set_v1
  evaluations:
  - scope:
      provider: tinfoil_v3_direct
      tier: model
      evidence_format: tinfoil_v3
    verification:
      context:
        verifier_build: sha256:<teep build>
        policy: sha256:<Tinfoil direct TDX effective policy>
      checks:
        required_membership: pass
        component_policy_coverage: pass
      exemptions: []
      evaluated_at: '2026-09-13T00:00:00Z'
    components:
    - role: code_release
      artifact:
        repository: tinfoilsh/confidential-example-model
        digest: sha256:<tinfoilsh/confidential-example-model release subject>
      required_binding: tdx_code_measurements
      verification:
        dependencies:
        - source: cache
          subject:
            kind: sigstore_trust_material
            digest: sha256:<canonical Sigstore trust material set>
            authority: sigstore_public_good
        evidence:
        - sha256:<tinfoilsh/confidential-example-model signed release>
        checks:
          release_signature: pass
          signer_identity: pass
          transparency: pass
        exemptions: []
        evaluated_at: '2026-09-13T00:00:00Z'
    - role: platform_reference
      artifact:
        repository: tinfoilsh/hardware-measurements
        digest: sha256:<tinfoilsh/hardware-measurements release subject>
      required_binding: tdx_hardware_measurements
      verification:
        dependencies:
        - source: cache
          subject:
            kind: sigstore_trust_material
            digest: sha256:<canonical Sigstore trust material set>
            authority: sigstore_public_good
        evidence:
        - sha256:<tinfoilsh/hardware-measurements signed release>
        checks:
          release_signature: pass
          signer_identity: pass
          transparency: pass
        exemptions: []
        evaluated_at: '2026-09-13T00:00:00Z'
verification_material:
- subject:
    kind: intel_tdx_collateral
    digest: sha256:<model canonical collateral set>
    fmspc: "<model FMSPC>"
    pck_ca: processor
    api_version: 4
  inputs:
    tcb_info: sha256:<model TCB information>
    qe_identity: sha256:<Intel QE identity>
    pck_crl: sha256:<Intel PCK CRL>
    root_ca_crl: sha256:<Intel root CA CRL>
    issuer_chains:
    - sha256:<model TCB issuer chain>
    - sha256:<QE identity issuer chain>
    - sha256:<PCK CRL issuer chain>
  verification:
    context:
      verifier_build: sha256:<teep build>
      policy: sha256:<intel_tdx_collateral applicable material policy>
    dependencies:
    - source: embedded
      kind: trust_anchor
      identity: sha256:<Intel root in this build>
    checks:
      signature_chains: pass
      collateral_scope: pass
    eligibility:
      basis: signed_evidence
      recheck:
      - validity
      - revocation
      - fresh_quote_platform_and_tcb
    evaluated_at: '2026-09-13T00:00:00Z'
- subject:
    kind: nvidia_jwks
    digest: sha256:<NVIDIA JWKS>
    authority: https://nras.attestation.nvidia.com/.well-known/jwks.json
  inputs:
    jwks: sha256:<NVIDIA JWKS>
  verification:
    context:
      verifier_build: sha256:<teep build>
      policy: sha256:<nvidia_jwks applicable material policy>
    dependencies:
    - source: configured
      kind: authenticated_origin
      identity: https://nras.attestation.nvidia.com/.well-known/jwks.json
    checks:
      origin_authentication: pass
      keyset_structure: pass
    eligibility:
      basis: authenticated_retrieval
      retrieved_at: '2026-09-13T00:00:00Z'
      recheck:
      - issuer_key_refresh_policy
      - new_token_signature_and_claims
    evaluated_at: '2026-09-13T00:00:00Z'
- subject:
    kind: sigstore_trust_material
    digest: sha256:<canonical Sigstore trust material set>
    authority: sigstore_public_good
  inputs:
    timestamp: sha256:<Sigstore timestamp>
    snapshot: sha256:<Sigstore snapshot>
    targets: sha256:<Sigstore targets>
    trusted_root: sha256:<Sigstore trusted_root>
    root_chain:
    - sha256:<Sigstore root>
  verification:
    context:
      verifier_build: sha256:<teep build>
      policy: sha256:<sigstore_trust_material applicable material policy>
    dependencies:
    - source: embedded
      kind: tuf_bootstrap_root
      identity: sha256:<Sigstore bootstrap root in this build>
    checks:
      tuf_signatures: pass
      versions_and_target_hashes: pass
    eligibility:
      basis: signed_evidence
      recheck:
      - tuf_expiry
      - rollback_and_root_rotation
      - signer_and_log_policy
    evaluated_at: '2026-09-13T00:00:00Z'
- subject:
    kind: ct_log_list
    digest: sha256:<CT log list>
    authority: chrome_ct_log_list
  inputs:
    log_list: sha256:<CT log list>
  verification:
    context:
      verifier_build: sha256:<teep build>
      policy: sha256:<ct_log_list applicable material policy>
    dependencies:
    - source: configured
      kind: authenticated_origin
      identity: "<configured CT log-list origin>"
    checks:
      origin_authentication: pass
      log_list_structure: pass
    eligibility:
      basis: authenticated_retrieval
      retrieved_at: '2026-09-13T00:00:00Z'
      recheck:
      - log_list_refresh_policy
      - live_peer_certificate_and_scts
    evaluated_at: '2026-09-13T00:00:00Z'
operator_decisions: []
evidence:
- digest: sha256:<tinfoilsh/confidential-example-model signed release>
  kind: sigstore_bundle
  payload_base64: "<complete original bytes of this evidence object>"
- digest: sha256:<tinfoilsh/hardware-measurements signed release>
  kind: sigstore_bundle
  payload_base64: "<complete original bytes of this evidence object>"
- digest: sha256:<model TCB information>
  kind: intel_tcb_info
  payload_base64: "<original signed TCB information JSON>"
- digest: sha256:<Intel QE identity>
  kind: intel_qe_identity
  payload_base64: "<original signed QE identity JSON>"
- digest: sha256:<Intel PCK CRL>
  kind: x509_crl
  payload_base64: "<original DER CRL for the applicable PCK CA>"
- digest: sha256:<Intel root CA CRL>
  kind: x509_crl
  payload_base64: "<original DER CRL>"
- digest: sha256:<NVIDIA JWKS>
  kind: jwk_set
  payload_base64: "<original NVIDIA JWKS JSON>"
- digest: sha256:<Sigstore root>
  kind: tuf_root
  payload_base64: "<original signed TUF root metadata>"
- digest: sha256:<Sigstore timestamp>
  kind: tuf_timestamp
  payload_base64: "<original signed TUF timestamp metadata>"
- digest: sha256:<Sigstore snapshot>
  kind: tuf_snapshot
  payload_base64: "<original signed TUF snapshot metadata>"
- digest: sha256:<Sigstore targets>
  kind: tuf_targets
  payload_base64: "<original signed TUF targets metadata>"
- digest: sha256:<Sigstore trusted_root>
  kind: sigstore_trusted_root
  payload_base64: "<original trusted-root target bytes>"
- digest: sha256:<CT log list>
  kind: ct_log_list
  payload_base64: "<original authenticated CT log-list JSON>"
- digest: sha256:<model TCB issuer chain>
  kind: intel_issuer_chain_header
  payload_base64: "<original header value bytes, without decoding or normalization>"
  header_name: Tcb-Info-Issuer-Chain
  encoding: percent_encoded_pem
- digest: sha256:<QE identity issuer chain>
  kind: intel_issuer_chain_header
  payload_base64: "<original header value bytes, without decoding or normalization>"
  header_name: Sgx-Enclave-Identity-Issuer-Chain
  encoding: percent_encoded_pem
- digest: sha256:<PCK CRL issuer chain>
  kind: intel_issuer_chain_header
  payload_base64: "<original header value bytes, without decoding or normalization>"
  header_name: Sgx-Pck-Crl-Issuer-Chain
  encoding: percent_encoded_pem
```

Cloud uses the same structure with `provider: tinfoil_v3_cloud`, `tier: gateway`,
and the router release repository. The current SEV code-verification path does not
fetch the TDX hardware registry; membership follows actual supported verification,
not a rule that cloud always has one component and direct always has two. Cloud
verifies the router, not backend model images. Direct resolves the model authority
and primary repository together and binds its release measurements to that model
CVM. A release match does not enumerate every internal package or container.

Tinfoil V3 may supply `tinfoilsh/platform-endorsements` and freshness collateral,
but the current parser checks collateral envelopes without interpreting their
contents. Retaining supplied bytes does not justify a software result. Independently
verify their signatures, subject identity, and measurement relationships through a
supported shared verifier before adding them as verified components. The existing
hardware-registry result is not interchangeable with a platform-endorsement entry.
No latest-release freshness requirement is introduced by this representation.

### 6d. Operator measurement decision

Decisions are readable list entries with exact subjects and evidence, not named
stanzas. Their observation verification has its own explicit context; it cannot
inherit a context from an unrelated software configuration. The match selects MRTD and MRSEAM only. The original observation evidence retains
the remaining measurements; RTMR values remain governed by their existing policy
and do not become additional match constraints merely because they were observed.

```yaml
schema_version: 1
policy_state:
  authority: "<deployment policy authority>"
  revision: 1
  removed_decisions: []
software: []
operator_decisions:
- scope:
    provider: neardirect
    evidence_format: near
    tier: model
  kind: measurement
  subject:
    platform: intel_tdx
    measurements:
      mrseam: "<observed MRSEAM>"
      mrtd: "<observed MRTD>"
  replaces_failure: measurement_not_listed
  action: pin_observed_value
  observation:
    context:
      verifier_build: sha256:<teep build>
      policy: sha256:<base measurement effective policy>
    evidence:
    - sha256:<fresh decision quote>
    checks:
      quote_signature: pass
      nonce_binding: pass
      reportdata_binding: pass
      measurement_policy: fail
    evaluated_at: '2026-09-13T00:00:00Z'
  reason: Operator accepts this observed measurement configuration.
  decided_at: '2026-09-13T00:01:00Z'
  risk_acknowledgements: []
evidence:
- digest: sha256:<fresh decision quote>
  kind: endpoint_attestation
  payload_base64: "<complete original bytes and verification dependencies>"
```

This record does not waive signatures, nonce binding, or unrelated factors. Before
export, the complete target must pass effective policy with the selected decisions.
Reference a decision, where needed, by the content digest of its canonical full
record; explanations, scope, acknowledgements, and evidence cannot be substituted.

### 6e. Venice: gateway compose without model software claims

After its required migration, Venice uses the same compose structure for ACI/1
gateway software. These four policy components have compose-only provenance; no
image-signature success is asserted. Fresh admission also needs its gateway quote,
event log, and independently checked key custody. Those are admission evidence,
not extra component repositories. Dstack model responses use model-tier records
under their own format policy; one format cannot satisfy the other.

```yaml
schema_version: 1
software:
- subject:
    kind: compose
    digest: sha256:<gateway compose>
    encoding: app_compose_json
  evaluations:
  - scope:
      provider: venice
      tier: gateway
      evidence_format: aci/1
    verification:
      context:
        verifier_build: sha256:<teep build>
        policy: sha256:<venice gateway effective policy>
      evidence:
      - sha256:<gateway compose>
      checks:
        required_membership: pass
        component_policy_coverage: pass
      exemptions: []
      evaluated_at: '2026-09-13T00:00:00Z'
    components:
    - role: container_image
      artifact:
        repository: ghcr.io/redpill-ai/private-ai-launcher
        digest: sha256:<ghcr.io/redpill-ai/private-ai-launcher image>
      verification:
        evidence: []
        provenance: compose_binding_only
        checks:
          repository_policy: pass
          image_signature: not_required
          transparency: not_required
        exemptions: []
        evaluated_at: '2026-09-13T00:00:00Z'
    - role: container_image
      artifact:
        repository: dstacktee/dstack-ingress
        digest: sha256:<dstacktee/dstack-ingress image>
      verification:
        evidence: []
        provenance: compose_binding_only
        checks:
          repository_policy: pass
          image_signature: not_required
          transparency: not_required
        exemptions: []
        evaluated_at: '2026-09-13T00:00:00Z'
    - role: container_image
      artifact:
        repository: dstacktee/dstack-verifier
        digest: sha256:<dstacktee/dstack-verifier image>
      verification:
        evidence: []
        provenance: compose_binding_only
        checks:
          repository_policy: pass
          image_signature: not_required
          transparency: not_required
        exemptions: []
        evaluated_at: '2026-09-13T00:00:00Z'
    - role: container_image
      artifact:
        repository: prom/node-exporter
        digest: sha256:<prom/node-exporter image>
      verification:
        evidence: []
        provenance: compose_binding_only
        checks:
          repository_policy: pass
          image_signature: not_required
          transparency: not_required
        exemptions: []
        evaluated_at: '2026-09-13T00:00:00Z'
verification_material:
- subject:
    kind: intel_tdx_collateral
    digest: sha256:<gateway canonical collateral set>
    fmspc: "<gateway FMSPC>"
    pck_ca: processor
    api_version: 4
  inputs:
    tcb_info: sha256:<gateway TCB information>
    qe_identity: sha256:<Intel QE identity>
    pck_crl: sha256:<Intel PCK CRL>
    root_ca_crl: sha256:<Intel root CA CRL>
    issuer_chains:
    - sha256:<gateway TCB issuer chain>
    - sha256:<QE identity issuer chain>
    - sha256:<PCK CRL issuer chain>
  verification:
    context:
      verifier_build: sha256:<teep build>
      policy: sha256:<intel_tdx_collateral applicable material policy>
    dependencies:
    - source: embedded
      kind: trust_anchor
      identity: sha256:<Intel root in this build>
    checks:
      signature_chains: pass
      collateral_scope: pass
    eligibility:
      basis: signed_evidence
      recheck:
      - validity
      - revocation
      - fresh_quote_platform_and_tcb
    evaluated_at: '2026-09-13T00:00:00Z'
- subject:
    kind: nvidia_jwks
    digest: sha256:<NVIDIA JWKS>
    authority: https://nras.attestation.nvidia.com/.well-known/jwks.json
  inputs:
    jwks: sha256:<NVIDIA JWKS>
  verification:
    context:
      verifier_build: sha256:<teep build>
      policy: sha256:<nvidia_jwks applicable material policy>
    dependencies:
    - source: configured
      kind: authenticated_origin
      identity: https://nras.attestation.nvidia.com/.well-known/jwks.json
    checks:
      origin_authentication: pass
      keyset_structure: pass
    eligibility:
      basis: authenticated_retrieval
      retrieved_at: '2026-09-13T00:00:00Z'
      recheck:
      - issuer_key_refresh_policy
      - new_token_signature_and_claims
    evaluated_at: '2026-09-13T00:00:00Z'
- subject:
    kind: ct_log_list
    digest: sha256:<CT log list>
    authority: chrome_ct_log_list
  inputs:
    log_list: sha256:<CT log list>
  verification:
    context:
      verifier_build: sha256:<teep build>
      policy: sha256:<ct_log_list applicable material policy>
    dependencies:
    - source: configured
      kind: authenticated_origin
      identity: "<configured CT log-list origin>"
    checks:
      origin_authentication: pass
      log_list_structure: pass
    eligibility:
      basis: authenticated_retrieval
      retrieved_at: '2026-09-13T00:00:00Z'
      recheck:
      - log_list_refresh_policy
      - live_peer_certificate_and_scts
    evaluated_at: '2026-09-13T00:00:00Z'
operator_decisions: []
evidence:
- digest: sha256:<gateway compose>
  kind: compose
  payload_base64: "<complete original bytes of this evidence object>"
- digest: sha256:<gateway TCB information>
  kind: intel_tcb_info
  payload_base64: "<original signed TCB information JSON>"
- digest: sha256:<Intel QE identity>
  kind: intel_qe_identity
  payload_base64: "<original signed QE identity JSON>"
- digest: sha256:<Intel PCK CRL>
  kind: x509_crl
  payload_base64: "<original DER CRL for the applicable PCK CA>"
- digest: sha256:<Intel root CA CRL>
  kind: x509_crl
  payload_base64: "<original DER CRL>"
- digest: sha256:<NVIDIA JWKS>
  kind: jwk_set
  payload_base64: "<original NVIDIA JWKS JSON>"
- digest: sha256:<CT log list>
  kind: ct_log_list
  payload_base64: "<original authenticated CT log-list JSON>"
- digest: sha256:<gateway TCB issuer chain>
  kind: intel_issuer_chain_header
  payload_base64: "<original header value bytes, without decoding or normalization>"
  header_name: Tcb-Info-Issuer-Chain
  encoding: percent_encoded_pem
- digest: sha256:<QE identity issuer chain>
  kind: intel_issuer_chain_header
  payload_base64: "<original header value bytes, without decoding or normalization>"
  header_name: Sgx-Enclave-Identity-Issuer-Chain
  encoding: percent_encoded_pem
- digest: sha256:<PCK CRL issuer chain>
  kind: intel_issuer_chain_header
  payload_base64: "<original header value bytes, without decoding or normalization>"
  header_name: Sgx-Pck-Crl-Issuer-Chain
  encoding: percent_encoded_pem
```

### 6f. Optional endpoint persistence references software by identity

Ordinary portable files omit this collection. This NearCloud fragment shows the
relationship to two complete software records using their actual semantic selectors,
not a local stanza ID. Tinfoil cloud selects gateway release sets under its
model-independent router scope; direct selects model release sets and its resolved
model authority. Neither uses one connection or authorization per component.

```yaml
schema_version: 1
endpoint_authorizations:
- scope:
    provider: nearcloud
    model: example-model
    authority: cloud-api.near.ai
  deployment:
    identity: "<deployment identity>"
    persisted_authorization: "<durable authorization identity>"
  identity:
    tls_spki_sha256: "<gateway SPKI>"
    backend_tls_spki_sha256: "<attested backend; not live gateway TLS peer>"
    model_ed25519_public_key: "<attested model key>"
  admission:
    context:
      verifier_build: sha256:<teep build>
      policy: sha256:<complete NearCloud admission effective policy>
    evidence:
    - sha256:<nearcloud original response envelope>
    - sha256:<historical backend NRAS JWT>
    - sha256:<historical backend Proof of Cloud response>
    - sha256:<historical gateway Proof of Cloud response>
    report: sha256:<complete immutable admission report>
    software:
    - scope:
        provider: nearcloud
        tier: model
        evidence_format: near
      subject:
        kind: compose
        digest: sha256:<model compose>
        encoding: app_compose_json
      policy: sha256:<nearcloud model effective policy>
    - scope:
        provider: nearcloud
        tier: gateway
        evidence_format: dstack
      subject:
        kind: compose
        digest: sha256:<gateway compose>
        encoding: app_compose_json
      policy: sha256:<nearcloud gateway effective policy>
    decisions: []
    evaluated_at: '2026-09-13T00:00:00Z'
    completion:
      required_non_deferred_checks: satisfied_effective_policy
      e2ee_usable: pass
  durable_state:
    state_identity: "<deployment state identity>"
    authorization_identity: "<durable authorization identity>"
    creation_revision: 12
evidence:
- digest: sha256:<complete immutable admission report>
  kind: verification_report
  payload_base64: >-
    <complete original normalized immutable admission report, including actual failed checks and
    permitted exceptions>
- digest: sha256:<historical backend NRAS JWT>
  kind: nras_response
  payload_base64: "<original signed response and report-binding metadata for the admitted backend
    GPU payload>"
- digest: sha256:<historical backend Proof of Cloud response>
  kind: proof_of_cloud_response
  payload_base64: "<original response bound to the admitted backend quote>"
- digest: sha256:<historical gateway Proof of Cloud response>
  kind: proof_of_cloud_response
  payload_base64: "<original response bound to the admitted gateway quote>"
```

Combine this extension with all software, material, and evidence records in 6a
into one cache artifact; this is an example relationship, not a YAML include or a
second cache-input flag. Identical evidence digests occur once. The additional
report and historical service responses above explain the completed admission;
they cannot authorize a new nonce or report. Before executable fixture coverage,
populate the full report, exact required encryption identity, and every dependency
used by the production authorization constructor. Do not retain inference payloads
or ephemeral secrets to demonstrate a successful E2EE check.

The service's separate restricted `cache_state_dir` contains the authoritative
state represented below. This is a distinct durable-state schema, not a second
portable trust artifact. Its deployment and state identities must agree with the
endpoint record. The binding digest covers the canonical complete persisted
authorization, so replacing its report/identity/dependencies cannot reuse the entry.

```yaml
schema_version: 1
kind: endpoint_persistence_state
deployment_identity: "<deployment identity>"
state_identity: "<deployment state identity>"
owner_clean: true
revision: 12
authorizations:
- identity: "<durable authorization identity>"
  authorization_digest: sha256:<canonical complete persisted authorization>
  creation_revision: 12
  state: active
- identity: "<previous invalidated authorization identity>"
  authorization_digest: sha256:<previous persisted authorization>
  creation_revision: 10
  state: invalidated
  invalidated_at_revision: 11
```

An authoritative `active` entry is necessary, not sufficient: apply all Section 2c
and 3c restoration checks, including current deployment/build/policy and scope.
This file is never imported from a portable evidence bundle. `owner_clean: true`
represents a fully drained predecessor, not a status an active process may retain.
Startup commits `owner_clean: false` before publishing any authorization; an unclean
predecessor requires fresh admission even when individual entries say active. Commit invalidation
or eviction through the mandatory writer before later restoration can succeed;
missing state is not an empty invalidation set. Restore only a matching committed
pair after crash recovery. Policy/configuration rollout must protect both files
against rollback. The illustrative ledger layout must retain these invariants
when the implementation defines crash-safe storage and retention.

The combined artifact and durable state satisfy the dependency layout required by
the Section 2c and durable-state contracts once populated and verified. These fields describe inputs to
the shared authorization constructor, not a bypass for report/key publication.
The backend fingerprint remains evidence about the backend, never the gateway TLS
peer. No consumable nonce pool, TLS connection, session ticket, or ephemeral secret
is serialized. `verify` always performs fresh admission even when these records exist.

### 6g. One subject, separate consumer evaluations and exceptions

This compact hypothetical compose has one component; it demonstrates relationships,
not the membership of a production NEAR compose. The original compose bytes are
identical for both consumers. NearCloud uses an explicit repository decision and a
pre-existing configured transparency exception; NearDirect's policy requires and
passes both checks. These are illustrative operator policies, not provider defaults.
The failed base checks remain visible. Neither consumer inherits the other's trust.

```yaml
schema_version: 1
policy_state:
  authority: "<deployment policy authority>"
  revision: 1
  removed_decisions: []
software:
- subject:
    kind: compose
    encoding: app_compose_json
    digest: sha256:<identical hypothetical compose bytes>
  evaluations:
  - scope:
      provider: nearcloud
      tier: model
      evidence_format: near
    verification:
      context:
        verifier_build: sha256:<build>
        policy: sha256:<applicable cloud rules and decision>
      evidence:
      - sha256:<identical hypothetical compose bytes>
      checks:
        required_membership: pass
        component_policy_coverage: pass
      evaluated_at: '2026-09-13T00:00:00Z'
    components:
    - role: container_image
      artifact:
        repository: example-org/worker
        digest: sha256:<worker image>
      verification:
        evidence:
        - sha256:<worker provenance>
        checks:
          repository_policy: fail
          signer_identity: pass
          transparency: fail
        decisions:
        - sha256:<canonical complete repository decision below>
        exemptions:
        - factor: build_transparency_log
          source: configured_allow_fail
          outcome: fail
        evaluated_at: '2026-09-13T00:00:00Z'
  - scope:
      provider: neardirect
      tier: model
      evidence_format: near
    verification:
      context:
        verifier_build: sha256:<build>
        policy: sha256:<applicable direct rules>
      evidence:
      - sha256:<identical hypothetical compose bytes>
      checks:
        required_membership: pass
        component_policy_coverage: pass
      evaluated_at: '2026-09-13T00:00:00Z'
    components:
    - role: container_image
      artifact:
        repository: example-org/worker
        digest: sha256:<worker image>
      verification:
        evidence:
        - sha256:<worker provenance>
        checks:
          repository_policy: pass
          signer_identity: pass
          transparency: pass
        decisions: []
        exemptions: []
        evaluated_at: '2026-09-13T00:00:00Z'
operator_decisions:
- scope:
    provider: nearcloud
    tier: model
    evidence_format: near
  kind: repository
  subject:
    repository: example-org/worker
  replaces_failure: repository_not_listed
  action: pin_observed_value
  observation:
    context:
      verifier_build: sha256:<build>
      policy: sha256:<cloud base rules>
    evidence:
    - sha256:<fresh authenticated observation>
    checks:
      quote_signature: pass
      nonce_binding: pass
      repository_policy: fail
    evaluated_at: '2026-09-13T00:00:00Z'
  reason: Accept this repository under the existing signer and binding requirements.
  decided_at: '2026-09-13T00:01:00Z'
  risk_acknowledgements: []
evidence:
- digest: sha256:<identical hypothetical compose bytes>
  kind: compose
  payload_base64: "<complete original compose>"
- digest: sha256:<worker provenance>
  kind: rekor_provenance
  payload_base64: "<complete evidence evaluated under each consumer's distinct requirements>"
- digest: sha256:<fresh authenticated observation>
  kind: endpoint_attestation
  payload_base64: "<complete fresh quote, binding and observation evidence>"
```

`decisions` uniformly contains canonical decision-record digests on verification
results and endpoint admission. Empty lists mean no decision was used. `exemptions`
records existing explicit policy exceptions; it is not a list of new decisions or
successful checks. The effective-policy hash includes these dependencies. Every
referenced decision and prerequisite must resolve before a result is reusable.

### 6h. Tinfoil cloud: SEV router admission prefill

This companion example includes the router release, its Sigstore trust dependencies,
the applicable AMD VCEK, and CT metadata. It contains no backend model authorization.
The certificate must match the fresh router report's chip, product, and TCB; a new
chip or TCB can require another certificate. Embedded AMD signing chains require no
retrieval. There is no TDX collateral, hardware registry, or GPU verdict for this
illustrated SEV router scope.

```yaml
schema_version: 1
software:
- subject:
    kind: release_set
    digest: sha256:<canonical complete router SEV component set>
    encoding: canonical_release_set_v1
  evaluations:
  - scope:
      provider: tinfoil_v3_cloud
      tier: gateway
      evidence_format: tinfoil_v3
    verification:
      context:
        verifier_build: sha256:<teep build>
        policy: sha256:<Tinfoil cloud SEV effective policy>
      checks:
        required_membership: pass
        component_policy_coverage: pass
      exemptions: []
      evaluated_at: '2026-09-13T00:00:00Z'
    components:
    - role: code_release
      artifact:
        repository: tinfoilsh/confidential-model-router
        digest: sha256:<tinfoilsh/confidential-model-router release subject>
      required_binding: sev_launch_measurement
      verification:
        dependencies:
        - source: cache
          subject:
            kind: sigstore_trust_material
            digest: sha256:<canonical Sigstore trust material set>
            authority: sigstore_public_good
        evidence:
        - sha256:<tinfoilsh/confidential-model-router signed release>
        checks:
          release_signature: pass
          signer_identity: pass
          transparency: pass
        exemptions: []
        evaluated_at: '2026-09-13T00:00:00Z'
verification_material:
- subject:
    kind: amd_vcek
    digest: sha256:<AMD VCEK>
    product: Genoa
    hwid: "<chip HWID>"
    tcb: "<complete certificate TCB extensions>"
  inputs:
    certificate: sha256:<AMD VCEK>
  verification:
    context:
      verifier_build: sha256:<teep build>
      policy: sha256:<amd_vcek applicable material policy>
    dependencies:
    - source: embedded
      kind: certificate_chain
      identity: sha256:<AMD Genoa signing chain in this build>
    checks:
      certificate_chain: pass
      certificate_extensions: pass
    eligibility:
      basis: signed_evidence
      recheck:
      - validity
      - applicable_revocation
      - fresh_report_hwid_and_tcb
    evaluated_at: '2026-09-13T00:00:00Z'
- subject:
    kind: sigstore_trust_material
    digest: sha256:<canonical Sigstore trust material set>
    authority: sigstore_public_good
  inputs:
    timestamp: sha256:<Sigstore timestamp>
    snapshot: sha256:<Sigstore snapshot>
    targets: sha256:<Sigstore targets>
    trusted_root: sha256:<Sigstore trusted_root>
    root_chain:
    - sha256:<Sigstore root>
  verification:
    context:
      verifier_build: sha256:<teep build>
      policy: sha256:<sigstore_trust_material applicable material policy>
    dependencies:
    - source: embedded
      kind: tuf_bootstrap_root
      identity: sha256:<Sigstore bootstrap root in this build>
    checks:
      tuf_signatures: pass
      versions_and_target_hashes: pass
    eligibility:
      basis: signed_evidence
      recheck:
      - tuf_expiry
      - rollback_and_root_rotation
      - signer_and_log_policy
    evaluated_at: '2026-09-13T00:00:00Z'
- subject:
    kind: ct_log_list
    digest: sha256:<CT log list>
    authority: chrome_ct_log_list
  inputs:
    log_list: sha256:<CT log list>
  verification:
    context:
      verifier_build: sha256:<teep build>
      policy: sha256:<ct_log_list applicable material policy>
    dependencies:
    - source: configured
      kind: authenticated_origin
      identity: "<configured CT log-list origin>"
    checks:
      origin_authentication: pass
      log_list_structure: pass
    eligibility:
      basis: authenticated_retrieval
      retrieved_at: '2026-09-13T00:00:00Z'
      recheck:
      - log_list_refresh_policy
      - live_peer_certificate_and_scts
    evaluated_at: '2026-09-13T00:00:00Z'
operator_decisions: []
evidence:
- digest: sha256:<tinfoilsh/confidential-model-router signed release>
  kind: sigstore_bundle
  payload_base64: "<complete original router Sigstore bundle>"
- digest: sha256:<AMD VCEK>
  kind: x509_certificate
  payload_base64: "<original VCEK DER certificate>"
- digest: sha256:<Sigstore root>
  kind: tuf_root
  payload_base64: "<original signed TUF root metadata>"
- digest: sha256:<Sigstore timestamp>
  kind: tuf_timestamp
  payload_base64: "<original signed TUF timestamp metadata>"
- digest: sha256:<Sigstore snapshot>
  kind: tuf_snapshot
  payload_base64: "<original signed TUF snapshot metadata>"
- digest: sha256:<Sigstore targets>
  kind: tuf_targets
  payload_base64: "<original signed TUF targets metadata>"
- digest: sha256:<Sigstore trusted_root>
  kind: sigstore_trusted_root
  payload_base64: "<original trusted-root target bytes>"
- digest: sha256:<CT log list>
  kind: ct_log_list
  payload_base64: "<original authenticated CT log-list JSON>"
```

## 7. Storage, parsing, and concurrency

Use bounded, strict YAML decoding. Reject unknown or missing required fields,
duplicates, null required values, invalid identifiers, unsupported schema versions,
multiple documents, cycles, aliases, and ambiguous references. Initial implementation
limits are 64 MiB encoded file size, 48 MiB total decoded evidence, 16 MiB per decoded
evidence object, 65,536 records, nesting depth 32, and dependency depth 16; lower
existing kind-specific parser/network bounds still apply. Check bounds while reading
and before allocating decoded payloads. These cover the sampled sub-MiB artifacts
without promising unlimited model coverage; all-model selection remains supported,
and an oversized explicit write fails with a size diagnostic rather than dropping
policy or required evidence. Exercise the maximum allowed artifact and parallel
loads in storage tests; do not claim startup latency from file size alone. Bound file size,
object count, decoded payload size, nesting, and reference traversal. Dispatch
evidence kinds explicitly; reject unsupported kinds. Parse embedded JSON through
`internal/jsonstrict`. Low-level parsers return unknown field names to the caller.

Validate digests against retained bytes and compare cryptographic values in
constant time. Require every verified subject's policy and identity to agree with its
verification record and actual authenticated evidence. Reject incomplete graphs;
never drop a malformed entry and continue with the rest. Checks in examples are
illustrative typed checks, not new report factor names.

Validate ownership, restrictive permissions, regular-file type, and absence of
symlinks for cache and durable state paths. Use safe file opening and replacement
to prevent path substitution between validation and access. No group/world writable
trust files. Treat read-only image-layer files as deployment inputs with equivalent
integrity guarantees. Preserve complete evidence on export; URLs alone are not an
offline artifact. YAML comments may be regenerated rather than preserved.

Put validated import, immutable export, and source-independent admission interfaces
beside the shared data-management code. Keep filesystem decoding and locking outside
request handlers. Reuse existing runtime synchronization, authorization capacity,
generation ownership, and cancellation semantics rather than wrapping them in a
competing cache. Coordinate prefill and live publication under the same ownership
rules: a delayed load cannot overwrite a newer authorization or restore one already
invalidated. Perform disk and network I/O outside the runtime store mutex; recheck
publication eligibility under synchronization before publishing the result.

A snapshot exports a complete dependency graph for each successful set. Canonicalize
set membership independently of presentation order; reject duplicate component
identities, ambiguous software selectors, and evidence-digest/content conflicts.
Do not bind parser behavior to the illustrated repositories or collection positions.
Nested results must use their enclosing evaluation's explicit scope and verification context;
reject attempts to substitute a result from another policy, tier, or build. Autocache
merges new components without overwriting sibling results or combining different
compose versions into a configuration never observed. Adding/removing a component
changes set identity; unchanged components remain reusable. Preserve evidence used
by any retained set. Whitelist proposals name the exact component and affected sets;
a decision for one repository cannot waive a sibling's provenance failure.

Use immutable snapshots for published entries. Keep mutable state on constructed
stores, not package globals. Bound entries, retained evidence bytes, and concurrent verification work. Use
reference-aware collection of unneeded older evaluations and evidence, preserving
active decisions and required dependencies. If safe collection cannot make room,
fail the explicit cache write or report failed optional persistence; never write
an oversized file that its own loader rejects. Do not grow tombstones or envelope
history indefinitely without an explicit trusted policy checkpoint/retention contract.
Deduplicate evidence retrieval and verification under exact subject/policy keys
with server-owned bounded contexts; one client's cancellation cannot cancel work
needed by another. Retain independent routing/discovery stores.

Use process-local synchronization for memory and a separate lock file for the
cross-process read-merge-write transaction. Under the lock, reread, validate, merge
without reviving invalidated records, write a restricted temporary file, sync,
rename atomically, and sync the containing directory. Define crash-safe ordering
between authorization publication and durable invalidation. Lock files must survive
cache-file replacement. Disjoint provider updates preserve each other's objects;
reference-aware collection must not delete evidence used by another verified subject.

Use a fixed lock order: acquire the file transaction lock without a runtime mutex;
load the authoritative artifact and policy revision; validate the immutable snapshot
against that state; resolve additions/removals and collect references; write/sync the
replacement; release the file lock. Runtime publication/invalidation rechecks its own
generation after any required disk work. Never acquire a file lock while holding the
runtime authorization mutex. A cancelled observer cannot cancel a transaction another
client needs; writes have a bounded owner context.

For endpoint persistence, write/sync the authorization object first, then commit/sync
its digest-bound active ledger entry before claiming persistence. A crash between
those steps leaves an unreferenced, unrestorable object. Invalidation/eviction must
prevent acquisition of that generation in memory while its invalidated ledger state
is synchronously committed; a failed commit blocks affected reuse. An interrupted
invalidation must not leave an old active authorization restorable: maintain a
write-ahead invalidation intent, treat any incomplete intent as invalidated on restart,
and never acknowledge durable completion before directory synchronization. Retain
state and intents under the separate deployment-owned directory; evidence-file
replacement cannot reset them. Storage tests must exercise every boundary and the
existing authorization generation race checks together. Offline whole-directory
rollback remains a trusted deployment responsibility, not a property of YAML revisions.

An optional evidence write failure can leave verified memory state intact. A
mandatory persistence failure cannot be converted to success. Logs identify target,
check, and failure without API keys, inference content, or private key material.

## 8. Planning findings and implementation phases

### Shared admission design established during planning

The current commonality is lower-level verification, not one shared orchestration
service. `proxy.Server.fetchVerified` owns serving orchestration and calls proxy
helpers; `verify.runEvidence` independently constructs verifiers, fetches evidence,
collects factors, and completes probes. The existing `SYNC` comments identify this
duplication. Both use attestation/provider primitives, but cache code must not be
inserted twice. `proxy.authorizationStore` owns the live generation, bounded shared
work, report/key/identity publication, eviction, and usability promotion. Its
`publish` method rechecks admission time immediately before storing a generation.

Extract `internal/admission` as a service layer importing provider and attestation
primitives, never proxy or CLI packages. Proxy and verify consume it. Keep provider
route resolution in existing immutable route APIs. Use three boundaries:

1. `Collect`: fetch a fresh nonce-bound raw response for the resolved route through
   the provider-owned attester and injected clients. Preserve original evidence and
   strict parsing; no cached historical quote can satisfy this operation.
2. `Evaluate`: run the shared CPU, GPU, PoC, software, binding, and effective-policy
   operations, returning an immutable verified-evidence snapshot and admission-time
   eligibility. Material resolvers are injected per owner; the snapshot includes
   original dependencies, actual failures/exemptions, and authenticated key/identity.
3. `ExportPortable`: derive only complete eligible portable graphs from that snapshot
   through the typed material adapters. A report's display fields are not export input.

These are responsibilities and proposed internal names, not new public CLI APIs.
Keep the existing proxy authorization store as the sole runtime generation owner;
it consumes the shared candidate and retains its constructor/publication checks.
`verify` always calls fresh collection/evaluation and then its required probe, without
acquiring proxy runtime authorization. `cache` uses the same collection/evaluation
and exports after non-deferred checks. Serving promotes E2EE usability only for the
generation/model that actually completed it; portable export and persisted complete
authorization therefore have distinct eligibility boundaries. Do not move probe or
retry policy into a second cache orchestration path.

Material lookup/population and policy projection are specified in Sections 2e, 4,
and 6. Preserve caller cancellation and server-owned bounded shared work; perform
network/disk I/O outside runtime locks. Source-independent prefill publishes through
the same generation checks as live population. The remaining work in Phases 0/1 is
permanent test infrastructure and this tested refactor, not rediscovering ownership.
Material, policy, and interface acceptance still require production-path tests; the
planning prototypes do not replace them.



Implement Phases 0 through 13 in order. Each phase builds on the completed preceding
phases; it is not independently applicable to an earlier checkout. Each phase must
produce one reviewable commit with a complete implementation, tests, and maintained
documentation for the behavior it introduces. If a phase proves too large for one
reviewable commit, divide its implementation and acceptance boundaries explicitly
before starting it, and update the dependent phase references.

Run `make check` before each commit. Major runtime changes also require
`make integration` and `make reports`; positive coverage uses production factor
enforcement. Apply the Section 1 Tinfoil direct live-validation prerequisite: retain
all unblocked checks and deterministic direct coverage, record deferred live checks
in the commit description, and complete them after the upstream fix is deployed.
Do not claim complete direct live validation while it remains blocked. These phases
specify work and acceptance requirements, not implementation status or test logs.

Every implementation phase owns its scope, upgrade, concurrency, failure, and request
count tests. There is no later phase that supplies missing correctness coverage.
Later phases add tests for the new interactions they introduce. Use production
cryptography, TLS test servers, bounded contexts, and real signed fixture material.
Never depend on ambient developer caches to satisfy a request budget. Each material
adapter must demonstrate same-build reuse, current-build local reevaluation, and
precise retrieval or rejection for missing/ineligible dependencies before completion.

Reject unsupported record kinds, decisions, and options until their implementing
phase enables them; never accept an uninterpreted trust record or silently ignore
its policy. Schema declarations do not enable a capability. Do not add compatibility
paths for intermediate implementations. Shared interface changes belong to the phase
that needs them, with all existing callers and tests updated in that commit.

| Milestone | First phase delivering the behavior |
| --- | --- |
| Internal shared prefill and export contracts | 1, with concrete portable storage in 2 and adapters in 3–7 |
| Goal 1: explicit preparation and portable reuse through all three commands | 8 |
| Consumption of exact operator decisions through shared effective policy | 9 |
| Goal 2: reviewed whitelist authoring, proposals, and withdrawals | 10 |
| Automatic portable persistence during serving | 11 |
| Optional endpoint persistence and restoration | 13, after durable-state acceptance in 12 |

Optional endpoint persistence remains part of this sequence and defaults off.
Chutes/Venice enablement and unresolved elevated decision classes are separate
conditional extensions described below; they do not block Near/Tinfoil delivery.
Each phase updates the documentation identified in
[Section 9c](#9c-phase-requirements-and-completion-checks).

Planning source map: [serving orchestration](../../internal/proxy/proxy.go),
[standalone verification](../../internal/verify/verify.go),
[runtime authorization ownership](../../internal/proxy/authorization.go),
[policy and factor evaluation](../../internal/attestation/report.go),
[Rekor verification](../../internal/attestation/rekor.go),
[TDX adapter](../../internal/attestation/tdx.go),
[SEV adapter](../../internal/attestation/sev.go),
[Tinfoil certificate retrieval](../../internal/provider/tinfoil/kds.go),
[Tinfoil release retrieval](../../internal/provider/tinfoil/sigstore.go),
[NVIDIA key ownership](../../internal/attestation/nvidia.go), and
[CT checker/client construction](../../internal/tlsct/checker.go).
Pinned dependency behavior comes from [go.mod](../../go.mod),
[Sigstore v1.2.1 TUF client](https://github.com/sigstore/sigstore-go/blob/v1.2.1/pkg/tuf/client.go),
[go-tuf trusted metadata](https://github.com/theupdateframework/go-tuf/blob/7e8f69f906ef/metadata/trustedmetadata/trustedmetadata.go),
and [go-tuf updater](https://github.com/theupdateframework/go-tuf/blob/7e8f69f906ef/metadata/updater/updater.go).
These establish the API boundaries; implementation tests must exercise the pinned
versions rather than assuming newer library behavior.

### Phase 0: Permanent request accounting

Implement permanent counters and regression fixtures for the planning baseline in
Sections 4b–4e; extend response-only captures to observe all actual attempts. Count all
teep-owned and dependency-owned HTTP attempts, including retries, redirects, CT/TUF
bootstrap and refresh, discovery, live attestation, NRAS, and Proof of Cloud.
Separate preparation, pre-inference work, inference/probe requests, and TLS handshakes.
Record provider, format, hardware scope, policy, and dependency-cache conditions.

Test successful service sequences and early failures separately; a shorter failure
sequence is not a successful-admission budget. Provide deterministic counters and
network-denial controls for later adapters without asserting unimplemented savings.
Use the [NEAR fixture loader](../../internal/integration/helpers_test.go),
[NearCloud fixtures](../../internal/integration/nearcloud_test.go),
[NearDirect fixtures](../../internal/integration/neardirect_test.go), and
[model-key binding tests](../../internal/integration/near_model_binding_test.go).
Establish extracted-byte and component parity without assuming different captures
share quotes, nonces, TLS identities, or complete compose bytes. This phase changes
no caching, policy, command, or admission behavior.

### Phase 1: Shared admission interfaces

Implement the shared admission boundaries and ownership specified above, including
source-independent collection, evaluation, immutable snapshots, and validated prefill.
Connect existing Near/Tinfoil live serving and ordinary live `verify` to the shared
services. Keep provider scope, key-use lifetime, report/key publication, admission-time
checks, bounded verification ownership, and generation-safe invalidation there.
Define the adapter boundary for original inputs and verifier-owned dependency stores;
do not create a second authorization store or build trust from display reports.

Test unchanged live outcomes, complete report/key/identity publication, required
NRAS admission-time checks, cancellation isolation, eviction, replacement races,
and unrelated HTTP/2 streams. Test prefill/publication ownership through the actual
shared services using verified in-memory inputs; disk encoding belongs to Phase 2.
No new CLI or disk restoration is enabled in this phase.

### Phase 2: Portable artifact storage

Implement strict decoding, canonical software/material identities, build and scoped
policy identities, explicit dependency resolution, validated import, immutable export,
and bounded reference-aware storage. Define the typed-list envelope and supported-kind
dispatch used by later adapters. Include the empty policy-state contract; nonempty
operator policy remains unsupported until Phase 9. Keep endpoint restoration disabled.

Implement secure file access and the cross-process read/validate/merge/write transaction:
separate stable lock file, restrictive temporary files, fsync, atomic replacement,
and directory synchronization. Keep encoding and disk I/O outside runtime mutexes.
Define hooks for current-policy validation under the lock; do not implement whitelist
editing here. Resolve repeated evaluations by subject/scope/policy/build and reject
conflicting results rather than selecting a newer pass.

Test malformed/unknown input, aliases/cycles, forged checks, duplicate or ambiguous
selectors, dangling dependencies, content mismatch, policy/build mismatch, untrusted
imports, size bounds, reference collection, symlinks, path substitution, permissions,
concurrent disjoint writers, cancellation, write failure, and crash boundaries.
Use supported production evidence representations for storage tests; new material
kinds become usable only with their verified adapter. No command is enabled yet.

### Phase 3: NEAR software reuse

Implement compose and component export/prefill through the shared supply-chain
verifier. Preserve original stapled envelope relationships and independent gateway
and backend verification. Share exact evidence and equivalent subchecks across
NearCloud/NearDirect without sharing endpoint authorization or consumer policy.
Implement the required-versus-diagnostic retrieval classification in Section 4;
never suppress an enforced factor because a component says `not_required`.

Test complete membership, later-component signature failure, arbitrary replacement,
list reordering, two versions of one repository, repository/digest aliasing, wrong
model selection, incomplete backend evidence, envelope containment tampering, and
same/different compose subjects sharing image bytes. Cover tier/provider isolation,
changed gateway/backend keys, policy dependency projections, same-key refresh,
concurrent imports, and prefill racing publication or eviction. Deny provenance
network access for eligible reuse and local upgrade reevaluation; assert remaining
required queries and accurately report unrefreshed optional diagnostics. These tests
complete the software portion of examples 6a/6b, not their collateral budgets.

### Phase 4: CPU collateral reuse

Add Intel collateral and AMD VCEK adapters through the existing CPU verifiers and
certificate retrieval paths. Implement quote-derived applicability, canonical input
sets, explicit issuer-chain dependencies including captured HTTP headers, signed
validity/revocation eligibility, and verified export. Preserve embedded signing roots
and chains and Tinfoil's configured VCEK origin. Cached material is not a verdict
on a new CPU quote.

Test FMSPC/CA selection, shared QE/CRL objects across scopes and providers, distinct
TCB objects, AMD product/HWID/TCB mismatch, certificate extensions, expiry, revocation,
missing chains, incorrect signatures, upgrade reevaluation, and concurrent retrieval
sharing. Prove zero eligible CPU-collateral retrievals with those origins denied,
and exact retrieval or rejection for ineligible objects. Do not add evidence expiry
to an already admitted runtime authorization or weaken fresh quote validation.

### Phase 5: Tinfoil release reuse

Implement complete code/platform release sets, signed measurement-reference reuse,
explicit Sigstore TUF dependencies, and bounded known-candidate release retrieval on a miss.
Use the existing signature and measurement-binding verifiers. Retain required root
transitions, timestamp/snapshot/targets and delegated metadata, and trust-target bytes.
Distinguish embedded bootstrap roots from retained evidence. Supplied V3 collateral
that lacks shared verification remains unsupported as a verified result.

Test direct TDX code/hardware binding independently and cloud SEV router scope.
Cover an older authenticated matching release, a newer unbound release, tag-only
references, incomplete bundles, component failures, and known-candidate count/byte/time
limits, including explicit failure of unsupported release enumeration. Exercise TUF signatures, expiry, root transitions, rollback/version checks,
missing dependencies, scoped policy changes, and local build-update reevaluation.
Deny eligible release/TUF retrievals and count cold discovery separately. Preserve
router sharing and direct authority isolation. These tests complete the release
portion of examples 6c/6h; CT prefill follows in Phase 7.

### Phase 6: NVIDIA key-material reuse

Implement authenticated JWKS snapshot/import/export through the existing NVIDIA
verifier. Preserve issuer/origin policy, original retrieval time, cache eligibility,
key-rotation refresh, and bounded concurrent retrieval. This phase does not introduce
a portable NRAS verdict or an independent NVIDIA reference-image verifier.

Test trusted import, stale and malformed keysets, incorrect authority, unknown key
IDs, eligible key rotation, refresh failure, concurrent consumers, and build/policy
changes. Deny JWKS requests on an eligible match while proving that new GPU evidence
still produces its required NRAS submission and signature/claim validation. Test
that copying a file does not extend key eligibility and another report's NRAS result
cannot satisfy a new nonce. Retain initial publication-time eligibility checks.

### Phase 7: CT metadata reuse

Implement CT log-list snapshot/import/export and prefill every relevant checker,
including clients owned by dependencies. Preserve bootstrap origin authentication,
original retrieval time, refresh rules, and live WebPKI, TLS identity, and SCT checks.
Use the same material interfaces; do not use an inference-client-only cache or
weaken bootstrap verification to avoid a request.

Test all checker construction paths, eligible prefill, expiry, malformed/substituted
lists, changed log policy, refresh failure, concurrent use, and local upgrade
reevaluation. Deny log-list retrieval on eligible inputs while checking live peer
certificates through production TLS. Compose with the TUF adapter to expose hidden
client traffic. Existing provider routing and connection scopes remain unchanged.

### Phase 8: Command integration and complete portable budgets

Enable ordinary `teep cache`, read-only `serve` prefill, and cache-aware read-only
`verify` together. Use one path resolver with the specified flag/environment/config/
default precedence. Implement target resolution, multi-target collection, complete
successful-target export, partial-failure reporting, and immutable loaded snapshots.
Use the established shared services, material adapters, and disk transaction layer;
introduce no provider-specific verification inside command orchestration.

Test all target forms and invalid syntax, discovery changes, complete dependencies,
missing implicit versus explicit files, destination creation, secure/read-only files,
concurrent replacement, and explicit unsupported providers. Cover the relevance rules
for implicit default data and `verify --no-cache`. Verify reports identify the exact
loaded artifact/build/policy; no-cache verification must not claim policy-rollout
validation. Test fresh admission, deferred usability versus required live probes,
and absence of cache/decision writes from `verify` or ordinary `serve`.

Make Near/Tinfoil examples 6a, 6b, 6c, and 6h executable fixtures with real signed
bytes. Assert the complete same-build and build-update budgets in Section 4d with
prepared groups denied network access, independent cold replica state, and remaining
live calls counted. Exercise cross-command enforcement equivalence, scoped policies,
multi-component failure, and partial success. Include multi-model/cloud-router scope
and concurrent clients. This phase delivers Goal 1; operator decisions, autocaching,
and endpoint persistence remain disabled until their respective phases.

### Phase 9: Operator policy evaluation

Implement exact operator-decision interpretation in the shared policy evaluator and
all three command consumers. Expose typed failure reasons in the owning verifiers:
unlisted measurements, repository/signer/content policy violations, missing evidence,
invalid signatures, expiry, and authenticated revocation must remain distinguishable.
Implement the fully specified ordinary classes from Section 5a; enumerate the exact
supported classes and prerequisites in tests and maintained documentation. Elevated
or otherwise unresolved classes remain explicitly rejected. Accept nonempty policy
only through validated trusted imports at this stage; authoring follows in Phase 10.

Include scoped effective-policy projection, cumulative decisions, compatibility with
a new build, removal-state validation, and references to the actual failed base checks.
Make existing evidence writers preserve authoritative policy under their transaction
lock and discard incompatible snapshots without resurrecting decisions. No class
inherits a factor-wide override simply because several failures share a factor.

Test every implemented class and retained prerequisite with production verification,
including selected MRTD/MRSEAM tuples, unrelated measurement fields, subject/tier
separation, sibling component failures, unknown decision kinds, and remaining enforced
failures. Test exact value comparison, decision dependency tampering, unrelated versus
applicable policy edits, withdrawn decisions, and new verifier restrictions. Prove
matching effective-policy outcomes across cache/serve/verify, including existing
`allow_fail` behavior, matching/unused decisions, and absence of relabeled successes.
Count requests replaced by each class separately from its creation prerequisites.
Update security and review instructions in this commit, when exceptions become usable.

### Phase 10: Whitelist editing and policy transactions

Enable interactive `--update-whitelist`, `--reason`, proposal generation, and explicit
proposal application on top of Phase 9's evaluator and Phase 2's transaction layer.
Implement concrete selections, explanations, frozen target discovery, ordinary bulk
selection, exact proposal validation, and the acknowledgement representation required
by future elevated classes. Do not enable those classes through the UI prematurely.
Replace `--update-config`, `--config-out`, and the corresponding old measurement-policy
input in this commit; reject obsolete fields and update help/configuration together.

Implement policy authority/revision/state comparisons under the lock, explicit removal,
newly reviewed reintroduction, and atomic eligible additions plus withdrawals. Bind
proposals to exact reviewed subjects/evidence/failures/build/policy and current policy
state. Reevaluate successful targets against the committed subset. Report known shared
scope when a successful target's decision also affects a failed target; do not export
a successful evaluation for that failed target.

Test interactive cancellation/confirmation, nonempty reasons, all-model and selected
model flows, empty/unbounded selections, unsupported failures, strict/tampered proposals,
stale revisions, policy/build incompatibility, and noninteractive use without explicit
apply. Test no trust writes during proposal generation, withdrawals without provider
connectivity, mixed additions/removals, nonzero live failures after a committed withdrawal,
and removal racing ordinary evidence writers. Reject missing or extraneous elevated
acknowledgements without enabling unsupported classes. Keep prompts/reports free of
credentials and inference content. This phase delivers Goal 2's authoring workflow.

### Phase 11: Automatic portable persistence

Enable `serve --autocache` through the established immutable snapshot/export and
current-policy disk transaction. Implement bounded queues, coalescing, startup
writability checks, destination creation, asynchronous errors, shutdown handling,
and crash-safe replacement. Export only material eligible at the specified admission
boundary; inference response success is not required. Never create operator decisions,
poll releases, or add background discovery/verification requests.

Test first live admission followed by restart from the committed file, changed compose
and release evidence, complete component coverage, permitted failed checks, and deferred
E2EE usability. Assert the same portable budgets as explicit preparation. Cover concurrent
cache-command/service writers, queue saturation, read-only conflicts, slow/failed writes,
recovery, shutdown flush, and stale snapshots racing policy withdrawal/reintroduction.
Prove optional write failure leaves independently completed in-memory authorization
usable, does not claim persistence, and cannot revive removed decisions or overwrite
another consumer's evaluation. Endpoint persistence remains disabled.

### Phase 12: Durable endpoint state

Implement the deployment-owned persisted-authorization binding and invalidation ledger,
secure state access, mandatory durable writer, exclusive-owner clean/unclean state, committed-pair
validation, and crash recovery described in Sections 3c and 6f. Keep this separate from optional evidence
write-back. Define initialization and bounded retention without interpreting missing
history as proof that an old authorization remains valid. Use the shared generation
identity and classification contracts; do not publish restored runtime authorization.

Test exclusive ownership, durable unclean marking before publication, clean marking
only after drained shutdown, refusal of unclean-predecessor restoration,
state/authorization digest binding, invalidation and eviction transactions,
missing/corrupt/read-only state, deployment mismatch, stale records, interrupted writes,
and crashes at each ordering boundary. Verify an old generation cannot invalidate a
replacement and a committed invalidation cannot disappear through ordinary cache
replacement. Exercise the documented deployment responsibility for whole-state rollback;
a self-declared revision is not independent rollback protection. No endpoint-persistence
configuration or restoration is enabled until Phase 13 connects these tested services.

### Phase 13: Endpoint persistence and restoration

Enable default-off `cache_endpoint_persistence` and `cache_state_dir` configuration,
requiring `serve --autocache` and validated durable state. Persist only complete eligible
live authorizations, including required transport/E2EE usability. Restore through the
same constructor, publication, acquisition, and invalidation paths as live authorization.
A restored authorization must satisfy current deployment/build/policy/scope requirements.
Mandatory invalidation failures retain their fail-closed behavior even when the optional
portable writer is slow or unavailable.

Test configuration validation, new-state provisioning, complete 6a/6f companion fixtures,
clean same-build restoration budgets, unclean-shutdown and build/policy-change fresh admission, route changes,
missing state, eviction, failed keys, concurrent streams, and crashes across runtime/disk
boundaries. Verify stale requests cannot delete replacements and affected authorizations
cannot reappear after restart. Test live E2EE completion and inference rate limits versus
portable export eligibility. `cache` and `verify` never create or restore endpoint
authorization; verify must perform fresh admission even when such records exist.
Update transport lifetime and storage references with this behavior in the same commit.

### Conditional extensions: provider enablement and elevated decisions

For Chutes and Venice, complete separate transport migrations before cache enablement.
Require immutable routes, atomic report/key publication, shared bounded verification,
generation-safe invalidation, and concurrent acquisition through the common interfaces.
Ordinary live `verify` must use shared admission services. Test actual HTTP/2 negotiation
and multiplexing under production TLS/CT; streaming success is insufficient. Migration
is tested with live-populated state and does not depend on disk support. No phase may
add a legacy-cache adapter to work around an incomplete migration.

After each migration and applicable core phases, use a separate provider-enablement
commit with explicit capabilities and full live/prefill equivalence. Venice covers both
dstack and ACI/1, format changes, independent gateway/model scopes, weaker compose
provenance, unbound metadata, custody/app-ID/KMS failures, and expired keysets. Extend
[ACI coverage](../../internal/integration/venice_aci_test.go),
[concurrent-format coverage](../../internal/integration/venice_concurrent_formats_test.go),
and [custody/keyset tests](../../internal/provider/venice/keyset_test.go). Make example 6e
executable and assert its conditional budgets. Never promote exempted failures or
absent backend evidence into verified results; reject restoration until eligible.

Chutes covers chute/instance/key scope, ML-KEM binding, consumable nonce ownership,
nonce exhaustion/expiry/replenishment, concurrent consumption, and exact MRTD/MRSEAM
decisions with retained unrelated failures. Test that portable files cannot supply
consumable request nonces. Provider enablement uses the established policy, command,
and writer contracts and introduces no parallel caching machinery. PhalaCloud and
NanoGPT remain out of scope.

Enable each elevated decision class only in a separately bounded implementation after
its typed prerequisites, exact replacement check, scope, acknowledgement, and retrieval
consequences are specified. Its commit must include evaluator, authoring, consumer,
reporting, security/review documentation, and per-class negative/request-count tests.
Unresolved classes remain rejected, not implicitly included in core completion.

## 9. Maintained documentation and agent discovery

Create a maintained `docs/cache/` reference directory alongside `docs/transport/`.
The cache reference covers evidence persistence, typed verification material and
its freshness/dependency contracts, per-provider request budgets, operator decisions, command use,
and deployment as well as transport integration. Organize files around the changes
an agent needs to make, with a small entry point and focused contract documents.
The paths below are planned files; add working links when the files are created.

| Document | Authoritative content |
| --- | --- |
| `docs/cache/README.md` | Entry point: purpose, terminology, architecture, both prefill levels, command/configuration reference, deployment modes, and links to detailed contracts and implementation entry points. |
| `docs/cache/storage.md` | Typed-list schema, nested component verification contexts, software and typed verification-material selectors, explicit dependencies and admission eligibility, content-addressed evidence, operator decisions, and optional endpoint records; build/policy identity; validated import/prefill and immutable export; file integrity; atomic writes; concurrency; upgrade compatibility; restoration eligibility and durable invalidation storage. Include representative YAML for NearCloud, NearDirect, and both Tinfoil modes. |
| `docs/cache/operator-decisions.md` | `--update-whitelist` interactive selection, proposal generation and explicit apply, exact subject scope, supported/unsupported and elevated-risk classes, acknowledgements, retained checks, diagnostics, decision deployment/removal, and interactions with existing policy controls. |
| `docs/cache/testing.md` | Request-count methodology and scenario budgets, live/prefill equivalence, concurrency and persistence-failure coverage, commands to reproduce checks, and links to actual regression tests. |

### 9a. Ownership and cross-references

Keep runtime authorization scope, construction/publication, acquisition, TLS/E2EE
key-use lifetime, eviction, and invalidation effects authoritative in
`docs/transport/`. Retry eligibility remains in `docs/transport/retries.md`. Cache
references describe how persisted material enters that shared machinery and link
to those contracts instead of maintaining another copy of them.

`docs/cache/storage.md` owns persisted-record eligibility, durable invalidation
format, crash recovery, and disk transaction ordering. The transport reference
links there when explaining restoration across restart and preventing a persisted
record from restoring invalidated trust. In the other direction, storage links to
the transport rules that identify which failures invalidate which authorization.
Document the interface between the two subjects; do not duplicate their full rules.

Keep provider-specific routing, gateway/backend boundaries, evidence limitations,
and public-key semantics in the provider references. Those documents link to the
shared cache and transport contracts and identify provider-specific applicability.
Cache examples illustrate these differences without creating independent provider
specifications. Create `docs/providers/venice/venice_support.md` for both dstack and
ACI/1, including configuration, endpoints, routing, evidence, gateway/backend trust
boundaries, E2EE/custody, supply-chain provenance, factor exemptions, cache capability
matrix, and tests. Link it from the cache entry point and retain a link to the
[Venice ACI gap analysis](../attestation_gaps/venice_aci_gateway.md). Do not describe unimplemented restoration or decision classes as
supported behavior. Create `docs/providers/chutes/chutes_support.md` with the same
coverage, plus chute/instance routing, ML-KEM key binding, consumable nonce ownership,
measurement decisions, and the lack of client image-provenance evidence. Link its
[sek8s gap analysis](../attestation_gaps/sek8s_integrity.md). Both references must
clearly distinguish current runtime behavior, blocked cache capabilities, and
post-migration contracts. Document the migration prerequisite in the cache support
matrix and agent entry points. State that PhalaCloud and NanoGPT are not planned;
do not create cache integration requirements for them.

### 9b. Agent discovery and repository entry points

Update [AGENTS.md](../../AGENTS.md) when the reference files are introduced. Add a
short task-to-reference mapping to its directory/documentation guidance:

| Change being made | Required reference entry point |
| --- | --- |
| HTTP/TLS transport or runtime authorization | `docs/transport/README.md` |
| Evidence reuse, cache schema, persistence, or `teep cache` | `docs/cache/README.md` |
| Prefill, endpoint restoration, or invalidation across restart | Both cache and transport entry points |
| Operator pins or policy exceptions | `docs/cache/operator-decisions.md` and the applicable provider policy reference |

Mirror relevant guidance in [.github/instructions](../../.github/instructions/)
so code review uses the same contracts. Keep these instructions short and link to
the authoritative documents rather than embedding their detailed rules.

Update [README.md](../../README.md) with setup and basic cache-command examples.
Update [README_ADVANCED.md](../../README_ADVANCED.md) with the shared admission,
prefill, and transport architecture and links to both reference directories. Update
[provider documentation](../providers/) for supported cache scope and limitations,
and [API support](../api_support.md) if endpoint or encryption behavior changes.
Keep configuration examples and CLI help consistent with the maintained command
reference. Give each cache document a clear scope and a link back to its entry point;
use stable descriptive headings and links to relevant source files and tests so an
agent can locate the implementation without reading this plan or discussion history.

### 9c. Phase requirements and completion checks

Update maintained references with each phase's actual contracts, tests, supported
inputs, and limitations. Planned options stay in this plan until implemented; do
not present them as usable commands in reference documentation. Provider references
must identify migration blockers rather than suggesting incomplete cache support.

| Phase | Documentation delivered or updated in the same commit |
| --- | --- |
| 0 | Establish `docs/cache/README.md` and `testing.md`, reproducible counting methodology, fixture links, and AGENTS.md discovery links. |
| 1 | Document shared service ownership and actual interfaces; cross-reference the transport contracts for scope, lifetime, publication, and invalidation. |
| 2 | Create/update `storage.md` for strict schema, identities, dependency resolution, supported-kind dispatch, trusted imports, bounds, and secure transactions. |
| 3 | Document NEAR software sharing, independent consumer evaluations, envelope handling, component completeness, and diagnostic retrieval behavior in storage/testing and provider references. |
| 4 | Document Intel/AMD material applicability, header-delivered dependencies, freshness/revocation rules, sharing, and regression tests. |
| 5 | Document Tinfoil release sets, TUF dependencies, matching-release discovery, direct/cloud scope, and the direct live-validation limitation. |
| 6 | Document NVIDIA JWKS trust and refresh rules, original retrieval-time handling, and retained report-bound NRAS work. |
| 7 | Document CT prefill across all client owners, bootstrap/refresh rules, retained live TLS checks, and counting coverage. |
| 8 | Publish implemented cache/serve/verify commands, common default path, target and failure behavior, `--no-cache`, read-only deployment, complete core YAML examples, and measured request methodology. Update setup/help/configuration and provider links. |
| 9 | Create/update `operator-decisions.md` for supported exact classes, prerequisites, effective policy, decision consumption, retained failures, and upgrade/withdrawal semantics. Update AGENTS.md and affected review instructions when exceptions become usable. |
| 10 | Publish interactive/proposal/withdrawal workflows, reasons, revision conflicts, partial/shared scope, deployment instructions, and replacement of old policy-edit inputs. Keep help and configuration examples consistent. |
| 11 | Document autocache opt-in, admission/export boundary, bounded asynchronous writes, errors, policy-preserving transactions, and restart budgets. |
| 12 | Document durable-state binding, mandatory transaction ordering, initialization, exclusive ownership and clean-shutdown eligibility, crash recovery, retention, and deployment rollback responsibility. State that restoration is not yet enabled. |
| 13 | Publish endpoint-persistence configuration, complete artifact/state examples, exclusions, restoration budgets, and failure behavior; update transport and provider references as the process-exit boundary changes. |
| Conditional extensions | Update provider capability matrices and per-class decision references with their implementing commits. Add newly supported examples, budgets, regression links, and security/review rules without changing the core contracts implicitly. |

Before completing the implementation, check that all references are reachable from
AGENTS.md and the repository entry points, links and test names resolve, YAML and CLI
examples match the implemented schema and flags, and each shared rule has one
authoritative home. Verify that the cache and transport descriptions agree on
restoration, lifetime, and invalidation. Keep measured run output in test artifacts;
the maintained docs describe contracts, methodology, and supported behavior.

At implementation completion, mark this plan as completed design context and link
to the maintained cache and transport entry points. Those references must be
sufficient for subsequent code changes without consulting the plan. Do not retain
unimplemented designs as statements of current behavior or turn the plan into a
running implementation/validation log.

## 10. Discussion sources and remaining policy work

Material research limitations and operator consequences are recorded in
[the planning issues](supply_chain_caching_issues.md). These include unsupported older-release
enumeration, bounded TUF snapshot reuse, ordinary-decision prerequisites, and clean-owner
endpoint restoration. They do not introduce additional CLI flags.


The following discussions provide source context; the requirements are specified
in this document:

- [#51](https://github.com/13rac1/teep/pull/51#issuecomment-4187937622): authenticate before caching; keep HTTP capture/replay separate.
- [#97](https://github.com/13rac1/teep/pull/97#issuecomment-4322899545): reduce repeated attestation work through caching and key reuse.
- [#118](https://github.com/13rac1/teep/issues/118#issuecomment-4984041862): provider-scoped repositories, signers, and operator decisions.
- [#127](https://github.com/13rac1/teep/pull/127#discussion_r3581356163): persist AMD certificate material and distinguish it from revocation freshness.
- [#134](https://github.com/13rac1/teep/issues/134): optional prompt-cache secret persistence, separate from public evidence.
- [#139](https://github.com/13rac1/teep/issues/139): measurement authority and Intel collateral enforcement prerequisites.
- [#140](https://github.com/13rac1/teep/issues/140): explicit policy authoring versus values authenticated under existing trust; noninteractive artifact deployment.
- [#143](https://github.com/13rac1/teep/pull/143#issuecomment-5387850387): avoid repeated GitHub retrievals and VPN/shared-IP rate limits. Latest-release enforcement is not a requirement of this plan.

Repository audit-status tracking and automatic advisory updates remain separate
work. The operator decision path is part of this plan. MR_SEAM and Intel collateral
exceptions must use the exact decision classes above, never an incidental weakening
of ordinary caching. Provider assertions of measurements require the
independent verification defined by the applicable policy.
