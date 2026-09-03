using System.Collections.ObjectModel;
using System.Security.Cryptography.X509Certificates;
using System.Text;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Domain.Auth;
using NexusGeregeDesktop.Domain.Primitives;
using NexusGeregeDesktop.Domain.Tokens;
using NexusGeregeDesktop.Presentation.Navigation;
using NexusGeregeDesktop.Presentation.Services;

namespace NexusGeregeDesktop.Presentation.ViewModels.Pages;

public partial class TokenDetailViewModel : ObservableObject, IDisposable
{
    public void Dispose()
    {
        _session?.Dispose();
        _session = null;
        GC.SuppressFinalize(this);
    }

    private readonly ILocalizationService _l10n;
    private readonly ITokenRegistry _registry;
    private readonly INavigationService _navigation;
    private readonly IUiDispatcher _ui;
    private readonly ICertEnrollmentService _enrollment;
    private readonly IWebSessionStore _sessions;

    private ITokenSession? _session;
    private string? _providerId;
    private string? _tokenId;

    public TokenDetailViewModel(
        ILocalizationService l10n,
        ITokenRegistry registry,
        INavigationService navigation,
        IUiDispatcher ui,
        ICertEnrollmentService enrollment,
        IWebSessionStore sessions)
    {
        _l10n = l10n;
        _registry = registry;
        _navigation = navigation;
        _ui = ui;
        _enrollment = enrollment;
        _sessions = sessions;
        RefreshLabels();
        _l10n.CultureChanged += (_, _) => RefreshLabels();
    }

    #region Labels

    [ObservableProperty] private string _title = string.Empty;
    [ObservableProperty] private string _backLabel = string.Empty;
    [ObservableProperty] private string _backendLabel = string.Empty;
    [ObservableProperty] private string _serialLabel = string.Empty;
    [ObservableProperty] private string _pinSectionLabel = string.Empty;
    [ObservableProperty] private string _pinPlaceholder = string.Empty;
    [ObservableProperty] private string _unlockLabel = string.Empty;
    [ObservableProperty] private string _objectsSectionLabel = string.Empty;
    [ObservableProperty] private string _objectsLoadingLabel = string.Empty;
    [ObservableProperty] private string _objectsEmptyLabel = string.Empty;
    [ObservableProperty] private string _signTestSectionLabel = string.Empty;
    [ObservableProperty] private string _signTestDataLabel = string.Empty;
    [ObservableProperty] private string _signTestRunLabel = string.Empty;
    [ObservableProperty] private string _signTestResultLabel = string.Empty;
    [ObservableProperty] private string _readCertLabel = string.Empty;
    [ObservableProperty] private string _statusMessage = string.Empty;
    [ObservableProperty] private string _errorMessage = string.Empty;
    [ObservableProperty] private string _cngHintLabel = string.Empty;

    #endregion

    [ObservableProperty]
    [NotifyPropertyChangedFor(nameof(NeedsPinEntry))]
    [NotifyPropertyChangedFor(nameof(IsCng))]
    [NotifyPropertyChangedFor(nameof(ShowVendorToolButton))]
    private string? _backend;

    public bool IsCng => string.Equals(Backend, nameof(TokenBackend.WindowsCng), StringComparison.Ordinal);
    public bool NeedsPinEntry => !IsCng && !IsLoggedIn;

    [ObservableProperty]
    [NotifyPropertyChangedFor(nameof(NeedsPinEntry))]
    private bool _isLoggedIn;

    [ObservableProperty] private string _tokenLabel = string.Empty;
    [ObservableProperty] private string _tokenSerial = string.Empty;
    [ObservableProperty] private string _tokenManufacturer = string.Empty;
    [ObservableProperty] private string _tokenModel = string.Empty;
    [ObservableProperty] private string _providerDisplay = string.Empty;

    /// State badge text shown next to the token label — e.g. "Blank",
    /// "Needs enrollment", "Ready". Drives the visibility of the
    /// first-time-setup wizard vs the M9.C enrollment vs nothing.
    [ObservableProperty] private string _stateLabel = string.Empty;

    /// Single-line summary of capabilities (SO PIN set, user PIN set,
    /// biometric sensor available, etc.) so an operator can sanity-check
    /// the discovered state before clicking through wizards.
    [ObservableProperty] private string _capabilitiesSummary = string.Empty;

    /// "Memory: 20 KB free / 60 KB total" — PKCS#11 only. Empty string
    /// when the backend can't read memory info (CNG path); XAML hides
    /// the row via StringNonEmptyToVisibilityConverter.
    [ObservableProperty] private string _memorySummary = string.Empty;

    [ObservableProperty]
    [NotifyPropertyChangedFor(nameof(NeedsFirstTimeSetup))]
    [NotifyPropertyChangedFor(nameof(NeedsKeyGeneration))]
    [NotifyPropertyChangedFor(nameof(NeedsEnrollment))]
    [NotifyPropertyChangedFor(nameof(IsFullyEnrolled))]
    [NotifyPropertyChangedFor(nameof(CanShowPinChange))]
    [NotifyPropertyChangedFor(nameof(ShowInitWizard))]
    private TokenState _state = TokenState.Unknown;

    [ObservableProperty]
    [NotifyPropertyChangedFor(nameof(HasBiometricSensor))]
    private TokenCapabilities _capabilities;

    public bool NeedsFirstTimeSetup => State is TokenState.Blank;

    /// T7 — explicit "show the init wizard" toggle. When the citizen
    /// clicks "Rename / re-initialize token", this flips to true and
    /// the destructive init form unfolds regardless of the discovery-
    /// reported State. Lets the citizen rename a token that's already
    /// provisioned/enrolled, at the cost of wiping its contents.
    [ObservableProperty]
    [NotifyPropertyChangedFor(nameof(ShowInitWizard))]
    private bool _formatRequested;

    public bool ShowInitWizard => NeedsFirstTimeSetup || FormatRequested;

    [RelayCommand]
    private void RequestFormat()
    {
        // Seed the label with the current token name so the citizen
        // can edit just that field and re-init. Re-initialization
        // requires fresh SO + user PINs, so those fields stay blank.
        InitLabel = TokenLabel;
        FormatRequested = true;
    }

    // ----- T11: BioPass fingerprint manager ------------------------------

    /// True when discovery flagged the token as having an on-board
    /// biometric sensor. Drives the "Fingerprint manager" card visibility.
    public bool ShowFingerprintCard => Capabilities.BiometricSensor;

    /// Vendor-specific routing: Feitian BioPass uses its own PKI
    /// Manager's "Fingerprint Manager" tab; Token2 K49/K50 Pro uses
    /// the Windows Biometric Framework (Sign-in options panel).
    /// Citizens click one of the two buttons depending on their card.

