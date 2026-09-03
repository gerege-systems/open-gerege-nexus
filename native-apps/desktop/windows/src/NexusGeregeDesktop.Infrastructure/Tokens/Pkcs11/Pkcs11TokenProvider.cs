using System.Globalization;
using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Application.Configuration;
using NexusGeregeDesktop.Domain.Primitives;
using NexusGeregeDesktop.Domain.Tokens;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Options;
using Net.Pkcs11Interop.Common;
using Net.Pkcs11Interop.HighLevelAPI;

namespace NexusGeregeDesktop.Infrastructure.Tokens.Pkcs11;

/// PKCS#11 backend — loads opensc-pkcs11.dll / eps2003csp11.dll and enumerates
/// slots with a present token. One IPkcs11Library instance per process.
public sealed class Pkcs11TokenProvider : ICryptoTokenProvider, IDisposable
{
    private readonly ILogger<Pkcs11TokenProvider> _logger;
    private readonly Pkcs11InteropFactories _factories = new();
    private readonly string? _modulePath;

    // Library handle is mutable to support RefreshAsync: scan-time
    // hot-plug detection on Feitian's eps2003csp11.dll only works after
    // a C_Finalize → C_Initialize cycle, which means disposing and
    // reloading the IPkcs11Library. _libGate serializes that across the
    // discovery + open + admin paths.
    private readonly object _libGate = new();
    private IPkcs11Library? _library;
    private bool _libraryInitialized;

    public Pkcs11TokenProvider(IOptions<NexusGeregeDesktopOptions> options, ILogger<Pkcs11TokenProvider> logger)
    {
        _logger = logger;
        var t = options.Value.Tokens;
        _modulePath = ResolveModulePath(t);
    }

    /// Explicit-path constructor. Used by DI when multiple PKCS#11
    /// modules are present on the host (OpenSC + Feitian, etc.) so each
    /// module gets its own provider instance and tokens from cards that
    /// only one of them supports still surface in the discovery list.
    public Pkcs11TokenProvider(string modulePath, ILogger<Pkcs11TokenProvider> logger)
    {
        _logger = logger;
        _modulePath = string.IsNullOrWhiteSpace(modulePath) ? null : modulePath;
    }

    /// Returns the loaded PKCS#11 library, loading it on first access.
    /// Subsequent RefreshAsync calls may swap the handle out — never
    /// cache the return value across awaits.
    private IPkcs11Library? GetLibrary()
    {
        lock (_libGate)
        {
            if (!_libraryInitialized)
            {
                _library = LoadLibrary();
                _libraryInitialized = true;
            }
            return _library;
        }
    }

    public string Id => _modulePath is null
        ? "pkcs11"
        : $"pkcs11:{Path.GetFileNameWithoutExtension(_modulePath).ToLowerInvariant()}";

    public string DisplayName => _modulePath is null
        ? "PKCS#11"
        : $"PKCS#11 ({Path.GetFileName(_modulePath)})";

    public TokenBackend Backend => TokenBackend.Pkcs11;

    public bool IsAvailable => GetLibrary() is not null;

    public string? ModulePath => _modulePath;

