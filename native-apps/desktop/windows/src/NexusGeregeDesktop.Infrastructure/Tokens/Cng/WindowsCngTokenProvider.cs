using System.Runtime.InteropServices;
using System.Runtime.Versioning;
using System.Security.Cryptography;
using System.Security.Cryptography.X509Certificates;
using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Domain.Primitives;
using NexusGeregeDesktop.Domain.Tokens;
using Microsoft.Extensions.Logging;

namespace NexusGeregeDesktop.Infrastructure.Tokens.Cng;

/// Reads smart-card / USB-token-resident certs via the Windows Cert Store
/// (CurrentUser\My is populated by Microsoft Base Smart Card Crypto Provider
/// when a card is inserted) and signs through CNG.
///
/// Limitations:
///   * Keypair generation goes through "Microsoft Smart Card Key Storage
///     Provider" using CngKeyCreationParameters with PIN policy = Default
///     (Windows pops the native PIN UX).
///   * The PIN parameter on OpenSession is ignored; Windows handles it.
///   * Certificate import works via X509Store.Add (the underlying KSP
///     associates by SubjectKeyIdentifier).
[SupportedOSPlatform("windows")]
public sealed class WindowsCngTokenProvider : ICryptoTokenProvider
{
    private const string SmartCardKsp = "Microsoft Smart Card Key Storage Provider";

    private readonly ILogger<WindowsCngTokenProvider> _logger;

    public WindowsCngTokenProvider(ILogger<WindowsCngTokenProvider> logger)
    {
        _logger = logger;
    }

    public string Id => "windows-cng";

    public string DisplayName => "Windows Smart Card (CNG)";

    public TokenBackend Backend => TokenBackend.WindowsCng;

    public bool IsAvailable => OperatingSystem.IsWindows() && SmartCardProviderExists();

    public Task<Result<IReadOnlyList<TokenInfo>>> ListTokensAsync(CancellationToken ct = default)
    {
        if (!IsAvailable)
        {
            return Task.FromResult(Result<IReadOnlyList<TokenInfo>>.Success(Array.Empty<TokenInfo>()));
        }

        try
        {
            // CurrentUser\My is where the Smart Card KSP places certs from
            // inserted cards. After M9.C enrollment there can be MULTIPLE
            // certs that share the same on-card private key (one self-
            // signed factory cert + one CA-issued enrollment cert). We
            // dedupe by container name but prefer the CA-issued one so
            // /web2app/v1/auth/token/verify can resolve cert.serial → user_id.
            // First pass — gather all on-card containers + their best
            // cert from the user cert store. Second pass below re-binds
            // any cached enrollment certs that Windows lost when the
            // card was last removed.
            var seen = new Dictionary<string, X509Certificate2>(StringComparer.Ordinal);
            using (var store = new X509Store(StoreName.My, StoreLocation.CurrentUser))
            {
                store.Open(OpenFlags.ReadOnly | OpenFlags.OpenExistingOnly);
                foreach (var cert in store.Certificates)
                {
                    if (!cert.HasPrivateKey)
                    {
                        continue;
                    }
                    if (!TryGetCngKey(cert, out var key, out _))
                    {
                        continue;
                    }
                    using (key)
                    {
                        if (key.Provider is null
                            || !string.Equals(key.Provider.Provider, SmartCardKsp, StringComparison.OrdinalIgnoreCase))
                        {
                            continue;
                        }
                        var containerName = key.KeyName ?? cert.Thumbprint;
                        if (!seen.TryGetValue(containerName, out var prev))
                        {
                            seen[containerName] = cert;
                            continue;
                        }
                        // Tiebreaker: prefer a cert that is NOT self-signed
                        // (subject != issuer means a real CA signed it). If
                        // both are self-signed (factory state) or both are
                        // CA-issued (re-enrolled), prefer the one with the
                        // later NotBefore — newer wins.
                        if (PreferReplacement(prev, cert))
                        {
                            seen[containerName] = cert;
                        }
                    }
                }
            }

            // Second pass — for each on-card container that surfaced
            // only as a self-signed factory cert, check the enrolled-
            // cert cache. If we previously enrolled this container with
            // a CA-signed cert, re-import it now so login + signing see
            // the right cert. This is the post-reinsert recovery path —
            // Windows smart-card propagation drops in-store-only certs
            // on card removal; we put them back on next discovery.
            RebindCachedCerts(seen);
            var infos = seen
                .Select(kv => new TokenInfo(
                    Id: kv.Key,
                    Label: kv.Value.GetNameInfo(X509NameType.SimpleName, false),
                    Manufacturer: "Smart Card KSP",
                    SerialNumber: kv.Value.SerialNumber,
                    Model: kv.Value.Issuer,
                    Backend: TokenBackend.WindowsCng,
                    Removable: true,
                    HardwareSlot: true,
                    IsLoggedIn: false,
                    State: ClassifyCngState(kv.Value),
                    // CNG path has no PKCS#11-style flag introspection. If the
                    // cert is visible here, SO + user PIN are necessarily set
                    // (the OS would not have placed the cert in CurrentUser\My
                    // otherwise). Biometric detection is vendor-specific and
                    // requires WBF (Windows Biometric Framework) — leave it
                    // as a heuristic on model string for now.
                    Capabilities: new TokenCapabilities(
                        ProtectedAuthPath: false,
                        BiometricSensor: GuessBiometric(kv.Value),
                        SoPinInitialized: true,
                        UserPinInitialized: true,
                        CanGenerateKeyPair: true),
                    PublicKeyFingerprint: ComputeFingerprint(kv.Value)))
                .ToArray();
            return Task.FromResult(Result<IReadOnlyList<TokenInfo>>.Success((IReadOnlyList<TokenInfo>)infos));
        }
        catch (Exception ex) when (ex is CryptographicException or System.ComponentModel.Win32Exception)
        {
            _logger.LogWarning(ex, "CNG enumeration failed");
            return Task.FromResult(Result<IReadOnlyList<TokenInfo>>.Failure(ApiError.Internal("cng_enumeration_failed", ex.Message)));
        }
    }