    // [RelayCommand] requires an instance method to wire the generated
    // OpenWindowsHelloSettingsCommand property — CA1822 wants it static.
#pragma warning disable CA1822
    [RelayCommand]
    private void OpenWindowsHelloSettings()
    {
        // ms-settings: URI for sign-in options → fingerprint section.
        // The Windows Shell handles this — no IO of our own.
        try
        {
            System.Diagnostics.Process.Start(new System.Diagnostics.ProcessStartInfo
            {
                FileName = "ms-settings:signinoptions",
                UseShellExecute = true,
            });
        }
        catch
        {
            // Best-effort — ms-settings: URI isn't expected to throw
            // but we don't want to crash the page if it does.
        }
    }
#pragma warning restore CA1822

    public bool CanShowSoPinChange => CanShowPinChange;

    [ObservableProperty]
    [NotifyCanExecuteChangedFor(nameof(ChangeSoPinCommand))]
    private string _oldSoPin = string.Empty;

    [ObservableProperty]
    [NotifyCanExecuteChangedFor(nameof(ChangeSoPinCommand))]
    private string _newSoPin = string.Empty;

    [ObservableProperty]
    [NotifyCanExecuteChangedFor(nameof(ChangeSoPinCommand))]
    private string _newSoPinConfirm = string.Empty;

    [ObservableProperty] private bool _isChangingSoPin;
    [ObservableProperty] private string _changeSoPinStatus = string.Empty;

    private bool CanChangeSoPin() =>
        !IsChangingSoPin
        && _providerId is not null && _tokenId is not null
        && OldSoPin.Length >= 4
        && NewSoPin.Length >= 4
        && string.Equals(NewSoPin, NewSoPinConfirm, StringComparison.Ordinal);

    [RelayCommand(CanExecute = nameof(CanChangeSoPin))]
    private async Task ChangeSoPinAsync(CancellationToken ct)
    {
        if (_providerId is null || _tokenId is null) return;
        _ui.Post(() =>
        {
            IsChangingSoPin = true;
            ChangeSoPinStatus = string.Empty;
            ErrorMessage = string.Empty;
        });
        try
        {
            if (IsCng)
            {
                _ui.Post(() =>
                {
                    ChangeSoPinStatus =
                        "Windows CNG cannot rotate a smart-card SO PIN programmatically. " +
                        "Open the vendor's PKI tool to change the admin PIN.";
                    ErrorMessage = "not_implemented_cng";
                });
                return;
            }
            var result = await _registry
                .ChangeSoPinAsync(_providerId, _tokenId, OldSoPin, NewSoPin, ct)
                .ConfigureAwait(false);
            _ui.Post(() =>
            {
                if (result.IsFailure)
                {
                    ErrorMessage = TranslateError(result.Error);
                    ChangeSoPinStatus = $"Change SO PIN failed: {result.Error.Message}";
                    return;
                }
                ErrorMessage = string.Empty;
                ChangeSoPinStatus = "✅ SO PIN updated.";
                OldSoPin = string.Empty;
                NewSoPin = string.Empty;
                NewSoPinConfirm = string.Empty;
            });
        }
        catch (Exception ex)
        {
            _ui.Post(() => ChangeSoPinStatus = $"change_so_pin_unhandled: {ex.GetType().Name}: {ex.Message}");
        }
        finally
        {
            _ui.Post(() => IsChangingSoPin = false);
        }
    }

    // ----- T8: Unlock user PIN with SO PIN -----------------------------

    /// Always visible — citizens who don't realise their PIN is locked
    /// may also need to use this preemptively (e.g. shared device).
    /// XAML can gate visibility on UserPinStatus if a tighter UI is
    /// desired, but Smart Card PIN unlocks are common enough to expose
    /// unconditionally alongside the standard Change PIN card.
    public bool CanShowPinUnlock => CanShowPinChange;

    [ObservableProperty]
    [NotifyCanExecuteChangedFor(nameof(UnlockPinCommand))]
    private string _unlockSoPin = string.Empty;

    [ObservableProperty]
    [NotifyCanExecuteChangedFor(nameof(UnlockPinCommand))]
    private string _unlockNewPin = string.Empty;

    [ObservableProperty]
    [NotifyCanExecuteChangedFor(nameof(UnlockPinCommand))]
    private string _unlockNewPinConfirm = string.Empty;

    [ObservableProperty] private bool _isUnlockingPin;
    [ObservableProperty] private string _unlockPinStatus = string.Empty;

    private bool CanUnlockPin() =>
        !IsUnlockingPin
        && _providerId is not null && _tokenId is not null
        && UnlockSoPin.Length >= 4
        && UnlockNewPin.Length >= 4
        && string.Equals(UnlockNewPin, UnlockNewPinConfirm, StringComparison.Ordinal);

    [RelayCommand(CanExecute = nameof(CanUnlockPin))]
    private async Task UnlockPinAsync(CancellationToken ct)
    {
        if (_providerId is null || _tokenId is null) return;

        _ui.Post(() =>
        {
            IsUnlockingPin = true;
            UnlockPinStatus = string.Empty;
            ErrorMessage = string.Empty;
        });

        try
        {
            if (IsCng)
            {
                _ui.Post(() =>
                {
                    UnlockPinStatus =
                        "Windows CNG cannot unlock a smart-card PIN programmatically. " +
                        "Open the vendor's PKI tool (e.g. EnterSafe PKI Manager → Unlock User PIN).";
                    ErrorMessage = "not_implemented_cng";
                });
                return;
            }

            var result = await _registry
                .UnlockUserPinAsync(_providerId, _tokenId, UnlockSoPin, UnlockNewPin, ct)
                .ConfigureAwait(false);
            _ui.Post(() =>
            {
                if (result.IsFailure)
                {
                    ErrorMessage = TranslateError(result.Error);
                    UnlockPinStatus = $"Unlock failed: {result.Error.Message}";
                    return;
                }
                ErrorMessage = string.Empty;
                UnlockPinStatus = "✅ PIN unlocked. Use the new PIN to log in.";
                UnlockSoPin = string.Empty;
                UnlockNewPin = string.Empty;
                UnlockNewPinConfirm = string.Empty;
            });
        }
        catch (Exception ex)
        {
            _ui.Post(() => UnlockPinStatus = $"unlock_unhandled: {ex.GetType().Name}: {ex.Message}");
        }
        finally
        {
            _ui.Post(() => IsUnlockingPin = false);
        }
    }
    public bool NeedsKeyGeneration => State is TokenState.Initialized;
    public bool NeedsEnrollment => State is TokenState.Provisioned;
    public bool IsFullyEnrolled => State is TokenState.Enrolled;
    public bool HasBiometricSensor => Capabilities.BiometricSensor;

    // ----- M9.D.3 PIN init wizard --------------------------------------

    /// Default Feitian ePass2003 SO PIN at factory. Citizens whose
    /// tokens are vendor-provisioned via Mongolia will get the value
    /// out-of-band; this default is purely a UX hint for testers.
    /// (Backed by an instance field rather than a static so x:Bind
    /// in the XAML doesn't need a qualifier.)
#pragma warning disable CA1822
    public string SoPinPlaceholder => "12345678 (factory default)";
#pragma warning restore CA1822

    [ObservableProperty]
    [NotifyCanExecuteChangedFor(nameof(InitTokenCommand))]
    private string _initSoPin = string.Empty;

