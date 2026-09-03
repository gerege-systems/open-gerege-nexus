using System.Globalization;
using System.Security.Cryptography;
using System.Security.Cryptography.X509Certificates;
using System.Text;
using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Domain.Primitives;
using NexusGeregeDesktop.Domain.Tokens;
using Microsoft.Extensions.Logging;
using Net.Pkcs11Interop.Common;
using Net.Pkcs11Interop.HighLevelAPI;

namespace NexusGeregeDesktop.Infrastructure.Tokens.Pkcs11;

internal sealed class Pkcs11TokenSession : ITokenSession
{
    private readonly ISession _session;
    private readonly ILogger _logger;
    private bool _disposed;

    public Pkcs11TokenSession(TokenInfo info, ISession session, ILogger logger)
    {
        Info = info;
        _session = session;
        _logger = logger;
    }

    public TokenInfo Info { get; }

    public bool IsLoggedIn { get; private set; }

    /// True once any C_Login attempt (success OR fail) has been made on
    /// this session. Used to gate the eps2003csp11.dll first-call quirk
    /// workaround — see LoginAsync.
    private bool _loginAttempted;

    public Task<Result> LoginAsync(string pin, CancellationToken ct = default)
    {
        var firstAttempt = !_loginAttempted;
        try
        {
            _loginAttempted = true;
            _session.Login(CKU.CKU_USER, Encoding.UTF8.GetBytes(pin));
            IsLoggedIn = true;
            return Task.FromResult(Result.Success());
        }
        catch (Pkcs11Exception ex) when (ex.RV == CKR.CKR_PIN_INCORRECT && firstAttempt)
        {
            // Feitian eps2003csp11.dll quirk — the very first C_Login on
            // a freshly-opened session sometimes returns
            // CKR_PIN_INCORRECT even when the PIN is correct. A single
            // immediate retry succeeds reliably. We gate this on
            // _loginAttempted so a genuinely wrong PIN typed twice
            // (which arrives as two sequential LoginAsync calls) still
            // surfaces as pin_incorrect on the SECOND user-visible
            // attempt — only the very first interaction gets the auto-
            // retry, costing at most one extra counter decrement in the
            // rare case of a real first-time mistype (mitigated by: a
            // successful C_Login resets the PIN retry counter on the
            // card per the PKCS#11 spec).
            _logger.LogInformation(
                "First C_Login returned CKR_PIN_INCORRECT; retrying once to work around eps2003csp11.dll quirk");
            try
            {
                _session.Login(CKU.CKU_USER, Encoding.UTF8.GetBytes(pin));
                IsLoggedIn = true;
                return Task.FromResult(Result.Success());
            }
            catch (Pkcs11Exception retryEx) when (retryEx.RV == CKR.CKR_PIN_INCORRECT)
            {
                return Task.FromResult(Result.Failure(ApiError.Unauthorized("pin_incorrect")));
            }
            catch (Pkcs11Exception retryEx) when (retryEx.RV == CKR.CKR_PIN_LOCKED)
            {
                return Task.FromResult(Result.Failure(ApiError.Unauthorized("pin_locked")));
            }
            catch (Pkcs11Exception retryEx)
            {
                return Task.FromResult(Result.Failure(ApiError.Unauthorized("login_failed", retryEx.Message)));
            }
        }
        catch (Pkcs11Exception ex) when (ex.RV == CKR.CKR_PIN_INCORRECT)
        {
            return Task.FromResult(Result.Failure(ApiError.Unauthorized("pin_incorrect")));
        }
        catch (Pkcs11Exception ex) when (ex.RV == CKR.CKR_PIN_LOCKED)
        {
            return Task.FromResult(Result.Failure(ApiError.Unauthorized("pin_locked")));
        }
        catch (Pkcs11Exception ex)
        {
            return Task.FromResult(Result.Failure(ApiError.Unauthorized("login_failed", ex.Message)));
        }
    }

    public Task<Result> LogoutAsync(CancellationToken ct = default)
    {
        try
        {
            if (IsLoggedIn) _session.Logout();
            IsLoggedIn = false;
            return Task.FromResult(Result.Success());
        }
        catch (Pkcs11Exception ex)
        {
            return Task.FromResult(Result.Failure(ApiError.Internal("logout_failed", ex.Message)));
        }
    }

