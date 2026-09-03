using System.Security.Cryptography;
using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Application.Security;
using NexusGeregeDesktop.Domain.Identity;
using Microsoft.Extensions.Logging;

namespace NexusGeregeDesktop.Infrastructure.Security.Identity;

/// First call generates a UUID v4 device id and a 32-byte cryptographically
/// random device secret, persists both via the ISecretVault, returns them
/// thereafter. The device secret is the long-term HMAC key shared with the
/// backend; it leaves the vault only into the HmacSigningHandler.
public sealed class DeviceIdentityProvider : IDeviceIdentity, IDisposable
{
    public void Dispose() => _gate.Dispose();

    private const string DeviceIdKey = "device_id";
    private const string DeviceSecretKey = "device_secret";

    private readonly ISecretVault _vault;
    private readonly ILogger<DeviceIdentityProvider> _logger;
    private readonly SemaphoreSlim _gate = new(1, 1);

    private DeviceId? _cachedId;
    private byte[]? _cachedSecret;

    public DeviceIdentityProvider(ISecretVault vault, ILogger<DeviceIdentityProvider> logger)
    {
        _vault = vault;
        _logger = logger;
    }

    public async ValueTask<DeviceId> GetOrCreateDeviceIdAsync(CancellationToken ct = default)
    {
        if (_cachedId is { } id)
        {
            return id;
        }

        await _gate.WaitAsync(ct).ConfigureAwait(false);
        try
        {
            if (_cachedId is { } cached)
            {
                return cached;
            }

            var stored = await _vault.ReadAsync(DeviceIdKey, ct).ConfigureAwait(false);
            if (stored is { Length: > 0 })
            {
                var existing = new Guid(stored);
                _cachedId = DeviceId.From(existing);
                return _cachedId.Value;
            }

            var fresh = DeviceId.NewRandom();
            await _vault.WriteAsync(DeviceIdKey, fresh.Value.ToByteArray(), ct).ConfigureAwait(false);
            _logger.LogInformation("Provisioned new device identity {DeviceId}", fresh);
            _cachedId = fresh;
            return fresh;
        }
        finally
        {
            _gate.Release();
        }
    }

    public async ValueTask<byte[]> GetOrCreateDeviceSecretAsync(CancellationToken ct = default)
    {
        if (_cachedSecret is { Length: HmacContract.DeviceSecretBytes })
        {
            return _cachedSecret;
        }

        await _gate.WaitAsync(ct).ConfigureAwait(false);
        try
        {
            if (_cachedSecret is { Length: HmacContract.DeviceSecretBytes } cached)
            {
                return cached;
            }

            var stored = await _vault.ReadAsync(DeviceSecretKey, ct).ConfigureAwait(false);
            if (stored is { Length: HmacContract.DeviceSecretBytes })
            {
                _cachedSecret = stored;
                return stored;
            }

            var fresh = RandomNumberGenerator.GetBytes(HmacContract.DeviceSecretBytes);
            await _vault.WriteAsync(DeviceSecretKey, fresh, ct).ConfigureAwait(false);
            _logger.LogInformation("Provisioned new device secret ({Bytes} bytes)", HmacContract.DeviceSecretBytes);
            _cachedSecret = fresh;
            return fresh;
        }
        finally
        {
            _gate.Release();
        }
    }

    public async ValueTask ResetAsync(CancellationToken ct = default)
    {
        await _gate.WaitAsync(ct).ConfigureAwait(false);
        try
        {
            if (_cachedSecret is not null)
            {
                CryptographicOperations.ZeroMemory(_cachedSecret);
            }
            _cachedSecret = null;
            _cachedId = null;

            await _vault.DeleteAsync(DeviceIdKey, ct).ConfigureAwait(false);
            await _vault.DeleteAsync(DeviceSecretKey, ct).ConfigureAwait(false);
            _logger.LogWarning("Device identity reset (both id + secret deleted)");
        }
        finally
        {
            _gate.Release();
        }
    }
}
