using System.Globalization;
using System.Security.Cryptography;
using System.Text;

namespace NexusGeregeDesktop.Infrastructure.Security.Hmac;

/// Pure HMAC computation, identical to backend
/// hmac_nonce.go canonicalRequest + HMACSHA256.
/// Format: METHOD\nPATH\nHEX(SHA256(body))\nTIMESTAMP\nNONCE
/// Output: HMAC-SHA256(secret, canonical) → hex lowercase, 64 chars.
public static class HmacSigner
{
    public static byte[] HashBody(ReadOnlySpan<byte> body)
    {
        Span<byte> hash = stackalloc byte[SHA256.HashSizeInBytes];
        SHA256.HashData(body, hash);
        return hash.ToArray();
    }

    public static string BuildCanonical(
        string method,
        string path,
        ReadOnlySpan<byte> bodyHash,
        string timestamp,
        string nonce)
    {
        if (string.IsNullOrEmpty(method))
        {
            throw new ArgumentException("Method must be non-empty.", nameof(method));
        }
        if (string.IsNullOrEmpty(path))
        {
            throw new ArgumentException("Path must be non-empty.", nameof(path));
        }
        if (bodyHash.Length != SHA256.HashSizeInBytes)
        {
            throw new ArgumentException(
                $"Body hash must be {SHA256.HashSizeInBytes} bytes, got {bodyHash.Length}.",
                nameof(bodyHash));
        }
        if (string.IsNullOrEmpty(timestamp))
        {
            throw new ArgumentException("Timestamp must be non-empty.", nameof(timestamp));
        }
        if (string.IsNullOrEmpty(nonce))
        {
            throw new ArgumentException("Nonce must be non-empty.", nameof(nonce));
        }

        var sb = new StringBuilder(method.Length + path.Length + 64 + timestamp.Length + nonce.Length + 4);
        sb.Append(method).Append('\n');
        sb.Append(path).Append('\n');
        sb.Append(Convert.ToHexString(bodyHash).ToLowerInvariant()).Append('\n');
        sb.Append(timestamp).Append('\n');
        sb.Append(nonce);
        return sb.ToString();
    }

    public static string Sign(ReadOnlySpan<byte> secret, string canonical)
    {
        if (secret.Length == 0)
        {
            throw new ArgumentException("Secret must be non-empty.", nameof(secret));
        }

        var canonicalBytes = Encoding.UTF8.GetBytes(canonical);
        Span<byte> tag = stackalloc byte[HMACSHA256.HashSizeInBytes];
        HMACSHA256.HashData(secret, canonicalBytes, tag);
        return Convert.ToHexString(tag).ToLowerInvariant();
    }

    public static string SignRequest(
        ReadOnlySpan<byte> secret,
        string method,
        string path,
        ReadOnlySpan<byte> body,
        long unixSeconds,
        string nonce)
    {
        var bodyHash = HashBody(body);
        var canonical = BuildCanonical(
            method,
            path,
            bodyHash,
            unixSeconds.ToString(CultureInfo.InvariantCulture),
            nonce);
        return Sign(secret, canonical);
    }
}