    public Task<Result> ChangePinAsync(string oldPin, string newPin, CancellationToken ct = default)
    {
        // C_SetPIN requires the session to be logged in as CKU_USER.
        // The token verifies the old PIN itself and rejects mismatched
        // ones with CKR_PIN_INCORRECT (no oracle — we don't need to
        // pre-verify on our side). ePass2003 enforces min length = 8
        // for user PINs via CKR_PIN_LEN_RANGE.
        if (!IsLoggedIn)
        {
            return Task.FromResult(Result.Failure(ApiError.Unauthorized("not_logged_in")));
        }
        try
        {
            _session.SetPin(
                Encoding.UTF8.GetBytes(oldPin),
                Encoding.UTF8.GetBytes(newPin));
            return Task.FromResult(Result.Success());
        }
        catch (Pkcs11Exception ex) when (ex.RV == CKR.CKR_PIN_INCORRECT)
        {
            return Task.FromResult(Result.Failure(ApiError.Unauthorized("pin_incorrect")));
        }
        catch (Pkcs11Exception ex) when (ex.RV == CKR.CKR_PIN_LOCKED)
        {
            return Task.FromResult(Result.Failure(ApiError.Unauthorized("pin_locked")));
        }
        catch (Pkcs11Exception ex) when (ex.RV == CKR.CKR_PIN_LEN_RANGE)
        {
            return Task.FromResult(Result.Failure(ApiError.BadRequest(
                "pin_length_invalid",
                "New PIN length outside the token's allowed range.")));
        }
        catch (Pkcs11Exception ex)
        {
            return Task.FromResult(Result.Failure(ApiError.Internal(
                $"pin_change_failed: RV=0x{(ulong)ex.RV:X} {ex.Message}",
                ex.StackTrace)));
        }
    }

    public Task<Result<IReadOnlyList<TokenObject>>> ListObjectsAsync(CancellationToken ct = default)
    {
        try
        {
            var results = new List<TokenObject>();
            // Private keys
            FindObjects(CKO.CKO_PRIVATE_KEY, results);
            // Public keys
            FindObjects(CKO.CKO_PUBLIC_KEY, results);
            // Certificates
            FindObjects(CKO.CKO_CERTIFICATE, results);
            return Task.FromResult(Result<IReadOnlyList<TokenObject>>.Success((IReadOnlyList<TokenObject>)results));
        }
        catch (Pkcs11Exception ex)
        {
            _logger.LogWarning(ex, "PKCS#11 list objects failed");
            return Task.FromResult(Result<IReadOnlyList<TokenObject>>.Failure(ApiError.Internal("list_failed", ex.Message)));
        }
    }

    public Task<Result<byte[]>> ReadCertificateAsync(string objectId, CancellationToken ct = default)
    {
        try
        {
            var template = new List<IObjectAttribute>
            {
                _session.Factories.ObjectAttributeFactory.Create(CKA.CKA_CLASS, CKO.CKO_CERTIFICATE),
                _session.Factories.ObjectAttributeFactory.Create(CKA.CKA_ID, HexToBytes(objectId)),
            };
            var found = _session.FindAllObjects(template);
            if (found.Count == 0)
            {
                return Task.FromResult(Result<byte[]>.Failure(ApiError.NotFound("cert_not_found")));
            }
            var attrs = _session.GetAttributeValue(found[0], new List<CKA> { CKA.CKA_VALUE });
            return Task.FromResult(Result<byte[]>.Success(attrs[0].GetValueAsByteArray()));
        }
        catch (Pkcs11Exception ex)
        {
            return Task.FromResult(Result<byte[]>.Failure(ApiError.Internal("read_cert_failed", ex.Message)));
        }
    }

