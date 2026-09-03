using System.Collections.ObjectModel;
using System.Globalization;
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

/// Mirrors the web /login state machine exactly:
///   idle -> loading -> waiting -> success / error
/// Backend `state` field maps to UI as in web/src/components/LoginCard.tsx.
public enum LoginPhase
{
    Idle,
    Loading,
    Waiting,
    Success,
    Error,
}

/// Matches the web tab switcher — "Регистрийн дугаар" vs "QR код" plus
/// the new "Токен" (USB hardware token) entry introduced in M9.B.
public enum LoginMode
{
    NationalId,
    Qr,
    Token,
}

public partial class LoginViewModel : ObservableObject, IDisposable
{
    public void Dispose()
    {
        _flowCts?.Cancel();
        _flowCts?.Dispose();
        _flowCts = null;
        GC.SuppressFinalize(this);
    }

    private readonly ILocalizationService _l10n;
    private readonly ICitizenAuthService _auth;
    private readonly IUserProfileService _profile;
    private readonly IWebSessionStore _sessions;
    private readonly INavigationService _navigation;
    private readonly IUiDispatcher _ui;
    private readonly IClock _clock;
    private readonly ITokenRegistry _tokens;

    private CancellationTokenSource? _flowCts;
    private DateTimeOffset _expiresAt;

    public LoginViewModel(
        ILocalizationService l10n,
        ICitizenAuthService auth,
        IUserProfileService profile,
        IWebSessionStore sessions,
        INavigationService navigation,
        IUiDispatcher ui,
        IClock clock,
        ITokenRegistry tokens)
    {
        _l10n = l10n;
        _auth = auth;
        _profile = profile;
        _sessions = sessions;
        _navigation = navigation;
        _ui = ui;
        _clock = clock;
        _tokens = tokens;
        RefreshLabels();
        _l10n.CultureChanged += (_, _) => RefreshLabels();
    }

    #region Labels

    [ObservableProperty] private string _title = string.Empty;
    [ObservableProperty] private string _subtitle = string.Empty;
    [ObservableProperty] private string _nationalIdLabel = string.Empty;
    [ObservableProperty] private string _nationalIdPlaceholder = string.Empty;
    [ObservableProperty] private string _nationalIdHint = string.Empty;
    [ObservableProperty] private string _verificationLabel = string.Empty;
    [ObservableProperty] private string _waitingTitle = string.Empty;
    [ObservableProperty] private string _waitingSubtitle = string.Empty;
    [ObservableProperty] private string _successTitle = string.Empty;
    [ObservableProperty] private string _successSubtitle = string.Empty;
    [ObservableProperty] private string _initiateLabel = string.Empty;
    [ObservableProperty] private string _cancelLabel = string.Empty;
    [ObservableProperty] private string _tryAgainLabel = string.Empty;
    [ObservableProperty] private string _signOutLabel = string.Empty;
    [ObservableProperty] private string _newAccountLabel = string.Empty;
    [ObservableProperty] private string _helpLabel = string.Empty;

    #endregion

    [ObservableProperty]
    [NotifyCanExecuteChangedFor(nameof(InitiateCommand))]
    private string _nationalId = string.Empty;

    [ObservableProperty]
    [NotifyPropertyChangedFor(nameof(IsNationalIdMode))]
    [NotifyPropertyChangedFor(nameof(IsQrMode))]
    [NotifyPropertyChangedFor(nameof(IsTokenMode))]
    [NotifyCanExecuteChangedFor(nameof(InitiateCommand))]
    [NotifyCanExecuteChangedFor(nameof(SignInWithTokenCommand))]
    private LoginMode _mode = LoginMode.NationalId;

    public bool IsNationalIdMode => Mode == LoginMode.NationalId;
    public bool IsQrMode => Mode == LoginMode.Qr;
    public bool IsTokenMode => Mode == LoginMode.Token;