    private static TokenState ClassifyCngState(X509Certificate2 cert)
    {
        // Self-signed → factory-provisioned but not yet enrolled with
        // our CA. CSR + enroll is the next action. CA-issued cert →
        // ready to log in via the token.
        var selfSigned = string.Equals(cert.Subject, cert.Issuer, StringComparison.Ordinal);
        return selfSigned ? TokenState.Provisioned : TokenState.Enrolled;
    }

    /// Tokens with built-in fingerprint sensors usually carry "Bio",
    /// "BioPass", or "Fingerprint" in their cert issuer / subject
    /// strings, or the smart card minidriver injects a recognizable
    /// substring. Heuristic only — promote to true positive once we
    /// wire Windows Biometric Framework lookups in M9.D.5.
    /// SHA-256 of the cert's RSA modulus (or EC public point), hex-
    /// encoded. Used as the cross-backend dedupe key for tokens that
    /// surface in both CNG and PKCS#11 listings. Returns null when the
    /// cert isn't RSA/EC or the public key can't be exported.
    private static string? ComputeFingerprint(X509Certificate2 cert)
    {
        try
        {
            using var rsa = cert.GetRSAPublicKey();
            if (rsa is not null)
            {
                var modulus = rsa.ExportParameters(false).Modulus;
                if (modulus is null) return null;
                var trimmed = TrimLeadingZeros(modulus);
                return Convert.ToHexString(System.Security.Cryptography.SHA256.HashData(trimmed));
            }
            using var ec = cert.GetECDsaPublicKey();
            if (ec is not null)
            {
                var p = ec.ExportParameters(false).Q;
                if (p.X is null || p.Y is null) return null;
                var combined = new byte[p.X.Length + p.Y.Length];
                Buffer.BlockCopy(p.X, 0, combined, 0, p.X.Length);
                Buffer.BlockCopy(p.Y, 0, combined, p.X.Length, p.Y.Length);
                return Convert.ToHexString(System.Security.Cryptography.SHA256.HashData(combined));
            }
        }
        catch
        {
            // Non-fatal — just skip dedup for this entry.
        }
        return null;
    }