    public Task<Result<byte[]>> SignAsync(string keyId, byte[] data, SigningAlg algorithm, CancellationToken ct = default)
    {
        ArgumentNullException.ThrowIfNull(data);
        if (!IsLoggedIn)
        {
            return Task.FromResult(Result<byte[]>.Failure(ApiError.Unauthorized("not_logged_in")));
        }
        try
        {
            var template = new List<IObjectAttribute>
            {
                _session.Factories.ObjectAttributeFactory.Create(CKA.CKA_CLASS, CKO.CKO_PRIVATE_KEY),
                _session.Factories.ObjectAttributeFactory.Create(CKA.CKA_ID, HexToBytes(keyId)),
            };
            var found = _session.FindAllObjects(template);
            if (found.Count == 0)
            {
                return Task.FromResult(Result<byte[]>.Failure(ApiError.NotFound("key_not_found")));
            }
            var key = found[0];
            var mech = _session.Factories.MechanismFactory.Create(MechanismFromAlg(algorithm));
            var sig = _session.Sign(mech, key, data);
            return Task.FromResult(Result<byte[]>.Success(sig));
        }
        catch (Pkcs11Exception ex)
        {
            return Task.FromResult(Result<byte[]>.Failure(ApiError.Internal("sign_failed", ex.Message)));
        }
    }

    public Task<Result<TokenObject>> GenerateKeyPairAsync(KeyParams parameters, CancellationToken ct = default)
    {
        if (!IsLoggedIn)
        {
            return Task.FromResult(Result<TokenObject>.Failure(ApiError.Unauthorized("not_logged_in")));
        }
        try
        {
            var id = RandomNumberGenerator.GetBytes(16);
            var publicAttrs = new List<IObjectAttribute>
            {
                _session.Factories.ObjectAttributeFactory.Create(CKA.CKA_CLASS, CKO.CKO_PUBLIC_KEY),
                _session.Factories.ObjectAttributeFactory.Create(CKA.CKA_TOKEN, true),
                _session.Factories.ObjectAttributeFactory.Create(CKA.CKA_LABEL, parameters.Label),
                _session.Factories.ObjectAttributeFactory.Create(CKA.CKA_ID, id),
                _session.Factories.ObjectAttributeFactory.Create(CKA.CKA_VERIFY, true),
            };
            var privateAttrs = new List<IObjectAttribute>
            {
                _session.Factories.ObjectAttributeFactory.Create(CKA.CKA_CLASS, CKO.CKO_PRIVATE_KEY),
                _session.Factories.ObjectAttributeFactory.Create(CKA.CKA_TOKEN, true),
                _session.Factories.ObjectAttributeFactory.Create(CKA.CKA_PRIVATE, true),
                _session.Factories.ObjectAttributeFactory.Create(CKA.CKA_SENSITIVE, parameters.Sensitive),
                _session.Factories.ObjectAttributeFactory.Create(CKA.CKA_EXTRACTABLE, parameters.Extractable),
                _session.Factories.ObjectAttributeFactory.Create(CKA.CKA_LABEL, parameters.Label),
                _session.Factories.ObjectAttributeFactory.Create(CKA.CKA_ID, id),
                _session.Factories.ObjectAttributeFactory.Create(CKA.CKA_SIGN, true),
            };

            IMechanism mech;
            string algName;
            int bits;
            switch (parameters.Algorithm)
            {
                case KeyAlgorithm.Rsa:
                    publicAttrs.Add(_session.Factories.ObjectAttributeFactory.Create(CKA.CKA_MODULUS_BITS, (ulong)parameters.RsaBits));
                    publicAttrs.Add(_session.Factories.ObjectAttributeFactory.Create(CKA.CKA_PUBLIC_EXPONENT, new byte[] { 0x01, 0x00, 0x01 }));
                    mech = _session.Factories.MechanismFactory.Create(CKM.CKM_RSA_PKCS_KEY_PAIR_GEN);
                    algName = "RSA";
                    bits = parameters.RsaBits;
                    break;
                case KeyAlgorithm.EcdsaP256:
                case KeyAlgorithm.EcdsaP384:
                case KeyAlgorithm.EcdsaP521:
                    var curveOid = parameters.Algorithm switch
                    {
                        KeyAlgorithm.EcdsaP256 => new byte[] { 0x06, 0x08, 0x2A, 0x86, 0x48, 0xCE, 0x3D, 0x03, 0x01, 0x07 }, // 1.2.840.10045.3.1.7
                        KeyAlgorithm.EcdsaP384 => new byte[] { 0x06, 0x05, 0x2B, 0x81, 0x04, 0x00, 0x22 },                   // 1.3.132.0.34
                        _                      => new byte[] { 0x06, 0x05, 0x2B, 0x81, 0x04, 0x00, 0x23 },                   // 1.3.132.0.35 (P-521)
                    };
                    publicAttrs.Add(_session.Factories.ObjectAttributeFactory.Create(CKA.CKA_EC_PARAMS, curveOid));
                    mech = _session.Factories.MechanismFactory.Create(CKM.CKM_EC_KEY_PAIR_GEN);
                    algName = "EC";
                    bits = parameters.Algorithm == KeyAlgorithm.EcdsaP256 ? 256
                         : parameters.Algorithm == KeyAlgorithm.EcdsaP384 ? 384 : 521;
                    break;
                default:
                    return Task.FromResult(Result<TokenObject>.Failure(ApiError.BadRequest("unsupported_algorithm")));
            }

            _session.GenerateKeyPair(mech, publicAttrs, privateAttrs, out _, out _);
            var idHex = Convert.ToHexString(id);
            return Task.FromResult(Result<TokenObject>.Success(new TokenObject(
                idHex, parameters.Label, TokenObjectKind.PrivateKey, algName, bits, true, false)));
        }
        catch (Pkcs11Exception ex)
        {
            _logger.LogError(ex, "PKCS#11 keypair gen failed");
            return Task.FromResult(Result<TokenObject>.Failure(ApiError.Internal("keypair_failed", ex.Message)));
        }
    }

