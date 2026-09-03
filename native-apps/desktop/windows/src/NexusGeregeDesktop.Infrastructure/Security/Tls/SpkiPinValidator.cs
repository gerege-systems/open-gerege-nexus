using System.Net.Security;
using System.Security.Cryptography;
using System.Security.Cryptography.X509Certificates;

namespace NexusGeregeDesktop.Infrastructure.Security.Tls;

/// SPKI-SHA256 cert pinning. Computes SHA-256 over the DER-encoded
/// SubjectPublicKeyInfo of each cert in the chain (leaf + intermediates),
/// matches against a pre-configured pin set. Matches the mobile clients'
/// pinning scheme (Android OkHttp CertificatePinner, iOS URLSession).
public sealed class SpkiPinValidator
{
    private readonly HashSet<string> _normalizedPins;

    public SpkiPinValidator(IEnumerable<string> base64Pins)
    {
        ArgumentNullException.ThrowIfNull(base64Pins);

        _normalizedPins = new HashSet<string>(
            base64Pins
                .Where(static p => !string.IsNullOrWhiteSpace(p))
                .Select(Normalize),
            StringComparer.Ordinal);
    }

    public bool HasPins => _normalizedPins.Count > 0;

    public bool Validate(X509Certificate2? leafCert, X509Chain? chain, SslPolicyErrors sslPolicyErrors)
    {
        if (sslPolicyErrors != SslPolicyErrors.None)
        {
            return false;
        }
        if (leafCert is null)
        {
            return false;
        }
        if (_normalizedPins.Count == 0)
        {
            return true;
        }

        if (MatchesPin(leafCert))
        {
            return true;
        }

        if (chain is not null)
        {
            foreach (var element in chain.ChainElements)
            {
                if (MatchesPin(element.Certificate))
                {
                    return true;
                }
            }
        }

        return false;
    }

    private bool MatchesPin(X509Certificate2 cert)
    {
        var spki = cert.PublicKey.ExportSubjectPublicKeyInfo();
        var hash = SHA256.HashData(spki);
        var encoded = Convert.ToBase64String(hash);
        return _normalizedPins.Contains(encoded);
    }

    private static string Normalize(string pin)
    {
        var trimmed = pin.Trim();
        const string prefix = "sha256/";
        if (trimmed.StartsWith(prefix, StringComparison.OrdinalIgnoreCase))
        {
            trimmed = trimmed[prefix.Length..];
        }
        return trimmed;
    }
}