    private static byte[] TrimLeadingZeros(byte[] x)
    {
        int i = 0;
        while (i < x.Length - 1 && x[i] == 0) i++;
        if (i == 0) return x;
        var result = new byte[x.Length - i];
        Buffer.BlockCopy(x, i, result, 0, result.Length);
        return result;
    }

    private static bool GuessBiometric(X509Certificate2 cert)
    {
        var hay = (cert.Subject + " " + cert.Issuer);
        return hay.Contains("Bio", StringComparison.OrdinalIgnoreCase)
            || hay.Contains("Fingerprint", StringComparison.OrdinalIgnoreCase);
    }

    /// Walks the discovered containers and, for any container whose
    /// only visible cert is self-signed (factory state), looks for a
    /// cached CA-signed cert from a prior enrollment. If one exists,
    /// re-binds it to the on-card key via CertSetCertificateContext
    /// Property + X509Store.Add. Updates the input map with the
    /// recovered cert in place.
    private void RebindCachedCerts(Dictionary<string, X509Certificate2> seen)
    {
        var cache = new EnrolledCertCache(_logger);
        foreach (var pair in seen.ToArray())
        {
            var containerName = pair.Key;
            var visible = pair.Value;
            // Skip containers that already have a CA-issued cert.
            if (!string.Equals(visible.Subject, visible.Issuer, StringComparison.Ordinal))
            {
                continue;
            }
            var cached = cache.TryLoad(containerName);
            if (cached is null)
            {
                continue;
            }
            try
            {
                // Re-apply the KSP linkage to the cached cert. The
                // container name is the SAME as before (smart-card
                // KSP uses stable per-card container ids), so the
                // CertSetCertificateContextProperty call rebinds
                // cleanly without needing the issuer to re-issue.
                BindCertToContainer(cached, SmartCardKsp, containerName);
                using var store = new X509Store(StoreName.My, StoreLocation.CurrentUser);
                store.Open(OpenFlags.ReadWrite);
                store.Add(cached);
                seen[containerName] = cached;
                _logger.LogInformation("RebindCachedCerts: restored cert for container {Container}", containerName);
            }
            catch (Exception ex)
            {
                _logger.LogWarning(ex, "RebindCachedCerts: failed for container {Container}", containerName);
            }
        }
    }

    // ---- CertSetCertificateContextProperty P/Invoke (mirror of the
    //      same helper in WindowsCngTokenSession; duplicated rather
    //      than exported to keep the session's surface focused). ----