    [ObservableProperty]
    [NotifyCanExecuteChangedFor(nameof(InitTokenCommand))]
    private string _initLabel = "eID Mongolia";

    [ObservableProperty]
    [NotifyCanExecuteChangedFor(nameof(InitTokenCommand))]
    private string _initUserPin = string.Empty;

    [ObservableProperty]
    [NotifyCanExecuteChangedFor(nameof(InitTokenCommand))]
    private string _initUserPinConfirm = string.Empty;

    [ObservableProperty] private bool _isInitializingToken;

    [ObservableProperty] private bool _isGeneratingKeyPair;

    // ----- M9.E PIN change ------------------------------------------

    [ObservableProperty]
    [NotifyCanExecuteChangedFor(nameof(ChangePinCommand))]
    private string _currentPin = string.Empty;

    [ObservableProperty]
    [NotifyCanExecuteChangedFor(nameof(ChangePinCommand))]
    private string _newPin = string.Empty;

    [ObservableProperty]
    [NotifyCanExecuteChangedFor(nameof(ChangePinCommand))]
    private string _newPinConfirm = string.Empty;

    [ObservableProperty] private bool _isChangingPin;
    [ObservableProperty] private string _pinChangeStatus = string.Empty;

    /// True only for CNG-discovered tokens — surfaces a button that
    /// launches the vendor's PKI tool (EnterSafe / Feitian / etc.)
    /// so the citizen doesn't have to dig through the Start menu to
    /// change a PIN that Windows CNG can't change for them.
    public bool ShowVendorToolButton => IsCng;

    private static readonly string[] VendorFastPaths =
    {
        @"C:\Program Files\EnterSafe\ePass2003\EnterSafePKIManager.exe",
        @"C:\Program Files (x86)\EnterSafe\ePass2003\EnterSafePKIManager.exe",
        @"C:\Program Files\Feitian\ePass2003\EnterSafePKIManager.exe",
        @"C:\Program Files (x86)\Feitian\ePass2003\EnterSafePKIManager.exe",
    };

    private static readonly string[] VendorNameNeedles =
    {
        "PKIManager", "EnterSafe", "ePass",
    };

    [RelayCommand]
    private void LaunchVendorPinTool()
    {
        // Step 1 — quick check on canonical paths.
        var hit = VendorFastPaths.FirstOrDefault(File.Exists);

        // Step 2 — recursive search through Program Files for a binary
        // whose name contains "PKIManager" or "EnterSafe". Bounded to
        // ProgramFiles roots so we don't crawl the whole filesystem.
        if (hit is null)
        {
            var roots = new[]
            {
                Environment.GetFolderPath(Environment.SpecialFolder.ProgramFiles),
                Environment.GetFolderPath(Environment.SpecialFolder.ProgramFilesX86),
            };
            hit = SearchVendorBinaries(roots, VendorNameNeedles);
        }

        // Step 3 — fall back to the Start Menu shortcut, which often
        // points at the binary even when the install lives somewhere
        // unexpected (per-user, drive D:, OEM Windows partition, ...).
        if (hit is null)
        {
            var menuRoots = new[]
            {
                Environment.GetFolderPath(Environment.SpecialFolder.CommonStartMenu),
                Environment.GetFolderPath(Environment.SpecialFolder.StartMenu),
            };
            hit = ResolveFromStartMenu(menuRoots, VendorNameNeedles);
        }

        if (hit is null)
        {
            _ui.Post(() => PinChangeStatus =
                "Vendor PKI tool not found by automatic search. " +
                "Open it manually from the Start menu (EnterSafe PKI Manager / Feitian ePass2003 Manager) " +
                "and use its Change User PIN action.");
            return;
        }
        try
        {
            System.Diagnostics.Process.Start(new System.Diagnostics.ProcessStartInfo
            {
                FileName = hit,
                UseShellExecute = true,
            });
            _ui.Post(() => PinChangeStatus = $"Launched: {hit}\nChange the PIN in that window, then return here.");
        }
        catch (Exception ex)
        {
            _ui.Post(() => PinChangeStatus = $"Failed to launch vendor tool: {ex.Message}");
        }
    }

    private static string? SearchVendorBinaries(string[] roots, string[] nameSubstrings)
    {
        foreach (var root in roots)
        {
            if (string.IsNullOrEmpty(root) || !Directory.Exists(root)) continue;
            try
            {
                foreach (var path in Directory.EnumerateFiles(root, "*.exe", SearchOption.AllDirectories))
                {
                    var name = Path.GetFileNameWithoutExtension(path);
                    if (nameSubstrings.Any(s => name.Contains(s, StringComparison.OrdinalIgnoreCase)))
                    {
                        return path;
                    }
                }
            }
            catch
            {
                // Access-denied dirs etc. — skip silently. The fallback
                // path below still gets a chance.
            }
        }
        return null;
    }

    private static string? ResolveFromStartMenu(string[] menuRoots, string[] nameSubstrings)
    {
        foreach (var root in menuRoots)
        {
            if (string.IsNullOrEmpty(root) || !Directory.Exists(root)) continue;
            try
            {
                foreach (var lnk in Directory.EnumerateFiles(root, "*.lnk", SearchOption.AllDirectories))
                {
                    var name = Path.GetFileNameWithoutExtension(lnk);
                    if (!nameSubstrings.Any(s => name.Contains(s, StringComparison.OrdinalIgnoreCase)))
                    {
                        continue;
                    }
                    // The shortcut's TARGET path is what we actually
                    // need to launch. Reading .lnk binaries cleanly
                    // takes a COM ref to Shell32; the shortcut path
                    // itself is launchable via UseShellExecute so we
                    // simply return the .lnk path.
                    return lnk;
                }
            }
            catch
            {
                // Skip on permission errors.
            }
        }
        return null;
    }

    /// PIN change is only meaningful for tokens that have a user PIN
    /// set in the first place (Blank tokens use the D.3 init wizard).
    public bool CanShowPinChange =>
        State is TokenState.Initialized or TokenState.Provisioned or TokenState.Enrolled;

    private bool CanChangePin() =>
        !IsChangingPin
        && CurrentPin.Length >= 4
        && NewPin.Length >= 4
        && string.Equals(NewPin, NewPinConfirm, StringComparison.Ordinal);

