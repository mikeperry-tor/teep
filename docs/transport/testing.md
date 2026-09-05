# Transport verification and provider migrations

Transport tests must establish authentication before transmission, isolation
between concurrent clients, and bounded resource use. Use TLS test servers and
the production encryption and decryption paths. When production clients retain
system WebPKI roots, use `testtls.RunWithFallbackRoot` and
`authority.NewTLSServer`; do not weaken production trust configuration for
tests.

## Required scenarios

| Behavior to preserve | Representative coverage |
| --- | --- |
| Reject SPKI mismatch before sending request bytes; enforce CT and WebPKI | `TestSPKIPinnedClientRejectsBeforeSendingRequest`, `TestSPKIPinnedClientRejectsModifiedTrust` in [pinned tests](../../internal/tlsct/pinned_test.go) |
| HTTP/2 physical bounds, concurrent overload rejection, and recovery after stream completion | `TestHTTP2ConcurrentStreamConnectionBound` in [connection tests](../../internal/tlsct/http2_limits_test.go) |
| HTTP/1.1 sequential reuse; closing one HTTP/2 stream preserves another | [Stream lifetime tests](../../internal/tlsct/stream_lifetime_test.go) |
| Provider, authority, and SPKI pool isolation | `TestAttestedPoolsRespectProviderAuthorityAndKey` in [pool tests](../../internal/proxy/tls_binding_internal_test.go) |
| Shared verification for the same authorization key, replacement generations, invalidation during verification, expiry, eviction, and blocked reports | [Authorization tests](../../internal/proxy/authorization_internal_test.go) |
| Authenticated expiry stops waiting for a connection, buffered response processing, and downstream writes | [Authorization wait tests](../../internal/proxy/authorization_wait_test.go), [response lifetime tests](../../internal/proxy/response_lifetime_test.go) |
| Exact rejection recognition, bounded parsing, body ownership, and unsupported endpoints | [Rejection tests](../../internal/provider/key_rejection_test.go) |
| Protocol failures do not cause replay after the transport consumes an encrypted POST body | `TestAuthorizedProtocolErrorNeverReplays` in [protocol tests](../../internal/proxy/http2_protocol_test.go) |
| Independent clients share HTTP/2 safely; a stale rejection cannot remove replacement authorization | [Authorized inference tests](../../internal/proxy/authorized_inference_test.go) |
| Cancellation retains authorization; the client authenticates encrypted errors; bodies close once; metrics remain separate for concurrent providers and models | [Failure and concurrency tests](../../internal/proxy/authorized_failure_test.go) |
| NEAR non-streaming read failures retain authorization; empty EHBP responses and partial frame headers fail without promoting E2EE success | [Response read failure tests](../../internal/proxy/authorized_read_failure_test.go) |
| Concurrent NEAR model misses retain distinct keys; old requests cannot delete or promote replacement generations | [NEAR authorization key tests](../../internal/proxy/authorization_near_keys_test.go) |
| Proxy and standalone request preparation and error authentication agree | [Preparation tests](../../internal/provider/inference_test.go), [standalone tests](../../internal/verify/e2ee_test.go) |
| Redirect targets receive zero requests; proxy omits `Location` | [Client redirect tests](../../internal/tlsct/redirect_test.go), [proxy redirect tests](../../internal/proxy/redirect_internal_test.go) |
| Environment proxy selection and connection setup budgets remain effective | [Common transport tests](../../internal/tlsct/pooled_test.go) |
| HTTPS proxies authenticate before CONNECT; origin pins reject before request transmission; HTTP/2 multiplexing and handshake budgets remain effective | [HTTPS proxy tests](../../internal/tlsct/pinned_proxy_test.go) |
| Explore does not claim E2EE when request encryption is disabled | [Explore authorization test](../../internal/proxy/explore_authorization_test.go) |
| The generic inference path rejects EHBP | [Generic encryption test](../../internal/proxy/generic_encryption_test.go) |
| SSE completion consumes trailing EHBP frames before reporting success | `TestAuthorizedEHBPStreamCompletion`, `TestStandaloneEHBPStreamCompletion`, and `TestReassembleSSECompletion` |
| Server cleanup closes provider clients; retry and capture wrappers forward cleanup | `TestServerCloseProviderConnections` and `TestAttestationClientCloseIdleConnections` |
| Concurrent report acquisition and promotion preserve ownership | `TestAuthorizationConcurrentReportOwnership` |
| Client cleanup reaches each wrapped connection pool | `TestWrappedClientClosesIdleConnections` in [transport tests](../../internal/tlsct/transport_test.go) |
| Concurrent NEAR clients and models use distinct backend keys and request content, separate provider pools, and fresh encryption sessions | [NEAR multiplexing tests](../../internal/proxy/near_http2_test.go) |
| Live NEAR attestation and inference use HTTP/2, the attested identity, sequential reuse, and overlapping encrypted requests | [NEAR HTTP/2 integration tests](../../internal/proxy/integration_near_http2_test.go) |