    [RelayCommand]
    private void SelectMode(string modeName)
    {
        var newMode = modeName?.ToLowerInvariant() switch
        {
            "qr"    => LoginMode.Qr,
            "token" => LoginMode.Token,
            _       => LoginMode.NationalId,
        };
        if (newMode == Mode)
        {
            return;
        }
        Mode = newMode;
        if (Phase != LoginPhase.Idle)
        {
            TryAgain();
        }
        // Web auto-initiates the QR session as soon as the QR tab is clicked;
        // mirror that here so the citizen sees the QR + code immediately.
        if (newMode == LoginMode.Qr && InitiateCommand.CanExecute(null))
        {
            InitiateCommand.Execute(null);
        }
        // Token tab auto-scans for available tokens so the user sees a
        // populated list without an extra click.
        if (newMode == LoginMode.Token && ScanTokensCommand.CanExecute(null))
        {
            ScanTokensCommand.Execute(null);
        }
    }

    /// Called from the page's OnNavigatedTo. WinUI 3 caches Page
    /// instances by default, so this same LoginPage (and its singleton
    /// LoginViewModel) is reused on every navigation — including after
    /// logout. Without an explicit reset, error messages and old QR
    /// payloads from a previous attempt would still be on screen when
    /// the citizen returns. We snap state back to Idle, drop the cached
    /// QR + control code, then auto-initiate the right path for the
    /// current Mode.
    public void OnEnteredPage()
    {
        _flowCts?.Cancel();
        _flowCts = null;
        _ui.Post(() =>
        {
            ResetVisualState();
            ErrorMessage = string.Empty;
            // Don't clobber a flow that's already in progress (could
            // happen if OnNavigatedTo fires twice in close succession).
            if (Phase is LoginPhase.Error or LoginPhase.Success)
            {
                Phase = LoginPhase.Idle;
            }
            if (Mode == LoginMode.Qr && Phase == LoginPhase.Idle && InitiateCommand.CanExecute(null))
            {
                InitiateCommand.Execute(null);
            }
            if (Mode == LoginMode.Token && ScanTokensCommand.CanExecute(null))
            {
                ScanTokensCommand.Execute(null);
            }
        });
    }

    [ObservableProperty]
    private byte[]? _qrPng;

    [ObservableProperty]
    private string _qrPayload = string.Empty;

    [ObservableProperty]
    [NotifyCanExecuteChangedFor(nameof(InitiateCommand))]
    [NotifyPropertyChangedFor(nameof(IsIdle))]
    [NotifyPropertyChangedFor(nameof(IsLoading))]
    [NotifyPropertyChangedFor(nameof(IsWaiting))]
    [NotifyPropertyChangedFor(nameof(IsSuccess))]
    [NotifyPropertyChangedFor(nameof(IsError))]
    [NotifyPropertyChangedFor(nameof(IsIdleOrError))]
    [NotifyPropertyChangedFor(nameof(InitiateButtonText))]
    private LoginPhase _phase = LoginPhase.Idle;

    public bool IsIdle => Phase == LoginPhase.Idle;
    public bool IsLoading => Phase == LoginPhase.Loading;
    public bool IsWaiting => Phase == LoginPhase.Waiting;
    public bool IsSuccess => Phase == LoginPhase.Success;
    public bool IsError => Phase == LoginPhase.Error;
    public bool IsIdleOrError => Phase is LoginPhase.Idle or LoginPhase.Error;

    public string InitiateButtonText => Phase == LoginPhase.Loading
        ? _l10n.GetString("Login_Initiate_Loading")
        : _l10n.GetString("Login_Initiate");

    [ObservableProperty] private string _digit1 = string.Empty;
    [ObservableProperty] private string _digit2 = string.Empty;
    [ObservableProperty] private string _digit3 = string.Empty;
    [ObservableProperty] private string _digit4 = string.Empty;

    [ObservableProperty] private string _countdown = "03:00";

    [ObservableProperty]
    [NotifyPropertyChangedFor(nameof(CountdownVariant))]
    private bool _isCountdownCritical;