    public Task<Result<TokenObject>> ImportCertificateAsync(byte[] derCert, string keyId, string? label, CancellationToken ct = default)
    {
        try
        {
            var idBytes = HexToBytes(keyId);
            var attrs = new List<IObjectAttribute>
            {
                _session.Factories.ObjectAttributeFactory.Create(CKA.CKA_CLASS, CKO.CKO_CERTIFICATE),
                _session.Factories.ObjectAttributeFactory.Create(CKA.CKA_TOKEN, true),
                _session.Factories.ObjectAttributeFactory.Create(CKA.CKA_CERTIFICATE_TYPE, CKC.CKC_X_509),
                _session.Factories.ObjectAttributeFactory.Create(CKA.CKA_LABEL, label ?? "NexusGeregeDesktop Certificate"),
                _session.Factories.ObjectAttributeFactory.Create(CKA.CKA_ID, idBytes),
                _session.Factories.ObjectAttributeFactory.Create(CKA.CKA_VALUE, derCert),
            };
            _session.CreateObject(attrs);
            return Task.FromResult(Result<TokenObject>.Success(new TokenObject(
                keyId, label, TokenObjectKind.Certificate, "X.509", derCert.Length * 8, false, false)));
        }
        catch (Pkcs11Exception ex)
        {
            return Task.FromResult(Result<TokenObject>.Failure(ApiError.Internal("cert_import_failed", ex.Message)));
        }
    }

    private void FindObjects(CKO klass, List<TokenObject> sink)
    {
        var template = new List<IObjectAttribute>
        {
            _session.Factories.ObjectAttributeFactory.Create(CKA.CKA_CLASS, klass),
        };
        var found = _session.FindAllObjects(template);
        foreach (var obj in found)
        {
            try
            {
                var requested = new List<CKA> { CKA.CKA_ID, CKA.CKA_LABEL };
                if (klass != CKO.CKO_CERTIFICATE)
                {
                    requested.Add(CKA.CKA_KEY_TYPE);
                    requested.Add(CKA.CKA_SIGN);
                    requested.Add(CKA.CKA_DECRYPT);
                }
                var attrs = _session.GetAttributeValue(obj, requested);
                var id = SafeHex(attrs[0]);
                var label = SafeString(attrs[1]);
                var keyAlg = "?";
                int bits = 0;
                bool canSign = false, canDecrypt = false;
                if (klass != CKO.CKO_CERTIFICATE && attrs.Count >= 3)
                {
                    try
                    {
                        var keyType = (CKK)attrs[2].GetValueAsUlong();
                        keyAlg = keyType switch
                        {
                            CKK.CKK_RSA => "RSA",
                            CKK.CKK_EC  => "EC",
                            _           => keyType.ToString(),
                        };
                    }
                    catch { }
                    if (attrs.Count >= 4) { try { canSign = attrs[3].GetValueAsBool(); } catch { } }
                    if (attrs.Count >= 5) { try { canDecrypt = attrs[4].GetValueAsBool(); } catch { } }
                }
                else
                {
                    keyAlg = "X.509";
                }
                var kind = klass switch
                {
                    CKO.CKO_PRIVATE_KEY => TokenObjectKind.PrivateKey,
                    CKO.CKO_PUBLIC_KEY  => TokenObjectKind.PublicKey,
                    CKO.CKO_CERTIFICATE => TokenObjectKind.Certificate,
                    _                   => TokenObjectKind.Data,
                };
                sink.Add(new TokenObject(id, label, kind, keyAlg, bits, canSign, canDecrypt));
            }
            catch (Pkcs11Exception ex)
            {
                _logger.LogDebug(ex, "Skipping unreadable object");
            }
        }
    }

