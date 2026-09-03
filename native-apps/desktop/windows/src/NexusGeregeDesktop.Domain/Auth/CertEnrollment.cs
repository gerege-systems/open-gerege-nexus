namespace NexusGeregeDesktop.Domain.Auth;

/// Backend response from POST /web2app/v1/cert/enroll. The desktop receives
/// this, installs the cert onto the token (so the same key gets a
/// CA-signed cert instead of self-signed), and from then on the token
/// can drive /web2app/v1/auth/token/verify logins.
public sealed record CertEnrollmentResult(
    string CertId,
    string SerialHex,
    string CertPem,
    DateTimeOffset IssuedAt,
    DateTimeOffset ExpiresAt);

/// Backend allowlist of cert profiles. P-CITIZEN-AUTH for login,
/// P-CITIZEN-SIGN for document signing. Matches ca/ca.go.
public static class CertProfiles
{
    public const string CitizenAuth = "P-CITIZEN-AUTH";
    public const string CitizenSign = "P-CITIZEN-SIGN";
}
