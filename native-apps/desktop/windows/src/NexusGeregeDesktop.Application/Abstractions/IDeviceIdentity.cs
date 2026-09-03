using NexusGeregeDesktop.Domain.Identity;

namespace NexusGeregeDesktop.Application.Abstractions;

public interface IDeviceIdentity
{
    ValueTask<DeviceId> GetOrCreateDeviceIdAsync(CancellationToken ct = default);

    /// 32-byte device secret used as HMAC key. Caller MUST NOT log or persist
    /// the returned buffer; it is the device's long-term root credential.
    ValueTask<byte[]> GetOrCreateDeviceSecretAsync(CancellationToken ct = default);

    ValueTask ResetAsync(CancellationToken ct = default);
}