    public string CountdownVariant => IsCountdownCritical ? "Bad" : "Idle";

    [ObservableProperty] private string _signedInName = string.Empty;
    [ObservableProperty] private string _signedInNationalId = string.Empty;
    [ObservableProperty] private string _signedInKycLevel = string.Empty;
    [ObservableProperty] private string _errorMessage = string.Empty;

    [RelayCommand(CanExecute = nameof(CanInitiate))]
    private async Task InitiateAsync()
    {
        ResetVisualState();
        Phase = LoginPhase.Loading;
        ErrorMessage = string.Empty;

        Domain.Primitives.Result<WebAuthSession> initResult;

        if (Mode == LoginMode.Qr)
        {
            initResult = await _auth.InitiateQrAsync(CancellationToken.None).ConfigureAwait(false);
        }
        else
        {
            var trimmed = (NationalId ?? string.Empty).Trim().ToUpperInvariant();
            if (string.IsNullOrEmpty(trimmed) || trimmed.Length < 10)
            {
                ErrorMessage = _l10n.GetString("Login_Error_NationalIdRequired");
                Phase = LoginPhase.Error;
                return;
            }
            NationalId = trimmed;
            initResult = await _auth.InitiateAsync(trimmed, CancellationToken.None).ConfigureAwait(false);
        }

        if (initResult.IsFailure)
        {
            _ui.Post(() =>
            {
                ErrorMessage = TranslateError(initResult.Error);
                Phase = LoginPhase.Error;
            });
            return;
        }

        var session = initResult.Value;
        byte[]? qrPngBytes = null;
        if (!string.IsNullOrWhiteSpace(session.DeviceLinkUrl))
        {
            try
            {
                qrPngBytes = GenerateQrPng(session.DeviceLinkUrl!);
            }
            catch (Exception ex)
            {
                System.Diagnostics.Debug.WriteLine($"QR generation failed: {ex}");
            }
        }

        _ui.Post(() =>
        {
            ApplyControlCode(session.ControlCode);
            QrPng = qrPngBytes;
            QrPayload = session.DeviceLinkUrl ?? string.Empty;
            _expiresAt = session.ExpiresAt;
            UpdateCountdownDisplay();
            Phase = LoginPhase.Waiting;
        });

        _flowCts?.Cancel();
        _flowCts = new CancellationTokenSource();
        var token = _flowCts.Token;
        _ = Task.Run(() => CountdownLoopAsync(token), token);
        _ = Task.Run(() => PollLoopAsync(session.SessionId, session.PollToken, token), token);
    }

    private bool CanInitiate()
    {
        if (Phase is LoginPhase.Loading or LoginPhase.Waiting)
        {
            return false;
        }
        if (Mode == LoginMode.Qr)
        {
            return true;
        }
        if (Mode == LoginMode.Token)
        {
            // Token mode uses SignInWithTokenCommand, not Initiate; report
            // false so the shared Initiate button never shows up enabled.
            return false;
        }
        return !string.IsNullOrWhiteSpace(NationalId) && NationalId.Trim().Length >= 10;
    }

    private static byte[] GenerateQrPng(string payload)
    {
        using var gen = new QRCoder.QRCodeGenerator();
        using var data = gen.CreateQrCode(payload, QRCoder.QRCodeGenerator.ECCLevel.M);
        var png = new QRCoder.PngByteQRCode(data);
        return png.GetGraphic(pixelsPerModule: 10);
    }

    [RelayCommand]
    private void Cancel()
    {
        _flowCts?.Cancel();
        _flowCts = null;
        _ui.Post(() =>
        {
            ResetVisualState();
            Phase = LoginPhase.Idle;
        });
    }

    [RelayCommand]
    private void TryAgain()
    {
        _flowCts?.Cancel();
        _flowCts = null;
        _ui.Post(() =>
        {
            ResetVisualState();
            ErrorMessage = string.Empty;
            Phase = LoginPhase.Idle;
            // QR mode self-restarts; ID mode waits for the user to retype.
            if (Mode == LoginMode.Qr && InitiateCommand.CanExecute(null))
            {
                InitiateCommand.Execute(null);
            }
        });
    }

