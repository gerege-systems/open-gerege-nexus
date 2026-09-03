using System.Runtime.InteropServices;
using System.Runtime.Versioning;
using System.Security.Cryptography;
using System.Security.Cryptography.X509Certificates;
using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Domain.Primitives;
using NexusGeregeDesktop.Domain.Tokens;
using Microsoft.Extensions.Logging;

namespace NexusGeregeDesktop.Infrastructure.Tokens.Cng;

[SupportedOSPlatform("windows")]
internal sealed class WindowsCngTokenSession : ITokenSession
{
    private readonly X509Certificate2 _cert;
    private readonly ILogger _logger;
    private bool _disposed;

    public WindowsCngTokenSession(TokenInfo info, X509Certificate2 cert, ILogger logger)
    {
        Info = info;
        _cert = cert;
        _logger = logger;
    }

    public TokenInfo Info { get; }

    public bool IsLoggedIn => true; // OS handles PIN on first signing call.

    public Task<Result> LoginAsync(string pin, CancellationToken ct = default)
    {
        _ = pin;
        return Task.FromResult(Result.Success());
    }

    public Task<Result> LogoutAsync(CancellationToken ct = default) =>
        Task.FromResult(Result.Success());

    public Task<Result> ChangePinAsync(string oldPin, string newPin, CancellationToken ct = default)
    {
        // Windows CNG / Smart Card minidriver does not expose a public
        // user-PIN-change API. Two paths are available, both out of
        // scope for the in-app flow:
        //   1. Vendor PKI tool (EnterSafe PKI Manager → Change User PIN)
        //   2. Win32 SCardChangeAuthenticator via interop on the
        //      smart-card minidriver — token-specific reader required.
        // Citizens on a CNG-discovered token are directed to the vendor
        // tool. PKCS#11 backend (Pkcs11TokenSession) implements the
        // real C_SetPIN flow.
        _ = oldPin; _ = newPin;
        return Task.FromResult(Result.Failure(ApiError.Internal(
            "not_implemented_cng",
            "Windows CNG cannot change a smart-card PIN programmatically. " +
            "Open the vendor's PKI tool (e.g. EnterSafe PKI Manager → Change User PIN).")));
    }

    public Task<Result<IReadOnlyList<TokenObject>>> ListObjectsAsync(CancellationToken ct = default)
    {
        var objects = new List<TokenObject>(2);
        WindowsCngTokenProvider.TryGetCngKey(_cert, out var key, out var alg);
        using (key)
        {
            var keyAlg = alg == CngAlgorithm.ECDsaP256 || alg == CngAlgorithm.ECDsaP384 || alg == CngAlgorithm.ECDsaP521
                ? "EC"
                : "RSA";
            var bits = GetKeyBits();
            objects.Add(new TokenObject(
                Id: Info.Id,
                Label: _cert.GetNameInfo(X509NameType.SimpleName, false),
                Kind: TokenObjectKind.PrivateKey,
                KeyAlgorithm: keyAlg,
                KeyBits: bits,
                CanSign: true,
                CanDecrypt: keyAlg == "RSA"));
            objects.Add(new TokenObject(
                Id: _cert.Thumbprint,
                Label: _cert.Subject,
                Kind: TokenObjectKind.Certificate,
                KeyAlgorithm: keyAlg,
                KeyBits: bits,
                CanSign: false,
                CanDecrypt: false));
        }
        return Task.FromResult(Result<IReadOnlyList<TokenObject>>.Success((IReadOnlyList<TokenObject>)objects));
    }

    public Task<Result<byte[]>> ReadCertificateAsync(string objectId, CancellationToken ct = default)
    {
        if (objectId == _cert.Thumbprint || objectId == Info.Id)
        {
            return Task.FromResult(Result<byte[]>.Success(_cert.Export(X509ContentType.Cert)));
        }
        return Task.FromResult(Result<byte[]>.Failure(ApiError.NotFound("not_found")));
    }

