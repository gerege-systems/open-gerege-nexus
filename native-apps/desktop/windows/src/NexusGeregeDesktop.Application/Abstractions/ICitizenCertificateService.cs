using NexusGeregeDesktop.Domain.Certificates;
using NexusGeregeDesktop.Domain.Primitives;

namespace NexusGeregeDesktop.Application.Abstractions;

/// <summary>
/// Нэвтэрсэн иргэний гэрчилгээний жагсаалт — first-party `POST /api/certificates`.
/// Session-bound: клиентийн personId-д итгэхгүй, login session-ий {sessionId, pollToken}
/// хосоор identity-г backend тал гаргана (BACKEND-INTEGRATION.md).
/// </summary>
public interface ICitizenCertificateService
{
    Task<Result<CitizenCertificateList>> ListAsync(CancellationToken ct = default);
}
