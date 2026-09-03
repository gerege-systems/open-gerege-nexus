using System.Collections.ObjectModel;
using System.Globalization;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Domain.Guardian;
using NexusGeregeDesktop.Domain.Primitives;
using NexusGeregeDesktop.Presentation.Navigation;
using NexusGeregeDesktop.Presentation.Services;

namespace NexusGeregeDesktop.Presentation.ViewModels.Pages;

public enum ChildrenView
{
    List,
    Add,
}

public enum ChildAddPhase
{
    Idle,
    Initiating,
    Waiting,   // асран хамгаалагч утсан дээрээ PIN2-оор зөвшөөрч байна
    Success,   // бүртгэлийн код бэлэн
    Error,
}

/// <summary>
/// "Миний хүүхдүүд" — mobile app-ын адил: зөвшөөрлөөр бүртгэгдсэн хүүхдүүдийг
/// жагсаах, шинэ хүүхэд нэмэх (РД → PIN2 зөвшөөрөл → бүртгэлийн код), цуцлах.
/// </summary>
public partial class ChildrenViewModel : ObservableObject, IDisposable
{
    private readonly ILocalizationService _l10n;
    private readonly IGuardianService _guardian;
    private readonly IWebSessionStore _sessions;
    private readonly INavigationService _navigation;
    private readonly IClipboardService _clipboard;
    private readonly IAuditLog _audit;
    private readonly IClock _clock;
    private readonly IUiDispatcher _ui;

    private CancellationTokenSource? _pollCts;

    public ChildrenViewModel(
        ILocalizationService l10n,
        IGuardianService guardian,
        IWebSessionStore sessions,
        INavigationService navigation,
        IClipboardService clipboard,
        IAuditLog audit,
        IClock clock,
        IUiDispatcher ui)
    {
        _l10n = l10n;
        _guardian = guardian;
        _sessions = sessions;
        _navigation = navigation;
        _clipboard = clipboard;
        _audit = audit;
        _clock = clock;
        _ui = ui;
        RefreshLabels();
        _l10n.CultureChanged += (_, _) => RefreshLabels();
    }

    public void Dispose()
    {
        _pollCts?.Cancel();
        _pollCts?.Dispose();
        _pollCts = null;
        GC.SuppressFinalize(this);
    }

    #region Labels

    [ObservableProperty] private string _title = string.Empty;
    [ObservableProperty] private string _subtitle = string.Empty;
    [ObservableProperty] private string _listHeader = string.Empty;
    [ObservableProperty] private string _emptyListLabel = string.Empty;
    [ObservableProperty] private string _addButtonLabel = string.Empty;
    [ObservableProperty] private string _backLabel = string.Empty;
    [ObservableProperty] private string _childRegNoLabel = string.Empty;
    [ObservableProperty] private string _addInitLabel = string.Empty;
    [ObservableProperty] private string _waitingLabel = string.Empty;
    [ObservableProperty] private string _verificationLabel = string.Empty;
    [ObservableProperty] private string _enrollHeaderLabel = string.Empty;
    [ObservableProperty] private string _enrollHintLabel = string.Empty;
    [ObservableProperty] private string _cancelLabel = string.Empty;
    [ObservableProperty] private string _doneLabel = string.Empty;
    [ObservableProperty] private string _revokeLabel = string.Empty;
    [ObservableProperty] private string _copyLabel = string.Empty;
    [ObservableProperty] private string _errorMessage = string.Empty;

    #endregion

    [ObservableProperty]
    [NotifyPropertyChangedFor(nameof(IsList))]
    [NotifyPropertyChangedFor(nameof(IsAdd))]
    private ChildrenView _view = ChildrenView.List;

    public bool IsList => View == ChildrenView.List;
    public bool IsAdd => View == ChildrenView.Add;

    public ObservableCollection<ChildRow> Items { get; } = new();

    [ObservableProperty] private bool _isLoadingList;

    #region Add fields

    [ObservableProperty] private string _childRegNoInput = string.Empty;

    [ObservableProperty]
    [NotifyPropertyChangedFor(nameof(AddIsIdle))]
    [NotifyPropertyChangedFor(nameof(AddIsWaiting))]
    [NotifyPropertyChangedFor(nameof(AddIsSuccess))]
    [NotifyPropertyChangedFor(nameof(AddIsError))]
    private ChildAddPhase _addPhase = ChildAddPhase.Idle;

    public bool AddIsIdle => AddPhase == ChildAddPhase.Idle;
    public bool AddIsWaiting => AddPhase is ChildAddPhase.Initiating or ChildAddPhase.Waiting;
    public bool AddIsSuccess => AddPhase == ChildAddPhase.Success;
    public bool AddIsError => AddPhase == ChildAddPhase.Error;