    private static CKM MechanismFromAlg(SigningAlg alg) => alg switch
    {
        SigningAlg.Sha256WithRsa => CKM.CKM_SHA256_RSA_PKCS,
        SigningAlg.Sha384WithRsa => CKM.CKM_SHA384_RSA_PKCS,
        SigningAlg.Sha512WithRsa => CKM.CKM_SHA512_RSA_PKCS,
        SigningAlg.EcdsaSha256   => CKM.CKM_ECDSA_SHA256,
        SigningAlg.EcdsaSha384   => CKM.CKM_ECDSA_SHA384,
        SigningAlg.EcdsaSha512   => CKM.CKM_ECDSA_SHA512,
        _                        => CKM.CKM_SHA256_RSA_PKCS,
    };

    private static byte[] HexToBytes(string hex)
    {
        if (string.IsNullOrEmpty(hex)) return Array.Empty<byte>();
        if (hex.Length % 2 != 0) hex = "0" + hex;
        return Convert.FromHexString(hex);
    }

    private static string SafeHex(IObjectAttribute attr)
    {
        try
        {
            var bytes = attr.GetValueAsByteArray();
            return bytes is null ? string.Empty : Convert.ToHexString(bytes);
        }
        catch { return string.Empty; }
    }

    private static string SafeString(IObjectAttribute attr)
    {
        try { return attr.GetValueAsString() ?? string.Empty; }
        catch { return string.Empty; }
    }