    [RelayCommand]
    private async Task SignOutAsync()
    {
        _flowCts?.Cancel();
        _flowCts = null;
        await _auth.LogoutAsync(CancellationToken.None).ConfigureAwait(false);
        _ui.Post(() =>
        {
            ResetVisualState();
            NationalId = string.Empty;
            Phase = LoginPhase.Idle;
        });
    }

    private async Task PollLoopAsync(Guid sessionId, string pollToken, CancellationToken ct)
    {
        try
        {
            // Web polls every 1.5s. We mirror that — backend has no long-poll on /web2app/v1/auth.
            using var timer = new PeriodicTimer(TimeSpan.FromMilliseconds(1500));
            while (!ct.IsCancellationRequested)
            {
                var result = await _auth.PollAsync(sessionId, pollToken, ct).ConfigureAwait(false);
                if (ct.IsCancellationRequested) return;

                if (result.IsFailure)
                {
                    _ui.Post(() =>
                    {
                        ErrorMessage = TranslateError(result.Error);
                        Phase = LoginPhase.Error;
                    });
                    return;
                }

                var status = result.Value;
                if (status.IsTerminal)
                {
                    await ApplyTerminalAsync(status, pollToken, ct).ConfigureAwait(false);
                    return;
                }

                try { if (!await timer.WaitForNextTickAsync(ct).ConfigureAwait(false)) return; }
                catch (OperationCanceledException) { return; }
            }
        }
        catch (OperationCanceledException) { }
    }

    private async Task CountdownLoopAsync(CancellationToken ct)
    {
        try
        {
            using var timer = new PeriodicTimer(TimeSpan.FromSeconds(1));
            while (await timer.WaitForNextTickAsync(ct).ConfigureAwait(false))
            {
                var remaining = _expiresAt - _clock.UtcNow;
                if (remaining <= TimeSpan.Zero)
                {
                    _ui.Post(() =>
                    {
                        ErrorMessage = _l10n.GetString("Login_Error_Timeout");
                        Phase = LoginPhase.Error;
                    });
                    _flowCts?.Cancel();
                    return;
                }
                _ui.Post(UpdateCountdownDisplay);
            }
        }
        catch (OperationCanceledException) { }
    }

    private async Task ApplyTerminalAsync(WebAuthSessionStatus status, string pollToken, CancellationToken ct)
    {
        switch (status.State)
        {
            case WebAuthState.Confirmed when !string.IsNullOrWhiteSpace(status.SessionToken) && status.UserId is { } uid:
                // Session-bound `/api/*` (dashboard, certificates, representations,
                // children) шаарддаг pollToken-ыг хадгална — login-ий (sessionId, pollToken)
                // хосыг дараагийн дата дуудлагад дахин ашиглана.
                var bootstrap = new WebSession(
                    status.SessionToken!,
                    uid,
                    status.ExpiresAt ?? _clock.UtcNow.AddHours(8),
                    NationalId: string.Empty,
                    FullName: string.Empty,
                    KycLevel: string.Empty,
                    PollToken: pollToken);
                await _sessions.SaveAsync(bootstrap, ct).ConfigureAwait(false);

                var profileResult = await _profile.GetMeAsync(ct).ConfigureAwait(false);
                if (profileResult.IsFailure)
                {
                    await _sessions.ClearAsync(ct).ConfigureAwait(false);
                    _ui.Post(() =>
                    {
                        ErrorMessage = TranslateError(profileResult.Error);
                        Phase = LoginPhase.Error;
                    });
                    return;
                }
                var p = profileResult.Value;
                var session = bootstrap with
                {
                    NationalId = p.NationalId,
                    FullName = p.FullName,
                    KycLevel = p.KycLevel,
                };
                await _sessions.SaveAsync(session, ct).ConfigureAwait(false);
                _ui.Post(() =>
                {
                    SignedInName = p.FullName;
                    SignedInNationalId = p.NationalId;
                    SignedInKycLevel = p.KycLevel;
                    Phase = LoginPhase.Success;
                });
                // Brief "redirecting..." pause to mirror web, then navigate.
                _ = Task.Run(async () =>
                {
                    try
                    {
                        await Task.Delay(1200, ct).ConfigureAwait(false);
                        _ui.Post(() => _navigation.NavigateTo(NavigationRoutes.Dashboard));
                    }
                    catch (OperationCanceledException) { }
                }, ct);
                break;

            case WebAuthState.Expired:
                _ui.Post(() =>
                {
                    ErrorMessage = _l10n.GetString("Login_Error_Timeout");
                    Phase = LoginPhase.Error;
                });
                break;

            case WebAuthState.Failed:
                _ui.Post(() =>
                {
                    ErrorMessage = MapFailureReason(status.FailureReason);
                    Phase = LoginPhase.Error;
                });
                break;

            default:
                _ui.Post(() =>
                {
                    ErrorMessage = _l10n.GetString("Login_Error_VerifyFailed");
                    Phase = LoginPhase.Error;
                });
                break;
        }
    }

