using System.Globalization;
using System.Security.Cryptography;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Domain.Auth;
using NexusGeregeDesktop.Domain.Primitives;
using NexusGeregeDesktop.Presentation.Services;

namespace NexusGeregeDesktop.Presentation.ViewModels.Pages;

/// PDF гарын үсэг (qualified) — web demo / macOS клиенттэй яг ижил урсгал:
///   1. PDF сонгоод SHA-256 digest-ийг локал тооцно (RpAuthService дотор)
///   2. POST /api/sign-pdf-start → {sessionId, vc, pollToken} — утас руу PIN2 push
///   3. GET /api/status poll → COMPLETE/OK хүртэл
///   4. POST /api/sign-pdf-download (multipart file+sessionId+pollToken) →
///      тамгалагдсан PDF-ийг FileSavePicker-ээр хадгална.
public enum SignPhase
{
    Idle,
    Uploading,
    Waiting,
    Completed,
    Error,
}

public partial class SignViewModel : ObservableObject, IDisposable
{
    public void Dispose()
    {
        _flowCts?.Cancel();
        _flowCts?.Dispose();
        _flowCts = null;
        GC.SuppressFinalize(this);
    }

    private const long MaxPdfBytes = 25L * 1024 * 1024;

    private readonly ILocalizationService _l10n;
    private readonly IRpAuthService _rp;
    private readonly IWebSessionStore _sessions;
    private readonly ISensitiveActionGuard _consent;
    private readonly IUiDispatcher _ui;

    private CancellationTokenSource? _flowCts;

    public SignViewModel(
        ILocalizationService l10n,
        IRpAuthService rp,
        IWebSessionStore sessions,
        ISensitiveActionGuard consent,
        IUiDispatcher ui)
    {
        _l10n = l10n;
        _rp = rp;
        _sessions = sessions;
        _consent = consent;
        _ui = ui;
        RefreshLabels();
        _l10n.CultureChanged += (_, _) => RefreshLabels();
    }

    #region Labels

    [ObservableProperty] private string _title = string.Empty;
    [ObservableProperty] private string _subtitle = string.Empty;
    [ObservableProperty] private string _pickTitle = string.Empty;
    [ObservableProperty] private string _pickHint = string.Empty;
    [ObservableProperty] private string _pickButton = string.Empty;
    [ObservableProperty] private string _notSignedInLabel = string.Empty;
    [ObservableProperty] private string _uploadingLabel = string.Empty;
    [ObservableProperty] private string _verificationLabel = string.Empty;
    [ObservableProperty] private string _waitingInstruction = string.Empty;
    [ObservableProperty] private string _waitingStatus = string.Empty;
    [ObservableProperty] private string _fileLabel = string.Empty;
    [ObservableProperty] private string _cancelLabel = string.Empty;
    [ObservableProperty] private string _completedTitle = string.Empty;
    [ObservableProperty] private string _completedHint = string.Empty;
    [ObservableProperty] private string _saveButton = string.Empty;
    [ObservableProperty] private string _savedNotice = string.Empty;
    [ObservableProperty] private string _newFileLabel = string.Empty;
    [ObservableProperty] private string _tryAgainLabel = string.Empty;

    #endregion

    [ObservableProperty]
    [NotifyPropertyChangedFor(nameof(IsIdle))]
    [NotifyPropertyChangedFor(nameof(IsUploading))]
    [NotifyPropertyChangedFor(nameof(IsWaiting))]
    [NotifyPropertyChangedFor(nameof(IsCompleted))]
    [NotifyPropertyChangedFor(nameof(IsError))]
    private SignPhase _phase = SignPhase.Idle;

    public bool IsIdle => Phase == SignPhase.Idle;
    public bool IsUploading => Phase == SignPhase.Uploading;
    public bool IsWaiting => Phase == SignPhase.Waiting;
    public bool IsCompleted => Phase == SignPhase.Completed;
    public bool IsError => Phase == SignPhase.Error;

    /// True only when a citizen is signed in (a national id is available to
    /// bind the signature to). The nav only surfaces this page post-auth, but
    /// we still gate the pick button so an idle-locked session can't sign.
    public bool CanSign => !string.IsNullOrWhiteSpace(SignerNationalId);

    [ObservableProperty] private string _fileName = string.Empty;
    [ObservableProperty] private string _hashHex = string.Empty;
    [ObservableProperty] private string _errorMessage = string.Empty;

    // Verification code, split into 4 boxes like the Login flow.
    [ObservableProperty] private string _digit1 = string.Empty;
    [ObservableProperty] private string _digit2 = string.Empty;
    [ObservableProperty] private string _digit3 = string.Empty;
    [ObservableProperty] private string _digit4 = string.Empty;

