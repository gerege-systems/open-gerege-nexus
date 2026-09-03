using System.Security.Cryptography;
using System.Security.Cryptography.X509Certificates;
using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Domain.Certificates;
using NexusGeregeDesktop.Domain.Primitives;

namespace NexusGeregeDesktop.Infrastructure.Certificates;

public sealed class CertificateService : ICertificateService
{
    private static readonly char[] LineSeparators = ['\n', '\r'];


    public Result<CertificateInfo> ParsePem(string pemContents)
    {
        if (string.IsNullOrWhiteSpace(pemContents))
        {
            return Result<CertificateInfo>.Failure(ApiError.BadRequest("Empty PEM input."));
        }

        try
        {
            using var cert = X509Certificate2.CreateFromPem(pemContents.AsSpan());
            return Result<CertificateInfo>.Success(ToInfo(cert, pemContents));
        }
        catch (CryptographicException ex)
        {
            return Result<CertificateInfo>.Failure(
                ApiError.BadRequest("PEM-ийг X.509 болгож parse хийж чадсангүй.", ex.Message));
        }
        catch (ArgumentException ex)
        {
            return Result<CertificateInfo>.Failure(
                ApiError.BadRequest("PEM формат буруу байна.", ex.Message));
        }
    }

    public Result<CertificateInfo> ParseFile(string filePath)
    {
        if (string.IsNullOrWhiteSpace(filePath))
        {
            return Result<CertificateInfo>.Failure(ApiError.BadRequest("File path empty."));
        }

        if (!File.Exists(filePath))
        {
            return Result<CertificateInfo>.Failure(ApiError.NotFound($"Файл олдсонгүй: {filePath}"));
        }

        try
        {
            var bytes = File.ReadAllBytes(filePath);

            if (LooksLikePem(bytes))
            {
                return ParsePem(System.Text.Encoding.UTF8.GetString(bytes));
            }

            using var cert = new X509Certificate2(bytes);
            var pem = ToPem(cert);
            return Result<CertificateInfo>.Success(ToInfo(cert, pem));
        }
        catch (CryptographicException ex)
        {
            return Result<CertificateInfo>.Failure(
                ApiError.BadRequest("Файлыг X.509 гэрчилгээ болгож parse хийж чадсангүй.", ex.Message));
        }
        catch (IOException ex)
        {
            return Result<CertificateInfo>.Failure(
                ApiError.Internal("Файл уншихад алдаа гарлаа.", ex.Message));
        }
    }

    private static bool LooksLikePem(ReadOnlySpan<byte> bytes)
    {
        if (bytes.Length < 27)
        {
            return false;
        }
        var head = System.Text.Encoding.ASCII.GetString(bytes[..Math.Min(64, bytes.Length)]);
        return head.Contains("-----BEGIN", StringComparison.Ordinal);
    }

    private static string ToPem(X509Certificate2 cert)
    {
        var b64 = Convert.ToBase64String(cert.RawData, Base64FormattingOptions.InsertLineBreaks);
        return $"-----BEGIN CERTIFICATE-----\n{b64}\n-----END CERTIFICATE-----\n";
    }

    private static CertificateInfo ToInfo(X509Certificate2 cert, string pem)
    {
        var spki = cert.PublicKey.ExportSubjectPublicKeyInfo();
        var spkiHash = SHA256.HashData(spki);

        var sha256 = SHA256.HashData(cert.RawData);
#pragma warning disable CA5350 // SHA-1 thumbprint is the historical X.509 thumbprint, shown for cross-tool compatibility, not used as a security primitive.
        var sha1 = SHA1.HashData(cert.RawData);
#pragma warning restore CA5350

        return new CertificateInfo(
            Subject: cert.Subject,
            Issuer: cert.Issuer,
            SerialNumberHex: cert.SerialNumber,
            NotBefore: new DateTimeOffset(cert.NotBefore.ToUniversalTime(), TimeSpan.Zero),
            NotAfter: new DateTimeOffset(cert.NotAfter.ToUniversalTime(), TimeSpan.Zero),
            SignatureAlgorithm: cert.SignatureAlgorithm.FriendlyName ?? cert.SignatureAlgorithm.Value ?? "—",
            PublicKeyAlgorithm: cert.PublicKey.Oid.FriendlyName ?? cert.PublicKey.Oid.Value ?? "—",
            PublicKeyBits: GetKeyBits(cert),
            SpkiSha256Base64: Convert.ToBase64String(spkiHash),
            ThumbprintSha1Hex: Convert.ToHexString(sha1).ToLowerInvariant(),
            ThumbprintSha256Hex: Convert.ToHexString(sha256).ToLowerInvariant(),
            KeyUsages: ExtractKeyUsages(cert),
            ExtendedKeyUsages: ExtractEku(cert),
            SubjectAlternativeNames: ExtractSan(cert),
            PemSource: pem);
    }

    private static int GetKeyBits(X509Certificate2 cert)
    {
        using var rsa = cert.GetRSAPublicKey();
        if (rsa is not null)
        {
            return rsa.KeySize;
        }
        using var ecdsa = cert.GetECDsaPublicKey();
        if (ecdsa is not null)
        {
            return ecdsa.KeySize;
        }
        return 0;
    }

    private static IReadOnlyList<string> ExtractKeyUsages(X509Certificate2 cert)
    {
        foreach (var ext in cert.Extensions)
        {
            if (ext is X509KeyUsageExtension ku)
            {
                var flags = ku.KeyUsages;
                var list = new List<string>();
                foreach (X509KeyUsageFlags f in Enum.GetValues<X509KeyUsageFlags>())
                {
                    if (f != X509KeyUsageFlags.None && (flags & f) != 0)
                    {
                        list.Add(f.ToString());
                    }
                }
                return list;
            }
        }
        return Array.Empty<string>();
    }

    private static IReadOnlyList<string> ExtractEku(X509Certificate2 cert)
    {
        foreach (var ext in cert.Extensions)
        {
            if (ext is X509EnhancedKeyUsageExtension eku)
            {
                return eku.EnhancedKeyUsages
                    .OfType<System.Security.Cryptography.Oid>()
                    .Select(o => o.FriendlyName ?? o.Value ?? "—")
                    .ToArray();
            }
        }
        return Array.Empty<string>();
    }

    private static IReadOnlyList<string> ExtractSan(X509Certificate2 cert)
    {
        foreach (var ext in cert.Extensions)
        {
            if (ext.Oid?.Value == "2.5.29.17")
            {
                var raw = ext.Format(true);
                return raw.Split(LineSeparators, StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries);
            }
        }
        return Array.Empty<string>();
    }
}
