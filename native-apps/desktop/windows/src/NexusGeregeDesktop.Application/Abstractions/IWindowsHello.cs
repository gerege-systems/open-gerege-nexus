namespace NexusGeregeDesktop.Application.Abstractions;

public interface IWindowsHello
{
    ValueTask<bool> IsAvailableAsync(CancellationToken ct = default);

    ValueTask<HelloResult> RequestConsentAsync(string reasonLocalized, CancellationToken ct = default);
}

public enum HelloResult
{
    Verified,
    NotAvailable,
    DeviceNotPresent,
    DisabledByPolicy,
    Cancelled,
    RetriesExhausted,
    Failed,
}