    public Task<Result<IReadOnlyList<TokenInfo>>> ListTokensAsync(CancellationToken ct = default)
    {
        if (GetLibrary() is null)
        {
            return Task.FromResult(Result<IReadOnlyList<TokenInfo>>.Success(Array.Empty<TokenInfo>()));
        }
        try
        {
            var slots = GetLibrary()!.GetSlotList(SlotsType.WithTokenPresent);
            // Диагностик: тухайн модуль хэдэн токентой slot харж байгаа. 0 бол модуль
            // токеныг танихгүй байна (жишээ HyperPKI хуучин EnterSafe токеныг), эсвэл
            // токен залгаагүй. Бүх slot-той харьцуулж лог хийнэ.
            int allSlots = 0;
            try { allSlots = GetLibrary()!.GetSlotList(SlotsType.WithOrWithoutTokenPresent).Count; } catch { }
            _logger.LogInformation("PKCS#11 [{Module}] slots: токентой={WithTok}, нийт={All}",
                System.IO.Path.GetFileName(_modulePath), slots.Count, allSlots);
            var tokens = new List<TokenInfo>(slots.Count);
            foreach (var slot in slots)
            {
                var slotInfo = slot.GetSlotInfo();
                var tokenInfo = slot.GetTokenInfo();
                var capabilities = ReadCapabilities(tokenInfo);
                var state = ClassifyState(tokenInfo, capabilities);
                var fingerprint = TryReadPublicKeyFingerprint(slot);
                tokens.Add(new TokenInfo(
                    Id: slot.SlotId.ToString(CultureInfo.InvariantCulture),
                    Label: tokenInfo.Label?.Trim() ?? "Unknown",
                    Manufacturer: tokenInfo.ManufacturerId?.Trim() ?? "Unknown",
                    SerialNumber: tokenInfo.SerialNumber?.Trim() ?? string.Empty,
                    Model: tokenInfo.Model?.Trim() ?? string.Empty,
                    Backend: TokenBackend.Pkcs11,
                    Removable: (slotInfo.SlotFlags?.RemovableDevice ?? false),
                    HardwareSlot: (slotInfo.SlotFlags?.HardwareSlot ?? true),
                    IsLoggedIn: false,
                    State: state,
                    Capabilities: capabilities,
                    PublicKeyFingerprint: fingerprint));
            }
            return Task.FromResult(Result<IReadOnlyList<TokenInfo>>.Success((IReadOnlyList<TokenInfo>)tokens));
        }
        catch (Pkcs11Exception ex)
        {
            _logger.LogWarning(ex, "PKCS#11 slot enumeration failed");
            return Task.FromResult(Result<IReadOnlyList<TokenInfo>>.Failure(
                ApiError.Internal("pkcs11_enumeration_failed", ex.Message)));
        }
    }

    public Task<Result<ITokenSession>> OpenSessionAsync(string tokenId, string? pin = null, CancellationToken ct = default)
    {
        if (GetLibrary() is null)
        {
            return Task.FromResult(Result<ITokenSession>.Failure(ApiError.NotFound("module_not_loaded")));
        }
        if (!ulong.TryParse(tokenId, NumberStyles.Integer, CultureInfo.InvariantCulture, out var slotId))
        {
            return Task.FromResult(Result<ITokenSession>.Failure(ApiError.BadRequest("invalid_slot_id")));
        }

        try
        {
            var slot = GetLibrary()!.GetSlotList(SlotsType.WithTokenPresent)
                .FirstOrDefault(s => s.SlotId == slotId);
            if (slot is null)
            {
                return Task.FromResult(Result<ITokenSession>.Failure(ApiError.NotFound("slot_not_found")));
            }
            var tokenInfo = slot.GetTokenInfo();
            var info = new TokenInfo(
                tokenId,
                tokenInfo.Label?.Trim() ?? "Unknown",
                tokenInfo.ManufacturerId?.Trim() ?? "Unknown",
                tokenInfo.SerialNumber?.Trim() ?? string.Empty,
                tokenInfo.Model?.Trim() ?? string.Empty,
                TokenBackend.Pkcs11,
                Removable: true,
                HardwareSlot: true,
                IsLoggedIn: false);

            var session = slot.OpenSession(SessionType.ReadWrite);
            // Pre-warm — Feitian eps2003csp11.dll's first C_Login on a
            // freshly opened session sometimes returns CKR_PIN_INCORRECT
            // even with the correct PIN; a no-op state read (C_GetSession
            // Info) settles the module's internal state so the very
            // first login attempt succeeds. Failure here is non-fatal —
            // the original session is still usable.
            try
            {
                _ = session.GetSessionInfo();
            }
            catch (Exception ex)
            {
                _logger.LogDebug(ex, "PKCS#11 session pre-warm GetSessionInfo failed (non-fatal)");
            }
            var wrapper = new Pkcs11TokenSession(info, session, _logger);
            if (!string.IsNullOrEmpty(pin))
            {
                var login = wrapper.LoginAsync(pin, ct).GetAwaiter().GetResult();
                if (login.IsFailure)
                {
                    wrapper.Dispose();
                    return Task.FromResult(Result<ITokenSession>.Failure(login.Error));
                }
            }
            return Task.FromResult(Result<ITokenSession>.Success((ITokenSession)wrapper));
        }
        catch (Pkcs11Exception ex)
        {
            _logger.LogError(ex, "PKCS#11 open session failed for slot {Slot}", tokenId);
            return Task.FromResult(Result<ITokenSession>.Failure(ApiError.Internal("pkcs11_open_failed", ex.Message)));
        }
    }