    private const uint CERT_KEY_PROV_INFO_PROP_ID = 2;
    private const uint CERT_NCRYPT_KEY_SPEC = 0xFFFFFFFF;

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
        IntPtr pCertContext, uint dwPropId, uint dwFlags, ref CRYPT_KEY_PROV_INFO pvData);

    private static void BindCertToContainer(X509Certificate2 cert, string providerName, string containerName)
    {
        var info = new CRYPT_KEY_PROV_INFO
        {
            pwszContainerName = containerName,
            pwszProvName = providerName,
            dwProvType = 0,
            dwFlags = 0,
            cProvParam = 0,
            rgProvParam = IntPtr.Zero,
            dwKeySpec = CERT_NCRYPT_KEY_SPEC,
        };
        if (!CertSetCertificateContextProperty(cert.Handle, CERT_KEY_PROV_INFO_PROP_ID, 0, ref info))
        {
            var err = Marshal.GetLastWin32Error();
            throw new CryptographicException(
                $"CertSetCertificateContextProperty failed (Win32={err:X8}, container={containerName})");
        }
    }

    private static bool PreferReplacement(X509Certificate2 current, X509Certificate2 candidate)
    {
        var currentSelfSigned = string.Equals(current.Subject, current.Issuer, StringComparison.Ordinal);
        var candidateSelfSigned = string.Equals(candidate.Subject, candidate.Issuer, StringComparison.Ordinal);
        // Replace self-signed factory cert with a CA-issued one.
        if (currentSelfSigned && !candidateSelfSigned) return true;
        // Don't replace CA-issued with self-signed.
        if (!currentSelfSigned && candidateSelfSigned) return false;
        // Both same flavor — newer NotBefore wins (handles re-enrollment).
        return candidate.NotBefore > current.NotBefore;
    }

    public Task<Result<ITokenSession>> OpenSessionAsync(string tokenId, string? pin = null, CancellationToken ct = default)
    {
        _ = pin; // Windows handles the PIN dialog itself.
        try
        {
            using var store = new X509Store(StoreName.My, StoreLocation.CurrentUser);
            store.Open(OpenFlags.ReadOnly | OpenFlags.OpenExistingOnly);

            // Iterate the WHOLE store (no early break) so we can apply
            // PreferReplacement when multiple certs share the requested
            // container — same logic as ListTokensAsync, so login picks
            // the CA-issued cert over the self-signed factory one.
            X509Certificate2? selected = null;
            foreach (var cert in store.Certificates)
            {
                if (!cert.HasPrivateKey) continue;
                if (!TryGetCngKey(cert, out var key, out _)) continue;
                try
                {
                    if (!string.Equals(key.KeyName, tokenId, StringComparison.Ordinal))
                    {
                        continue;
                    }
                    if (selected is null || PreferReplacement(selected, cert))
                    {
                        selected = cert;
                    }
                }
                finally
                {
                    key.Dispose();
                }
            }

            if (selected is null)
            {
                return Task.FromResult(Result<ITokenSession>.Failure(
                    ApiError.NotFound("token_not_found", $"No CNG container '{tokenId}'.")));
            }

            var info = new TokenInfo(tokenId,
                selected.GetNameInfo(X509NameType.SimpleName, false),
                "Smart Card KSP",
                selected.SerialNumber,
                selected.Issuer,
                TokenBackend.WindowsCng,
                Removable: true,
                HardwareSlot: true,
                IsLoggedIn: true);

            var session = new WindowsCngTokenSession(info, selected, _logger);
            return Task.FromResult(Result<ITokenSession>.Success((ITokenSession)session));
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Failed to open CNG session for {Token}", tokenId);
            return Task.FromResult(Result<ITokenSession>.Failure(ApiError.Internal("cng_open_failed", ex.Message)));
        }
    }

    private static bool SmartCardProviderExists()
    {
        // CngKeyOpenOptions.Silent: critical. Without it, the Smart Card KSP
        // can pop a Windows "select a smart card device" dialog (or PIN
        // enrollment UI for factory-state cards) when probed. Silent forces
        // CNG to fail synchronously without UI, which is what we want for an
        // availability check.
        try
        {
            using var test = CngKey.Open("DOES_NOT_EXIST", new CngProvider(SmartCardKsp), CngKeyOpenOptions.Silent);
            return true;
        }
        catch (CryptographicException ex) when (
            ex.HResult == unchecked((int)0x80090016) || // NTE_BAD_KEYSET — provider OK, key missing
            ex.HResult == unchecked((int)0x80090014) || // NTE_KEYSET_NOT_DEF — provider exists
            ex.HResult == unchecked((int)0x80090029))   // NTE_NOT_SUPPORTED — KSP installed but not initialised
        {
            return true;
        }
        catch
        {
            return false;
        }
    }

    internal static bool TryGetCngKey(X509Certificate2 cert, out CngKey key, out CngAlgorithm algorithm)
    {
        // Touching the private key on a smart-card-backed cert can wake the
        // KSP (PIN dialog, "insert card", enrolment UX). For enumeration we
        // need the key *handle* only, not signing power — so reopen via
        // CngKey.Open with Silent.
        try
        {
            string? containerName = null;
            CngProvider? provider = null;
            using (var rsa = cert.GetRSAPrivateKey() as RSACng)
            {
                if (rsa is not null)
                {
                    containerName = rsa.Key.KeyName;
                    provider = rsa.Key.Provider;
                }
            }
            if (containerName is null)
            {
                using var ec = cert.GetECDsaPrivateKey() as ECDsaCng;
                if (ec is not null)
                {
                    containerName = ec.Key.KeyName;
                    provider = ec.Key.Provider;
                }
            }
            if (containerName is not null && provider is not null)
            {
                key = CngKey.Open(containerName, provider, CngKeyOpenOptions.Silent);
                algorithm = key.Algorithm;
                return true;
            }
        }
        catch
        {
            // Swallow — UI-emitting branch.
        }
        key = null!;
        algorithm = CngAlgorithm.Rsa;
        return false;
    }

    public Task<Result<BiometricEnrollOutcome>> EnrollFingerprintAsync(string tokenId, string userPin, CancellationToken ct = default)
    {
        // Windows Smart Card KSP does not surface biometric enrollment
        // through CNG. Vendor-specific tools (FT BioPass Manager,
        // Yubico Authenticator's bio section, etc.) handle it. Default
        // posture: instruct the citizen to run the vendor tool.
        _ = tokenId; _ = userPin;
        return Task.FromResult(Result<BiometricEnrollOutcome>.Success(new BiometricEnrollOutcome(
            Mode: BiometricEnrollMode.DelegatedTool,
            FingerCount: 0,
            Instructions:
                "Windows CNG cannot enroll fingerprints on this token. " +
                "Please use the vendor's enrollment tool (e.g. FT BioPass Manager) " +
                "to register your fingerprint(s), then return here and click Refresh.")));
    }

    public Task<Result> ChangeSoPinAsync(string tokenId, string oldSoPin, string newSoPin, CancellationToken ct = default)
    {
        _ = tokenId; _ = oldSoPin; _ = newSoPin;
        return Task.FromResult(Result.Failure(ApiError.Internal(
            "not_implemented_cng",
            "Windows CNG cannot rotate a smart-card SO PIN programmatically. " +
            "Open the vendor's PKI tool to change the admin PIN.")));
    }

    public Task<Result> UnlockUserPinAsync(string tokenId, string soPin, string newUserPin, CancellationToken ct = default)
    {
        // Microsoft Smart Card KSP does not expose a "reset user PIN
        // with SO PIN" API through CNG. Vendor tools (EnterSafe PKI
        // Manager → Unlock User PIN, ActivClient, etc.) implement this
        // via the card minidriver's CardChangeAuthenticator/Ex with
        // PIN_CHANGE_FLAG_UNBLOCK. Caller should route to the vendor
        // PKI tool from the UI.
        _ = tokenId; _ = soPin; _ = newUserPin;
        return Task.FromResult(Result.Failure(ApiError.Internal(
            "not_implemented_cng",
            "Windows CNG cannot unlock a smart-card PIN programmatically. " +
            "Open the vendor's PKI tool (e.g. EnterSafe PKI Manager → Unlock User PIN).")));
    }

    public Task<Result> InitializeTokenAsync(string tokenId, string soPin, string label, string newUserPin, CancellationToken ct = default)
    {
        // Microsoft Smart Card KSP does not expose admin-PIN
        // initialization through CNG. The card vendor's tool (FT_ePass
        // 2003 Manager, ActivClient, GIDS-Tool, etc.) handles this
        // one-time operation. Callers should use the PKCS#11 backend
        // for blank-token init, then switch to CNG once the card is
        // initialized (the Smart Card KSP picks it up automatically
        // once a user PIN exists).
        _ = tokenId; _ = soPin; _ = label; _ = newUserPin;
        return Task.FromResult(Result.Failure(ApiError.Internal(
            "not_implemented_cng",
            "Use the PKCS#11 backend or the vendor tool to initialize a blank card. CNG re-discovers the card after PIN init completes.")));
    }
}