    private string MapFailureReason(string? reason)
    {
        if (string.IsNullOrWhiteSpace(reason))
        {
            return _l10n.GetString("Login_Error_VerifyFailed");
        }
        if (reason.Contains("decline", StringComparison.OrdinalIgnoreCase)
            || reason.Contains("reject", StringComparison.OrdinalIgnoreCase)
            || reason.Contains("user_refused", StringComparison.OrdinalIgnoreCase))
        {
            return _l10n.GetString("Login_Error_Declined");
        }
        return _l10n.GetString("Login_Error_VerifyFailed");
    }

    private string TranslateError(ApiError error)
    {
        if (error.Message == "endpoint_not_deployed")
        {
            return _l10n.GetString("Login_Error_EndpointMissing");
        }
        return error.Code switch
        {
            ApiErrorCode.NotFound => _l10n.GetString("Login_Error_UserNotFound"),
            ApiErrorCode.Network or ApiErrorCode.Timeout => _l10n.GetString("Login_Error_Backend"),
            ApiErrorCode.Unauthorized => _l10n.GetString("Login_Error_Backend"),
            _ => string.IsNullOrWhiteSpace(error.Message)
                    ? _l10n.GetString("Login_Error_Generic")
                    : error.Message,
        };
    }

    private void UpdateCountdownDisplay()
    {
        var remaining = _expiresAt - _clock.UtcNow;
        if (remaining < TimeSpan.Zero) remaining = TimeSpan.Zero;
        Countdown = remaining.ToString(@"mm\:ss", CultureInfo.InvariantCulture);
        IsCountdownCritical = remaining.TotalSeconds < 60;
    }

    private void ApplyControlCode(string code)
    {
        var padded = (code ?? string.Empty).PadLeft(4, '0');
        Digit1 = padded.Length > 0 ? padded[^4].ToString(CultureInfo.InvariantCulture) : string.Empty;
        Digit2 = padded.Length > 1 ? padded[^3].ToString(CultureInfo.InvariantCulture) : string.Empty;
        Digit3 = padded.Length > 2 ? padded[^2].ToString(CultureInfo.InvariantCulture) : string.Empty;
        Digit4 = padded.Length > 3 ? padded[^1].ToString(CultureInfo.InvariantCulture) : string.Empty;
    }

    private void ResetVisualState()
    {
        Digit1 = Digit2 = Digit3 = Digit4 = string.Empty;
        SignedInName = SignedInNationalId = SignedInKycLevel = string.Empty;
        Countdown = "03:00";
        IsCountdownCritical = false;
        _expiresAt = default;
        QrPng = null;
        QrPayload = string.Empty;
    }