    /// Reads PKCS#11 `CK_TOKEN_INFO.flags` and maps them to our
    /// capability struct. Defensive against vendor implementations
    /// that throw on unsupported flags — anything unknown reads false.
    private static TokenCapabilities ReadCapabilities(ITokenInfo info)
    {
        bool TryFlag(Func<bool> fn)
        {
            try { return fn(); }
            catch { return false; }
        }

        var protectedAuth = TryFlag(() => info.TokenFlags?.ProtectedAuthenticationPath ?? false);
        var soPin = TryFlag(() => info.TokenFlags?.TokenInitialized ?? false);
        var userPin = TryFlag(() => info.TokenFlags?.UserPinInitialized ?? false);

        // PIN-counter threshold flags — PKCS#11 doesn't give an exact
        // retry count, only "low / final / locked" tiers. We surface
        // them as a TokenPinStatus the UI maps to a colored badge.
        var userLocked = TryFlag(() => info.TokenFlags?.UserPinLocked ?? false);
        var userFinal = TryFlag(() => info.TokenFlags?.UserPinFinalTry ?? false);
        var userLow = TryFlag(() => info.TokenFlags?.UserPinCountLow ?? false);
        var soLocked = TryFlag(() => info.TokenFlags?.SoPinLocked ?? false);
        var soFinal = TryFlag(() => info.TokenFlags?.SoPinFinalTry ?? false);
        var soLow = TryFlag(() => info.TokenFlags?.SoPinCountLow ?? false);

        TokenPinStatus userPinStatus =
            userLocked ? TokenPinStatus.Locked :
            userFinal ? TokenPinStatus.FinalTry :
            userLow ? TokenPinStatus.Low :
            TokenPinStatus.Ok;
        TokenPinStatus soPinStatus =
            soLocked ? TokenPinStatus.Locked :
            soFinal ? TokenPinStatus.FinalTry :
            soLow ? TokenPinStatus.Low :
            TokenPinStatus.Ok;

        // Memory usage. PKCS#11 splits storage into "private" and
        // "public" object regions; cards differ on which one they
        // report. Fall back to public when private comes back as
        // CK_UNAVAILABLE_INFORMATION (~0UL).
        ulong UnavailMask() => ulong.MaxValue;
        ulong PickMemory(Func<ulong> primary, Func<ulong> fallback)
        {
            var v = TryUlong(primary);
            if (v == 0 || v == UnavailMask()) v = TryUlong(fallback);
            return v == UnavailMask() ? 0 : v;
        }
        ulong total = PickMemory(
            () => info.TotalPrivateMemory,
            () => info.TotalPublicMemory);
        ulong free = PickMemory(
            () => info.FreePrivateMemory,
            () => info.FreePublicMemory);

        static ulong TryUlong(Func<ulong> fn)
        {
            try { return fn(); } catch { return 0; }
        }

        // Heuristic for "this token has a fingerprint sensor": we don't
        // have a clean PKCS#11 flag for biometrics — but a token that
        // advertises CKF_PROTECTED_AUTHENTICATION_PATH is asking the
        // app NOT to collect a PIN, which on USB tokens is almost
        // always because it has its own input (PIN pad or fingerprint).
        // Refine later with vendor-specific manufacturer strings
        // (Feitian BioPass, Yubico Bio, etc.).
        var bioGuess = protectedAuth
            || (info.ManufacturerId?.Contains("BioPass", StringComparison.OrdinalIgnoreCase) ?? false)
            || (info.Model?.Contains("Bio", StringComparison.OrdinalIgnoreCase) ?? false);

        return new TokenCapabilities(
            ProtectedAuthPath: protectedAuth,
            BiometricSensor: bioGuess,
            SoPinInitialized: soPin,
            UserPinInitialized: userPin,
            // PKCS#11 doesn't advertise on-board keygen via a flag —
            // every real hardware token supports CKM_RSA_PKCS_KEY_PAIR
            // _GEN at the mechanism level. Default to true; if
            // GenerateKeyPair fails, surface that at call time.
            CanGenerateKeyPair: true,
            UserPinStatus: userPinStatus,
            SoPinStatus: soPinStatus,
            MemoryTotalBytes: total,
            MemoryFreeBytes: free);
    }

