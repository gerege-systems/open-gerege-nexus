namespace NexusGeregeDesktop.Domain.Tokens;

/// Logical provider behind a token. Both flow through ICryptoTokenProvider
/// but have different unlock / PIN-handling semantics.
public enum TokenBackend
{
    WindowsCng,   // Microsoft Base Smart Card Crypto Provider — Windows shows PIN dialog
    Pkcs11,       // OpenSC opensc-pkcs11.dll, Feitian eps2003csp11.dll, etc. — we call C_Login(pin)
}

/// Information about a connected token / smart card slot.
public sealed record TokenInfo(
    string Id,                  // Stable identifier — for CNG: container name; for PKCS#11: slotId
    string Label,               // Human-readable label burned into the token (or "Unknown")
    string Manufacturer,
    string SerialNumber,
    string Model,
    TokenBackend Backend,
    bool Removable,
    bool HardwareSlot,          // false for soft tokens
    bool IsLoggedIn,
    TokenState State = TokenState.Unknown,
    TokenCapabilities Capabilities = default,
    // SHA-256 of the primary on-card public key modulus, hex-encoded.
    // Set ONLY when the provider can read it cheaply at discovery time
    // without forcing a login (PKCS#11: CKA_MODULUS is a public
    // attribute; CNG: X509Certificate2.GetRSAPublicKey().Modulus). Used
    // by TokenRegistry to dedupe entries that represent the SAME
    // physical card visible through two backends (e.g. a Feitian
    // ePass2003 that appears in both eps2003csp11.dll and the Windows
    // Smart Card KSP after cert propagation). Null is fine — non-RSA
    // tokens, blank tokens, and pre-enrollment cards have no usable
    // fingerprint, so dedup just skips them.
    string? PublicKeyFingerprint = null);

/// Lifecycle state of the token's secure store. Drives whether the
/// desktop should show "first-time setup", "enroll cert", or "ready to
/// sign in" affordances.
///
///   Blank        — no keys, no certs, no user PIN. Needs full setup
///                  (SO PIN init → user PIN → keypair → CSR → enroll).
///   Initialized  — SO PIN + user PIN set, but no usable signing key
///                  yet. Needs keypair gen + CSR + enroll.
///   Provisioned  — user PIN set AND a usable keypair on the token,
///                  but the cert is self-signed or absent. M9.C path:
///                  CSR + enroll using the existing key. (This is the
///                  "Test Mongolia Sign" factory state.)
///   Enrolled     — keypair + CA-issued cert linked. Login-ready.
///   Unknown      — discovery couldn't classify (unsupported card,
///                  vendor middleware missing, etc.). UI surfaces a
///                  diagnostic message rather than guessing.
public enum TokenState
{
    Unknown,
    Blank,
    Initialized,
    Provisioned,
    Enrolled,
}

/// Capability flags surfaced by discovery so the UI can show/hide
/// flows that the underlying hardware can't actually perform. Each
/// flag is a fact about the inserted token, not a permission — the
/// app still gates actions on user PIN entry, server enrollment etc.
public readonly record struct TokenCapabilities(
    /// Reports CKF_PROTECTED_AUTHENTICATION_PATH (PKCS#11) — the token
    /// has its own input device (PIN pad, fingerprint sensor) so the
    /// app does NOT prompt for a PIN; C_Login(NULL) is used and the
    /// hardware confirms the user via that path.
    bool ProtectedAuthPath,
    /// Token advertises a biometric sensor (vendor-specific quirk —
    /// inferred from model string until a clean PKCS#11 / CNG signal
    /// exists). When true, the first-time-setup wizard offers a
    /// "Enroll fingerprint" step before keypair generation.
    bool BiometricSensor,
    /// SO PIN (admin PIN) has been initialized. False on factory-fresh
    /// cards; once C_InitToken runs, true.
    bool SoPinInitialized,
    /// User PIN has been set. Required before any C_Login(USER, pin)
    /// or any CNG signing op against the card.
    bool UserPinInitialized,
    /// Token can generate keys on-board (CKM_RSA_PKCS_KEY_PAIR_GEN
    /// advertised, or the CNG provider permits CngKey.Create).
    bool CanGenerateKeyPair,
    /// Reflects CK_TOKEN_INFO.flags PIN-state bits. PKCS#11 doesn't
    /// expose an exact retry count, only thresholds: "low" (warning),
    /// "final try" (one attempt remaining), and "locked". CNG path
    /// can't read these — all default to false, surfacing as an
    /// unknown-but-OK status.
    TokenPinStatus UserPinStatus = TokenPinStatus.Ok,
    TokenPinStatus SoPinStatus = TokenPinStatus.Ok,
    /// Total private-object storage on the token, in bytes. 0 when the
    /// backend can't read it (CNG path, vendor middleware that reports
    /// CK_UNAVAILABLE_INFORMATION). PKCS#11 tokens generally fill at
    /// least one of TotalPrivate / TotalPublic.
    ulong MemoryTotalBytes = 0,
    /// Free private-object storage on the token, in bytes. Same caveat
    /// as MemoryTotalBytes — read only when the underlying API exposes
    /// it. UI hides the row entirely when both Total and Free are 0.
    ulong MemoryFreeBytes = 0);

/// Threshold reported by PKCS#11 for the user / SO PIN counter. Order
/// matters — higher values = more dangerous state. UI maps these to
/// colored badges (green / amber / red / black) so the citizen sees
/// at a glance whether re-entering the PIN is safe.
public enum TokenPinStatus
{
    Ok = 0,
    Low = 1,
    FinalTry = 2,
    Locked = 3,
}

/// An object stored on the token (key handle, certificate, etc.).
public sealed record TokenObject(
    string Id,                  // Object ID (CKA_ID for PKCS#11, container name for CNG)
    string? Label,
    TokenObjectKind Kind,
    string KeyAlgorithm,        // "RSA", "EC", "ECDSA-P256", ...
    int KeyBits,
    bool CanSign,
    bool CanDecrypt);

public enum TokenObjectKind
{
    PrivateKey,
    PublicKey,
    Certificate,
    Data,
}

public enum SigningAlg
{
    Sha256WithRsa,
    Sha384WithRsa,
    Sha512WithRsa,
    EcdsaSha256,
    EcdsaSha384,
    EcdsaSha512,
}

public enum KeyAlgorithm
{
    Rsa,
    EcdsaP256,
    EcdsaP384,
    EcdsaP521,
}

public sealed record KeyParams(
    KeyAlgorithm Algorithm,
    int RsaBits = 3072,
    string Label = "NexusGeregeDesktop device key",
    bool Sensitive = true,
    bool Extractable = false);