    [RelayCommand(CanExecute = nameof(CanChangePin))]
    private async Task ChangePinAsync(CancellationToken ct)
    {
        if (_session is null) return;

        _ui.Post(() =>
        {
            IsChangingPin = true;
            PinChangeStatus = string.Empty;
            ErrorMessage = string.Empty;
        });

        try
        {
            // PKCS#11 requires the session to be logged in as CKU_USER
            // before C_SetPIN. CNG ignores the old PIN entirely (we
            // pass it through but the backend rejects with
            // not_implemented_cng — UI surfaces a vendor-tool hint).
            if (IsCng)
            {
                _ui.Post(() =>
                {
                    PinChangeStatus =
                        "Windows CNG cannot change a smart-card PIN programmatically. " +
                        "Open the vendor's PKI tool (e.g. EnterSafe PKI Manager → Change User PIN).";
                    ErrorMessage = "not_implemented_cng";
                });
                return;
            }

            if (!_session.IsLoggedIn)
            {
                var login = await _session.LoginAsync(CurrentPin, ct).ConfigureAwait(false);
                if (login.IsFailure)
                {
                    _ui.Post(() =>
                    {
                        ErrorMessage = TranslateError(login.Error);
                        PinChangeStatus = $"Login failed: {login.Error.Message}";
                    });
                    return;
                }
            }

            var result = await _session.ChangePinAsync(CurrentPin, NewPin, ct).ConfigureAwait(false);
            _ui.Post(() =>
            {
                if (result.IsFailure)
                {
                    ErrorMessage = TranslateError(result.Error);
                    PinChangeStatus = $"PIN change failed: {result.Error.Message}";
                    return;
                }
                ErrorMessage = string.Empty;
                PinChangeStatus = "✅ PIN changed successfully.";
                // Wipe entered PINs from memory.
                CurrentPin = string.Empty;
                NewPin = string.Empty;
                NewPinConfirm = string.Empty;
            });
        }
        catch (Exception ex)
        {
            _ui.Post(() => ErrorMessage = $"pin_change_unhandled: {ex.GetType().Name}: {ex.Message}");
        }
        finally
        {
            _ui.Post(() => IsChangingPin = false);
        }
    }

    [RelayCommand]
    private async Task GenerateKeyPairAsync(CancellationToken ct)
    {
        if (_session is null) return;

        _ui.Post(() =>
        {
            IsGeneratingKeyPair = true;
            ErrorMessage = string.Empty;
            EnrollStatusMessage = string.Empty;
        });

        try
        {
            // Defaults: RSA-2048, non-exportable signing key. Both
            // CNG (Smart Card KSP) and PKCS#11 backends now have an
            // implementation. Citizen-token cert profile requires
            // RSA-2048+ (backend cert/profile.go floor).
            var keyParams = new KeyParams(
                Algorithm: KeyAlgorithm.Rsa,
                RsaBits: 2048,
                Label: TokenLabel,
                Sensitive: true,
                Extractable: false);

            var result = await _session.GenerateKeyPairAsync(keyParams, ct).ConfigureAwait(false);
            _ui.Post(() =>
            {
                if (result.IsFailure)
                {
                    ErrorMessage = TranslateError(result.Error);
                    EnrollStatusMessage = $"Keygen failed: {result.Error.Message}";
                    return;
                }
                ErrorMessage = string.Empty;
                EnrollStatusMessage = $"✅ Keypair generated (id={result.Value.Id}, {result.Value.KeyAlgorithm}/{result.Value.KeyBits}). Continue with CA enrollment below.";
                // Advance state so the UI swaps the keygen card for the
                // enroll card automatically.
                State = TokenState.Provisioned;
                StateLabel = FormatStateLabel(State);
            });
            // Reload object list so the new key appears for enrollment.
            await LoadObjectsAsync(ct).ConfigureAwait(false);
        }
        catch (Exception ex)
        {
            _ui.Post(() => ErrorMessage = $"keygen_unhandled: {ex.GetType().Name}: {ex.Message}");
        }
        finally
        {
            _ui.Post(() => IsGeneratingKeyPair = false);
        }
    }

    private bool CanInitToken() =>
        !IsInitializingToken
        && InitSoPin.Length >= 4
        && InitUserPin.Length >= 4
        && string.Equals(InitUserPin, InitUserPinConfirm, StringComparison.Ordinal);

    [RelayCommand(CanExecute = nameof(CanInitToken))]
    private async Task InitTokenAsync(CancellationToken ct)
    {
        if (_providerId is null || _tokenId is null) return;

        _ui.Post(() =>
        {
            IsInitializingToken = true;
            ErrorMessage = string.Empty;
            EnrollStatusMessage = string.Empty;
        });

        try
        {
            // DESTRUCTIVE — backend interface comment is explicit. The
            // XAML hint string warns the user; we don't add an extra
            // confirm dialog here because the wizard step is reached
            // only when State == Blank.
            var result = await _registry.InitializeTokenAsync(
                _providerId, _tokenId, InitSoPin, InitLabel, InitUserPin, ct).ConfigureAwait(false);

            _ui.Post(() =>
            {
                if (result.IsFailure)
                {
                    ErrorMessage = TranslateError(result.Error);
                    EnrollStatusMessage = $"PIN init failed: {result.Error.Message}";
                    return;
                }
                ErrorMessage = string.Empty;
                EnrollStatusMessage = "✅ Token initialized. Re-scan from Settings to continue with keypair generation + enrollment.";
                // Drop the entered PINs from memory — the wizard succeeded.
                InitSoPin = string.Empty;
                InitUserPin = string.Empty;
                InitUserPinConfirm = string.Empty;
                // State will refresh on next page entry / scan.
                State = TokenState.Initialized;
                StateLabel = FormatStateLabel(State);
            });
        }
        catch (Exception ex)
        {
            _ui.Post(() => ErrorMessage = $"init_unhandled: {ex.GetType().Name}: {ex.Message}");
        }
        finally
        {
            _ui.Post(() => IsInitializingToken = false);
        }
    }

    [ObservableProperty] private string _pinInput = string.Empty;
    [ObservableProperty] private bool _isUnlocking;
    [ObservableProperty] private bool _isLoadingObjects;
    [ObservableProperty] private bool _isSigning;

    [ObservableProperty] private string _signData = "Hello, e-ID Mongolia";
    [ObservableProperty] private string _signResultHex = string.Empty;
    [ObservableProperty] private string _signResultAlg = string.Empty;
    [ObservableProperty] private TokenObjectRow? _selectedSignKey;

    public ObservableCollection<TokenObjectRow> Objects { get; } = new();
    public ObservableCollection<TokenObjectRow> PrivateKeys { get; } = new();
    public ObservableCollection<TokenObjectRow> Certificates { get; } = new();