    /// Classify the lifecycle stage from the flags + (eventually) an
    /// object scan. Right now we infer Blank / Initialized / Provisioned
    /// from token flags alone; once we actually enumerate objects we
    /// can promote Provisioned → Enrolled when a CA-issued cert is
    /// found (issuer != subject).
    private static TokenState ClassifyState(ITokenInfo info, TokenCapabilities caps)
    {
        if (!caps.SoPinInitialized) return TokenState.Blank;
        if (!caps.UserPinInitialized) return TokenState.Initialized;
        // Flags don't distinguish Provisioned vs Enrolled — that needs
        // an object scan. Default to Provisioned; the session-open path
        // refines it after ListObjectsAsync.
        return TokenState.Provisioned;
    }

    public Task<Result<BiometricEnrollOutcome>> EnrollFingerprintAsync(string tokenId, string userPin, CancellationToken ct = default)
    {
        // PKCS#11 has no standard mechanism for biometric enrollment —
        // CKF_PROTECTED_AUTHENTICATION_PATH only signals that LOGIN
        // uses the on-board sensor, not that enrollment is exposed.
        // Vendor-specific SDKs (Feitian FT_BioManage, etc.) handle
        // enrollment via custom C_*_Ex functions or out-of-band tools.
        //
        // Forward-compatible posture: tell the caller to use the
        // vendor tool, include the module name so the UI can launch a
        // sibling executable when it knows the vendor.
        _ = tokenId; _ = userPin;
        var moduleHint = _modulePath is null
            ? "(no PKCS#11 module loaded)"
            : Path.GetFileName(_modulePath);
        return Task.FromResult(Result<BiometricEnrollOutcome>.Success(new BiometricEnrollOutcome(
            Mode: BiometricEnrollMode.DelegatedTool,
            FingerCount: 0,
            Instructions:
                $"Fingerprint enrollment for this token requires the vendor's tool. " +
                $"Loaded module: {moduleHint}. " +
                "Run the vendor's enrollment app, then return here and click Refresh.")));
    }

