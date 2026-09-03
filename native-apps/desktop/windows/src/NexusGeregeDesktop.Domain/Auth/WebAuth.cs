namespace NexusGeregeDesktop.Domain.Auth;

/// State enum matching the backend web flow
/// (backend/internal/auth/web_login_service.go).
public enum WebAuthState
{
    Pushed,
    Confirmed,
    Expired,
    Failed,
}

public sealed record WebAuthSession(
    Guid SessionId,
    string ControlCode,
    string PollToken,
    DateTimeOffset ExpiresAt,
    string? DeviceLinkUrl = null);

public sealed record WebAuthSessionStatus(
    WebAuthState State,
    string? SessionToken,
    Guid? UserId,
    string? FailureReason,
    DateTimeOffset? ExpiresAt)
{
    public bool IsTerminal => State != WebAuthState.Pushed;
}

public sealed record WebSession(
    string Token,
    Guid UserId,
    DateTimeOffset ExpiresAt,
    string NationalId,
    string FullName,
    string KycLevel,
    // Login session-ий pollToken. Session-bound `/api/*` (dashboard, certificates,
    // representations, children)-д {sessionId=Token, pollToken} хосоор дамжуулна —
    // backend identity-г эндээс гаргана (клиентийн personId-д итгэхгүй).
    string PollToken = "")
{
    public bool IsValid(DateTimeOffset utcNow) => utcNow < ExpiresAt;
}

public sealed record UserProfile(
    Guid UserId,
    string NationalId,
    string FullName,
    string FullNameLatin,
    string KycLevel,
    string Status,
    DateTimeOffset CreatedAt);

// ----- Hardware-token web login (M9.B) -----------------------------------

/// Backend response from POST /web2app/v1/auth/token/challenge. The desktop
/// signs SHA256(Nonce) with the USB token's private key and posts the
/// signature back via VerifyTokenAsync.
public sealed record WebTokenChallenge(
    Guid SessionId,
    byte[] Nonce,
    string AlgHint,
    DateTimeOffset ExpiresAt);

/// What VerifyTokenAsync hands back on success — the desktop saves it via
/// IWebSessionStore exactly as it would for the QR / national-id flows.
public sealed record WebTokenLoginResult(
    string SessionToken,
    Guid UserId,
    DateTimeOffset ExpiresAt);

/// Algorithms the desktop is allowed to send on /token/verify. The
/// backend's allowedTokenAlg gate uses the same wire strings.
public enum WebTokenSignatureAlg
{
    RsaSha256,
    EcdsaSha256,
}

public static class WebTokenSignatureAlgExtensions
{
    public static string ToWireString(this WebTokenSignatureAlg alg) => alg switch
    {
        WebTokenSignatureAlg.RsaSha256 => "rsa-sha256",
        WebTokenSignatureAlg.EcdsaSha256 => "ecdsa-sha256",
        _ => "rsa-sha256",
    };
}
