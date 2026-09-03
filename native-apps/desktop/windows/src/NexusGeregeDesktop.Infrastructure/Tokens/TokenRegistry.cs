using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Domain.Primitives;
using NexusGeregeDesktop.Domain.Tokens;
using Microsoft.Extensions.Logging;

namespace NexusGeregeDesktop.Infrastructure.Tokens;

public sealed class TokenRegistry : ITokenRegistry
{
    // Keep ALL providers (not just the ones reporting IsAvailable at
    // construction time). A PKCS#11 module that wasn't yet loaded
    // because no token was plugged in at startup should still get a
    // chance to discover one mid-session. Per-scan re-check happens
    // inside DiscoverAllAsync; unavailable providers contribute zero
    // tokens but stay in the list so OpenAsync can route to them once
    // a token shows up.
    private readonly IReadOnlyList<ICryptoTokenProvider> _all;
    private readonly ILogger<TokenRegistry> _logger;

    public TokenRegistry(IEnumerable<ICryptoTokenProvider> providers, ILogger<TokenRegistry> logger)
    {
        _all = providers.ToArray();
        _logger = logger;
        _logger.LogInformation("Token providers configured: {Names}",
            string.Join(", ", _all.Select(p => $"{p.Id} ({p.DisplayName})")));
    }

    public IReadOnlyList<ICryptoTokenProvider> Providers => _all;

    public async Task<Result<IReadOnlyList<DiscoveredToken>>> DiscoverAllAsync(CancellationToken ct = default)
    {
        // Refresh BEFORE listing. Feitian's eps2003csp11.dll (and a few
        // other widely-used PKCS#11 modules) cache the slot table at
        // C_Initialize time and never re-read it during the process
        // lifetime — a token plugged in after app startup stays
        // invisible until the module is finalized + re-initialized.
        // CNG and other backends ship a no-op default for RefreshAsync.
        foreach (var p in _all)
        {
            try
            {
                await p.RefreshAsync(ct).ConfigureAwait(false);
            }
            catch (Exception ex)
            {
                _logger.LogWarning(ex, "Provider {Provider} threw during RefreshAsync", p.Id);
            }
        }

        var sink = new List<DiscoveredToken>();
        foreach (var p in _all)
        {
            // Re-check IsAvailable each scan so a PKCS#11 module that
            // wasn't loadable at startup (DLL missing, dependency
            // failed) gets re-evaluated. Skipping unavailable
            // providers here keeps the LIST in Providers stable while
            // letting hot-plugged tokens surface on the next scan.
            if (!p.IsAvailable) continue;
            try
            {
                var result = await p.ListTokensAsync(ct).ConfigureAwait(false);
                if (result.IsSuccess)
                {
                    foreach (var t in result.Value)
                    {
                        sink.Add(new DiscoveredToken(p, t));
                    }
                }
                else
                {
                    _logger.LogWarning("Provider {Provider} listing failed: {Msg}", p.Id, result.Error.Message);
                }
            }
            catch (Exception ex)
            {
                _logger.LogWarning(ex, "Provider {Provider} threw during ListTokensAsync", p.Id);
            }
        }

        // Cross-backend dedupe — a Feitian ePass2003 enrolled via PKCS#11
        // gets its cert auto-propagated by the Windows smart-card service
        // into CurrentUser\My, where the CNG provider then picks it up.
        // Both rows describe the same physical card, so we collapse them
        // to one (matching the EnterSafe PKI Manager UX: one entry per
        // token). Matching key: SHA-256 of the on-card RSA modulus, set
        // by each provider at discovery time. PKCS#11 wins when both are
        // present — it exposes the full admin surface (change PIN, init,
        // unlock) that CNG can't reach. Entries without a fingerprint
        // (blank tokens, EC tokens not yet supported here) opt out of
        // dedup and pass through unchanged.
        var deduped = DedupeByFingerprint(sink);
        return Result<IReadOnlyList<DiscoveredToken>>.Success(deduped);
    }

    private static IReadOnlyList<DiscoveredToken> DedupeByFingerprint(List<DiscoveredToken> input)
    {
        var seen = new Dictionary<string, DiscoveredToken>(StringComparer.OrdinalIgnoreCase);
        var noFingerprint = new List<DiscoveredToken>();
        foreach (var dt in input)
        {
            var fp = dt.Info.PublicKeyFingerprint;
            if (string.IsNullOrEmpty(fp))
            {
                noFingerprint.Add(dt);
                continue;
            }
            if (!seen.TryGetValue(fp, out var prev))
            {
                seen[fp] = dt;
                continue;
            }
            // Both entries share a modulus → same physical card. PKCS#11
            // keeps the admin surface so it wins over CNG. If both are
            // the same backend (shouldn't happen, but defensive), keep
            // the first one encountered.
            if (prev.Info.Backend != TokenBackend.Pkcs11 && dt.Info.Backend == TokenBackend.Pkcs11)
            {
                seen[fp] = dt;
            }
        }
        var result = new List<DiscoveredToken>(seen.Count + noFingerprint.Count);
        result.AddRange(seen.Values);
        result.AddRange(noFingerprint);
        return result;
    }

    public Task<Result<ITokenSession>> OpenAsync(string providerId, string tokenId, string? pin = null, CancellationToken ct = default)
    {
        var provider = Providers.FirstOrDefault(p => string.Equals(p.Id, providerId, StringComparison.Ordinal));
        if (provider is null)
        {
            return Task.FromResult(Result<ITokenSession>.Failure(ApiError.NotFound("provider_not_found")));
        }
        return provider.OpenSessionAsync(tokenId, pin, ct);
    }

    public Task<Result> InitializeTokenAsync(string providerId, string tokenId, string soPin, string label, string newUserPin, CancellationToken ct = default)
    {
        var provider = Providers.FirstOrDefault(p => string.Equals(p.Id, providerId, StringComparison.Ordinal));
        if (provider is null)
        {
            return Task.FromResult(Result.Failure(ApiError.NotFound("provider_not_found")));
        }
        return provider.InitializeTokenAsync(tokenId, soPin, label, newUserPin, ct);
    }

    public Task<Result> UnlockUserPinAsync(string providerId, string tokenId, string soPin, string newUserPin, CancellationToken ct = default)
    {
        var provider = Providers.FirstOrDefault(p => string.Equals(p.Id, providerId, StringComparison.Ordinal));
        if (provider is null)
        {
            return Task.FromResult(Result.Failure(ApiError.NotFound("provider_not_found")));
        }
        return provider.UnlockUserPinAsync(tokenId, soPin, newUserPin, ct);
    }

    public Task<Result> ChangeSoPinAsync(string providerId, string tokenId, string oldSoPin, string newSoPin, CancellationToken ct = default)
    {
        var provider = Providers.FirstOrDefault(p => string.Equals(p.Id, providerId, StringComparison.Ordinal));
        if (provider is null)
        {
            return Task.FromResult(Result.Failure(ApiError.NotFound("provider_not_found")));
        }
        return provider.ChangeSoPinAsync(tokenId, oldSoPin, newSoPin, ct);
    }
}