    // Signed PDF bytes held between DownloadSignedPdfAsync and the code-behind
    // FileSavePicker write. Cleared on reset.
    [ObservableProperty]
    [NotifyPropertyChangedFor(nameof(HasSignedPdf))]
    private byte[]? _signedPdfBytes;

    [ObservableProperty] private bool _isSaved;

    [ObservableProperty] private string _suggestedFileName = "document-signed.pdf";

    public bool HasSignedPdf => SignedPdfBytes is { Length: > 0 };

    private string SignerNationalId => (_sessions.Current?.NationalId ?? string.Empty).Trim();

    /// Entry point from the page code-behind (after the FileOpenPicker read the
    /// PDF bytes). Runs the full start → poll → download flow off the UI thread;
    /// every UI-bound mutation is marshalled back via <see cref="IUiDispatcher"/>.
    public void Start(byte[] pdfBytes, string fileName)
    {
        ArgumentNullException.ThrowIfNull(pdfBytes);

        _flowCts?.Cancel();
        _flowCts?.Dispose();
        _flowCts = new CancellationTokenSource();
        var token = _flowCts.Token;

        ResetVisualState();
        FileName = fileName;
        SuggestedFileName = BuildSignedName(fileName);
        Phase = SignPhase.Uploading;
        ErrorMessage = string.Empty;

        _ = Task.Run(() => RunAsync(pdfBytes, fileName, token), token);
    }

    private async Task RunAsync(byte[] pdfBytes, string fileName, CancellationToken ct)
    {
        try
        {
            var signer = SignerNationalId;
            if (string.IsNullOrEmpty(signer))
            {
                Fail(_l10n.GetString("Sign_NotSignedIn"));
                return;
            }
            if (pdfBytes.Length > MaxPdfBytes)
            {
                Fail(_l10n.GetString("Sign_Error_TooLarge"));
                return;
            }

            // Утас руу PIN2 push илгээхийн өмнө эзэн нь компьютерийнхээ ард байгааг
            // Windows Hello-оор батална (Security.RequireWindowsHello).
            var consent = await _consent
                .RequireConsentAsync(_l10n.GetString("Consent_Reason_SignPdf"), "sign.pdf", ct)
                .ConfigureAwait(false);
            if (ct.IsCancellationRequested) return;
            if (consent.IsFailure)
            {
                Fail(TranslateError(consent.Error));
                return;
            }

            // Local SHA-256 hex purely for display parity with the phone prompt;
            // RpAuthService computes and sends the digest itself.
            var hashHex = Convert.ToHexString(SHA256.HashData(pdfBytes)).ToLowerInvariant();

            var initResult = await _rp
                .InitiateAsync(new InitiateAuthRequest(signer, PdfBytes: pdfBytes, FileName: fileName), ct)
                .ConfigureAwait(false);
            if (ct.IsCancellationRequested) return;
            if (initResult.IsFailure)
            {
                Fail(TranslateError(initResult.Error));
                return;
            }

            var session = initResult.Value;
            _ui.Post(() =>
            {
                HashHex = hashHex;
                ApplyControlCode(session.VerificationCode);
                Phase = SignPhase.Waiting;
            });

            var terminal = await PollLoopAsync(session.SessionId, ct).ConfigureAwait(false);
            if (ct.IsCancellationRequested || terminal is null) return;

            if (terminal.State != AuthSessionState.Complete)
            {
                Fail(terminal.State switch
                {
                    AuthSessionState.Expired => _l10n.GetString("Sign_Error_Timeout"),
                    AuthSessionState.Refused => _l10n.GetString("Sign_Error_Declined"),
                    _ => _l10n.GetString("Sign_Error_Generic"),
                });
                return;
            }

            var downloadResult = await _rp
                .DownloadSignedPdfAsync(session.SessionId, pdfBytes, fileName, ct)
                .ConfigureAwait(false);
            if (ct.IsCancellationRequested) return;
            if (downloadResult.IsFailure)
            {
                Fail(TranslateError(downloadResult.Error));
                return;
            }

            _ui.Post(() =>
            {
                SignedPdfBytes = downloadResult.Value;
                IsSaved = false;
                Phase = SignPhase.Completed;
            });
        }
        catch (OperationCanceledException)
        {
            // Cancelled by the user — leave whatever state Cancel() set.
        }
    }

