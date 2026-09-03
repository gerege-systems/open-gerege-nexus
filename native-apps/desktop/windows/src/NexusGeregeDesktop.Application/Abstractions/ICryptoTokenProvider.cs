using NexusGeregeDesktop.Domain.Primitives;
using NexusGeregeDesktop.Domain.Tokens;

namespace NexusGeregeDesktop.Application.Abstractions;

public interface ICryptoTokenProvider
{
    /// Stable identifier of this backend, e.g. "windows-cng" or "pkcs11:opensc".
    string Id { get; }

    /// Display name for UI.
    string DisplayName { get; }

    TokenBackend Backend { get; }

    /// True if this provider is available on the current host (DLL loaded,
    /// service running, etc.).
    bool IsAvailable { get; }

    Task<Result<IReadOnlyList<TokenInfo>>> ListTokensAsync(CancellationToken ct = default);

    /// Force the underlying driver to re-enumerate slots so hot-plugged
    /// tokens become visible. Default: no-op (most backends re-scan on
    /// every ListTokensAsync). PKCS#11 modules with a cached slot table
    /// (Feitian eps2003csp11.dll being the canonical offender) override
    /// this to perform a C_Finalize → C_Initialize cycle. Called by
    /// TokenRegistry.DiscoverAllAsync before each scan.
    Task RefreshAsync(CancellationToken ct = default) => Task.CompletedTask;

    /// Opens a session against a specific token. The pin is optional —
    /// Windows CNG ignores it (the OS shows a PIN dialog on first use);
    /// PKCS#11 uses it for C_Login.
    Task<Result<ITokenSession>> OpenSessionAsync(string tokenId, string? pin = null, CancellationToken ct = default);

    /// M9.D — initialize a blank or reset-needing token. Workflow:
    ///   1. C_InitToken(slot, soPin, label) — wipes the token, sets the
    ///      admin PIN and label. Factory-default SO PIN comes from the
    ///      vendor (Feitian ePass2003: `12345678`).
    ///   2. Open an SO session, C_Login(CKU_SO, soPin), C_InitPIN(userPin)
    ///      — sets the user PIN.
    /// After this returns Success, the next ListTokens reports
    /// TokenState.Initialized and the M9.D.4 keypair-generation flow
    /// is unblocked.
    ///
    /// DESTRUCTIVE: any existing keys/certs on the token are erased.
    /// The UI MUST confirm with the citizen before calling this.
    ///
    /// CNG path: not implemented — Microsoft Smart Card KSP exposes no
    /// admin-PIN-set API. Users on CNG must use the vendor tool
    /// (FT_ePass2003 Manager, GIDS-Tool, etc.) for the one-time init.
    Task<Result> InitializeTokenAsync(
        string tokenId,
        string soPin,
        string label,
        string newUserPin,
        CancellationToken ct = default);

    /// T8 — reset a locked user PIN using the SO (admin) PIN. PKCS#11
    /// flow: open R/W session → C_Login(CKU_SO, soPin) → C_InitPIN(new
    /// userPin). The on-card keys and certs are NOT touched, unlike
    /// InitializeTokenAsync which wipes everything. Citizens hit this
    /// after too many wrong PIN entries trip CKF_USER_PIN_LOCKED.
    /// CNG: no portable API; returns not_implemented_cng so the UI
    /// can route to the vendor tool.
    Task<Result> UnlockUserPinAsync(
        string tokenId,
        string soPin,
        string newUserPin,
        CancellationToken ct = default);

    /// T9 — rotate the SO (admin) PIN. PKCS#11 flow: open R/W session
    /// → C_Login(CKU_SO, oldSoPin) → C_SetPIN(oldSoPin, newSoPin). On-
    /// card keys / certs / user PIN are not touched. Use cases: shared
    /// device handover, suspected admin-PIN leak, periodic rotation.
    /// CNG: no portable API; returns not_implemented_cng.
    Task<Result> ChangeSoPinAsync(
        string tokenId,
        string oldSoPin,
        string newSoPin,
        CancellationToken ct = default);

    /// M9.D.5 — fingerprint enrollment on tokens with an on-board
    /// biometric sensor. Two completion modes:
    ///   ManagedInline  — the provider drove the sensor itself, the
    ///                    citizen's finger is captured + stored. Token
    ///                    is now biometrics-ready for C_Login(NULL).
    ///   DelegatedTool  — the provider can't drive the sensor (no SDK
    ///                    on the host). It returns instructions / a
    ///                    process the UI launches (e.g. the vendor's
    ///                    "BioPass Manager"). Citizen completes it
    ///                    out-of-band and clicks Refresh.
    /// On tokens without a sensor: returns BadRequest("no_sensor").
    Task<Result<BiometricEnrollOutcome>> EnrollFingerprintAsync(
        string tokenId,
        string userPin,
        CancellationToken ct = default);
}