    [ObservableProperty] private string _pendingChildName = string.Empty;
    [ObservableProperty] private string _verificationCode = string.Empty;
    [ObservableProperty] private string _enrollCode = string.Empty;
    [ObservableProperty] private string _enrollCountdown = string.Empty;

    #endregion

    [RelayCommand]
    private async Task LoadListAsync(CancellationToken ct)
    {
        if (_sessions.Current is null)
        {
            _navigation.NavigateTo(NavigationRoutes.Login);
            return;
        }
        IsLoadingList = true;
        try
        {
            var result = await _guardian.ListChildrenAsync(ct).ConfigureAwait(false);
            _ui.Post(() =>
            {
                Items.Clear();
                if (result.IsFailure)
                {
                    ErrorMessage = TranslateError(result.Error);
                    return;
                }
                ErrorMessage = string.Empty;
                foreach (var c in result.Value) Items.Add(MapRow(c));
            });
        }
        finally
        {
            _ui.Post(() => IsLoadingList = false);
        }
    }

    [RelayCommand]
    private void ShowAdd()
    {
        ResetAdd();
        View = ChildrenView.Add;
    }

    [RelayCommand]
    private void BackToList()
    {
        _pollCts?.Cancel();
        ResetAdd();
        View = ChildrenView.List;
        _ = LoadListAsync(CancellationToken.None);
    }

    [RelayCommand]
    private async Task AddInitAsync(CancellationToken ct)
    {
        var reg = ChildRegNoInput.Trim();
        if (reg.Length < 5)
        {
            ErrorMessage = _l10n.GetString("Children_Error_RegNoTooShort");
            AddPhase = ChildAddPhase.Error;
            return;
        }
        ErrorMessage = string.Empty;
        AddPhase = ChildAddPhase.Initiating;

        var result = await _guardian.AddChildInitAsync(reg, ct).ConfigureAwait(false);
        if (result.IsFailure)
        {
            _ui.Post(() =>
            {
                ErrorMessage = TranslateError(result.Error);
                AddPhase = ChildAddPhase.Error;
            });
            return;
        }

        var sess = result.Value;
        _ui.Post(() =>
        {
            PendingChildName = sess.ChildName;
            VerificationCode = sess.VerificationCode;
            AddPhase = ChildAddPhase.Waiting;
        });

        _pollCts?.Cancel();
        _pollCts = new CancellationTokenSource();
        var token = _pollCts.Token;
        _ = Task.Run(() => CodePollLoopAsync(sess.SessionId, token), token);
    }

    private async Task CodePollLoopAsync(string sessionId, CancellationToken ct)
    {
        try
        {
            using var timer = new PeriodicTimer(TimeSpan.FromMilliseconds(2000));
            var deadline = DateTimeOffset.UtcNow.AddMinutes(5);
            while (!ct.IsCancellationRequested && DateTimeOffset.UtcNow < deadline)
            {
                var result = await _guardian.AddChildCodeAsync(sessionId, ct).ConfigureAwait(false);
                if (ct.IsCancellationRequested) return;
                if (result.IsSuccess)
                {
                    _ui.Post(() =>
                    {
                        EnrollCode = result.Value.EnrollCode;
                        ErrorMessage = string.Empty;
                        AddPhase = ChildAddPhase.Success;
                    });
                    StartEnrollCountdown(result.Value.ExpiresInSeconds, ct);
                    return;
                }
                var err = result.Error;
                // Backend байхгүй / жинхэнэ алдаа → зогсоох. Бусад (зөвшөөрөл хүлээгдэж
                // байгаа) тохиолдолд дахин poll.
                if (err.Message == "guardian_backend_not_available"
                    || err.Code is ApiErrorCode.Unauthorized or ApiErrorCode.ServerError)
                {
                    _ui.Post(() =>
                    {
                        ErrorMessage = TranslateError(err);
                        AddPhase = ChildAddPhase.Error;
                    });
                    return;
                }
                try { if (!await timer.WaitForNextTickAsync(ct).ConfigureAwait(false)) return; }
                catch (OperationCanceledException) { return; }
            }
            if (!ct.IsCancellationRequested)
            {
                _ui.Post(() =>
                {
                    ErrorMessage = _l10n.GetString("Children_Error_Timeout");
                    AddPhase = ChildAddPhase.Error;
                });
            }
        }
        catch (OperationCanceledException) { }
    }