    public async Task InitializeAsync(string providerId, TokenInfo info)
    {
        // Header fields run synchronously before the first await, so the
        // initial UI thread context is preserved — direct property sets
        // here are safe. The danger zone starts after the first awaited
        // ConfigureAwait(false) call: any property set from there must
        // funnel through _ui.Post or PropertyChanged dispatch fires
        // RPC_E_WRONG_THREAD inside the XAML binding converter lookup.
        _providerId = providerId;
        _tokenId = info.Id;
        Backend = info.Backend.ToString();
        TokenLabel = string.IsNullOrEmpty(info.Label) ? "(no label)" : info.Label;
        TokenSerial = info.SerialNumber;
        TokenManufacturer = info.Manufacturer;
        TokenModel = info.Model;
        var prov = _registry.Providers.FirstOrDefault(p => p.Id == providerId);
        ProviderDisplay = prov?.DisplayName ?? providerId;
        State = info.State;
        Capabilities = info.Capabilities;
        StateLabel = FormatStateLabel(info.State);
        CapabilitiesSummary = FormatCapabilities(info.Capabilities);
        MemorySummary = FormatMemory(info.Capabilities);

        StatusMessage = _l10n.GetString("Token_Status_Opening");
        var result = await _registry.OpenAsync(providerId, info.Id, pin: null, CancellationToken.None).ConfigureAwait(false);
        if (result.IsFailure)
        {
            _ui.Post(() =>
            {
                ErrorMessage = TranslateError(result.Error);
                StatusMessage = _l10n.GetString("Token_Status_Failed");
            });
            return;
        }
        _session = result.Value;
        var loggedIn = _session.IsLoggedIn;
        _ui.Post(() =>
        {
            IsLoggedIn = loggedIn;
            StatusMessage = IsCng
                ? _l10n.GetString("Token_Status_Ready_Cng")
                : _l10n.GetString("Token_Status_Ready_Pkcs11");
        });
        // Гэрчилгээг PIN-гүйгээр урьдчилан унших. eps2003/PKCS#11 нь нийтийн cert
        // объектыг login-гүйгээр уншихыг зөвшөөрдөг тул 2 ижил токеныг PIN оруулахгүйгээр
        // эзэмшигчийн нэр/РД-гээр ялгах боломжтой (буруу PIN олон удаа хийж түгжихээс сэргийлнэ).
        // (Өмнө зөвхөн CNG-д хийдэг байсан.)
        await LoadObjectsAsync(CancellationToken.None).ConfigureAwait(false);
    }

    [RelayCommand]
    private async Task UnlockAsync(CancellationToken ct)
    {
        if (_session is null) return;
        if (string.IsNullOrEmpty(PinInput))
        {
            ErrorMessage = _l10n.GetString("Token_Error_PinRequired");
            return;
        }
        IsUnlocking = true;
        try
        {
            var result = await _session.LoginAsync(PinInput, ct).ConfigureAwait(false);
            _ui.Post(() =>
            {
                if (result.IsFailure)
                {
                    ErrorMessage = TranslateError(result.Error);
                    return;
                }
                ErrorMessage = string.Empty;
                IsLoggedIn = true;
                PinInput = string.Empty;
                StatusMessage = _l10n.GetString("Token_Status_Unlocked");
            });
            if (result.IsSuccess)
            {
                await LoadObjectsAsync(ct).ConfigureAwait(false);
            }
        }
        finally
        {
            _ui.Post(() => IsUnlocking = false);
        }
    }

    [RelayCommand]
    private void Back()
    {
        Dispose();
        _navigation.NavigateTo(NavigationRoutes.Settings);
    }

    [RelayCommand]
    private async Task SignTestAsync(CancellationToken ct)
    {
        if (_session is null || SelectedSignKey is null)
        {
            ErrorMessage = _l10n.GetString("Token_Error_NoKeySelected");
            return;
        }
        if (string.IsNullOrEmpty(SignData))
        {
            ErrorMessage = _l10n.GetString("Token_Error_SignDataEmpty");
            return;
        }
        IsSigning = true;
        SignResultHex = string.Empty;
        try
        {
            var alg = SelectedSignKey.KeyAlgorithm switch
            {
                "EC" => SigningAlg.EcdsaSha256,
                _    => SigningAlg.Sha256WithRsa,
            };
            var bytes = Encoding.UTF8.GetBytes(SignData);
            var result = await _session.SignAsync(SelectedSignKey.Id, bytes, alg, ct).ConfigureAwait(false);
            _ui.Post(() =>
            {
                if (result.IsFailure)
                {
                    ErrorMessage = TranslateError(result.Error);
                    return;
                }
                ErrorMessage = string.Empty;
                SignResultHex = Convert.ToHexString(result.Value);
                SignResultAlg = alg.ToString();
                StatusMessage = _l10n.GetString("Token_Status_SignOk");
            });
        }
        finally
        {
            _ui.Post(() => IsSigning = false);
        }
    }

    /// Read the DER bytes of an on-token certificate. Page code-behind
    /// uses this to populate the cert-detail dialog (T1). Returns null
    /// when the session is closed or the row is not actually a cert.
    public async Task<byte[]?> ReadCertificateDerAsync(TokenObjectRow row, CancellationToken ct = default)
    {
        if (_session is null || row is null || !row.IsCertificate) return null;
        var result = await _session.ReadCertificateAsync(row.Id, ct).ConfigureAwait(false);
        return result.IsSuccess ? result.Value : null;
    }

    /// <summary>Гэрчилгээний DER-ээс эзэмшигч/РД/зориулалтыг задлах (жагсаалтад харуулах).</summary>
    private static CertDetail ParseCertDetail(byte[] der)
    {
        try
        {
            using var x = new X509Certificate2(der);
            string usage = "";
            foreach (var ext in x.Extensions)
                if (ext is X509KeyUsageExtension ku)
                    usage = (ku.KeyUsages & X509KeyUsageFlags.NonRepudiation) != 0 ? "Гарын үсэг"
                          : (ku.KeyUsages & X509KeyUsageFlags.DigitalSignature) != 0 ? "Нэвтрэлт" : "";
            return new CertDetail(
                DnField(x.Subject, "CN"),
                CleanRegno(DnFieldAny(x.Subject, "SERIALNUMBER", "OID.2.5.4.5")),
                DnField(x.Subject, "O"),
                CleanRegno(DnFieldAny(x.Subject, "OID.2.5.4.97", "organizationIdentifier")),
                DnField(x.Issuer, "O"),
                usage,
                x.NotAfter.ToString("yyyy-MM-dd"));
        }
        catch { return new CertDetail("(гэрчилгээ)", "", "", "", "", "", ""); }
    }

    private static string CleanRegno(string v)
    {
        if (string.IsNullOrEmpty(v)) return "";
        int dash = v.IndexOf('-');
        return (dash >= 0 ? v[(dash + 1)..] : v).Trim();
    }
    private static string DnField(string dn, string key)
    {
        foreach (var p in dn.Split(','))
        {
            var kv = p.Trim();
            if (kv.StartsWith(key + "=", System.StringComparison.OrdinalIgnoreCase)) return kv[(key.Length + 1)..].Trim();
        }
        return "";
    }
    private static string DnFieldAny(string dn, params string[] keys)
    {
        foreach (var k in keys) { var v = DnField(dn, k); if (!string.IsNullOrEmpty(v)) return v; }
        return "";
    }

    /// T5 — install a user-supplied DER cert blob onto this token's
    /// existing on-card key. Reuses the same ITokenSession.Install
    /// IssuedCertificateAsync path the CSR-enroll flow goes through, so
    /// the cert lands in the Windows cert store bound to the current
    /// key container (CNG) or written to the card (PKCS#11). Page
    /// code-behind drives the FileOpenPicker; this method is the
    /// transport-agnostic part.
    [ObservableProperty] private bool _isImportingCert;
    [ObservableProperty] private string _importCertStatus = string.Empty;

