namespace NexusGeregeDesktop.Domain.Auth;

public enum AuthSessionState
{
    Running,
    Complete,
    Expired,
    Refused,
}

public sealed record AuthSession(
    Guid SessionId,
    string VerificationCode,
    DateTimeOffset ExpiresAt,
    string SessionSecret,
    string AuthCodeHashable);

public sealed record AuthSessionStatus(
    AuthSessionState State,
    string? Result,
    string? Nonce,
    AuthIdentity? Identity,
    AuthCertificate? Certificate,
    AuthSignature? Signature)
{
    public bool IsTerminal => State != AuthSessionState.Running;
}

public sealed record AuthIdentity(
    string NationalId,
    string? CivilId,
    string? GivenName,
    string? Surname,
    string FullName,
    string KycLevel);

public sealed record AuthCertificate(
    string Pem,
    string SerialNumber,
    DateTimeOffset ValidUntil);

public sealed record AuthSignature(
    string Value,
    string Algorithm,
    string Hashable);
