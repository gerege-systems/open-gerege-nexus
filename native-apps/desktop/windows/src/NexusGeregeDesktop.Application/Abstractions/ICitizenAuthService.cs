using NexusGeregeDesktop.Domain.Auth;
using NexusGeregeDesktop.Domain.Primitives;

namespace NexusGeregeDesktop.Application.Abstractions;

public interface ICitizenAuthService
{
    Task<Result<WebAuthSession>> InitiateAsync(string nationalId, CancellationToken ct = default);

    /// QR-based citizen auth. Backend returns control_code + device_link_url
    /// that the citizen scans with the e-ID mobile app. No national_id needed.
    Task<Result<WebAuthSession>> InitiateQrAsync(CancellationToken ct = default);

    Task<Result<WebAuthSessionStatus>> PollAsync(Guid sessionId, string pollToken, CancellationToken ct = default);

    /// M9.B step 1 — request a fresh nonce + session_id. Pair with
    /// VerifyTokenAsync after the desktop has signed SHA256(Nonce).
    Task<Result<WebTokenChallenge>> InitiateTokenChallengeAsync(CancellationToken ct = default);

    /// M9.B step 2 — submit the cert + signature. Synchronous: on
    /// success the response carries the bearer + user_id directly, no
    /// polling required.
    Task<Result<WebTokenLoginResult>> VerifyTokenAsync(
        Guid sessionId,
        string certPem,
        byte[] signature,
        WebTokenSignatureAlg signatureAlg,
        CancellationToken ct = default);

    Task<Result> LogoutAsync(CancellationToken ct = default);
}

public interface IUserProfileService
{
    Task<Result<UserProfile>> GetMeAsync(CancellationToken ct = default);
}