| Additional regression | Tests |
| --- | --- |
| TUF verification cancellation reaches headers, body reads, and subsequent downloads without canceling other operations | `TestTrustedRootVerificationCancellation` |
| Backend and gateway SEV expiry contributes only after successful verification; captured evidence supplies an expiry | `TestSEVReportEvidenceValidity`, `TestVerifyRun_Tinfoil_Fixture` |
| Delayed discovery callers reuse a newly published mapping; stale mappings still require refresh | `TestDiscoveryDelayedRefresh` in both NEAR direct and Tinfoil |
| Concurrent key rejections run one shared full online re-attestation, create fresh retry sessions, and preserve replacement authorization against a delayed rejection | `TestIntegration_NearDirectKeyRecovery`, `TestIntegration_NearCloudKeyRecovery`, `TestIntegration_TinfoilKeyRecovery` |
| Router verification is shared across models while report outcomes remain separate and bounded | `TestAuthorizationRouterSharesVerificationAcrossModels`, `TestAuthorizationRouterModelViewsBounded` |
| Attestation socket overload returns 503/backoff without negative caching and permits subsequent verification | `TestAuthorizationAttestationCapacityRecovery` |
| NEAR non-streaming reassembly bounds aggregate input and final content/tool-call output | `TestReassemblyInputLimit`, `TestReassemblyResponseLimit` |
| Non-streaming EHBP rejects oversize and bad trailing frames; speech keeps its media type | `TestAuthorizedEHBPResponseBoundary` |
| Explicit report authority selects only that cached scope without discovery | `TestAuthorizedReportLookup` |
| Malformed SSE retains authorization while authentication failures invalidate the used generation | `TestAuthorizedSSEFailureClassification` |
| Reassembly decrypts tool-call fields once through production NEAR cryptography | `TestReassemblyDecryptsToolCallsOnce` |

For each provider migration, add provider-specific coverage for route
discovery, the live attestation peer, separation of backend and gateway
identities where applicable, and supported endpoints. A shared transport test
does not prove that a provider supplies the correct identity or encryption
key. Positive integration cases must use the same factor policy as
`teep serve` and `teep verify`.

## Validation workflow

Run `make check` before committing. Transport and concurrency changes also
require `make integration`; major changes require `make reports`. Captured
fixtures provide deterministic provider verification, while live suites check
the deployed protocol. Live tests require their configured opt-in or API keys.
Do not weaken policy to make a provider pass. Record external failures and any
excluded suites in the PR rather than treating an incomplete run as success.

Run the transport, authorization, request preparation, and standalone tests
with the race detector on the minimum supported Go version and the other
versions in [CI](../../.github/workflows/ci.yml). The CI matrix is the
maintained version list. Also run the patched Intel verifier tests in
`third_party/go-tdx-guest/verify`.

For Go upgrades, explicitly retain tests of consumed POST errors, HTTP/2
stream saturation, physical socket accounting, connection waiting, and
cancellation. These protect assumptions about transport behavior that a
successful handshake or a single request cannot establish.

Update this reference when renaming or replacing tests. Keep assertions about
the required behavior. Remove checks for obsolete implementation details, such
as manual HTTP/1.1 writing or an independent SPKI cache.

`TestAuthorizedConnectionCapacityRetainsAuthorization` checks concurrent HTTP 503
responses without sending inference or invalidating shared authorization.
`TestRetryTransportRejectsCapacityWithoutRetry` and
`TestInferenceRetryClassification` exclude local capacity errors from retries.