    private void RefreshLabels()
    {
        Title = _l10n.GetString("Login_Title");
        Subtitle = _l10n.GetString("Login_Subtitle");
        NationalIdLabel = _l10n.GetString("Login_NationalId");
        NationalIdPlaceholder = _l10n.GetString("Login_NationalId_Placeholder");
        NationalIdHint = _l10n.GetString("Login_NationalId_Hint");
        VerificationLabel = _l10n.GetString("Login_VerificationCode");
        WaitingTitle = _l10n.GetString("Login_Waiting_Title");
        WaitingSubtitle = _l10n.GetString("Login_Waiting_Subtitle");
        SuccessTitle = _l10n.GetString("Login_Success_Title");
        SuccessSubtitle = _l10n.GetString("Login_Success_Subtitle");
        InitiateLabel = _l10n.GetString("Login_Initiate");
        CancelLabel = _l10n.GetString("Login_Cancel");
        TryAgainLabel = _l10n.GetString("Login_TryAgain");
        SignOutLabel = _l10n.GetString("Login_SignOut");
        NewAccountLabel = _l10n.GetString("Login_Footer_NewAccount");
        HelpLabel = _l10n.GetString("Login_Footer_Help");
        TokenSectionLabel = _l10n.GetString("Login_Token_Section");
        TokenEmptyLabel = _l10n.GetString("Login_Token_Empty");
        TokenScanLabel = _l10n.GetString("Login_Token_Scan");
        TokenPinLabel = _l10n.GetString("Login_Token_Pin");
        TokenPinPlaceholder = _l10n.GetString("Login_Token_PinPlaceholder");
        TokenSignInLabel = _l10n.GetString("Login_Token_SignIn");
        OnPropertyChanged(nameof(InitiateButtonText));
    }

    // ----- M9.B: Token-based login state ---------------------------------

    [ObservableProperty] private string _tokenSectionLabel = string.Empty;
    [ObservableProperty] private string _tokenEmptyLabel = string.Empty;
    [ObservableProperty] private string _tokenScanLabel = string.Empty;
    [ObservableProperty] private string _tokenPinLabel = string.Empty;
    [ObservableProperty] private string _tokenPinPlaceholder = string.Empty;
    [ObservableProperty] private string _tokenSignInLabel = string.Empty;
    [ObservableProperty] private string _tokenStatusMessage = string.Empty;

    [ObservableProperty] private bool _isScanningTokens;

    [ObservableProperty]
    [NotifyPropertyChangedFor(nameof(RequiresPin))]
    [NotifyCanExecuteChangedFor(nameof(SignInWithTokenCommand))]
    private TokenRow? _selectedToken;

    [ObservableProperty]
    [NotifyCanExecuteChangedFor(nameof(SignInWithTokenCommand))]
    private string _tokenPin = string.Empty;

    public bool RequiresPin =>
        SelectedToken is { Backend: var b } && string.Equals(b, nameof(TokenBackend.Pkcs11), StringComparison.Ordinal);

    public ObservableCollection<TokenRow> AvailableTokens { get; } = new();

    [RelayCommand]
    private async Task ScanTokensAsync(CancellationToken ct)
    {
        if (IsScanningTokens) return;
        IsScanningTokens = true;
        try
        {
            var discovered = await _tokens.DiscoverAllAsync(ct).ConfigureAwait(false);
            _ui.Post(() =>
            {
                AvailableTokens.Clear();
                if (discovered.IsFailure)
                {
                    TokenStatusMessage = TranslateError(discovered.Error);
                    SelectedToken = null;
                    return;
                }
                foreach (var d in discovered.Value)
                {
                    AvailableTokens.Add(new TokenRow(
                        d.Provider.Id,
                        d.Info.Id,
                        d.Info.Label,
                        d.Info.Backend.ToString(),
                        d.Info.Manufacturer,
                        d.Info.Model,
                        d.Info.SerialNumber));
                }
                SelectedToken = AvailableTokens.FirstOrDefault();
                TokenStatusMessage = AvailableTokens.Count == 0
                    ? _l10n.GetString("Login_Token_Empty")
                    : string.Empty;
            });
        }
        finally
        {
            _ui.Post(() => IsScanningTokens = false);
        }
    }