    /// T6 — destroy an on-token object. Page code-behind drives the
    /// confirmation dialog and passes the row.Id + Kind. Refreshes the
    /// object list on success.
    public async Task<bool> DeleteObjectAsync(TokenObjectRow row, CancellationToken ct = default)
    {
        if (_session is null || row is null) return false;
        var kind = row.Kind switch
        {
            nameof(TokenObjectKind.Certificate) => TokenObjectKind.Certificate,
            nameof(TokenObjectKind.PrivateKey)  => TokenObjectKind.PrivateKey,
            nameof(TokenObjectKind.PublicKey)   => TokenObjectKind.PublicKey,
            _                                   => TokenObjectKind.Data,
        };
        var result = await _session.DeleteObjectAsync(row.Id, kind, ct).ConfigureAwait(false);
        if (result.IsFailure)
        {
            _ui.Post(() => ErrorMessage = TranslateError(result.Error));
            return false;
        }
        await LoadObjectsAsync(ct).ConfigureAwait(false);
        return true;
    }

    public async Task ImportCertificateAsync(byte[] derCert, CancellationToken ct = default)
    {
        if (_session is null || derCert is null || derCert.Length == 0)
        {
            _ui.Post(() => ImportCertStatus = "No active session or empty cert payload.");
            return;
        }
        _ui.Post(() =>
        {
            IsImportingCert = true;
            ImportCertStatus = _l10n.GetString("Token_Import_Installing");
            ErrorMessage = string.Empty;
        });
        try
        {
            var result = await _session.InstallIssuedCertificateAsync(derCert, ct).ConfigureAwait(false);
            _ui.Post(() =>
            {
                if (result.IsFailure)
                {
                    ImportCertStatus = $"Install failed: {result.Error.Message}";
                    ErrorMessage = TranslateError(result.Error);
                    return;
                }
                ImportCertStatus = _l10n.GetString("Token_Import_Done");
            });
            // Refresh object list so the new cert shows up.
            await LoadObjectsAsync(ct).ConfigureAwait(false);
        }
        finally
        {
            _ui.Post(() => IsImportingCert = false);
        }
    }

    private async Task LoadObjectsAsync(CancellationToken ct)
    {
        if (_session is null) return;
        // IsLoadingObjects is observed by a XAML Visibility binding. When
        // LoadObjectsAsync is called from a thread-pool continuation
        // (e.g. M9.C's EnrollTokenInternalAsync, which awaits with
        // ConfigureAwait(false)), setting it directly throws
        // RPC_E_WRONG_THREAD inside the binding's converter lookup —
        // Application.Resources is a UI-thread-only WinRT API. Funnel
        // through _ui.Post so the setter (and PropertyChanged dispatch)
        // run on the UI thread.
        _ui.Post(() => IsLoadingObjects = true);
        try
        {
            var result = await _session.ListObjectsAsync(ct).ConfigureAwait(false);

            // Гэрчилгээ бүрийн DER-г уншиж, subject-ийг задалж бэлдэх (эзэмшигч/РД харуулахад).
            var certDetails = new Dictionary<string, CertDetail>(StringComparer.Ordinal);
            if (result.IsSuccess)
            {
                foreach (var o in result.Value)
                {
                    if (o.Kind != TokenObjectKind.Certificate) continue;
                    try
                    {
                        var der = await _session.ReadCertificateAsync(o.Id, ct).ConfigureAwait(false);
                        if (der.IsSuccess && der.Value is { Length: > 0 })
                            certDetails[o.Id] = ParseCertDetail(der.Value);
                    }
                    catch { /* нэг cert уншигдахгүй бол алгасна */ }
                }
            }

            _ui.Post(() =>
            {
                Objects.Clear();
                PrivateKeys.Clear();
                Certificates.Clear();
                if (result.IsFailure)
                {
                    ErrorMessage = TranslateError(result.Error);
                    return;
                }
                ErrorMessage = string.Empty;
                foreach (var o in result.Value)
                {
                    certDetails.TryGetValue(o.Id, out var cd);
                    var row = new TokenObjectRow(
                        o.Id,
                        string.IsNullOrEmpty(o.Label) ? o.Id : o.Label!,
                        o.Kind.ToString(),
                        o.KeyAlgorithm,
                        o.KeyBits,
                        o.CanSign)
                    {
                        OwnerName = cd?.OwnerName ?? "",
                        PersonRegno = cd?.PersonRegno ?? "",
                        OrgName = cd?.OrgName ?? "",
                        OrgRegno = cd?.OrgRegno ?? "",
                        Issuer = cd?.Issuer ?? "",
                        Usage = cd?.Usage ?? "",
                        ValidTo = cd?.ValidTo ?? "",
                    };
                    Objects.Add(row);
                    if (o.Kind == TokenObjectKind.PrivateKey) PrivateKeys.Add(row);
                    if (o.Kind == TokenObjectKind.Certificate) Certificates.Add(row);
                }
                SelectedSignKey = PrivateKeys.FirstOrDefault();

                // Refine State from the actual on-card objects. PKCS#11
                // flag-only classification can't distinguish between
                // "Initialized but no keypair" and "Provisioned with a
                // signing key" — both look like UserPinInitialized=true.
                // ClassifyState defaults to Provisioned in that ambiguous
                // case; we correct it here once the object list is real.
                //
                // Rules:
                //   no private key                  → Initialized (need keygen)
                //   ≥1 private key, no CA-issued    → Provisioned (need enroll)
                //   ≥1 private key + CA-issued cert → Enrolled (ready)
                //
                // CA-issued is detected by the existence of a cert whose
                // Label differs from "self-signed factory" markers; here
                // we rely on the cert COUNT as a proxy (any cert on a
                // PKCS#11 token is treated as enrollment evidence — the
                // PKCS#11 path doesn't ship factory certs the way Smart
                // Card KSP does).
                var refined =
                    PrivateKeys.Count == 0 ? TokenState.Initialized :
                    Certificates.Count == 0 ? TokenState.Provisioned :
                    TokenState.Enrolled;
                if (refined != State)
                {
                    State = refined;
                    StateLabel = FormatStateLabel(State);
                }
            });
        }
        finally
        {
            _ui.Post(() => IsLoadingObjects = false);
        }
    }

    private string TranslateError(ApiError err)
    {
        return err.Message switch
        {
            "pin_incorrect"   => _l10n.GetString("Token_Error_PinIncorrect"),
            "pin_locked"      => _l10n.GetString("Token_Error_PinLocked"),
            "pin_required"    => _l10n.GetString("Token_Error_PinRequired"),
            "not_logged_in"   => _l10n.GetString("Token_Error_NotLoggedIn"),
            "key_not_found"   => _l10n.GetString("Token_Error_KeyNotFound"),
            "not_implemented" => _l10n.GetString("Token_Error_NotImplemented"),
            null or ""        => _l10n.GetString("Token_Error_Generic"),
            _                 => err.Message,
        };
    }

