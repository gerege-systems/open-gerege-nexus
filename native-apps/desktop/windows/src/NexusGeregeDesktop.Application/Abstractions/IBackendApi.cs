using NexusGeregeDesktop.Domain.Primitives;

namespace NexusGeregeDesktop.Application.Abstractions;

public interface IBackendApi
{
    Task<Result<HealthStatus>> GetHealthAsync(CancellationToken ct = default);
}

public sealed record HealthStatus(bool Live, bool Ready, string? Version);