    public Task<Result> ChangeSoPinAsync(string tokenId, string oldSoPin, string newSoPin, CancellationToken ct = default)
    {
        if (GetLibrary() is null)
        {
            return Task.FromResult(Result.Failure(ApiError.NotFound("module_not_loaded")));
        }
        if (!ulong.TryParse(tokenId, NumberStyles.Integer, CultureInfo.InvariantCulture, out var slotId))
        {
            return Task.FromResult(Result.Failure(ApiError.BadRequest("invalid_slot_id")));
        }
        if (string.IsNullOrEmpty(oldSoPin) || string.IsNullOrEmpty(newSoPin))
        {
            return Task.FromResult(Result.Failure(ApiError.BadRequest("pin_required")));
        }
        try
        {
            var slot = GetLibrary()!.GetSlotList(SlotsType.WithTokenPresent)
                .FirstOrDefault(s => s.SlotId == slotId);
            if (slot is null)
            {
                return Task.FromResult(Result.Failure(ApiError.NotFound("slot_not_found")));
            }
            // Open R/W session, login as SO, rotate SO PIN, logout.
            using var session = slot.OpenSession(SessionType.ReadWrite);
            session.Login(CKU.CKU_SO, oldSoPin);
            session.SetPin(
                System.Text.Encoding.UTF8.GetBytes(oldSoPin),
                System.Text.Encoding.UTF8.GetBytes(newSoPin));
            session.Logout();
            return Task.FromResult(Result.Success());
        }
        catch (Pkcs11Exception ex)
        {
            var code = (ulong)ex.RV;
            var label = code switch
            {
                0xA0 => "so_pin_incorrect",
                0xA2 => "so_pin_length_invalid",
                0xA4 => "so_pin_locked",
                _    => "change_so_pin_failed",
            };
            _logger.LogWarning(ex, "SO PIN change failed (RV={Rv:X})", code);
            return Task.FromResult(Result.Failure(ApiError.Internal(
                $"{label}: PKCS#11 RV=0x{code:X} {ex.Message}", ex.StackTrace)));
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "SO PIN change unexpected failure");
            return Task.FromResult(Result.Failure(ApiError.Internal(
                $"change_so_pin_unexpected: {ex.GetType().Name}: {ex.Message}", ex.StackTrace)));
        }
    }

    public Task<Result> UnlockUserPinAsync(string tokenId, string soPin, string newUserPin, CancellationToken ct = default)
    {
        if (GetLibrary() is null)
        {
            return Task.FromResult(Result.Failure(ApiError.NotFound("module_not_loaded")));
        }
        if (!ulong.TryParse(tokenId, NumberStyles.Integer, CultureInfo.InvariantCulture, out var slotId))
        {
            return Task.FromResult(Result.Failure(ApiError.BadRequest("invalid_slot_id")));
        }
        if (string.IsNullOrEmpty(soPin))
        {
            return Task.FromResult(Result.Failure(ApiError.BadRequest("so_pin_required")));
        }
        if (string.IsNullOrEmpty(newUserPin))
        {
            return Task.FromResult(Result.Failure(ApiError.BadRequest("user_pin_required")));
        }
        try
        {
            var slot = GetLibrary()!.GetSlotList(SlotsType.WithTokenPresent)
                .FirstOrDefault(s => s.SlotId == slotId);
            if (slot is null)
            {
                return Task.FromResult(Result.Failure(ApiError.NotFound("slot_not_found")));
            }
            // PKCS#11 PIN unlock: SO logs in, calls C_InitPIN which
            // overwrites the user PIN. On-card keys / certs are not
            // affected — only the user PIN counter is reset.
            using var session = slot.OpenSession(SessionType.ReadWrite);
            session.Login(CKU.CKU_SO, soPin);
            session.InitPin(newUserPin);
            session.Logout();
            return Task.FromResult(Result.Success());
        }
        catch (Pkcs11Exception ex)
        {
            var code = (ulong)ex.RV;
            var label = code switch
            {
                0xA0 => "so_pin_incorrect",
                0xA2 => "user_pin_length_invalid",
                0xA4 => "so_pin_locked",
                _    => "unlock_failed",
            };
            _logger.LogWarning(ex, "PIN unlock failed (RV={Rv:X})", code);
            return Task.FromResult(Result.Failure(ApiError.Internal(
                $"{label}: PKCS#11 RV=0x{code:X} {ex.Message}", ex.StackTrace)));
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "PIN unlock unexpected failure");
            return Task.FromResult(Result.Failure(ApiError.Internal(
                $"unlock_unexpected: {ex.GetType().Name}: {ex.Message}", ex.StackTrace)));
        }
    }

    public Task<Result> InitializeTokenAsync(string tokenId, string soPin, string label, string newUserPin, CancellationToken ct = default)
    {
        if (GetLibrary() is null)
        {
            return Task.FromResult(Result.Failure(ApiError.NotFound("module_not_loaded")));
        }
        if (!ulong.TryParse(tokenId, NumberStyles.Integer, CultureInfo.InvariantCulture, out var slotId))
        {
            return Task.FromResult(Result.Failure(ApiError.BadRequest("invalid_slot_id")));
        }
        if (string.IsNullOrEmpty(soPin))
        {
            return Task.FromResult(Result.Failure(ApiError.BadRequest("so_pin_required")));
        }
        if (string.IsNullOrEmpty(newUserPin))
        {
            return Task.FromResult(Result.Failure(ApiError.BadRequest("user_pin_required")));
        }
        // PKCS#11 C_InitToken caps the label at 32 bytes (CK_UTF8CHAR_PTR
        // padded with spaces to exactly 32 chars). Pkcs11Interop pads/
        // truncates on our behalf, but we still trim here so labels with
        // longer source text get a stable visible prefix.
        var labelOrDefault = string.IsNullOrWhiteSpace(label) ? "eID Mongolia" : label.Trim();
        if (labelOrDefault.Length > 32) labelOrDefault = labelOrDefault[..32];

        try
        {
            var slot = GetLibrary()!.GetSlotList(SlotsType.WithTokenPresent)
                .FirstOrDefault(s => s.SlotId == slotId);
            if (slot is null)
            {
                return Task.FromResult(Result.Failure(ApiError.NotFound("slot_not_found")));
            }

            // Step 1 — C_InitToken: wipes the token, sets SO PIN + label.
            // The token must support CKF_TOKEN_INITIALIZED == false at
            // call time (factory state) OR the supplied SO PIN must
            // match the previous one (re-init). Otherwise C_InitToken
            // fails with CKR_PIN_INCORRECT.
            slot.InitToken(soPin, labelOrDefault);

            // Step 2 — open a R/W SO session, login as SO, set user PIN.
            using var session = slot.OpenSession(SessionType.ReadWrite);
            session.Login(CKU.CKU_SO, soPin);
            session.InitPin(newUserPin);
            session.Logout();
            return Task.FromResult(Result.Success());
        }
        catch (Pkcs11Exception ex)
        {
            // Common return codes:
            //   CKR_PIN_INCORRECT (0xa0)     — wrong SO PIN
            //   CKR_PIN_LEN_RANGE (0xa2)     — new PIN too short/long for token policy
            //   CKR_TOKEN_NOT_RECOGNIZED (...) — vendor middleware mismatch
            //   CKR_DEVICE_ERROR (0x30)      — physical cable/seat issue
            var code = (ulong)ex.RV;
            var label2 = code switch
            {
                0xA0 => "so_pin_incorrect",
                0xA2 => "user_pin_length_invalid",
                _    => "init_failed",
            };
            _logger.LogWarning(ex, "Token init failed (RV={Rv:X})", code);
            return Task.FromResult(Result.Failure(ApiError.Internal(
                $"{label2}: PKCS#11 RV=0x{code:X} {ex.Message}", ex.StackTrace)));
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Token init unexpected failure");
            return Task.FromResult(Result.Failure(ApiError.Internal(
                $"init_unexpected: {ex.GetType().Name}: {ex.Message}", ex.StackTrace)));
        }
    }

    private IPkcs11Library? LoadLibrary()
    {
        if (string.IsNullOrEmpty(_modulePath))
        {
            return null;
        }
        try
        {
            // OS-thread safety: AppType.MultiThreaded matches our .NET threading model.
            var lib = _factories.Pkcs11LibraryFactory.LoadPkcs11Library(_factories, _modulePath, AppType.MultiThreaded);
            _logger.LogInformation("Loaded PKCS#11 module {Path}", _modulePath);
            return lib;
        }
        catch (Pkcs11Exception ex)
        {
            _logger.LogWarning(ex, "Failed to load PKCS#11 module {Path}", _modulePath);
            return null;
        }
        catch (Exception ex) when (ex is DllNotFoundException or BadImageFormatException)
        {
            _logger.LogWarning(ex, "PKCS#11 module not loadable: {Path}", _modulePath);
            return null;
        }
    }

    /// Read the modulus of the first CKO_PUBLIC_KEY on the slot and
    /// return its SHA-256 (hex). Cheap discovery-time signal because
    /// CKA_MODULUS is a public attribute on every PKCS#11 module — no
    /// C_Login needed. Used by TokenRegistry to dedupe entries that
    /// represent the same physical card across multiple backends. Any
    /// failure (blank token, EC-only slot, module quirk) returns null,
    /// which makes the entry opt-out of dedup.
    private string? TryReadPublicKeyFingerprint(ISlot slot)
    {
        try
        {
            using var readSession = slot.OpenSession(SessionType.ReadOnly);
            var pubHandles = readSession.FindAllObjects(new List<IObjectAttribute>
            {
                readSession.Factories.ObjectAttributeFactory.Create(CKA.CKA_CLASS, CKO.CKO_PUBLIC_KEY),
            });
            if (pubHandles.Count == 0) return null;
            var attrs = readSession.GetAttributeValue(pubHandles[0], new List<CKA> { CKA.CKA_MODULUS });
            var modulus = attrs[0].GetValueAsByteArray();
            if (modulus is null || modulus.Length == 0) return null;
            // Strip leading zeros so the fingerprint matches X509's
            // ExportParameters().Modulus encoding (both are unsigned big-
            // endian; some modules pad to a fixed byte length).
            int i = 0;
            while (i < modulus.Length - 1 && modulus[i] == 0) i++;
            var trimmed = i == 0 ? modulus : modulus[i..];
            return Convert.ToHexString(System.Security.Cryptography.SHA256.HashData(trimmed));
        }
        catch (Exception ex)
        {
            _logger.LogDebug(ex, "Skipping fingerprint read for slot {Slot}", slot.SlotId);
            return null;
        }
    }

    private static string? ResolveModulePath(TokenOptions options)
    {
        var all = ResolveAllModulePaths(options);
        return all.Count == 0 ? null : all[0];
    }

    /// Enumerate every PKCS#11 module DLL the configured candidate list
    /// resolves to. Used by DI to register one Pkcs11TokenProvider per
    /// module — different cards need different modules (opensc-pkcs11
    /// covers PIV / OpenPGP / GIDS, eps2003csp11 only knows Feitian
    /// ePass2003, etc.), and a card that only one module recognizes
    /// must still surface in the unified discovery list.
    ///
    /// Order is preserved from TokenOptions.Pkcs11ModuleCandidates; an
    /// explicit Pkcs11ModulePath override short-circuits to a single-
    /// element list. Duplicates (e.g. bare name resolving to a path
    /// already present) are de-duped by full path.
    public static IReadOnlyList<string> ResolveAllModulePaths(TokenOptions options)
    {
        var hits = new List<string>();
        if (options is null || !options.EnablePkcs11)
        {
            return hits;
        }
        if (!string.IsNullOrWhiteSpace(options.Pkcs11ModulePath) && File.Exists(options.Pkcs11ModulePath))
        {
            hits.Add(Path.GetFullPath(options.Pkcs11ModulePath));
            return hits;
        }
        var seen = new HashSet<string>(StringComparer.OrdinalIgnoreCase);
        foreach (var candidate in options.Pkcs11ModuleCandidates)
        {
            if (string.IsNullOrWhiteSpace(candidate)) continue;
            string? resolved = null;
            if (File.Exists(candidate))
            {
                resolved = Path.GetFullPath(candidate);
            }
            else
            {
                // Bare DLL name → System32.
                var sys32 = Path.Combine(Environment.SystemDirectory, candidate);
                if (File.Exists(sys32))
                {
                    resolved = Path.GetFullPath(sys32);
                }
            }
            if (resolved is not null && seen.Add(resolved))
            {
                hits.Add(resolved);
            }
        }
        return hits;
    }

    /// Force a C_Finalize → C_Initialize cycle on the underlying PKCS#11
    /// module. Required because Feitian's eps2003csp11.dll (and a few
    /// other widely-used modules) cache the slot table at C_Initialize
    /// time and never re-read it during the process lifetime — a token
    /// plugged in after app startup is invisible until reinit. Called
    /// from TokenRegistry.DiscoverAllAsync before every scan.
    public Task RefreshAsync(CancellationToken ct = default)
    {
        lock (_libGate)
        {
            try
            {
                _library?.Dispose();
            }
            catch (Exception ex)
            {
                _logger.LogWarning(ex, "PKCS#11 library dispose during refresh failed");
            }
            _library = LoadLibrary();
            _libraryInitialized = true;
        }
        return Task.CompletedTask;
    }

    public void Dispose()
    {
        lock (_libGate)
        {
            try { _library?.Dispose(); }
            catch (Exception ex) { _logger.LogWarning(ex, "PKCS#11 library dispose failed"); }
            _library = null;
            _libraryInitialized = true;
        }
    }
}