    private void RefreshLabels()
    {
        Title = _l10n.GetString("Token_Detail_Title");
        BackLabel = _l10n.GetString("Token_Back");
        BackendLabel = _l10n.GetString("Token_Field_Backend");
        SerialLabel = _l10n.GetString("Token_Field_Serial");
        PinSectionLabel = _l10n.GetString("Token_Section_Pin");
        PinPlaceholder = _l10n.GetString("Token_Pin_Placeholder");
        UnlockLabel = _l10n.GetString("Token_Unlock");
        ObjectsSectionLabel = _l10n.GetString("Token_Section_Objects");
        ObjectsLoadingLabel = _l10n.GetString("Token_Loading_Objects");
        ObjectsEmptyLabel = _l10n.GetString("Token_Empty_Objects");
        SignTestSectionLabel = _l10n.GetString("Token_Section_SignTest");
        SignTestDataLabel = _l10n.GetString("Token_SignTest_Data");
        SignTestRunLabel = _l10n.GetString("Token_SignTest_Run");
        SignTestResultLabel = _l10n.GetString("Token_SignTest_Result");
        ReadCertLabel = _l10n.GetString("Token_ReadCert");
        CngHintLabel = _l10n.GetString("Token_Hint_Cng");
        EnrollSectionLabel = _l10n.GetString("Token_Section_Enroll");
        EnrollHintLabel = _l10n.GetString("Token_Enroll_Hint");
        EnrollRunLabel = _l10n.GetString("Token_Enroll_Run");
    }

    // ----- M9.C: Enrollment ---------------------------------------------

    [ObservableProperty] private string _enrollSectionLabel = string.Empty;
    [ObservableProperty] private string _enrollHintLabel = string.Empty;
    [ObservableProperty] private string _enrollRunLabel = string.Empty;
    [ObservableProperty] private bool _isEnrolling;
    [ObservableProperty] private string _enrollStatusMessage = string.Empty;

    /// True when the user is logged in via QR/national-id and the
    /// /web2app/v1/cert/enroll endpoint can be called (it requires a bearer).
    public bool HasWebSession => _sessions.Current is not null;

    [RelayCommand]
    private async Task EnrollTokenAsync(CancellationToken ct)
    {
        // Reset the log on every click (status box becomes the current
        // attempt's transcript). ErrorMessage stays — the bottom InfoBar
        // is the canonical "last error" surface.
        _ui.Post(() => EnrollStatusMessage = string.Empty);

        try
        {
            await EnrollTokenInternalAsync(ct).ConfigureAwait(false);
        }
        catch (Exception ex)
        {
            var hresult = ex.HResult.ToString("X8", System.Globalization.CultureInfo.InvariantCulture);
            var frames = (ex.StackTrace ?? string.Empty)
                .Split('\n')
                .Select(f => f.Trim())
                .Where(f => f.Length > 0 && !f.Contains("ExceptionHelpers") && !f.Contains("ExceptionDispatchInfo"))
                .Take(3)
                .ToArray();
            var traceSummary = string.Join(" | ", frames);
            AppendStatus($"[X] UNHANDLED {ex.GetType().Name} HR=0x{hresult}: {ex.Message} | {traceSummary}");
            _ui.Post(() =>
            {
                ErrorMessage = $"enroll_unhandled: {ex.GetType().Name}: {ex.Message}";
                IsEnrolling = false;
            });
        }
    }

    private async Task EnrollTokenInternalAsync(CancellationToken ct)
    {
        // WinUI 3 binding rule: ObservableProperty setters that have UI
        // observers MUST run on the UI thread, otherwise PropertyChanged
        // marshalling throws RPC_E_WRONG_THREAD. Property writes below
        // funnel through _ui.Post for that reason.
        //
        // EnrollStatusMessage doubles as a debug log — each step appends
        // a line so the user can see exactly which step ran and which
        // failed. Don't clobber it; only append.
        AppendStatus("[1] enroll clicked");

        if (_session is null)
        {
            AppendStatus("[!] _session is null — re-open the token first");
            _ui.Post(() => ErrorMessage = _l10n.GetString("Token_Error_Generic"));
            return;
        }
        AppendStatus($"[2] session ok ({Backend})");

        await _sessions.LoadAsync(ct).ConfigureAwait(false);
        _ui.Post(() => OnPropertyChanged(nameof(HasWebSession)));
        if (_sessions.Current is null)
        {
            AppendStatus("[!] not logged in to web — open Login first");
            _ui.Post(() => ErrorMessage = _l10n.GetString("Token_Enroll_Error_NotLoggedIn"));
            return;
        }
        AppendStatus("[3] web bearer present");

        var privateKey = PrivateKeys.FirstOrDefault(k => k.CanSign);
        if (privateKey is null)
        {
            AppendStatus($"[!] no private key in PrivateKeys (count={PrivateKeys.Count})");
            _ui.Post(() => ErrorMessage = _l10n.GetString("Token_Enroll_Error_NoKey"));
            return;
        }
        AppendStatus($"[4] picked key {privateKey.Id} ({privateKey.KeyAlgorithm}/{privateKey.KeyBits})");

        _ui.Post(() =>
        {
            IsEnrolling = true;
            ErrorMessage = string.Empty;
        });

        try
        {
            AppendStatus("[5] building CSR (PIN dialog may appear)...");
            var csrResult = await _session.BuildCsrAsync(privateKey.Id, TokenLabel, ct).ConfigureAwait(false);
            if (csrResult.IsFailure)
            {
                AppendStatus($"[!] BuildCsr failed: {csrResult.Error.Code} / {csrResult.Error.Message}");
                _ui.Post(() => ErrorMessage = TranslateError(csrResult.Error));
                return;
            }
            AppendStatus($"[6] CSR built ({csrResult.Value.Length} chars)");

            AppendStatus("[7] requesting certificate enrollment...");
            var enrollResult = await _enrollment
                .EnrollAsync(CertProfiles.CitizenAuth, csrResult.Value, ct)
                .ConfigureAwait(false);
            if (enrollResult.IsFailure)
            {
                AppendStatus($"[!] EnrollAsync failed: {enrollResult.Error.Code} / {enrollResult.Error.Message}");
                _ui.Post(() => ErrorMessage = TranslateError(enrollResult.Error));
                return;
            }
            AppendStatus($"[8] cert issued — serial={enrollResult.Value.SerialHex}, expires={enrollResult.Value.ExpiresAt:yyyy-MM-dd}");

            AppendStatus("[9] installing cert + binding to on-card key...");
            var der = PemToDer(enrollResult.Value.CertPem);
            if (der is null)
            {
                AppendStatus("[!] PEM→DER decode failed");
                _ui.Post(() => ErrorMessage = _l10n.GetString("Token_Enroll_Error_BadPem"));
                return;
            }
            var installResult = await _session.InstallIssuedCertificateAsync(der, ct).ConfigureAwait(false);
            if (installResult.IsFailure)
            {
                AppendStatus($"[!] Install failed: {installResult.Error.Code} / {installResult.Error.Message}");
                _ui.Post(() => ErrorMessage = TranslateError(installResult.Error));
                return;
            }
            AppendStatus("[10] cert installed — refreshing object list...");
            _ui.Post(() =>
            {
                ErrorMessage = string.Empty;
                StatusMessage = _l10n.GetString("Token_Status_Enrolled");
                // Flip the state machine immediately so the "Setup for
                // login" card disappears and the green "Token enrolled
                // and ready" card takes its place without waiting for
                // the next discovery scan.
                State = TokenState.Enrolled;
                StateLabel = FormatStateLabel(State);
                // Big visible success banner — the diagnostic log is
                // too easy to miss in the corner of the page.
                EnrollSuccessVisible = true;
            });
            await LoadObjectsAsync(ct).ConfigureAwait(false);
            AppendStatus("[✓] DONE — token is enrolled. Sign in via the USB Token tab.");
        }
        finally
        {
            _ui.Post(() => IsEnrolling = false);
        }
    }

