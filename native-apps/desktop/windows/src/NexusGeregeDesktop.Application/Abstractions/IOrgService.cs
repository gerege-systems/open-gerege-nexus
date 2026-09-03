using NexusGeregeDesktop.Domain.Org;
using NexusGeregeDesktop.Domain.Primitives;

namespace NexusGeregeDesktop.Application.Abstractions;

public interface IOrgService
{
    Task<Result<IReadOnlyList<Organization>>> ListAsync(CancellationToken ct = default);

    Task<Result<OrgLookupResult>> LookupAsync(string regNumber, CancellationToken ct = default);

    Task<Result<OrgRegisterSession>> RegisterAsync(string regNumber, OrgExpiryClass expiry, CancellationToken ct = default);

    Task<Result<Organization>> FinalizeRegisterAsync(Guid sessionId, CancellationToken ct = default);

    Task<Result<IReadOnlyList<OrgMember>>> ListMembersAsync(Guid orgId, CancellationToken ct = default);

    Task<Result<OrgMember>> AddMemberAsync(Guid orgId, string nationalId, OrgRole role, CancellationToken ct = default);

    Task<Result> RemoveMemberAsync(Guid orgId, Guid userId, CancellationToken ct = default);

    Task<Result> UpdateEnglishNameAsync(Guid orgId, string nameEn, CancellationToken ct = default);

    /// Backend currently returns 501 (HSM-backed cert not implemented).
    /// UI can wire this up but expect a "not_implemented" error.
    Task<Result<OrgXroadCert>> SignXroadCsrAsync(Guid orgId, string csrPem, string profile, CancellationToken ct = default);
}
