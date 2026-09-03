using System.Text;
using System.Text.Json;
using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Domain.Auth;
using Microsoft.Extensions.Logging;

namespace NexusGeregeDesktop.Infrastructure.Auth;

public sealed class WebSessionStore : IWebSessionStore, IDisposable
{
    public void Dispose()
    {
        _gate.Dispose();
        GC.SuppressFinalize(this);
    }


    private const string VaultKey = "web_session";

    private readonly ISecretVault _vault;
    private readonly IClock _clock;
    private readonly ILogger<WebSessionStore> _logger;
    private readonly SemaphoreSlim _gate = new(1, 1);

    private WebSession? _cached;
    private bool _loadedOnce;

    public WebSessionStore(ISecretVault vault, IClock clock, ILogger<WebSessionStore> logger)
    {
        _vault = vault;
        _clock = clock;
        _logger = logger;
    }

    public WebSession? Current => _cached;

    public bool IsAuthenticated => _cached is not null && _cached.IsValid(_clock.UtcNow);

    public event EventHandler<WebSession?>? Changed;

    public async ValueTask<WebSession?> LoadAsync(CancellationToken ct = default)
    {
        if (_loadedOnce)
        {
            return _cached;
        }

        await _gate.WaitAsync(ct).ConfigureAwait(false);
        try
        {
            if (_loadedOnce)
            {
                return _cached;
            }

            var bytes = await _vault.ReadAsync(VaultKey, ct).ConfigureAwait(false);
            if (bytes is { Length: > 0 })
            {
                try
                {
                    var session = JsonSerializer.Deserialize<WebSession>(bytes);
                    if (session is not null && session.IsValid(_clock.UtcNow))
                    {
                        _cached = session;
                        _logger.LogInformation("Restored web session for {NationalId} valid until {ExpiresAt}",
                            session.NationalId, session.ExpiresAt);
                    }
                    else if (session is not null)
                    {
                        _logger.LogInformation("Cached session expired at {ExpiresAt}, clearing.", session.ExpiresAt);
                        await _vault.DeleteAsync(VaultKey, ct).ConfigureAwait(false);
                    }
                }
                catch (JsonException ex)
                {
                    _logger.LogWarning(ex, "Failed to deserialize cached web session; clearing.");
                    await _vault.DeleteAsync(VaultKey, ct).ConfigureAwait(false);
                }
            }
            _loadedOnce = true;
            return _cached;
        }
        finally
        {
            _gate.Release();
        }
    }

    public async ValueTask SaveAsync(WebSession session, CancellationToken ct = default)
    {
        ArgumentNullException.ThrowIfNull(session);
        await _gate.WaitAsync(ct).ConfigureAwait(false);
        try
        {
            var bytes = JsonSerializer.SerializeToUtf8Bytes(session);
            await _vault.WriteAsync(VaultKey, bytes, ct).ConfigureAwait(false);
            _cached = session;
            _loadedOnce = true;
        }
        finally
        {
            _gate.Release();
        }
        Changed?.Invoke(this, session);
    }

    public async ValueTask ClearAsync(CancellationToken ct = default)
    {
        await _gate.WaitAsync(ct).ConfigureAwait(false);
        try
        {
            await _vault.DeleteAsync(VaultKey, ct).ConfigureAwait(false);
            _cached = null;
            _loadedOnce = true;
        }
        finally
        {
            _gate.Release();
        }
        Changed?.Invoke(this, null);
    }
}