    /// Sticky banner shown for a few seconds after a successful enroll
    /// so the citizen sees a clear "done" signal instead of having to
    /// read the diagnostic log. XAML binds an InfoBar's IsOpen to this.
    [ObservableProperty] private bool _enrollSuccessVisible;

    private readonly object _statusGate = new();

    /// Appends a timestamped line to EnrollStatusMessage so the user can
    /// see every step of the flow in the Setup card. Persistent — only
    /// the explicit clear at the start of EnrollTokenAsync resets it.
    private void AppendStatus(string line)
    {
        var stamp = DateTime.Now.ToString("HH:mm:ss.fff", System.Globalization.CultureInfo.InvariantCulture);
        var entry = $"{stamp} {line}";
        _ui.Post(() =>
        {
            lock (_statusGate)
            {
                EnrollStatusMessage = string.IsNullOrEmpty(EnrollStatusMessage)
                    ? entry
                    : EnrollStatusMessage + "\n" + entry;
            }
        });
    }

    private string FormatStateLabel(TokenState s) => s switch
    {
        TokenState.Blank        => _l10n.GetString("Token_State_Blank"),
        TokenState.Initialized  => _l10n.GetString("Token_State_Initialized"),
        TokenState.Provisioned  => _l10n.GetString("Token_State_Provisioned"),
        TokenState.Enrolled     => _l10n.GetString("Token_State_Enrolled"),
        _                       => _l10n.GetString("Token_State_Unknown"),
    };

    private string FormatCapabilities(TokenCapabilities c)
    {
        var parts = new List<string>(6);
        if (c.SoPinInitialized)   parts.Add(_l10n.GetString("Token_Cap_SoPinSet"));
        else                      parts.Add(_l10n.GetString("Token_Cap_SoPinUnset"));
        if (c.UserPinInitialized) parts.Add(_l10n.GetString("Token_Cap_UserPinSet"));
        else                      parts.Add(_l10n.GetString("Token_Cap_UserPinUnset"));
        // PIN retry threshold — show only when non-OK so the row stays
        // readable on healthy tokens. PKCS#11-only signal (CNG defaults
        // every status to Ok), so absence means "unknown but assumed ok".
        var pinWarn = FormatPinStatus(c.UserPinStatus);
        if (!string.IsNullOrEmpty(pinWarn)) parts.Add(pinWarn);
        if (c.BiometricSensor)    parts.Add(_l10n.GetString("Token_Cap_Biometric"));
        if (c.ProtectedAuthPath)  parts.Add(_l10n.GetString("Token_Cap_ProtectedAuth"));
        return string.Join(" · ", parts);
    }

    private string FormatPinStatus(TokenPinStatus status) => status switch
    {
        TokenPinStatus.Low      => _l10n.GetString("Token_Pin_StatusLow"),
        TokenPinStatus.FinalTry => _l10n.GetString("Token_Pin_StatusFinalTry"),
        TokenPinStatus.Locked   => _l10n.GetString("Token_Pin_StatusLocked"),
        _                       => string.Empty,
    };

    /// "Memory: 20 KB free / 60 KB total". Returns empty string when
    /// the backend reported 0/0 (CNG, or unsupported by middleware).
    private string FormatMemory(TokenCapabilities c)
    {
        if (c.MemoryTotalBytes == 0 && c.MemoryFreeBytes == 0)
        {
            return string.Empty;
        }
        var prefix = _l10n.GetString("Token_Memory_Prefix");
        return $"{prefix}: {FormatBytes(c.MemoryFreeBytes)} free / {FormatBytes(c.MemoryTotalBytes)} total";
    }

    private static string FormatBytes(ulong bytes)
    {
        if (bytes == 0) return "0 B";
        if (bytes < 1024) return $"{bytes} B";
        if (bytes < 1024UL * 1024) return $"{bytes / 1024.0:F1} KB";
        return $"{bytes / (1024.0 * 1024):F2} MB";
    }

    private static byte[]? PemToDer(string pem)
    {
        if (string.IsNullOrWhiteSpace(pem)) return null;
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

/// <summary>Гэрчилгээнээс задалсан дэлгэрэнгүй (дотоод).</summary>
internal sealed record CertDetail(
    string OwnerName, string PersonRegno, string OrgName, string OrgRegno,
    string Issuer, string Usage, string ValidTo);

public sealed record TokenObjectRow(
    string Id,
    string Label,
    string Kind,
    string KeyAlgorithm,
    int KeyBits,
    bool CanSign)
{
    /// True when this row is a CKO_CERTIFICATE / X.509 cert. Drives
    /// the per-row "View" button (visible only for certs; private/
    /// public keys have no meaningful detail panel).
    public bool IsCertificate => string.Equals(Kind, nameof(NexusGeregeDesktop.Domain.Tokens.TokenObjectKind.Certificate), StringComparison.Ordinal);

    // --- Гэрчилгээний уншсан дэлгэрэнгүй (2 ижил токеныг ялгах) ---
    public string OwnerName { get; init; } = "";     // эзэмшигчийн нэр (CN)
    public string PersonRegno { get; init; } = "";    // хувь хүний РД (SERIALNUMBER)
    public string OrgName { get; init; } = "";        // байгууллага (O)
    public string OrgRegno { get; init; } = "";       // ААН-ийн РД
    public string Issuer { get; init; } = "";         // олгогч
    public string Usage { get; init; } = "";          // "Нэвтрэлт"/"Гарын үсэг"
    public string ValidTo { get; init; } = "";        // хүчинтэй хугацаа

    /// <summary>Гэрчилгээний хүн танихад тохиромжтой мөр (жагсаалтад харуулах).</summary>
    public string OwnerLine => !IsCertificate ? "" :
        $"{OwnerName}   ·   РД: {(string.IsNullOrEmpty(PersonRegno) ? (string.IsNullOrEmpty(OrgRegno) ? "—" : OrgRegno) : PersonRegno)}"
        + (string.IsNullOrEmpty(OrgName) ? "" : $"   ·   {OrgName}");
    public string DetailLine => !IsCertificate ? "" :
        $"{Usage}   ·   Олгогч: {Issuer}   ·   Хүчинтэй: {ValidTo}";
    public bool HasOwnerInfo => IsCertificate && !string.IsNullOrEmpty(OwnerName);
}