    private void StartEnrollCountdown(int seconds, CancellationToken ct)
    {
        if (seconds <= 0) { _ui.Post(() => EnrollCountdown = string.Empty); return; }
        var end = DateTimeOffset.UtcNow.AddSeconds(seconds);
        _ = Task.Run(async () =>
        {
            try
            {
                using var timer = new PeriodicTimer(TimeSpan.FromSeconds(1));
                while (await timer.WaitForNextTickAsync(ct).ConfigureAwait(false))
                {
                    var rem = end - DateTimeOffset.UtcNow;
                    if (rem < TimeSpan.Zero) rem = TimeSpan.Zero;
                    _ui.Post(() => EnrollCountdown = rem.ToString(@"mm\:ss", CultureInfo.InvariantCulture));
                    if (rem == TimeSpan.Zero) return;
                }
            }
            catch (OperationCanceledException) { }
        }, ct);
    }

    /// Бүртгэлийн кодыг хуулна. Код нь нэг удаагийн нууц утга тул
    /// IClipboardService нь ClipboardClearSeconds-ийн дараа clipboard-оос
    /// автоматаар устгана (өөр зүйл хуулсан бол хөндөхгүй).
    [RelayCommand]
    private void CopyEnrollCode()
    {
        if (string.IsNullOrWhiteSpace(EnrollCode))
        {
            return;
        }
        _clipboard.SetText(EnrollCode, sensitive: true);
        _audit.Write(new AuditEvent(
            _clock.UtcNow,
            AuditEvents.SensitiveCopied,
            Actor: null,
            new Dictionary<string, string?>(StringComparer.Ordinal) { ["kind"] = "child.enroll_code" }));
        CopiedLabel = _l10n.GetString("Children_Copied");
    }

    /// Хуулсны дараа товч зуур харагдах баталгаа ("Хуулагдлаа").
    [ObservableProperty] private string _copiedLabel = string.Empty;

    [RelayCommand]
    private async Task RevokeAsync(ChildRow row)
    {
        if (row is null || string.IsNullOrWhiteSpace(row.RegNo)) return;
        var result = await _guardian.RevokeChildAsync(row.RegNo, CancellationToken.None).ConfigureAwait(false);
        _ui.Post(() =>
        {
            if (result.IsFailure) ErrorMessage = TranslateError(result.Error);
        });
        await LoadListAsync(CancellationToken.None).ConfigureAwait(false);
    }

    private ChildRow MapRow(GuardianChild c) => new(
        c.Etsi,
        c.RegNo,
        c.Name,
        c.BirthDate ?? "—",
        c.Registered ? _l10n.GetString("Children_Registered") : _l10n.GetString("Children_NotRegistered"),
        c.Registered ? "OK" : "Warn",
        c.CertNotAfter is { } d ? d.LocalDateTime.ToString("yyyy-MM-dd", CultureInfo.InvariantCulture) : "—");

    private string TranslateError(ApiError err)
    {
        if (!string.IsNullOrEmpty(err.Message))
        {
            return err.Message switch
            {
                "guardian_backend_not_available" => _l10n.GetString("Children_Error_NotAvailable"),
                "not_guardian" => _l10n.GetString("Children_Error_NotGuardian"),
                "xyp_unavailable" => _l10n.GetString("Children_Error_XypUnavailable"),
                _ => err.Message,
            };
        }
        return _l10n.GetString("Children_Error_Generic");
    }

    private void ResetAdd()
    {
        _pollCts?.Cancel();
        _pollCts = null;
        ChildRegNoInput = string.Empty;
        PendingChildName = string.Empty;
        VerificationCode = string.Empty;
        EnrollCode = string.Empty;
        CopiedLabel = string.Empty;
        EnrollCountdown = string.Empty;
        ErrorMessage = string.Empty;
        AddPhase = ChildAddPhase.Idle;
    }

    private void RefreshLabels()
    {
        Title = _l10n.GetString("Children_Title");
        Subtitle = _l10n.GetString("Children_Subtitle");
        ListHeader = _l10n.GetString("Children_List_Header");
        EmptyListLabel = _l10n.GetString("Children_List_Empty");
        AddButtonLabel = _l10n.GetString("Children_Add_Button");
        BackLabel = _l10n.GetString("Children_Back");
        ChildRegNoLabel = _l10n.GetString("Children_ChildRegNo");
        AddInitLabel = _l10n.GetString("Children_Add_Init");
        WaitingLabel = _l10n.GetString("Children_Waiting");
        VerificationLabel = _l10n.GetString("Children_Verification");
        EnrollHeaderLabel = _l10n.GetString("Children_EnrollCode_Header");
        EnrollHintLabel = _l10n.GetString("Children_EnrollCode_Hint");
        CancelLabel = _l10n.GetString("Children_Cancel");
        DoneLabel = _l10n.GetString("Children_Done");
        CopyLabel = _l10n.GetString("Children_Copy");
        RevokeLabel = _l10n.GetString("Children_Revoke");
    }
}

public sealed record ChildRow(
    string Etsi,
    string RegNo,
    string Name,
    string BirthDate,
    string StatusLabel,
    string StatusVariant,
    string CertUntil);
