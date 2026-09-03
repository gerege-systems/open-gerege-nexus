namespace NexusGeregeDesktop.Domain.Org;

public enum OrgStatus
{
    Pending,
    Active,
    Suspended,
    Removed,
    Unknown,
}

public enum OrgRole
{
    Signer,
    Admin,
}

public enum OrgMemberStatus
{
    Active,
    Pending,
    Removed,
}

public enum OrgExpiryClass
{
    /// 5-minute signing window (smart-id default).
    Now,
    /// 3 days.
    ThreeDays,
    /// 7 days.
    SevenDays,
    /// 30 days.
    ThirtyDays,
}

public sealed record Organization(
    Guid Id,
    string RegistrationNumber,
    string Name,
    string? NameEn,
    string? Phone,
    bool DanVerified,
    OrgStatus Status,
    DateTimeOffset CreatedAt);

public sealed record OrgMember(
    Guid Id,
    Guid OrgId,
    Guid? UserId,
    OrgRole Role,
    OrgMemberStatus Status,
    string? UserName,
    string NationalId,
    DateTimeOffset CreatedAt);

public sealed record OrgLookupResult(
    bool Found,
    string? Name,
    string? Type,
    string? Ceo,
    string? Phone);

public sealed record OrgRegisterSession(
    Guid SessionId,
    string VerificationCode,
    DateTimeOffset ExpiresAt);

public sealed record OrgXroadCert(
    string CertPem,
    string Profile,
    string SerialNumber,
    DateTimeOffset NotAfter);