    private async Task<AuthSessionStatus?> PollLoopAsync(Guid sessionId, CancellationToken ct)
    {
        try
        {
            // `/api/status` long-holds ~1s server-side; re-poll on a short cadence.
            using var timer = new PeriodicTimer(TimeSpan.FromMilliseconds(1200));
            while (!ct.IsCancellationRequested)
            {
                var result = await _rp.PollAsync(sessionId, timeoutMs: 0, ct).ConfigureAwait(false);
                if (ct.IsCancellationRequested) return null;
                if (result.IsFailure)
                {
                    Fail(TranslateError(result.Error));
                    return null;
                }

                var status = result.Value;
                if (status.IsTerminal)
                {
                    return status;
                }

                try
                {
                    if (!await timer.WaitForNextTickAsync(ct).ConfigureAwait(false)) return null;
                }
                catch (OperationCanceledException)
                {
                    return null;
                }
            }
        }
        catch (OperationCanceledException)
        {
            // fallthrough
        }
        return null;
    }

    [RelayCommand]
    private void Cancel()
    {
        _flowCts?.Cancel();
        _ui.Post(() =>
        {
            ResetVisualState();
            Phase = SignPhase.Idle;
        });
    }

    [RelayCommand]
    private void Reset()
    {
        _flowCts?.Cancel();
        _ui.Post(() =>
        {
            ResetVisualState();
            Phase = SignPhase.Idle;
        });
    }

    /// Called by the page code-behind once the signed bytes are written to disk.
    public void MarkSaved()
    {
        IsSaved = true;
    }

    private void Fail(string message)
    {
        _ui.Post(() =>
        {
            ErrorMessage = message;
            Phase = SignPhase.Error;
        });
    }

    private string TranslateError(ApiError error)
    {
        // Windows Hello баталгаа өгөөгүй — код биш, ойлгомжтой мессеж харуулна.
        if (error.Message is "hello_cancelled" or "hello_failed")
        {
            return _l10n.GetString("Consent_Error_Declined");
        }
        return error.Code switch
        {
            ApiErrorCode.Cancelled => _l10n.GetString("Sign_Error_Generic"),
            ApiErrorCode.Timeout => _l10n.GetString("Sign_Error_Timeout"),
            ApiErrorCode.Network => _l10n.GetString("Sign_Error_Network"),
            _ => string.IsNullOrWhiteSpace(error.Message)
                ? _l10n.GetString("Sign_Error_Generic")
                : error.Message,
        };
    }

    private void ApplyControlCode(string code)
    {
        var padded = (code ?? string.Empty).PadLeft(4, '0');
        Digit1 = padded[^4].ToString(CultureInfo.InvariantCulture);
        Digit2 = padded[^3].ToString(CultureInfo.InvariantCulture);
        Digit3 = padded[^2].ToString(CultureInfo.InvariantCulture);
        Digit4 = padded[^1].ToString(CultureInfo.InvariantCulture);
    }

    private void ResetVisualState()
    {
        FileName = string.Empty;
        HashHex = string.Empty;
        ErrorMessage = string.Empty;
        Digit1 = Digit2 = Digit3 = Digit4 = string.Empty;
        SignedPdfBytes = null;
        IsSaved = false;
        SuggestedFileName = "document-signed.pdf";
    }

    private static string BuildSignedName(string fileName)
    {
        if (string.IsNullOrWhiteSpace(fileName))
        {
            return "document-signed.pdf";
        }
        var dot = fileName.LastIndexOf('.');
        var baseName = dot > 0 ? fileName[..dot] : fileName;
        return $"{baseName}-signed.pdf";
    }

    private void RefreshLabels()
    {
        Title = _l10n.GetString("Sign_Title");
        Subtitle = _l10n.GetString("Sign_Subtitle");
        PickTitle = _l10n.GetString("Sign_Pick_Title");
        PickHint = _l10n.GetString("Sign_Pick_Hint");
        PickButton = _l10n.GetString("Sign_Pick_Button");
        NotSignedInLabel = _l10n.GetString("Sign_NotSignedIn");
        UploadingLabel = _l10n.GetString("Sign_Uploading");
        VerificationLabel = _l10n.GetString("Sign_VerificationLabel");
        WaitingInstruction = _l10n.GetString("Sign_Waiting_Instruction");
        WaitingStatus = _l10n.GetString("Sign_Waiting_Status");
        FileLabel = _l10n.GetString("Sign_File_Label");
        CancelLabel = _l10n.GetString("Sign_Cancel");
        CompletedTitle = _l10n.GetString("Sign_Completed_Title");
        CompletedHint = _l10n.GetString("Sign_Completed_Hint");
        SaveButton = _l10n.GetString("Sign_Save_Button");
        SavedNotice = _l10n.GetString("Sign_Saved_Notice");
        NewFileLabel = _l10n.GetString("Sign_NewFile");
        TryAgainLabel = _l10n.GetString("Sign_TryAgain");
    }
}