/// Result of a biometric enrollment attempt.
public sealed record BiometricEnrollOutcome(
    BiometricEnrollMode Mode,
    /// Number of fingers successfully enrolled (e.g., 2 = primary +
    /// backup). 0 when Mode = DelegatedTool.
    int FingerCount,
    /// Vendor-supplied instructions (printable text) for the
    /// DelegatedTool path. Empty for the inline path.
    string Instructions);

public enum BiometricEnrollMode
{
    ManagedInline,
    DelegatedTool,
}

public interface ITokenSession : IAsyncDisposable, IDisposable
{
    TokenInfo Info { get; }

    bool IsLoggedIn { get; }

    Task<Result> LoginAsync(string pin, CancellationToken ct = default);

    Task<Result> LogoutAsync(CancellationToken ct = default);

    /// M9.E — change the user PIN. Caller supplies both the current
    /// PIN and the new one. Reasonable lengths (token-defined min/max
    /// — for ePass2003 the floor is 8 chars per the EnterSafe spec).
    /// PKCS#11: maps to C_SetPIN(session, oldPin, newPin) once the
    /// session is authenticated. CNG: no public API — caller is
    /// directed to the vendor's PKI tool.
    Task<Result> ChangePinAsync(string oldPin, string newPin, CancellationToken ct = default);

    Task<Result<IReadOnlyList<TokenObject>>> ListObjectsAsync(CancellationToken ct = default);

    /// Read a DER-encoded X.509 certificate stored on the token.
    Task<Result<byte[]>> ReadCertificateAsync(string objectId, CancellationToken ct = default);

    /// Sign a raw data buffer (the implementation hashes it per `algorithm`).
    /// For PSS / other padding modes, extend this contract.
    Task<Result<byte[]>> SignAsync(string keyId, byte[] data, SigningAlg algorithm, CancellationToken ct = default);

    /// Generate a key pair on the token. Returns the new private-key object.
    Task<Result<TokenObject>> GenerateKeyPairAsync(KeyParams parameters, CancellationToken ct = default);

    /// Import a DER-encoded X.509 certificate, associating it with the
    /// previously-generated private key (by `keyId`).
    Task<Result<TokenObject>> ImportCertificateAsync(byte[] derCert, string keyId, string? label, CancellationToken ct = default);

    /// M9.C — build a PKCS#10 CertificateRequest signed by the token's
    /// private key. Returns PEM with the "CERTIFICATE REQUEST" block.
    /// The subject CN is a placeholder (the backend CA overrides Subject
    /// DN with the citizen's registered identity) but a non-empty CN is
    /// required so the request parses cleanly server-side.
    Task<Result<string>> BuildCsrAsync(string keyId, string subjectCn, CancellationToken ct = default);

    /// M9.C — install a CA-issued cert so the token is usable for login
    /// going forward. CNG: writes to CurrentUser\My and lets the Smart
    /// Card KSP bind it to the on-card key via SubjectPublicKeyInfo.
    /// PKCS#11: writes a CKO_CERTIFICATE object with matching CKA_ID.
    Task<Result> InstallIssuedCertificateAsync(byte[] derCert, CancellationToken ct = default);

    /// T6 — destroy an on-token object by its id. PKCS#11 routes to
    /// C_DestroyObject. CNG: removes a cert from CurrentUser\My (for
    /// CKO_CERTIFICATE-like rows) or calls CngKey.Delete() for the
    /// matching container (for CKO_PRIVATE_KEY-like rows). Destructive
    /// and irreversible — callers MUST confirm with the citizen first.
    Task<Result> DeleteObjectAsync(string objectId, TokenObjectKind kind, CancellationToken ct = default);
}

/// Multiplexes all configured providers; the UI calls this rather than
/// individual providers.
public interface ITokenRegistry
{
    IReadOnlyList<ICryptoTokenProvider> Providers { get; }

    Task<Result<IReadOnlyList<DiscoveredToken>>> DiscoverAllAsync(CancellationToken ct = default);

    Task<Result<ITokenSession>> OpenAsync(string providerId, string tokenId, string? pin = null, CancellationToken ct = default);

    /// Routes a blank-token initialization request to the matching
    /// provider (PKCS#11 today, CNG returns not_implemented). See
    /// [ICryptoTokenProvider.InitializeTokenAsync] for semantics.
    Task<Result> InitializeTokenAsync(
        string providerId,
        string tokenId,
        string soPin,
        string label,
        string newUserPin,
        CancellationToken ct = default);

    /// Routes a SO-PIN-driven user PIN unlock. See
    /// [ICryptoTokenProvider.UnlockUserPinAsync] for semantics.
    Task<Result> UnlockUserPinAsync(
        string providerId,
        string tokenId,
        string soPin,
        string newUserPin,
        CancellationToken ct = default);

    /// Routes an SO PIN rotation. See
    /// [ICryptoTokenProvider.ChangeSoPinAsync] for semantics.
    Task<Result> ChangeSoPinAsync(
        string providerId,
        string tokenId,
        string oldSoPin,
        string newSoPin,
        CancellationToken ct = default);
}

public sealed record DiscoveredToken(ICryptoTokenProvider Provider, TokenInfo Info);
