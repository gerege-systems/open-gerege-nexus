using System.Runtime.Versioning;
using System.Security.Cryptography.X509Certificates;
using System.Text;
using Microsoft.Extensions.Logging;

namespace NexusGeregeDesktop.Infrastructure.Tokens.Cng;

/// Persists enrolled CA-signed certs alongside the on-card key container
/// name, so reinserting the smart card re-binds the cert automatically.
///
/// Why this exists: Windows Smart Card propagation runs `CardReadFile` for
/// every cert stored ON THE CARD when a card is inserted, and refreshes
/// CurrentUser\My to match. Our enrolled cert is NOT on the card itself
/// (only the on-card key + the original factory cert are) — it lives in
/// the Windows cert store, bound to the KSP container via
/// CERT_KEY_PROV_INFO_PROP_ID. When the card is removed, that cert sits
/// in the store with HasPrivateKey=false; when the card returns,
/// propagation drops our CA cert from the linked set because it was
/// never on the card to begin with, and the citizen ends up back at
/// State=Provisioned.
///
/// Workaround: on every successful enroll, dump the cert PEM into
/// %LOCALAPPDATA%\NexusGeregeDesktop\enrolled-certs\<container>.pem. On every
/// discovery, if the on-card key has no CA-signed cert in store, re-
/// import the cached PEM and re-run CertSetCertificateContextProperty.
/// The cert lives keyed by container name so multiple tokens enrolled
/// on the same machine each get their own row.
[SupportedOSPlatform("windows")]
internal sealed class EnrolledCertCache
{
    private readonly ILogger _logger;
    private readonly string _root;

    public EnrolledCertCache(ILogger logger)
    {
        _logger = logger;
        _root = Path.Combine(
            Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
            "NexusGeregeDesktop",
            "enrolled-certs");
        try { Directory.CreateDirectory(_root); }
        catch (Exception ex) { _logger.LogWarning(ex, "EnrolledCertCache: cannot create root {Root}", _root); }
    }

    public void Save(string containerName, string certPem)
    {
        if (string.IsNullOrWhiteSpace(containerName) || string.IsNullOrWhiteSpace(certPem))
        {
            return;
        }
        try
        {
            var path = PathFor(containerName);
            File.WriteAllText(path, certPem, new UTF8Encoding(false));
            _logger.LogInformation("EnrolledCertCache: saved {Path}", path);
        }
        catch (Exception ex)
        {
            _logger.LogWarning(ex, "EnrolledCertCache: save failed for {Container}", containerName);
        }
    }

    public X509Certificate2? TryLoad(string containerName)
    {
        if (string.IsNullOrWhiteSpace(containerName)) return null;
        try
        {
            var path = PathFor(containerName);
            if (!File.Exists(path)) return null;
            var pem = File.ReadAllText(path);
            var der = PemToDer(pem);
            if (der is null) return null;
            return new X509Certificate2(der);
        }
        catch (Exception ex)
        {
            _logger.LogWarning(ex, "EnrolledCertCache: load failed for {Container}", containerName);
            return null;
        }
    }

    private string PathFor(string containerName)
    {
        // Container names from CNG can include path-unfriendly chars on
        // some KSPs (curly braces, dashes, equals). Hash the input to
        // get a stable, filesystem-safe filename. We never need to
        // round-trip the file back to a container — the binding is
        // re-applied via CertSetCertificateContextProperty using the
        // *current* on-card container — so a hashed name is fine.
        var safe = string.Concat(containerName.Select(c =>
            char.IsLetterOrDigit(c) || c == '_' || c == '-' ? c : '_'));
        if (safe.Length > 64) safe = safe[..64];
        return Path.Combine(_root, safe + ".pem");
    }

    private static byte[]? PemToDer(string pem)
    {
        var begin = pem.IndexOf("-----BEGIN", StringComparison.Ordinal);
        var end = pem.IndexOf("-----END", StringComparison.Ordinal);
        if (begin < 0 || end < 0 || end <= begin) return null;
        var afterBeginHeader = pem.IndexOf('\n', begin);
        if (afterBeginHeader < 0) return null;
        var body = pem.Substring(afterBeginHeader + 1, end - afterBeginHeader - 1)
            .Replace("\r", string.Empty)
            .Replace("\n", string.Empty)
            .Trim();
        try { return Convert.FromBase64String(body); }
        catch { return null; }
    }
}
