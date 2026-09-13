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
teep verify [existing target/options] [--cache-file PATH]
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
| `teep serve --autocache` | Automatically persist eligible evidence/results after successful admission through the shared runtime path. Writes are asynchronous; this creates no whitelist decisions or persisted endpoint authorizations. Without the flag, `serve` reads portable cache material without writing it. |

Cache path precedence is `--cache-file`, `$TEEP_CACHE_FILE`, configured `cache_file`,
then `~/.config/teep/cache.yaml`, identically for `cache`, `serve`, and `verify`.
All three load an existing default without requiring `--cache-file`; a missing
implicit default starts empty. `cache` and `serve --autocache` can create their
output file. An explicitly selected missing file is an error for `verify` and for
`serve` without `--autocache`.
Autocaching conflicts with a declared read-only cache. Noninteractive whitelist
updates require proposal generation or explicit apply; there is no implicit consent.

`teep verify` imports eligible portable evidence and operator decisions, performs
fresh endpoint admission, and never exports or updates cache state. It does not
restore persisted endpoint authorizations. There is no separate `--whitelist` input.
Remove `--update-config` and `--config-out` when implementing this command surface.
Existing `--force` is not a whitelist-selection or trusted-cache-generation option.
Optional endpoint persistence has separate eligibility requirements; this plan does
not assign it an additional CLI flag. See [commands and deployment](#5-commands-and-deployment)
and [operator decisions](#5b-operator-decision-command-and-reporting) for detailed
validation, partial-failure, proposal, and acknowledgement rules.

### Evidence and runtime baseline

An image must use an authenticated release consistent with its compose or
attestation binding, unless an explicit supported operator decision replaces a
particular requirement. It does not have to use the latest release. Ordinary
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
Source URLs and retrieval times are diagnostic metadata, not trust roots.

Verification records also belong to the evidence class. Each records:

- Exact evidence references and authenticated subject identifiers.
- The teep verifier/build identity that performed the checks.
- Effective verification policy identity, including trust roots, signer rules,
  applicable factor requirements, and explicit exemptions.
- Verification time, checks performed, results, and any admission-time validity
  information needed to explain or reevaluate the result.

Build identity must identify the actual verifier implementation, dependencies,
security-relevant build options, and local modifications. A human version string
alone is insufficient. Define deterministic canonical encodings for build and
policy identities before implementation. Never include API keys in those encodings.

### 2b. Verified subjects

A verified subject identifies an authenticated artifact, its permitted use under an
exact policy, and the verification record that established it. The verifier/build
identity is not duplicated in the verified subject: the loader follows the evidence
reference to enforce the upgrade boundary.

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
was used, reference it through `operator_decision_refs` and retain the failed base
check in the verification record. A subject admitted solely by a content pin belongs
to `operator_decisions`, not to a fabricated signature-verification result. Endpoint
authorizations can reference both verified subjects and applicable operator decisions.

The effective policy includes enabled operator decisions. Derive its identity from
the base policy and canonical decisions. A build change invalidates derived
verification results, but does not silently erase the operator's intent or make it
an unconditional override. The current verifier checks whether the decision kind,
scope, risk acknowledgement, and base-policy compatibility remain permitted. New
hard restrictions or unsupported decisions block affected reuse with a clear error;
do not reinterpret them as broader exemptions. Revalidating a decision is local
unless its documented evidence prerequisites require retrieval.

Decisions may be portable when their subjects are portable (image digest, signer,
measurement set). Endpoint-specific key/identity facts retain endpoint scope.
Removing a decision changes the effective policy and requires a deployment update
and restart to withdraw its runtime effect. Do not let concurrent merge or an older
cache restore a removed decision. Decisions belong to deployment-controlled policy
state; service write-back may add evidence, not resurrect policy entries.

### 2f. Provider and format capabilities

Cache support is a set of capabilities, not a provider-wide boolean. Select the
format from strictly parsed evidence, not a model-name list or cached assumption.
Include provider, evidence format, principal/tier, authenticated subject, and
applicable policy in verification-result reuse scope. Content-addressed original
bytes may be deduplicated without sharing policy conclusions. A format change
requires fresh evaluation of its evidence coverage and enforcement policy.

| Provider / format | Portable input prefill | Operator decisions | Unified runtime authorization / restoration |
| --- | --- | --- | --- |
| NearDirect / dstack | Model compose/images, eligible collateral | Applicable exact model software/measurement decisions | Existing shared runtime path; restoration requires the planned durable-state support. |
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
whose live-peer binding it does not establish.

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
| Inference / E2EE | Normal inference sends a request and establishes actual encryption usability. Standalone verification may issue its own probe. | Do not add a preliminary inference probe to `serve` merely to consume the cache. Count `teep cache`/`verify` probes separately from pre-inference metadata overhead. |

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
exit nonzero if any target failed. Failed targets must not gain or replace
verified subjects or operator decisions. A target is successful only after all
required checks under its effective policy are satisfied. Validate all target syntax before network activity or writes. Do not
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
`teep cache`. Without this option, `serve` does not write portable evidence. A declared read-only cache
supports image-layer deployment without attempted writes. Failed optional evidence write-back
leaves a separately completed in-memory verification valid, emits an error, and
must not claim persistence. This is distinct from mandatory durable invalidation
for restored endpoint authorization, whose failure blocks affected reuse.

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
policy and required verification steps; unsupported cryptographic failures still
block. Report selected models and observed endpoints without implying coverage of
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

Cache-aware verification is a migration acceptance requirement for Chutes and Venice.
Until migration, reject cache-backed verification of those providers explicitly;
ordinary live verification without cache material retains its existing behavior.
PhalaCloud and NanoGPT remain outside cache scope. Do not silently ignore a loaded
cache for an unsupported verification target or add legacy-cache adapters.

Remove `--update-config` and `--config-out` when implementing `teep cache`. Move
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

Reject `--autocache` with a declared read-only cache or an unusable destination at
startup, before accepting requests. Validate existing files strictly; the option
must not overwrite malformed or insecure input. A later write failure leaves an
independently verified in-memory authorization usable, emits a non-secret error,
and records that persistence failed. Use bounded retry with backoff, coalescing
repeated work; expose pending writes, failures, and the last successful write.
On orderly shutdown, attempt a bounded flush and report unfinished persistence.
A crash may lose pending optional evidence writes; atomic replacement must preserve
a valid committed file. Never claim that asynchronous enqueueing guarantees durability.

The option covers portable evidence/results, not complete endpoint authorizations.
Endpoint persistence requires its separately enabled durable-invalidation contract;
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
**Elevated** means a proposed candidate that additionally needs an exact risk-class
acknowledgement. Elevated support must remain disabled until its typed checks,
prerequisites, and tests exist; the generic flag does not enable it. **Unsupported**
means this command cannot turn the failure into a reusable decision. These are
requirements for the new decision path, not changes to existing `allow_fail` or
release/debug enforcement semantics.

| Observed condition / factor family | Exact decision and prerequisites | Class | Request effect on later admission |
| --- | --- | --- | --- |
| Unlisted repository: `component_recognition` | Add exact canonical repository within provider/tier; retain signature, signer, and attested-content checks. Repository recognition alone grants no signature trust. | Ordinary | Local list update; independently verified image evidence is still required. |
| New or changed component/provider signer: `provider_signer_recognition`, `component_signature_recognition` | Pin exact key fingerprint or OIDC issuer plus workflow identity, repository, and tier. Verify possession/signature and existing certificate chain independently; do not pin a mere name or arbitrary root. | Ordinary TOFU | Identity comparison becomes local; cached signature/transparency material eliminates retrieval only when otherwise sufficient. |
| Unlisted TDX MR_SEAM/MRTD/RTMR or SEV launch measurement: `tee_measurement`, allowlist portions of `tee_hardware_config` / `tee_boot_config` | Pin complete observed measurement tuple for the platform/provider/tier from an authenticated fresh quote. Retain debug, TCB, revocation, quote-signature, event-log, and REPORTDATA checks. Avoid combining unrelated register observations into an unobserved permitted configuration. | Ordinary TOFU | Local expected-value comparison. Does not eliminate PCS, NRAS, or fresh quote requests. |
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
| Invalid quote/image/JWT signature, untrusted hardware root, nonce mismatch, REPORTDATA/compose/event-log mismatch, key substitution, malformed evidence, unusable encryption, TLS/WebPKI/CT failure | No stable independently authenticated subject or required secure transport. New evidence or implementation/policy work is needed; an observed fingerprint alone does not repair binding. | Unsupported | No bypass or successful cache result. |

For missing provenance, distinguish absence from an invalid supplied signature.
The absence decision must not absorb a definitive cryptographic failure. For
measurement changes, separate authentication of the quote from trust in its measured
software: TOFU supplies the latter only. An authenticated but revoked subject is
not the same condition as a forged signature; inventory and diagnostics must retain
that distinction even where elevated support is not yet implemented.

### 5b. Operator decision command and reporting

The normal workflow selects concrete proposed changes, not decision-class names.
Keep the typed inventory internal to validation, serialization, and diagnostics;
operators do not need to learn it to select a firmware or repository change.

```sh
teep cache --model neardirect:example-model --update-whitelist
teep cache --all-models --update-whitelist
```

Fetch and verify current evidence before presenting choices. Show each eligible
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
selection controls discovery scope; it does not automatically accept failures.
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
success for each target being written.

Run ordinary checks first, then construct decisions only for selected eligible
failures. Rerun policy evaluation with the exact decisions without suppressing
unrelated failures. If any remaining enforced failure exists, write no decisions,
verified subjects, or endpoint authorization for that target; return nonzero with
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

The proposed top-level classes are `evidence`, `verified_subjects`,
`operator_decisions`, and optional `endpoint_authorizations`. Within `evidence`, `objects` hold original material and
`verifications` hold the records that checked it. Verified-subject IDs are local references;
security identity comes from validated subjects, evidence, and policy, not IDs.
These YAML objects are inputs to validated prefill adapters, not serialized runtime
Go objects or independent request-time caches. The examples' endpoint records map
to the shared immutable authorization type only after restoration eligibility is
established. `policy_id` refers to the canonical effective policy selected by the consumer, not
an arbitrary string that grants permission.

The following are separate illustrative fragments of one schema. Values inside
angle brackets are deliberately non-operational placeholders, including hashes,
keys, and encoded evidence. Referenced objects omitted for space must exist in a
real artifact. A loader must reject these fragments as incomplete, reject literal
placeholders, and require complete policy/check coverage. Repo names illustrate
current provider policy; the examples assert no particular deployed release.

### 6a. Near cloud

Gateway and model software have separate verified subjects. The model compose evidence
can be shared with NearDirect, but policy-specific verification and verified subjects stay
separate. `backend_tls_spki_sha256` describes attested backend evidence; it is not
the TLS peer of teep's connection to the gateway.

```yaml
schema_version: 1
evidence:
  objects:
    near_model_image:
      kind: sigstore_bundle
      content_sha256: "<bundle digest>"
      payload_base64: "<complete bundle, certificates, and transparency proof>"
      subject:
        repository: nearaidev/compose-manager
        digest: "sha256:<model image digest>"
    near_gateway_image:
      kind: sigstore_bundle
      content_sha256: "<gateway bundle digest>"
      payload_base64: "<complete gateway bundle>"
      subject:
        repository: nearaidev/cloud-api
        digest: "sha256:<gateway image digest>"
    near_model_compose:
      kind: compose
      content_sha256: "<exact compose digest>"
      payload_base64: "<complete attestation-bound compose bytes>"
    near_pcs:
      kind: intel_pcs_collateral
      content_sha256: "<collateral digest>"
      payload_base64: "<signed TCB info, QE identity, chains, and CRLs>"
      scope:
        fmspc: "<platform FMSPC>"
        issuer: "<authenticated collateral issuer>"
    near_cloud_admission:
      kind: endpoint_attestation
      content_sha256: "<admission evidence digest>"
      payload_base64: "<complete gateway and model evidence, including nonce context>"
  verifications:
    cloud_model_check:
      verifier_build: "sha256:<teep build identity>"
      policy_id: "sha256:<nearcloud model software policy>"
      verified_at: "2026-09-13T00:00:00Z"
      evidence_refs: [near_model_image, near_model_compose]
      checks: {image_signature: pass, signer_identity: pass, transparency: pass}
      exemptions: []
    cloud_gateway_check:
      verifier_build: "sha256:<teep build identity>"
      policy_id: "sha256:<nearcloud gateway software policy>"
      verified_at: "2026-09-13T00:00:00Z"
      evidence_refs: [near_gateway_image]
      checks: {image_signature: pass, signer_identity: pass, transparency: pass}
      exemptions: []
verified_subjects:
  cloud_model_image:
    kind: image
    provider: nearcloud
    tier: model
    subject:
      repository: nearaidev/compose-manager
      digest: "sha256:<model image digest>"
    policy_id: "sha256:<nearcloud model software policy>"
    verification_ref: cloud_model_check
  cloud_gateway_image:
    kind: image
    provider: nearcloud
    tier: gateway
    subject:
      repository: nearaidev/cloud-api
      digest: "sha256:<gateway image digest>"
    policy_id: "sha256:<nearcloud gateway software policy>"
    verification_ref: cloud_gateway_check
endpoint_authorizations:
  near_cloud_endpoint:
    persisted_id: "<stable deployment authorization identity>"
    deployment_id: "<local deployment identity>"
    provider: nearcloud
    model: example-model
    authority: cloud-api.near.ai
    policy_id: "sha256:<complete endpoint admission policy>"
    verification_ref: "<complete endpoint verification record>"
    evidence_refs: [near_cloud_admission, near_pcs]
    verified_subject_refs: [cloud_model_image, cloud_gateway_image]
    report_ref: "<complete admission report object>"
    identity:
      tls_spki_sha256: "<gateway SPKI digest>"
      backend_tls_spki_sha256: "<attested model backend SPKI digest>"
      model_ed25519_public_key: "<32-byte model key as hex>"
```

The complete endpoint record must cover both attestations and every required
compose component, not only the two example verified image subjects. NearCloud's model
key also determines `X-Model-Pub-Key`; do not persist a separate header key with a
different lifetime. Validate its X25519 conversion through the production pathway.

### 6b. Near direct

The same original image bundle can support a NearDirect verification result without another
retrieval. Its verification record must match NearDirect's policy. The selected
authority and attested key are endpoint facts, not properties of that image.

```yaml
schema_version: 1
evidence:
  objects:
    near_model_image:
      kind: sigstore_bundle
      content_sha256: "<same bundle digest as the cloud example>"
      payload_base64: "<same complete image bundle>"
      subject:
        repository: nearaidev/compose-manager
        digest: "sha256:<same model image digest>"
    direct_compose:
      kind: compose
      content_sha256: "<direct compose digest>"
      payload_base64: "<complete direct compose bytes>"
    direct_admission:
      kind: endpoint_attestation
      content_sha256: "<fresh direct admission evidence digest>"
      payload_base64: "<complete direct quote, GPU evidence, and client nonce context>"
  verifications:
    direct_image_check:
      verifier_build: "sha256:<teep build identity>"
      policy_id: "sha256:<neardirect software policy>"
      verified_at: "2026-09-13T00:00:00Z"
      evidence_refs: [near_model_image]
      checks: {image_signature: pass, signer_identity: pass, transparency: pass}
      exemptions: []
    direct_compose_check:
      verifier_build: "sha256:<teep build identity>"
      policy_id: "sha256:<neardirect software policy>"
      verified_at: "2026-09-13T00:00:00Z"
      evidence_refs: [direct_compose, near_model_image]
      checks: {compose_policy: pass, required_image_coverage: pass}
      exemptions: []
verified_subjects:
  direct_image:
    kind: image
    provider: neardirect
    tier: model
    subject:
      repository: nearaidev/compose-manager
      digest: "sha256:<same model image digest>"
    policy_id: "sha256:<neardirect software policy>"
    verification_ref: direct_image_check
  direct_compose:
    kind: compose
    provider: neardirect
    tier: model
    subject:
      compose_sha256: "<direct compose digest>"
    policy_id: "sha256:<neardirect software policy>"
    verification_ref: direct_compose_check
    image_verified_subject_refs: [direct_image, "<every other required verified image subject>"]
endpoint_authorizations:
  near_direct_endpoint:
    persisted_id: "<stable deployment authorization identity>"
    deployment_id: "<local deployment identity>"
    provider: neardirect
    model: example-model
    authority: example-model-i7.completions.near.ai
    policy_id: "sha256:<complete endpoint admission policy>"
    verification_ref: "<complete endpoint verification record>"
    evidence_refs: [direct_admission]
    verified_subject_refs: [direct_compose]
    report_ref: "<complete admission report object>"
    identity:
      tls_spki_sha256: "<selected backend SPKI digest>"
      model_ed25519_public_key: "<32-byte model key as hex>"
```

The compose check above requires all referenced components in a real file.
Neither this example authority nor a saved index bypasses the
[NEAR selection contract](../providers/near/near_attestation.md#neardirect-backend-selection).
No gateway identity or `X-Model-Pub-Key` hint is added to NearDirect.

### 6c. Tinfoil cloud and direct

The cloud example verifies a signed router release and binds its measurement to
SEV-SNP admission. The direct example uses a model enclave release and signed
hardware reference material. A real direct endpoint's required CPU evidence depends
on its attested platform; do not assume every direct endpoint is TDX.

```yaml
schema_version: 1
evidence:
  objects:
    router_release:
      kind: sigstore_bundle
      content_sha256: "<router bundle digest>"
      payload_base64: "<complete signed router release and measurement predicate>"
      subject:
        repository: tinfoilsh/confidential-model-router
        digest: "sha256:<router release subject digest>"
      release_tag: "<authenticated release label; no latest requirement>"
    hardware_registry:
      kind: sigstore_bundle
      content_sha256: "<registry bundle digest>"
      payload_base64: "<signed platform measurement registry>"
      subject:
        repository: tinfoilsh/hardware-measurements
        digest: "sha256:<registry subject digest>"
    model_release:
      kind: sigstore_bundle
      content_sha256: "<model release bundle digest>"
      payload_base64: "<complete signed model release and measurement predicate>"
      subject:
        repository: tinfoilsh/confidential-example-model
        digest: "sha256:<model release subject digest>"
    router_vcek:
      kind: amd_vcek
      content_sha256: "<certificate digest>"
      payload_base64: "<complete DER VCEK certificate>"
      scope:
        product: "<attested supported AMD product>"
        hwid: "<chip hardware identity>"
        reported_tcb: "<TCB extensions matched to report>"
      source: https://kds-proxy.tinfoil.sh
    router_admission:
      kind: endpoint_attestation
      content_sha256: "<router admission evidence digest>"
      payload_base64: "<complete nonce-bound router SEV-SNP evidence>"
  verifications:
    router_release_check:
      verifier_build: "sha256:<teep build identity>"
      policy_id: "sha256:<tinfoil cloud software policy>"
      verified_at: "2026-09-13T00:00:00Z"
      evidence_refs: [router_release]
      checks: {release_signature: pass, signer_identity: pass, transparency: pass}
      exemptions: []
    direct_release_check:
      verifier_build: "sha256:<teep build identity>"
      policy_id: "sha256:<tinfoil direct software policy>"
      verified_at: "2026-09-13T00:00:00Z"
      evidence_refs: [model_release, hardware_registry]
      checks: {release_signature: pass, signer_identity: pass, transparency: pass}
      exemptions: []
verified_subjects:
  router_release:
    kind: release
    provider: tinfoil_v3_cloud
    tier: gateway
    subject:
      repository: tinfoilsh/confidential-model-router
      digest: "sha256:<router release subject digest>"
    policy_id: "sha256:<tinfoil cloud software policy>"
    verification_ref: router_release_check
  direct_release:
    kind: release
    provider: tinfoil_v3_direct
    tier: model
    subject:
      repository: tinfoilsh/confidential-example-model
      digest: "sha256:<model release subject digest>"
    policy_id: "sha256:<tinfoil direct software policy>"
    verification_ref: direct_release_check
endpoint_authorizations:
  tinfoil_router:
    persisted_id: "<stable router authorization identity>"
    deployment_id: "<local deployment identity>"
    provider: tinfoil_v3_cloud
    authority: inference.tinfoil.sh
    policy_id: "sha256:<complete cloud admission policy>"
    verification_ref: "<complete router admission verification record>"
    evidence_refs: [router_admission, router_vcek, router_release]
    verified_subject_refs: [router_release]
    report_ref: "<complete router admission report object>"
    identity:
      tls_spki_sha256: "<router SPKI digest>"
      hpke_public_key: "<32-byte router HPKE key as hex>"
  tinfoil_direct:
    persisted_id: "<stable direct authorization identity>"
    deployment_id: "<local deployment identity>"
    provider: tinfoil_v3_direct
    model: example-model
    authority: "<resolved Tinfoil backend authority>"
    policy_id: "sha256:<complete direct admission policy>"
    verification_ref: "<complete direct admission verification record>"
    evidence_refs: [model_release, hardware_registry, "<fresh direct evidence object>"]
    verified_subject_refs: [direct_release]
    report_ref: "<complete direct admission report object>"
    identity:
      tls_spki_sha256: "<backend SPKI digest>"
      hpke_public_key: "<32-byte backend HPKE key as hex>"
```

There is intentionally no model field on `tinfoil_router`. It cannot be used as an
verification of a particular backend model's code. No example caches `e2ee_usable` as
an admission fact. VCEK scope and signed release predicates must be verified from
original bytes; the descriptive YAML fields are not substitutes for those checks.

### 6d. Explicit operator measurement decision

This fragment shows goal 2 for an authenticated NearDirect measurement that the
base policy does not list. It records the failed base check and the operator's
pin separately. It does not claim that the base policy or every endpoint check
passed. The effective policy identity incorporates this exact decision.

```yaml
schema_version: 1
evidence:
  objects:
    observed_direct_quote:
      kind: endpoint_attestation
      content_sha256: "<complete observed evidence digest>"
      payload_base64: "<fresh quote, chain, event log, and client nonce context>"
  verifications:
    observed_measurements:
      verifier_build: "sha256:<teep build identity>"
      policy_id: "sha256:<base policy identity>"
      verified_at: "2026-09-13T00:00:00Z"
      evidence_refs: [observed_direct_quote]
      checks:
        quote_signature: pass
        certificate_chain: pass
        client_nonce: pass
        reportdata_binding: pass
        measurement_allowlist: fail
      failure_codes: [measurement_not_listed]
      exemptions: []
operator_decisions:
  direct_measurement_pin:
    kind: measurement
    provider: neardirect
    tier: model
    base_policy_id: "sha256:<base policy identity>"
    evidence_refs: [observed_direct_quote]
    observation_verification_ref: observed_measurements
    decision: pin_observed_value
    replaces_failure: measurement_not_listed
    subject:
      platform: intel_tdx
      measurements:
        mrseam: "<observed MR_SEAM>"
        mrtd: "<observed MRTD>"
        rtmr0: "<observed RTMR0>"
        rtmr1: "<observed RTMR1>"
        rtmr2: "<observed RTMR2>"
        rtmr3: "<observed RTMR3>"
    decided_at: "2026-09-13T00:01:00Z"
    reason: "Operator accepts the observed deployment measurement change."
    risk_acknowledgements: []
verified_subjects: {}
```

A complete artifact additionally records successful checks under the effective
policy and any resulting endpoint authorization. The empty verified-subject set
above emphasizes that this decision is not an image-signature result. The decision
replaces only the selected measurement-list failure. Fresh nonce/quote and
REPORTDATA checks, hardware safety requirements, TCB, revocation, NRAS, and other
required factors retain their existing behavior and request costs.

### 6e. Venice ACI/1 portable gateway inputs

This post-migration illustrative fragment contains no endpoint authorization. The verification
record describes a gateway compose/repository check, not image-signature or backend
verification. A complete file must include every required component and admission
record; placeholders have the same non-operational meaning as the other examples.

```yaml
schema_version: 1
evidence:
  objects:
    venice_gateway_compose:
      kind: compose
      content_sha256: "<exact gateway compose digest>"
      payload_base64: "<complete gateway compose bytes>"
    venice_gateway_quote:
      kind: endpoint_attestation
      content_sha256: "<ACI gateway evidence digest>"
      payload_base64: "<complete quote, nonce context, and RTMR event log>"
    venice_key_custody:
      kind: aci_key_custody
      content_sha256: "<keyset and custody evidence digest>"
      payload_base64: "<complete original keyset and custody signature chains>"
    venice_pcs:
      kind: intel_pcs_collateral
      content_sha256: "<signed collateral digest>"
      payload_base64: "<complete eligible collateral and chains>"
  verifications:
    venice_compose_check:
      verifier_build: "sha256:<teep build identity>"
      policy_id: "sha256:<Venice ACI gateway software policy>"
      provider: venice
      evidence_format: aci/1
      tier: gateway
      verified_at: "2026-09-13T00:00:00Z"
      evidence_refs: [venice_gateway_compose, venice_gateway_quote]
      checks:
        compose_binding: pass
        repository_policy: pass
      exemptions: []
verified_subjects:
  venice_gateway_configuration:
    kind: compose
    provider: venice
    evidence_format: aci/1
    tier: gateway
    subject:
      compose_sha256: "<exact gateway compose digest>"
    policy_id: "sha256:<Venice ACI gateway software policy>"
    verification_ref: venice_compose_check
    components:
      - repository: ghcr.io/redpill-ai/private-ai-launcher
        digest: "sha256:<compose-pinned image digest>"
        provenance: compose_binding_only
      # Every remaining compose component is required in a real artifact.
operator_decisions: {}
```

`venice_key_custody` is retained evidence, not a verified result in this fragment.
The parser for that proposed evidence kind must be added explicitly. A new admission
still authenticates the gateway and binds its current compose to the cached subject.
No model CPU/software success, independently authenticated downstream TLS identity,
or durable endpoint restoration can be inferred from this file.

## 7. Storage, parsing, and concurrency

Use bounded, strict YAML decoding. Reject unknown or missing required fields,
duplicates, null required values, invalid identifiers, unsupported schema versions,
multiple documents, cycles, aliases, and ambiguous references. Bound file size,
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

Use immutable snapshots for published entries. Keep mutable state on constructed
stores, not package globals. Bound entries and concurrent verification work.
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

An optional evidence write failure can leave verified memory state intact. A
mandatory persistence failure cannot be converted to success. Logs identify target,
check, and failure without API keys, inference content, or private key material.

## 8. Implementation phases and required coverage

Each implementation phase requires its own commit and `make check`. Major runtime
changes also require `make integration` and `make reports`; positive integration
coverage uses production factor enforcement. Apply the Section 1 Tinfoil direct
live-validation prerequisite to affected integration/report runs: retain all
unblocked checks, record deferred direct coverage in the commit description, and
complete that coverage after the upstream fix is deployed. Do not claim complete
direct live validation while it remains blocked. This section specifies future work,
not validation logs or implementation status. Each phase must update the maintained
documentation for the behavior it establishes, as specified in
[documentation requirements](#9-maintained-documentation-and-agent-discovery).

### Provider migration acceptance (separate prerequisite work)

For Chutes and Venice, require immutable route/identity scope, atomic report/key
publication, shared bounded verification, generation-safe invalidation, and concurrent
request acquisition through the common authorization interfaces. Test live-populated
state before adding disk prefill. Verify actual HTTP/2 negotiation and multiplexing
under production TLS/CT requirements; streaming success alone is insufficient.
Preserve provider-specific evidence gaps and existing factor enforcement. Cover
key changes, eviction, failed requests racing replacement, and unrelated streams.
Require cache-aware `verify` to use the same admission interfaces and policy outcomes.
Chutes additionally requires the nonce rules in Section 4; Venice requires both
format scopes and custody admission rules. Completion enables capability evaluation,
not automatic endpoint persistence. No cache phase may implement a legacy-cache
adapter to work around an incomplete migration.

### Phase 0: Request accounting and typed decision reasons

Add the provider/format scenario count suite described in Section 4c. Confirm exact
request counts including dependency traffic and capture preparation costs separately.
Expose typed failure reasons for the whitelist inventory, distinguishing unlisted
values, missing evidence, invalid signatures, expiry, and authenticated revocation.
Record the supported per-class prerequisites and count expectations in tests. No
class may inherit a factor-wide override merely because the factor has several
failure modes.

### Phase 1: Shared data management and portable schema

Identify and extract reusable admission and authorization-management interfaces
from the existing HTTP/2 attestation path. Keep provider scope and key-use lifetimes
in that shared layer. Define validated prefill and immutable export operations before
adding command-specific orchestration. Define evidence objects, verification records,
verified subjects, operator decisions, and canonical identities,
strict parsing, trusted import rules, and bounded storage. Share existing production
verification functions; do not serialize structs as a substitute for designing the
trust boundary. Test malformed input, forged result flags, dangling references,
content mismatch, policy mismatch, exemption mismatch, and cross-provider isolation.

### Phase 2: `teep cache` and portable reuse

Build the ordinary cache command on shared collection/verification and snapshot
export. Add disk import/prefill adapters, multi-provider targets, atomic merging, and
read-only deployment mode. Start with image/compose verified subjects, signed measurement
registries, AMD certificates, and Intel collateral. Define Proof of Cloud and NVIDIA
reference-material contracts before extending portable reuse. Update the config and
CLI migration and examples together. Test partial failure, complete image coverage,
concurrent writers, bounded retrieval sharing, cancellation, and write failures.
Use the common cache path resolver and strict loader in all three commands. Test
identical flag/environment/config/default precedence, default-file discovery,
missing implicit versus explicit paths, insecure/malformed input, and read-only
verification that never creates or modifies a file.
Implement `serve --autocache` through the same snapshot/export transaction. Test
changed compose/image and Tinfoil release evidence, rejection of incomplete
admissions, retained permitted failures, and absence of automatic whitelist edits.
Test concurrent explicit-command/service writers, bounded queue saturation,
coalescing, deletion racing a snapshot, read-only conflicts, destination creation,
startup validation, runtime write failure/recovery, shutdown flush, and crash-safe
replacement. Prove that slow or failed optional writes do not block independently
authorized inference and that mandatory endpoint invalidation retains its rules.
Initially enable Near and Tinfoil only. Add Chutes and Venice inputs through these
same services after their separate migration acceptance; otherwise retain explicit
unsupported-target diagnostics and proceed without them. Test target rejection for
unmigrated providers and for PhalaCloud/NanoGPT, including multi-provider requests.

### Phase 3: Admission integration and upgrade behavior

Connect prefill to the existing admission and authorization acquisition used by
Tinfoil and Near; do not introduce a parallel request path. Add equivalence tests
showing live-populated and disk-prefilled state yield the same scope checks, policy
outcomes, immutable report/key bindings, and invalidation behavior. Cover a prefill
racing live publication, eviction, shutdown, and generation replacement. Test that
two replicas reuse software verification results without duplicate retrieval while independently obtaining
fresh endpoint evidence. Test different builds and policies reevaluating retained
evidence locally, retrieving only missing/ineligible material, and rejecting
withdrawn subjects. Test an older authenticated release passing exact binding,
a newer unbound release failing, and tag-only references not claiming digest binding.
Test that another report's NRAS result cannot authorize a new nonce.

Keep the current runtime key-use lifetime and generation-safe invalidation. Cover
NearCloud gateway/backend separation, NearDirect indexed routes, Tinfoil direct
multiple authorities, and Tinfoil cloud router sharing with model-specific outcomes.
Use TLS test servers and production cryptography for connection and encryption tests.
Assert the request budgets in Section 4c, including local upgrade reevaluation with
network access to cached software groups denied.
For autocaching, measure the first live admission, then restart from the committed
file and assert the same eligible retrieval savings as explicit preparation.
Count background disk work separately; enabling autocaching must not introduce
additional discovery, release polling, or verification network requests.
Integrate cache-aware `verify` through shared admission services. Test fresh nonce
admission despite persisted endpoint records, matching and unused decisions,
remaining enforced failures, effective-policy equivalence, eligible retrieval
savings, exact artifact/build/policy reporting, concurrent file replacement, and
unsupported-provider rejection. Verify must never persist newly fetched material.

After Venice migration, extend [ACI integration coverage](../../internal/integration/venice_aci_test.go)
and [concurrent-format coverage](../../internal/integration/venice_concurrent_formats_test.go)
with prefill and request-count cases. Test weaker gateway compose provenance,
unbound source metadata, failed custody/app-ID/KMS checks, expired keysets, missing
model evidence, format changes, and rejection of endpoint restoration. Verify that
prefill does not turn any exempted model failure into success or skip a fresh nonce.
Use [keyset regression tests](../../internal/provider/venice/keyset_test.go) for the
production custody pathway. After Chutes migration, add prefill/live equivalence,
instance/key isolation, nonce-pool concurrency, and request-count coverage using
production encryption. Assert that copied cache files cannot restore request nonces.

### Phase 4: Operator decision path

Enable decisions only for providers whose shared-runtime migration is complete.
After Chutes migration, test exact MRTD/MRSEAM decisions together, retained unrelated
allowed failures, and rejection of signature/nonce failures as pin candidates.
Implement `--update-whitelist` for the ordinary inventory classes with exact target
and concrete-change selection, reasons, diagnostics, and atomic per-target writes. Add elevated
classes individually only after their typed prerequisites and risk acknowledgement
are specified. Keep unsupported and unimplemented classes rejected. Update the
security/review instructions with the explicit exception mechanism when implementing
it; do not silently expand existing `allow_fail` or debug-force behavior.

Test that a selected base-policy violation can produce a usable exact decision,
unselected failures block output, empty/broad selections fail, unsupported crypto
failures never become pins, and observations cannot broaden the decision. Cover
provider/tier separation, cumulative decisions, decision removal, trusted deployment,
upgrade compatibility, and concurrent service write-back. Assert retained failed
base checks and decision references in reports. Use request counters to prove that
only the selected replacement checks eliminate retrievals; test separately the
creation cost and subsequent reuse cost for each decision class.
Test interactive selection/cancellation and final confirmation, unselectable
unsupported failures, per-change elevated acknowledgement, and noninteractive
invocation without explicit apply. Cover proposal generation without trust writes,
strict parsing, tampered evidence, changed subject/failure/scope, policy/build
incompatibility, conflicting targets, and exact reviewed selections surviving apply
without silent substitution. Test all-model interactive and proposal flows, bulk
ordinary selection, shared-subject deduplication without scope broadening,
per-target partial failure, and model discovery changing after proposal generation.
Keep prompts and proposal output free of credentials
and inference content.

### Phase 5: Optional endpoint persistence

Implement endpoint restoration through the shared authorization constructor,
publication, acquisition, and invalidation paths. Enable it only with durable
invalidation and crash recovery defined and tested. Test that request handlers
cannot distinguish source in authorization behavior, except for diagnostics and
request counts.
Cover same-build restoration, policy/build-change rejection of direct reuse,
missing state, read-only operation, eviction, stale cache deployment, failed writes,
and crashes at each persistence boundary. Verify that an old request cannot delete
a replacement, a failed key cannot reappear after restart, and unrelated HTTP/2
streams remain usable. Update the maintained transport and provider references in
the implementation change that extends the process-exit boundary.

## 9. Maintained documentation and agent discovery

Create a maintained `docs/cache/` reference directory alongside `docs/transport/`.
The cache reference covers evidence persistence, operator decisions, command use,
and deployment as well as transport integration. Organize files around the changes
an agent needs to make, with a small entry point and focused contract documents.
The paths below are planned files; add working links when the files are created.

| Document | Authoritative content |
| --- | --- |
| `docs/cache/README.md` | Entry point: purpose, terminology, architecture, both prefill levels, command/configuration reference, deployment modes, and links to detailed contracts and implementation entry points. |
| `docs/cache/storage.md` | Evidence, verification-record, verified-subject, and operator-decision schemas; build/policy identity; validated import/prefill and immutable export; file integrity; atomic writes; concurrency; upgrade compatibility; restoration eligibility and durable invalidation storage. Include representative YAML for NearCloud, NearDirect, and both Tinfoil modes. |
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

- **Phase 0:** Introduce `docs/cache/testing.md` with reproducible request accounting
  and links to the count suite. Establish the cache entry point and agent-discovery
  links as soon as the first maintained reference exists.
- **Phase 1:** Document shared data-management ownership and the schema/import/export
  contracts in the cache entry point and storage reference. Link to the transport
  contract and the actual shared implementation interfaces.
- **Phase 2:** Document implemented command/configuration behavior, portable material,
  read-only deployment, merge semantics, and valid YAML examples. Update setup docs,
  CLI help, configuration examples, and provider links in the same change. Document
  shared default-path resolution for `cache`, `serve`, and `verify`, read-only
  cache-aware verification and rollout reports, plus `serve --autocache`, its opt-in
  behavior, path creation, asynchronous durability,
  failure diagnostics, retained policy failures, and endpoint-persistence exclusion. Include Chutes and Venice provider references with explicit migration blockers and post-migration capability scope.
- **Phase 3:** Document prefill integration, local upgrade reevaluation, remaining
  network requests, and provider-specific scope. Add bidirectional transport/cache
  links and regression-test references.
- **Phase 4:** Publish the implemented operator-decision inventory, command examples,
  diagnostics, retained checks, and request effects. Update AGENTS.md and affected
  review instructions with the supported exception mechanism.
- **Phase 5:** Update storage, transport, and provider references together when
  endpoint persistence changes the process-exit boundary. Document durable-state
  requirements, restoration exclusions, failure behavior, and crash-recovery tests.

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