    public Task<Result<byte[]>> SignAsync(string keyId, byte[] data, SigningAlg algorithm, CancellationToken ct = default)
    {
        ArgumentNullException.ThrowIfNull(data);
        try
        {
            var hash = ComputeHash(data, algorithm);
            using var rsa = _cert.GetRSAPrivateKey();
            using var ec = _cert.GetECDsaPrivateKey();
            if (rsa is not null && IsRsaAlg(algorithm))
            {
                var sig = rsa.SignHash(hash, HashAlgFromAlg(algorithm), RSASignaturePadding.Pkcs1);
                return Task.FromResult(Result<byte[]>.Success(sig));
            }
            if (ec is not null && IsEcAlg(algorithm))
            {
                var sig = ec.SignHash(hash);
                return Task.FromResult(Result<byte[]>.Success(sig));
            }
            return Task.FromResult(Result<byte[]>.Failure(ApiError.BadRequest(
                "algorithm_key_mismatch",
                $"Cert key type does not match {algorithm}.")));
        }
        catch (CryptographicException ex) when (ex.HResult == unchecked((int)0x80090027))
        {
            return Task.FromResult(Result<byte[]>.Failure(ApiError.Unauthorized("pin_required", ex.Message)));
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "CNG sign failed");
            return Task.FromResult(Result<byte[]>.Failure(ApiError.Internal("cng_sign_failed", ex.Message)));
        }
    }

    public Task<Result<TokenObject>> GenerateKeyPairAsync(KeyParams parameters, CancellationToken ct = default)
    {
        // M9.D.4 — generate an on-card RSA/ECDSA keypair via Microsoft
        // Smart Card Key Storage Provider. Windows shows the smart-card
        // PIN dialog during creation (the KSP requires user-PIN auth
        // before it can write a new container).
        //
        // Container name strategy: callers pass a stable label via
        // KeyParams.Label. We don't reuse an existing container — the
        // KSP would reject Create if it exists. Suffix with a short
        // random token so re-runs don't collide.
        try
        {
            var alg = parameters.Algorithm switch
            {
                KeyAlgorithm.Rsa       => CngAlgorithm.Rsa,
                KeyAlgorithm.EcdsaP256 => CngAlgorithm.ECDsaP256,
                KeyAlgorithm.EcdsaP384 => CngAlgorithm.ECDsaP384,
                KeyAlgorithm.EcdsaP521 => CngAlgorithm.ECDsaP521,
                _                      => CngAlgorithm.Rsa,
            };

            var containerName = $"eidmn-{Guid.NewGuid():N}".Substring(0, 24);

            var creationParams = new CngKeyCreationParameters
            {
                Provider = new CngProvider("Microsoft Smart Card Key Storage Provider"),
                // sensitive=true, non-exportable — matches the
                // citizen-cert profile in the backend CA.
                ExportPolicy = CngExportPolicies.None,
                KeyUsage = CngKeyUsages.Signing,
                KeyCreationOptions = CngKeyCreationOptions.None,
            };
            if (parameters.Algorithm == KeyAlgorithm.Rsa)
            {
                creationParams.Parameters.Add(new CngProperty(
                    "Length",
                    BitConverter.GetBytes(parameters.RsaBits),
                    CngPropertyOptions.None));
            }

            using var key = CngKey.Create(alg, containerName, creationParams);

            var keyAlg = parameters.Algorithm == KeyAlgorithm.Rsa ? "RSA" : "EC";
            var bits = parameters.Algorithm switch
            {
                KeyAlgorithm.Rsa       => parameters.RsaBits,
                KeyAlgorithm.EcdsaP256 => 256,
                KeyAlgorithm.EcdsaP384 => 384,
                KeyAlgorithm.EcdsaP521 => 521,
                _                      => parameters.RsaBits,
            };

            return Task.FromResult(Result<TokenObject>.Success(new TokenObject(
                Id: containerName,
                Label: parameters.Label,
                Kind: TokenObjectKind.PrivateKey,
                KeyAlgorithm: keyAlg,
                KeyBits: bits,
                CanSign: true,
                CanDecrypt: parameters.Algorithm == KeyAlgorithm.Rsa)));
        }
        catch (CryptographicException ex) when (ex.HResult == unchecked((int)0x80090027))
        {
            return Task.FromResult(Result<TokenObject>.Failure(
                ApiError.Unauthorized("pin_required", ex.Message)));
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "CNG keypair generation failed");
            return Task.FromResult(Result<TokenObject>.Failure(ApiError.Internal(
                $"cng_keygen_failed: {ex.GetType().Name}: {ex.Message} (HR=0x{ex.HResult:X8})",
                ex.StackTrace)));
        }
    }

    public Task<Result<TokenObject>> ImportCertificateAsync(byte[] derCert, string keyId, string? label, CancellationToken ct = default) =>
        Task.FromResult(Result<TokenObject>.Failure(ApiError.Internal(
            "not_implemented",
            "Cert provisioning for CNG happens through the Windows Cert Store + KSP association.")));

    public Task<Result<string>> BuildCsrAsync(string keyId, string subjectCn, CancellationToken ct = default)
    {
        try
        {
            // We sign with the cert's private key (delegates to the smart
            // card via Microsoft Smart Card KSP). keyId is informational —
            // the WindowsCngTokenSession is bound to a single cert at
            // construction time, so the key in use is unambiguous.
            _ = keyId;
            var safeCn = string.IsNullOrWhiteSpace(subjectCn) ? "e-ID Mongolia citizen" : subjectCn.Trim();
            var name = new X500DistinguishedName($"CN={safeCn}");

            using var rsa = _cert.GetRSAPrivateKey();
            if (rsa is not null)
            {
                var req = new CertificateRequest(name, rsa, HashAlgorithmName.SHA256, RSASignaturePadding.Pkcs1);
                var der = req.CreateSigningRequest();
                return Task.FromResult(Result<string>.Success(DerToCsrPem(der)));
            }

            using var ec = _cert.GetECDsaPrivateKey();
            if (ec is not null)
            {
                var req = new CertificateRequest(name, ec, HashAlgorithmName.SHA256);
                var der = req.CreateSigningRequest();
                return Task.FromResult(Result<string>.Success(DerToCsrPem(der)));
            }

            return Task.FromResult(Result<string>.Failure(ApiError.Internal(
                "cng_no_private_key",
                "Cert has no usable private key handle.")));
        }
        catch (CryptographicException ex) when (ex.HResult == unchecked((int)0x80090027))
        {
            // SCARD_W_CANCELLED_BY_USER / PIN canceled.
            return Task.FromResult(Result<string>.Failure(ApiError.Unauthorized("pin_required", ex.Message)));
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "CSR build failed");
            return Task.FromResult(Result<string>.Failure(ApiError.Internal("csr_build_failed", ex.Message)));
        }
    }

    public Task<Result> InstallIssuedCertificateAsync(byte[] derCert, CancellationToken ct = default)
    {
        try
        {
            // CopyWithPrivateKey for a smart-card-backed RSACng returns
            // NTE_INVALID_PARAMETER (0x80090027) because the KSP rejects
            // an unrelated cert being attached to an existing on-card
            // key. The Win32 contract — set CERT_KEY_PROV_INFO_PROP_ID
            // on the cert context directly with the matching container
            // — is what the Smart Card KSP actually understands. We
            // call CertSetCertificateContextProperty via P/Invoke.
            //
            // After this, X509Store.Add persists both the cert blob AND
            // the prov-info pointer, so next enumeration returns
            // HasPrivateKey=true on the new cert and discovery picks it
            // up via WindowsCngTokenProvider.ListTokensAsync.
            if (!WindowsCngTokenProvider.TryGetCngKey(_cert, out var srcKey, out _))
            {
                return Task.FromResult(Result.Failure(ApiError.Internal(
                    "cng_source_key_unavailable",
                    "Cannot read the on-card key handle from the current cert.")));
            }

            string containerName;
            string providerName;
            using (srcKey)
            {
                containerName = srcKey.KeyName ?? string.Empty;
                providerName = srcKey.Provider?.Provider ?? "Microsoft Smart Card Key Storage Provider";
            }
            if (string.IsNullOrEmpty(containerName))
            {
                return Task.FromResult(Result.Failure(ApiError.Internal(
                    "cng_no_container_name",
                    "On-card key has no container name to bind the cert to.")));
            }

            using var issued = new X509Certificate2(derCert);
            BindCertToKspContainer(issued, providerName, containerName);

            using var store = new X509Store(StoreName.My, StoreLocation.CurrentUser);
            store.Open(OpenFlags.ReadWrite);
            store.Add(issued);

            // Stash the issued cert under the container name so the
            // provider can re-bind it on every reinsert. Smart Card
            // propagation drops in-store-only certs when the card is
            // removed; this disk cache survives that and lets discovery
            // restore the binding the next time the same on-card key
            // shows up.
            var pem = DerToPem(derCert);
            new EnrolledCertCache(_logger).Save(containerName, pem);
            return Task.FromResult(Result.Success());
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Install issued cert failed");
            return Task.FromResult(Result.Failure(ApiError.Internal(
                $"install: {ex.GetType().Name}: {ex.Message} (HR=0x{ex.HResult:X8})",
                ex.StackTrace)));
        }
    }

    // ----- Win32 cert-context property: bind CA cert to KSP container ---

    private const uint CERT_KEY_PROV_INFO_PROP_ID = 2;
    private const uint CERT_NCRYPT_KEY_SPEC = 0xFFFFFFFF; // CNG key

    [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
    private struct CRYPT_KEY_PROV_INFO
    {
        public string pwszContainerName;
        public string pwszProvName;
        public uint dwProvType;
        public uint dwFlags;
        public uint cProvParam;
        public IntPtr rgProvParam;
        public uint dwKeySpec;
    }

    [DllImport("crypt32.dll", SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool CertSetCertificateContextProperty(
        IntPtr pCertContext,
        uint dwPropId,
        uint dwFlags,
        ref CRYPT_KEY_PROV_INFO pvData);

    public Task<Result> DeleteObjectAsync(string objectId, TokenObjectKind kind, CancellationToken ct = default)
    {
        try
        {
            if (kind == TokenObjectKind.Certificate)
            {
                // Cert objectId for CNG = thumbprint OR container name
                // (see ListObjectsAsync). Iterate the store and remove
                // by thumbprint match; we keep the on-card key intact
                // so a re-import / re-enroll can rebind cleanly later.
                using var store = new X509Store(StoreName.My, StoreLocation.CurrentUser);
                store.Open(OpenFlags.ReadWrite);
                X509Certificate2? hit = null;
                foreach (var cert in store.Certificates)
                {
                    if (string.Equals(cert.Thumbprint, objectId, StringComparison.OrdinalIgnoreCase))
                    {
                        hit = cert;
                        break;
                    }
                }
                if (hit is null)
                {
                    return Task.FromResult(Result.Failure(ApiError.NotFound("cert_not_found")));
                }
                store.Remove(hit);
                return Task.FromResult(Result.Success());
            }

            if (kind == TokenObjectKind.PrivateKey || kind == TokenObjectKind.PublicKey)
            {
                // CNG private-key delete = CngKey.Delete() on the
                // container. This writes through the Smart Card KSP to
                // the card itself — irreversible. objectId for keys is
                // the container name (see ListObjectsAsync).
                using var key = System.Security.Cryptography.CngKey.Open(
                    objectId,
                    new System.Security.Cryptography.CngProvider("Microsoft Smart Card Key Storage Provider"),
                    System.Security.Cryptography.CngKeyOpenOptions.Silent);
                key.Delete();
                return Task.FromResult(Result.Success());
            }

            return Task.FromResult(Result.Failure(ApiError.BadRequest("unsupported_object_kind")));
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "CNG delete object failed");
            return Task.FromResult(Result.Failure(ApiError.Internal(
                $"cng_delete_failed: {ex.GetType().Name}: {ex.Message} (HR=0x{ex.HResult:X8})")));
        }
    }

    private static string DerToPem(byte[] der)
    {
        var b64 = Convert.ToBase64String(der);
        var sb = new System.Text.StringBuilder(b64.Length + 64);
        sb.Append("-----BEGIN CERTIFICATE-----\n");
        for (var i = 0; i < b64.Length; i += 64)
        {
            sb.Append(b64.AsSpan(i, Math.Min(64, b64.Length - i)));
            sb.Append('\n');
        }
        sb.Append("-----END CERTIFICATE-----\n");
        return sb.ToString();
    }

    private static void BindCertToKspContainer(X509Certificate2 cert, string providerName, string containerName)
    {
        var info = new CRYPT_KEY_PROV_INFO
        {
            pwszContainerName = containerName,
            pwszProvName = providerName,
            dwProvType = 0,                     // 0 == CNG (vs legacy CSP)
            dwFlags = 0,
            cProvParam = 0,
            rgProvParam = IntPtr.Zero,
            dwKeySpec = CERT_NCRYPT_KEY_SPEC,   // signals CNG
        };
        if (!CertSetCertificateContextProperty(cert.Handle, CERT_KEY_PROV_INFO_PROP_ID, 0, ref info))
        {
            var err = Marshal.GetLastWin32Error();
            throw new CryptographicException(
                $"CertSetCertificateContextProperty failed (Win32={err:X8}, container={containerName}, provider={providerName})");
        }
    }

    private static string DerToCsrPem(byte[] der)
    {
        // .NET 5+ has PemEncoding.Write but that returns a char[] with
        // platform-specific line endings; manual base64 + LF keeps the
        // wire format stable for the Go server's pem.Decode (which is
        // line-ending tolerant but a fixed format helps debugging).
        var b64 = Convert.ToBase64String(der);
        var sb = new System.Text.StringBuilder(b64.Length + 64);
        sb.Append("-----BEGIN CERTIFICATE REQUEST-----\n");
        for (var i = 0; i < b64.Length; i += 64)
        {
            sb.Append(b64.AsSpan(i, Math.Min(64, b64.Length - i)));
            sb.Append('\n');
        }
        sb.Append("-----END CERTIFICATE REQUEST-----\n");
        return sb.ToString();
    }

    private int GetKeyBits()
    {
        try
        {
            using var rsa = _cert.GetRSAPublicKey();
            if (rsa is not null) return rsa.KeySize;
            using var ec = _cert.GetECDsaPublicKey();
            if (ec is not null) return ec.KeySize;
        }
        catch { }
        return 0;
    }

    private static byte[] ComputeHash(byte[] data, SigningAlg alg)
    {
        HashAlgorithmName name = HashAlgFromAlg(alg);
        return name.Name switch
        {
            nameof(HashAlgorithmName.SHA256) => SHA256.HashData(data),
            nameof(HashAlgorithmName.SHA384) => SHA384.HashData(data),
            nameof(HashAlgorithmName.SHA512) => SHA512.HashData(data),
            _ => SHA256.HashData(data),
        };
    }

    private static HashAlgorithmName HashAlgFromAlg(SigningAlg alg) => alg switch
    {
        SigningAlg.Sha256WithRsa or SigningAlg.EcdsaSha256 => HashAlgorithmName.SHA256,
        SigningAlg.Sha384WithRsa or SigningAlg.EcdsaSha384 => HashAlgorithmName.SHA384,
        SigningAlg.Sha512WithRsa or SigningAlg.EcdsaSha512 => HashAlgorithmName.SHA512,
        _ => HashAlgorithmName.SHA256,
    };

    private static bool IsRsaAlg(SigningAlg a) =>
        a is SigningAlg.Sha256WithRsa or SigningAlg.Sha384WithRsa or SigningAlg.Sha512WithRsa;

    private static bool IsEcAlg(SigningAlg a) =>
        a is SigningAlg.EcdsaSha256 or SigningAlg.EcdsaSha384 or SigningAlg.EcdsaSha512;

    public void Dispose()
    {
        if (_disposed) return;
        _disposed = true;
        _cert.Dispose();
    }

    public ValueTask DisposeAsync()
    {
        Dispose();
        return ValueTask.CompletedTask;
    }
}
