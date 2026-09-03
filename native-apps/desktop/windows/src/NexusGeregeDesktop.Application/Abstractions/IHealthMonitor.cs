namespace NexusGeregeDesktop.Application.Abstractions;

public interface IHealthMonitor
{
    HealthSnapshot Current { get; }

    event EventHandler<HealthSnapshot>? Updated;

    void Start();

    void Stop();

    Task RefreshAsync(CancellationToken ct = default);
}

public sealed record HealthSnapshot(
    HealthState State,
    string? Version,
    DateTimeOffset CheckedAt,
    string? Detail);

public enum HealthState
{
    Unknown = 0,
    Checking = 1,
    Healthy = 2,
    Degraded = 3,
    Unreachable = 4,
}
