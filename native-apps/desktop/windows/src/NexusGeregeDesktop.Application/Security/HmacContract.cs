namespace NexusGeregeDesktop.Application.Security;

/// HMAC contract constants — MUST stay in lockstep with
/// backend/internal/platform/server/middleware/hmac_nonce.go.
/// Drift here causes 401 signature-mismatch from the server.
public static class HmacContract
{
    public const string DeviceIdHeader = "X-Device-Id";
    public const string TimestampHeader = "X-Timestamp";
    public const string NonceHeader = "X-Nonce";
    public const string SignatureHeader = "X-Signature";

    public const int DeviceSecretBytes = 32;

    /// Drift the server tolerates between X-Timestamp and its own clock.
    /// Backend default: hmac_nonce.go:38-39 Drift = 60s.
    public static readonly TimeSpan DefaultClockDriftAllowance = TimeSpan.FromSeconds(60);

    /// Replay-cache TTL on the server. Nonces are single-use within this
    /// window per (device_id, nonce). Backend default: NonceTTL = 5min.
    public static readonly TimeSpan DefaultNonceTtl = TimeSpan.FromMinutes(5);
}