    private bool CanSignInWithToken()
    {
        if (Phase is LoginPhase.Loading or LoginPhase.Waiting or LoginPhase.Success)
        {
            return false;
        }
        if (SelectedToken is null)
        {
            return false;
        }
        if (RequiresPin && string.IsNullOrEmpty(TokenPin))
        {
            return false;
        }
        return true;
    }

    [RelayCommand(CanExecute = nameof(CanSignInWithToken))]
    private async Task SignInWithTokenAsync(CancellationToken ct)
    {
        if (SelectedToken is null) return;

        ResetVisualState();
        Phase = LoginPhase.Loading;
        ErrorMessage = string.Empty;
        TokenStatusMessage = _l10n.GetString("Login_Token_Status_Opening");

        // 1. Open the session. CNG ignores the supplied PIN (Windows shows
        //    its own dialog on first signing call); PKCS#11 needs the PIN
        //    typed into our box.
        var openResult = await _tokens
            .OpenAsync(SelectedToken.ProviderId, SelectedToken.TokenId, RequiresPin ? TokenPin : null, ct)
            .ConfigureAwait(false);

        if (openResult.IsFailure)
        {
            _ui.Post(() =>
            {
                ErrorMessage = TranslateError(openResult.Error);
                Phase = LoginPhase.Error;
            });
            return;
        }

        var session = openResult.Value;
        try
        {
            _ui.Post(() => TokenStatusMessage = _l10n.GetString("Login_Token_Status_Reading"));

            // 2. Find a private key + cert pair.
            var objectsResult = await session.ListObjectsAsync(ct).ConfigureAwait(false);
            if (objectsResult.IsFailure)
            {
                _ui.Post(() =>
                {
                    ErrorMessage = TranslateError(objectsResult.Error);
                    Phase = LoginPhase.Error;
                });
                return;
            }
            var privateKey = objectsResult.Value.FirstOrDefault(o => o.Kind == TokenObjectKind.PrivateKey && o.CanSign);
            var certObj = objectsResult.Value.FirstOrDefault(o => o.Kind == TokenObjectKind.Certificate);
            if (privateKey is null || certObj is null)
            {
                _ui.Post(() =>
                {
                    ErrorMessage = _l10n.GetString("Login_Token_Error_NoKeyOrCert");
                    Phase = LoginPhase.Error;
                });
                return;
            }

            var certResult = await session.ReadCertificateAsync(certObj.Id, ct).ConfigureAwait(false);
            if (certResult.IsFailure)
            {
                _ui.Post(() =>
                {
                    ErrorMessage = TranslateError(certResult.Error);
                    Phase = LoginPhase.Error;
                });
                return;
            }
            var certPem = DerToPem(certResult.Value);

            // 3. Backend challenge.
            _ui.Post(() => TokenStatusMessage = _l10n.GetString("Login_Token_Status_Challenge"));
            var challengeResult = await _auth.InitiateTokenChallengeAsync(ct).ConfigureAwait(false);
            if (challengeResult.IsFailure)
            {
                _ui.Post(() =>
                {
                    ErrorMessage = TranslateError(challengeResult.Error);
                    Phase = LoginPhase.Error;
                });
                return;
            }
            var challenge = challengeResult.Value;

            // 4. Sign SHA256(nonce) with the token's private key. ITokenSession
            //    hashes the data internally per SigningAlg, so we pass the
            //    raw nonce and get back signature(SHA256(nonce)).
            _ui.Post(() => TokenStatusMessage = _l10n.GetString("Login_Token_Status_Signing"));
            var signingAlg = string.Equals(privateKey.KeyAlgorithm, "EC", StringComparison.Ordinal)
                ? SigningAlg.EcdsaSha256
                : SigningAlg.Sha256WithRsa;
            var wireAlg = signingAlg == SigningAlg.EcdsaSha256
                ? WebTokenSignatureAlg.EcdsaSha256
                : WebTokenSignatureAlg.RsaSha256;

            var signResult = await session.SignAsync(privateKey.Id, challenge.Nonce, signingAlg, ct).ConfigureAwait(false);
            if (signResult.IsFailure)
            {
                _ui.Post(() =>
                {
                    ErrorMessage = TranslateError(signResult.Error);
                    Phase = LoginPhase.Error;
                });
                return;
            }

            // 5. Verify with backend.
            _ui.Post(() => TokenStatusMessage = _l10n.GetString("Login_Token_Status_Verifying"));
            var verifyResult = await _auth
                .VerifyTokenAsync(challenge.SessionId, certPem, signResult.Value, wireAlg, ct)
                .ConfigureAwait(false);
            if (verifyResult.IsFailure)
            {
                _ui.Post(() =>
                {
                    ErrorMessage = TranslateError(verifyResult.Error);
                    Phase = LoginPhase.Error;
                });
                return;
            }
            var login = verifyResult.Value;

            // 6. Save session, fetch profile, navigate. Reuses the same shape
            //    as the QR/national-id terminal path so downstream pages
            //    don't need to know which method was used.
            var bootstrap = new WebSession(
                login.SessionToken,
                login.UserId,
                login.ExpiresAt,
                NationalId: string.Empty,
                FullName: string.Empty,
                KycLevel: string.Empty);
            await _sessions.SaveAsync(bootstrap, ct).ConfigureAwait(false);

            var profileResult = await _profile.GetMeAsync(ct).ConfigureAwait(false);
            if (profileResult.IsFailure)
            {
                await _sessions.ClearAsync(ct).ConfigureAwait(false);
                _ui.Post(() =>
                {
                    ErrorMessage = TranslateError(profileResult.Error);
                    Phase = LoginPhase.Error;
                });
                return;
            }
            var p = profileResult.Value;
            var enriched = bootstrap with
            {
                NationalId = p.NationalId,
                FullName = p.FullName,
                KycLevel = p.KycLevel,
            };
            await _sessions.SaveAsync(enriched, ct).ConfigureAwait(false);

            _ui.Post(() =>
            {
                SignedInName = p.FullName;
                SignedInNationalId = p.NationalId;
                SignedInKycLevel = p.KycLevel;
                Phase = LoginPhase.Success;
                TokenStatusMessage = string.Empty;
                TokenPin = string.Empty;
            });
            _ = Task.Run(async () =>
            {
                try
                {
                    await Task.Delay(1200, ct).ConfigureAwait(false);
                    _ui.Post(() => _navigation.NavigateTo(NavigationRoutes.Dashboard));
                }
                catch (OperationCanceledException) { }
            }, ct);
        }
        finally
        {
            // Always dispose the token session — for PKCS#11 this releases
            // the C_Login. The web session bearer is already saved; nothing
            // further needs the hardware.
            session.Dispose();
        }
    }

    private static string DerToPem(byte[] der)
    {
        var b64 = Convert.ToBase64String(der);
        var sb = new StringBuilder(b64.Length + 64);
        sb.Append("-----BEGIN CERTIFICATE-----\n");
        for (var i = 0; i < b64.Length; i += 64)
        {
            sb.Append(b64.AsSpan(i, Math.Min(64, b64.Length - i)));
            sb.Append('\n');
        }
        sb.Append("-----END CERTIFICATE-----\n");
        return sb.ToString();
    }
}

/// One row in the Token tab's "Available USB tokens" list — flattened
/// from DiscoveredToken so XAML x:Bind can hit primitive properties.
public sealed record TokenRow(
    string ProviderId,
    string TokenId,
    string Label,
    string Backend,
    string Manufacturer,
    string Model,
    string SerialNumber);
