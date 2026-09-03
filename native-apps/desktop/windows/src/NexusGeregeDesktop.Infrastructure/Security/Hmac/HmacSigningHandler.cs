using System.Globalization;
using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Application.Security;
using Microsoft.Extensions.Logging;

namespace NexusGeregeDesktop.Infrastructure.Security.Hmac;

/// DelegatingHandler that applies the device-HMAC contract to every outgoing
/// request: hashes the body, builds the canonical string, signs it with the
/// device secret, and attaches the four contract headers.
///
/// Lives between any retry/resilience handler and the primary socket handler
/// so retries get a fresh nonce + timestamp (avoids replay false-positives).
public sealed class HmacSigningHandler : DelegatingHandler
{
    private readonly IDeviceIdentity _identity;
    private readonly IClock _clock;
    private readonly ILogger<HmacSigningHandler> _logger;

    public HmacSigningHandler(IDeviceIdentity identity, IClock clock, ILogger<HmacSigningHandler> logger)
    {
        _identity = identity;
        _clock = clock;
        _logger = logger;
    }

    protected override async Task<HttpResponseMessage> SendAsync(
        HttpRequestMessage request,
        CancellationToken cancellationToken)
    {
        ArgumentNullException.ThrowIfNull(request);

        var deviceId = await _identity.GetOrCreateDeviceIdAsync(cancellationToken).ConfigureAwait(false);
        var secret = await _identity.GetOrCreateDeviceSecretAsync(cancellationToken).ConfigureAwait(false);

        var method = request.Method.Method;
        var path = request.RequestUri?.AbsolutePath
            ?? throw new InvalidOperationException("Request URI must be absolute before HMAC signing.");

        // Only mobile/device endpoints use the device-HMAC contract.
        // RP routes (/rp/v1/*) authenticate via X-Road; web routes (/web2app/v1/*)
        // use a separate flow. Skip signing for non-mobile paths.
        if (!path.StartsWith("/mobile/", StringComparison.Ordinal))
        {
            return await base.SendAsync(request, cancellationToken).ConfigureAwait(false);
        }

        byte[] body = Array.Empty<byte>();
        if (request.Content is not null)
        {
            body = await request.Content
                .ReadAsByteArrayAsync(cancellationToken)
                .ConfigureAwait(false);
        }

        var timestamp = _clock.UnixSeconds.ToString(CultureInfo.InvariantCulture);
        var nonce = Guid.NewGuid().ToString("D");

        var signature = HmacSigner.SignRequest(
            secret,
            method,
            path,
            body,
            long.Parse(timestamp, CultureInfo.InvariantCulture),
            nonce);

        SetOrReplace(request, HmacContract.DeviceIdHeader, deviceId.ToString());
        SetOrReplace(request, HmacContract.TimestampHeader, timestamp);
        SetOrReplace(request, HmacContract.NonceHeader, nonce);
        SetOrReplace(request, HmacContract.SignatureHeader, signature);

        _logger.LogDebug(
            "HMAC signed {Method} {Path} (device={DeviceId} ts={Timestamp})",
            method, path, deviceId, timestamp);

        return await base.SendAsync(request, cancellationToken).ConfigureAwait(false);
    }

    private static void SetOrReplace(HttpRequestMessage request, string name, string value)
    {
        if (request.Headers.Contains(name))
        {
            request.Headers.Remove(name);
        }
        request.Headers.TryAddWithoutValidation(name, value);
    }
}