    public Task<Result<string>> BuildCsrAsync(string keyId, string subjectCn, CancellationToken ct = default)
    {
        if (!IsLoggedIn)
        {
            return Task.FromResult(Result<string>.Failure(ApiError.Unauthorized("not_logged_in")));
        }
        try
        {
            var idBytes = HexToBytes(keyId);

            // Locate the on-card private key handle (used as the signing
            // identity by the custom X509SignatureGenerator below) and
            // the matching public key (we read modulus + exponent from
            // it to seed the CertificateRequest's PublicKey).
            var privHandles = _session.FindAllObjects(new List<IObjectAttribute>
            {
                _session.Factories.ObjectAttributeFactory.Create(CKA.CKA_CLASS, CKO.CKO_PRIVATE_KEY),
                _session.Factories.ObjectAttributeFactory.Create(CKA.CKA_ID, idBytes),
            });
            if (privHandles.Count == 0)
            {
                return Task.FromResult(Result<string>.Failure(ApiError.NotFound("private_key_not_found")));
            }

            var pubHandles = _session.FindAllObjects(new List<IObjectAttribute>
            {
                _session.Factories.ObjectAttributeFactory.Create(CKA.CKA_CLASS, CKO.CKO_PUBLIC_KEY),
                _session.Factories.ObjectAttributeFactory.Create(CKA.CKA_ID, idBytes),
            });
            if (pubHandles.Count == 0)
            {
                return Task.FromResult(Result<string>.Failure(ApiError.NotFound("public_key_not_found")));
            }

            var pubAttrs = _session.GetAttributeValue(pubHandles[0], new List<CKA>
            {
                CKA.CKA_MODULUS,
                CKA.CKA_PUBLIC_EXPONENT,
            });
            var modulus = pubAttrs[0].GetValueAsByteArray();
            var exponent = pubAttrs[1].GetValueAsByteArray();

            // Build an in-process RSA holding only public params. Used
            // ONLY to populate the CertificateRequest's PublicKey field
            // — the actual signing is done by Pkcs11RsaSignatureGenerator
            // which routes through C_Sign on the on-card private key.
            using var publicRsa = RSA.Create();
            publicRsa.ImportParameters(new RSAParameters
            {
                Modulus = modulus,
                Exponent = exponent,
            });

            var subjectDn = new X500DistinguishedName($"CN={subjectCn}");
            var signatureGen = new Pkcs11RsaSignatureGenerator(_session, privHandles[0], publicRsa);
            var req = new CertificateRequest(subjectDn, signatureGen.PublicKey, HashAlgorithmName.SHA256);
            var csrDer = req.CreateSigningRequest(signatureGen);

            var pem = "-----BEGIN CERTIFICATE REQUEST-----\n"
                + Convert.ToBase64String(csrDer, Base64FormattingOptions.InsertLineBreaks)
                + "\n-----END CERTIFICATE REQUEST-----\n";
            return Task.FromResult(Result<string>.Success(pem));
        }
        catch (Pkcs11Exception ex)
        {
            _logger.LogError(ex, "PKCS#11 CSR build failed");
            return Task.FromResult(Result<string>.Failure(ApiError.Internal("csr_build_failed", ex.Message)));
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "CSR build unexpected failure");
            return Task.FromResult(Result<string>.Failure(ApiError.Internal(
                $"csr_unexpected: {ex.GetType().Name}: {ex.Message}", ex.StackTrace)));
        }
    }

    /// X509SignatureGenerator backed by an on-card private key. .NET's
    /// CertificateRequest.CreateSigningRequest needs a signature
    /// generator that exposes the public key + signs the TBS bytes; the
    /// signing override here calls C_Sign with CKM_SHA256_RSA_PKCS (or
    /// the matching SHA-384/512 variant) so the token's private key
    /// produces the signature without ever leaving the card.
    private sealed class Pkcs11RsaSignatureGenerator : X509SignatureGenerator
    {
        private readonly ISession _session;
        private readonly IObjectHandle _privateKey;
        private readonly RSA _publicRsa;

        public Pkcs11RsaSignatureGenerator(ISession session, IObjectHandle privateKey, RSA publicRsa)
        {
            _session = session;
            _privateKey = privateKey;
            _publicRsa = publicRsa;
        }

        protected override PublicKey BuildPublicKey() => new PublicKey(_publicRsa);

        public override byte[] GetSignatureAlgorithmIdentifier(HashAlgorithmName hashAlgorithm) =>
            hashAlgorithm.Name switch
            {
                // AlgorithmIdentifier DER: OID + NULL params.
                // 1.2.840.113549.1.1.11 = sha256WithRSAEncryption
                "SHA256" => new byte[] { 0x30, 0x0D, 0x06, 0x09, 0x2A, 0x86, 0x48, 0x86, 0xF7, 0x0D, 0x01, 0x01, 0x0B, 0x05, 0x00 },
                // 1.2.840.113549.1.1.12 = sha384WithRSAEncryption
                "SHA384" => new byte[] { 0x30, 0x0D, 0x06, 0x09, 0x2A, 0x86, 0x48, 0x86, 0xF7, 0x0D, 0x01, 0x01, 0x0C, 0x05, 0x00 },
                // 1.2.840.113549.1.1.13 = sha512WithRSAEncryption
                "SHA512" => new byte[] { 0x30, 0x0D, 0x06, 0x09, 0x2A, 0x86, 0x48, 0x86, 0xF7, 0x0D, 0x01, 0x01, 0x0D, 0x05, 0x00 },
                _ => throw new NotSupportedException($"Hash algorithm {hashAlgorithm.Name} not supported"),
            };

        public override byte[] SignData(byte[] data, HashAlgorithmName hashAlgorithm)
        {
            // CKM_SHAxxx_RSA_PKCS hashes the data on the token and
            // signs with PKCS#1 v1.5 padding — exactly what the
            // CertificateRequest TBS path needs.
            var mech = hashAlgorithm.Name switch
            {
                "SHA256" => CKM.CKM_SHA256_RSA_PKCS,
                "SHA384" => CKM.CKM_SHA384_RSA_PKCS,
                "SHA512" => CKM.CKM_SHA512_RSA_PKCS,
                _ => throw new NotSupportedException($"Hash algorithm {hashAlgorithm.Name} not supported"),
            };
            var mechanism = _session.Factories.MechanismFactory.Create(mech);
            return _session.Sign(mechanism, _privateKey, data);
        }
    }

    public Task<Result> DeleteObjectAsync(string objectId, TokenObjectKind kind, CancellationToken ct = default)
    {
        // PKCS#11 doesn't expose objects by external string ID — we
        // store CKA_ID as upper-hex in ListObjectsAsync, so reverse
        // it and search by that attribute. C_DestroyObject is the
        // standard write op for object removal.
        if (!IsLoggedIn)
        {
            return Task.FromResult(Result.Failure(ApiError.Unauthorized("not_logged_in")));
        }
        var klass = kind switch
        {
            TokenObjectKind.Certificate => CKO.CKO_CERTIFICATE,
            TokenObjectKind.PrivateKey  => CKO.CKO_PRIVATE_KEY,
            TokenObjectKind.PublicKey   => CKO.CKO_PUBLIC_KEY,
            _                           => CKO.CKO_DATA,
        };
        try
        {
            byte[] ckaId;
            try { ckaId = Convert.FromHexString(objectId); }
            catch { ckaId = Encoding.UTF8.GetBytes(objectId); }

            var template = new List<IObjectAttribute>
            {
                _session.Factories.ObjectAttributeFactory.Create(CKA.CKA_CLASS, (ulong)klass),
                _session.Factories.ObjectAttributeFactory.Create(CKA.CKA_ID, ckaId),
            };
            var found = _session.FindAllObjects(template);
            if (found.Count == 0)
            {
                return Task.FromResult(Result.Failure(ApiError.NotFound("object_not_found")));
            }
            foreach (var obj in found)
            {
                _session.DestroyObject(obj);
            }
            return Task.FromResult(Result.Success());
        }
        catch (Pkcs11Exception ex)
        {
            return Task.FromResult(Result.Failure(ApiError.Internal(
                $"pkcs11_delete_failed: RV=0x{(ulong)ex.RV:X} {ex.Message}")));
        }
    }

    public Task<Result> InstallIssuedCertificateAsync(byte[] derCert, CancellationToken ct = default)
    {
        if (!IsLoggedIn)
        {
            return Task.FromResult(Result.Failure(ApiError.Unauthorized("not_logged_in")));
        }
        try
        {
            // Parse the CA-issued cert to recover its public key. The
            // CKA_ID we use to write the cert object must match the on-
            // card key with the SAME public modulus — that's how the
            // PKCS#11 module ties cert ↔ private key for later signing.
            using var cert = new X509Certificate2(derCert);
            using var certRsa = cert.GetRSAPublicKey();
            if (certRsa is null)
            {
                return Task.FromResult(Result.Failure(ApiError.BadRequest(
                    "cert_not_rsa", "Only RSA certificates are supported on this PKCS#11 token.")));
            }
            var certParams = certRsa.ExportParameters(includePrivateParameters: false);
            var certModulus = certParams.Modulus ?? Array.Empty<byte>();

            // Iterate every on-card public key and compare its modulus
            // to the cert's. The match wins. We deliberately search
            // CKO_PUBLIC_KEY (not CKO_PRIVATE_KEY) — modulus is a
            // public attribute and the private key may hide it for
            // sensitive=true tokens. CKA_ID is the same on the paired
            // CKO_PRIVATE_KEY by convention (GenerateKeyPairAsync
            // assigns the same id to both).
            var pubHandles = _session.FindAllObjects(new List<IObjectAttribute>
            {
                _session.Factories.ObjectAttributeFactory.Create(CKA.CKA_CLASS, CKO.CKO_PUBLIC_KEY),
            });
            string? matchedId = null;
            foreach (var pub in pubHandles)
            {
                try
                {
                    var attrs = _session.GetAttributeValue(pub, new List<CKA>
                    {
                        CKA.CKA_MODULUS,
                        CKA.CKA_ID,
                    });
                    var modulus = attrs[0].GetValueAsByteArray();
                    if (modulus is not null && ModulusEquals(modulus, certModulus))
                    {
                        matchedId = Convert.ToHexString(attrs[1].GetValueAsByteArray() ?? Array.Empty<byte>());
                        break;
                    }
                }
                catch (Pkcs11Exception ex)
                {
                    _logger.LogDebug(ex, "Skipping unreadable public key during cert install match");
                }
            }
            if (matchedId is null)
            {
                return Task.FromResult(Result.Failure(ApiError.NotFound(
                    "matching_key_not_found",
                    "No on-card key matches this certificate's public key — generate a key first or import a cert for the right key.")));
            }

            var label = cert.GetNameInfo(X509NameType.SimpleName, false);
            if (string.IsNullOrEmpty(label)) label = "NexusGeregeDesktop Certificate";

            var attrsToCreate = new List<IObjectAttribute>
            {
                _session.Factories.ObjectAttributeFactory.Create(CKA.CKA_CLASS, CKO.CKO_CERTIFICATE),
                _session.Factories.ObjectAttributeFactory.Create(CKA.CKA_TOKEN, true),
                _session.Factories.ObjectAttributeFactory.Create(CKA.CKA_CERTIFICATE_TYPE, CKC.CKC_X_509),
                _session.Factories.ObjectAttributeFactory.Create(CKA.CKA_LABEL, label),
                _session.Factories.ObjectAttributeFactory.Create(CKA.CKA_ID, HexToBytes(matchedId)),
                _session.Factories.ObjectAttributeFactory.Create(CKA.CKA_SUBJECT, cert.SubjectName.RawData),
                _session.Factories.ObjectAttributeFactory.Create(CKA.CKA_ISSUER, cert.IssuerName.RawData),
                _session.Factories.ObjectAttributeFactory.Create(CKA.CKA_SERIAL_NUMBER, cert.GetSerialNumber()),
                _session.Factories.ObjectAttributeFactory.Create(CKA.CKA_VALUE, derCert),
            };
            _session.CreateObject(attrsToCreate);
            return Task.FromResult(Result.Success());
        }
        catch (Pkcs11Exception ex)
        {
            _logger.LogError(ex, "PKCS#11 cert install failed");
            return Task.FromResult(Result.Failure(ApiError.Internal("cert_install_failed", ex.Message)));
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Cert install unexpected failure");
            return Task.FromResult(Result.Failure(ApiError.Internal(
                $"cert_install_unexpected: {ex.GetType().Name}: {ex.Message}", ex.StackTrace)));
        }
    }

    /// Compare two RSA moduli for equality, tolerating leading-zero
    /// padding differences. PKCS#11 typically stores moduli WITHOUT a
    /// leading 0x00 sign byte (it's a CK_BYTE array, not ASN.1 INTEGER);
    /// .NET's RSAParameters.Modulus is also unsigned. But certs parsed
    /// via X509Certificate2.GetRSAPublicKey can occasionally include
    /// the high bit in a way that differs by one byte — strip leading
    /// zeros from both before comparing.
    private static bool ModulusEquals(byte[] a, byte[] b)
    {
        ReadOnlySpan<byte> ta = TrimLeadingZeros(a);
        ReadOnlySpan<byte> tb = TrimLeadingZeros(b);
        return ta.SequenceEqual(tb);
    }

    private static ReadOnlySpan<byte> TrimLeadingZeros(byte[] x)
    {
        int i = 0;
        while (i < x.Length - 1 && x[i] == 0) i++;
        return x.AsSpan(i);
    }

    public void Dispose()
    {
        if (_disposed) return;
        _disposed = true;
        try { if (IsLoggedIn) _session.Logout(); } catch { }
        _session.Dispose();
    }

    public ValueTask DisposeAsync()
    {
        Dispose();
        return ValueTask.CompletedTask;
    }
}
