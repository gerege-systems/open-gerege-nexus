using NexusGeregeDesktop.Domain.Primitives;

namespace NexusGeregeDesktop.Application.Abstractions;

public interface IDashboardService
{
    Task<Result<DashboardSnapshot>> LoadAsync(CancellationToken ct = default);
}

public sealed record DashboardSnapshot(
    DashboardUser User,
    IReadOnlyList<DashboardDevice> Devices,
    IReadOnlyList<DashboardActivity> Sessions,
    int Certificates,
    int TotalLogins);

public sealed record DashboardUser(
    string NationalId,
    string FullName,
    string FullNameLatin,
    string KycLevel,
    string Status,
    DateTimeOffset CreatedAt);

public sealed record DashboardDevice(
    Guid DeviceId,
    string Platform,
    string Status,
    DateTimeOffset? LastUsedAt,
    DateTimeOffset CreatedAt);

public sealed record DashboardActivity(
    Guid Id,
    string SessionType,
    string State,
    string Result,
    string RpName,
    string? PushText,
    DateTimeOffset CreatedAt,
    string SubsystemName = "",
    string InteractionType = "",
    string DocumentNumber = "",
    string ClientIp = "");
