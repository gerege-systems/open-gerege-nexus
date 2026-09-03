using System.Collections.ObjectModel;
using System.Globalization;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Domain.Org;
using NexusGeregeDesktop.Domain.Primitives;
using NexusGeregeDesktop.Presentation.Navigation;
using NexusGeregeDesktop.Presentation.Services;

namespace NexusGeregeDesktop.Presentation.ViewModels.Pages;

public enum OrgView
{
    List,
    Register,
    Detail,
}

public enum OrgRegisterPhase
{
    Idle,
    Looking,
    LookedUp,
    Initiating,
    Waiting,
    Success,
    Error,
}

public partial class OrganizationsViewModel : ObservableObject, IDisposable
{
    public void Dispose()
    {
        _signCts?.Cancel();
        _signCts?.Dispose();
        _signCts = null;
        GC.SuppressFinalize(this);
    }

    private readonly ILocalizationService _l10n;
    private readonly IOrgService _orgs;
    private readonly IWebSessionStore _sessions;
    private readonly INavigationService _navigation;
    private readonly IUiDispatcher _ui;
    private readonly IClock _clock;

    private CancellationTokenSource? _signCts;
    private DateTimeOffset _signExpiresAt;
    private Guid _activeRegisterSession;

    public OrganizationsViewModel(
        ILocalizationService l10n,
        IOrgService orgs,
        IWebSessionStore sessions,
        INavigationService navigation,
        IUiDispatcher ui,
        IClock clock)
    {
        _l10n = l10n;
        _orgs = orgs;
        _sessions = sessions;
        _navigation = navigation;
        _ui = ui;
        _clock = clock;
        BuildExpiryChoices();
        RefreshLabels();
        _l10n.CultureChanged += (_, _) =>
        {
            BuildExpiryChoices();
            RefreshLabels();
        };
    }

    #region Labels

    [ObservableProperty] private string _title = string.Empty;
    [ObservableProperty] private string _subtitle = string.Empty;
    [ObservableProperty] private string _listHeader = string.Empty;
    [ObservableProperty] private string _emptyListLabel = string.Empty;
    [ObservableProperty] private string _registerButtonLabel = string.Empty;
    [ObservableProperty] private string _backLabel = string.Empty;
    [ObservableProperty] private string _regNumberLabel = string.Empty;
    [ObservableProperty] private string _searchLabel = string.Empty;
    [ObservableProperty] private string _expirySectionLabel = string.Empty;
    [ObservableProperty] private string _verificationLabel = string.Empty;
    [ObservableProperty] private string _waitingLabel = string.Empty;
    [ObservableProperty] private string _cancelLabel = string.Empty;
    [ObservableProperty] private string _continueLabel = string.Empty;
    [ObservableProperty] private string _membersSectionLabel = string.Empty;
    [ObservableProperty] private string _addMemberLabel = string.Empty;
    [ObservableProperty] private string _roleSignerLabel = string.Empty;
    [ObservableProperty] private string _roleAdminLabel = string.Empty;
    [ObservableProperty] private string _englishNameLabel = string.Empty;
    [ObservableProperty] private string _englishNameUnset = string.Empty;
    [ObservableProperty] private string _editLabel = string.Empty;
    [ObservableProperty] private string _saveLabel = string.Empty;
    [ObservableProperty] private string _errorMessage = string.Empty;

    #endregion

    [ObservableProperty]
    [NotifyPropertyChangedFor(nameof(IsList))]
    [NotifyPropertyChangedFor(nameof(IsRegister))]
    [NotifyPropertyChangedFor(nameof(IsDetail))]
    private OrgView _view = OrgView.List;

    public bool IsList => View == OrgView.List;
    public bool IsRegister => View == OrgView.Register;
    public bool IsDetail => View == OrgView.Detail;

    public ObservableCollection<OrgRow> Items { get; } = new();
    public ObservableCollection<MemberRow> Members { get; } = new();

    [ObservableProperty] private bool _isLoadingList;
    [ObservableProperty] private bool _isLoadingMembers;

    #region Register fields

    [ObservableProperty] private string _regNumberInput = string.Empty;

    [ObservableProperty]
    [NotifyPropertyChangedFor(nameof(RegisterIsIdle))]
    [NotifyPropertyChangedFor(nameof(RegisterIsLooking))]
    [NotifyPropertyChangedFor(nameof(RegisterIsLookedUp))]
    [NotifyPropertyChangedFor(nameof(RegisterIsWaiting))]
    [NotifyPropertyChangedFor(nameof(RegisterIsSuccess))]
    [NotifyPropertyChangedFor(nameof(RegisterIsError))]
    private OrgRegisterPhase _registerPhase = OrgRegisterPhase.Idle;

    public bool RegisterIsIdle => RegisterPhase == OrgRegisterPhase.Idle;
    public bool RegisterIsLooking => RegisterPhase == OrgRegisterPhase.Looking;
    public bool RegisterIsLookedUp => RegisterPhase == OrgRegisterPhase.LookedUp;
    public bool RegisterIsWaiting => RegisterPhase == OrgRegisterPhase.Waiting;
    public bool RegisterIsSuccess => RegisterPhase == OrgRegisterPhase.Success;
    public bool RegisterIsError => RegisterPhase == OrgRegisterPhase.Error;

    [ObservableProperty] private string _lookupName = string.Empty;
    [ObservableProperty] private string _lookupType = string.Empty;
    [ObservableProperty] private string _lookupCeo = string.Empty;
    [ObservableProperty] private string _lookupPhone = string.Empty;

    public ObservableCollection<ExpiryChoice> ExpiryChoices { get; } = new();
    [ObservableProperty] private ExpiryChoice? _selectedExpiry;

    [ObservableProperty] private string _verificationCode = string.Empty;
    [ObservableProperty] private string _countdown = "05:00";

    #endregion

    #region Detail fields

    [ObservableProperty] private OrgRow? _detail;
    [ObservableProperty] private string _detailNameEn = string.Empty;
    [ObservableProperty] private bool _isEditingNameEn;
    [ObservableProperty] private string _addMemberNationalId = string.Empty;
    [ObservableProperty] private OrgRole _addMemberRole = OrgRole.Signer;

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
            var result = await _orgs.ListAsync(ct).ConfigureAwait(false);
            _ui.Post(() =>
            {
                Items.Clear();
                if (result.IsFailure)
                {
                    ErrorMessage = TranslateError(result.Error);
                    return;
                }
                ErrorMessage = string.Empty;
                foreach (var o in result.Value)
                {
                    Items.Add(MapRow(o));
                }
            });
        }
        finally
        {
            _ui.Post(() => IsLoadingList = false);
        }
    }

    [RelayCommand]
    private void ShowRegister()
    {
        ResetRegister();
        View = OrgView.Register;
    }

    [RelayCommand]
    private void BackToList()
    {
        _signCts?.Cancel();
        ResetRegister();
        Detail = null;
        Members.Clear();
        View = OrgView.List;
        _ = LoadListAsync(CancellationToken.None);
    }

    [RelayCommand]
    private async Task LookupAsync(CancellationToken ct)
    {
        var reg = RegNumberInput.Trim();
        if (reg.Length < 5)
        {
            ErrorMessage = _l10n.GetString("Org_Error_RegNumberTooShort");
            RegisterPhase = OrgRegisterPhase.Error;
            return;
        }
        ErrorMessage = string.Empty;
        RegisterPhase = OrgRegisterPhase.Looking;
        var result = await _orgs.LookupAsync(reg, ct).ConfigureAwait(false);
        _ui.Post(() =>
        {
            if (result.IsFailure)
            {
                ErrorMessage = TranslateError(result.Error);
                RegisterPhase = OrgRegisterPhase.Error;
                return;
            }
            var lr = result.Value;
            if (!lr.Found)
            {
                ErrorMessage = _l10n.GetString("Org_Error_NotFound");
                RegisterPhase = OrgRegisterPhase.Error;
                return;
            }
            LookupName = lr.Name ?? string.Empty;
            LookupType = lr.Type ?? string.Empty;
            LookupCeo = lr.Ceo ?? string.Empty;
            LookupPhone = lr.Phone ?? string.Empty;
            RegisterPhase = OrgRegisterPhase.LookedUp;
        });
    }

    [RelayCommand]
    private async Task RegisterAsync(CancellationToken ct)
    {
        if (SelectedExpiry is null)
        {
            ErrorMessage = _l10n.GetString("Org_Error_SelectExpiry");
            return;
        }
        RegisterPhase = OrgRegisterPhase.Initiating;
        var result = await _orgs.RegisterAsync(RegNumberInput.Trim(), SelectedExpiry.Value, ct).ConfigureAwait(false);
        if (result.IsFailure)
        {
            _ui.Post(() =>
            {
                ErrorMessage = TranslateError(result.Error);
                RegisterPhase = OrgRegisterPhase.Error;
            });
            return;
        }

        var session = result.Value;
        _activeRegisterSession = session.SessionId;
        _signExpiresAt = session.ExpiresAt;
        _ui.Post(() =>
        {
            VerificationCode = session.VerificationCode;
            UpdateCountdown();
            RegisterPhase = OrgRegisterPhase.Waiting;
        });

        _signCts?.Cancel();
        _signCts = new CancellationTokenSource();
        var token = _signCts.Token;
        _ = Task.Run(() => CountdownLoopAsync(token), token);
        _ = Task.Run(() => RegisterPollLoopAsync(session.SessionId, token), token);
    }

    [RelayCommand]
    private void CancelRegister()
    {
        _signCts?.Cancel();
        _ui.Post(() =>
        {
            ResetRegister();
            View = OrgView.List;
        });
    }

    private async Task RegisterPollLoopAsync(Guid sessionId, CancellationToken ct)
    {
        try
        {
            using var timer = new PeriodicTimer(TimeSpan.FromMilliseconds(2000));
            while (!ct.IsCancellationRequested)
            {
                var result = await _orgs.FinalizeRegisterAsync(sessionId, ct).ConfigureAwait(false);
                if (ct.IsCancellationRequested) return;
                if (result.IsSuccess)
                {
                    _ui.Post(() =>
                    {
                        RegisterPhase = OrgRegisterPhase.Success;
                        ErrorMessage = string.Empty;
                    });
                    await Task.Delay(1200, ct).ConfigureAwait(false);
                    _ui.Post(() => BackToList());
                    return;
                }
                // Finalize 4xx other than "not_completed_yet" surfaces as error.
                var err = result.Error;
                if (err.Code == ApiErrorCode.BadRequest
                    && !string.IsNullOrEmpty(err.Message)
                    && err.Message.Contains("not_completed", StringComparison.OrdinalIgnoreCase))
                {
                    // Citizen still confirming on phone — keep polling.
                }
                else if (err.Code == ApiErrorCode.NotFound)
                {
                    // Session may not yet be finalisable; keep polling.
                }
                else if (err.Code == ApiErrorCode.Network)
                {
                    // Transient — keep polling.
                }
                else
                {
                    _ui.Post(() =>
                    {
                        ErrorMessage = TranslateError(err);
                        RegisterPhase = OrgRegisterPhase.Error;
                    });
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
                if (_signExpiresAt <= _clock.UtcNow)
                {
                    _ui.Post(() =>
                    {
                        ErrorMessage = _l10n.GetString("Org_Error_SignTimeout");
                        RegisterPhase = OrgRegisterPhase.Error;
                    });
                    _signCts?.Cancel();
                    return;
                }
                _ui.Post(UpdateCountdown);
            }
        }
        catch (OperationCanceledException) { }
    }

    private void UpdateCountdown()
    {
        var remaining = _signExpiresAt - _clock.UtcNow;
        if (remaining < TimeSpan.Zero) remaining = TimeSpan.Zero;
        Countdown = remaining.ToString(@"mm\:ss", CultureInfo.InvariantCulture);
    }

    [RelayCommand]
    private async Task OpenDetailAsync(OrgRow row)
    {
        if (row is null) return;
        Detail = row;
        DetailNameEn = row.NameEn ?? string.Empty;
        View = OrgView.Detail;
        await ReloadMembersAsync(row.Id, CancellationToken.None).ConfigureAwait(false);
    }

    private async Task ReloadMembersAsync(Guid orgId, CancellationToken ct)
    {
        IsLoadingMembers = true;
        try
        {
            var result = await _orgs.ListMembersAsync(orgId, ct).ConfigureAwait(false);
            _ui.Post(() =>
            {
                Members.Clear();
                if (result.IsFailure)
                {
                    ErrorMessage = TranslateError(result.Error);
                    return;
                }
                foreach (var m in result.Value)
                {
                    Members.Add(MapMember(m));
                }
            });
        }
        finally
        {
            _ui.Post(() => IsLoadingMembers = false);
        }
    }

    [RelayCommand]
    private async Task AddMemberAsync(CancellationToken ct)
    {
        if (Detail is null || string.IsNullOrWhiteSpace(AddMemberNationalId)) return;
        var result = await _orgs.AddMemberAsync(Detail.Id, AddMemberNationalId, AddMemberRole, ct).ConfigureAwait(false);
        _ui.Post(() =>
        {
            if (result.IsFailure)
            {
                ErrorMessage = TranslateError(result.Error);
                return;
            }
            ErrorMessage = string.Empty;
            AddMemberNationalId = string.Empty;
        });
        await ReloadMembersAsync(Detail.Id, ct).ConfigureAwait(false);
    }

    [RelayCommand]
    private async Task RemoveMemberAsync(MemberRow row)
    {
        if (Detail is null || row is null || row.UserId is null) return;
        var result = await _orgs.RemoveMemberAsync(Detail.Id, row.UserId.Value, CancellationToken.None).ConfigureAwait(false);
        _ui.Post(() =>
        {
            if (result.IsFailure)
            {
                ErrorMessage = TranslateError(result.Error);
            }
        });
        await ReloadMembersAsync(Detail.Id, CancellationToken.None).ConfigureAwait(false);
    }

    [RelayCommand]
    private void StartEditNameEn()
    {
        IsEditingNameEn = true;
    }

    [RelayCommand]
    private async Task SaveNameEnAsync(CancellationToken ct)
    {
        if (Detail is null) return;
        var result = await _orgs.UpdateEnglishNameAsync(Detail.Id, DetailNameEn ?? string.Empty, ct).ConfigureAwait(false);
        _ui.Post(() =>
        {
            if (result.IsFailure)
            {
                ErrorMessage = TranslateError(result.Error);
                return;
            }
            IsEditingNameEn = false;
            // Update the row in-place so the list reflects the change.
            var existing = Items.FirstOrDefault(i => i.Id == Detail.Id);
            if (existing is not null)
            {
                var idx = Items.IndexOf(existing);
                Items[idx] = existing with { NameEn = DetailNameEn };
            }
            Detail = Detail with { NameEn = DetailNameEn };
        });
    }

    [RelayCommand]
    private void CancelEditNameEn()
    {
        IsEditingNameEn = false;
        DetailNameEn = Detail?.NameEn ?? string.Empty;
    }

    private OrgRow MapRow(Organization o) => new(
        o.Id,
        o.RegistrationNumber,
        o.Name,
        o.NameEn,
        StatusLabel(o.Status),
        o.Status == OrgStatus.Active ? "OK" : (o.Status == OrgStatus.Pending ? "Warn" : "Idle"));

    private MemberRow MapMember(OrgMember m) => new(
        m.Id,
        m.UserId,
        m.UserName ?? string.Empty,
        m.NationalId,
        m.Role == OrgRole.Admin ? _l10n.GetString("Org_Role_Admin") : _l10n.GetString("Org_Role_Signer"),
        m.Status == OrgMemberStatus.Active ? _l10n.GetString("Org_Member_Active") : _l10n.GetString("Org_Member_Pending"),
        m.Status == OrgMemberStatus.Active ? "OK" : "Warn",
        m.Role == OrgRole.Admin);

    private string StatusLabel(OrgStatus status) => status switch
    {
        OrgStatus.Active    => _l10n.GetString("Org_Status_Active"),
        OrgStatus.Pending   => _l10n.GetString("Org_Status_Pending"),
        OrgStatus.Suspended => _l10n.GetString("Org_Status_Suspended"),
        OrgStatus.Removed   => _l10n.GetString("Org_Status_Removed"),
        _ => _l10n.GetString("Org_Status_Unknown"),
    };

    private string TranslateError(ApiError err)
    {
        if (!string.IsNullOrEmpty(err.Message))
        {
            return err.Message switch
            {
                "not_ceo" => _l10n.GetString("Org_Error_NotCeo"),
                "already_registered" => _l10n.GetString("Org_Error_AlreadyRegistered"),
                "expiry_class_exceeds_cap" => _l10n.GetString("Org_Error_ExpiryCap"),
                "xyp_unavailable" => _l10n.GetString("Org_Error_XypUnavailable"),
                "not_implemented" => _l10n.GetString("Org_Error_NotImplemented"),
                // OrgService-ийн org-WRITE stub-ууд (backend-д /api/* эквивалент алга).
                "not_available_on_first_party_backend" => _l10n.GetString("Org_Error_NotAvailable"),
                _ => err.Message,
            };
        }
        return _l10n.GetString("Org_Error_Generic");
    }

    private void ResetRegister()
    {
        _signCts?.Cancel();
        _signCts = null;
        _activeRegisterSession = Guid.Empty;
        _signExpiresAt = default;
        RegNumberInput = string.Empty;
        LookupName = LookupType = LookupCeo = LookupPhone = string.Empty;
        VerificationCode = string.Empty;
        Countdown = "05:00";
        ErrorMessage = string.Empty;
        RegisterPhase = OrgRegisterPhase.Idle;
        SelectedExpiry = ExpiryChoices.FirstOrDefault();
    }

    private void BuildExpiryChoices()
    {
        ExpiryChoices.Clear();
        ExpiryChoices.Add(new ExpiryChoice(OrgExpiryClass.Now,        _l10n.GetString("Org_Expiry_Now")));
        ExpiryChoices.Add(new ExpiryChoice(OrgExpiryClass.ThreeDays,  _l10n.GetString("Org_Expiry_3Days")));
        ExpiryChoices.Add(new ExpiryChoice(OrgExpiryClass.SevenDays,  _l10n.GetString("Org_Expiry_7Days")));
        ExpiryChoices.Add(new ExpiryChoice(OrgExpiryClass.ThirtyDays, _l10n.GetString("Org_Expiry_30Days")));
        SelectedExpiry = ExpiryChoices.FirstOrDefault();
    }

    private void RefreshLabels()
    {
        Title = _l10n.GetString("Org_Title");
        Subtitle = _l10n.GetString("Org_Subtitle");
        ListHeader = _l10n.GetString("Org_List_Header");
        EmptyListLabel = _l10n.GetString("Org_List_Empty");
        RegisterButtonLabel = _l10n.GetString("Org_Register_Button");
        BackLabel = _l10n.GetString("Org_Back");
        RegNumberLabel = _l10n.GetString("Org_RegNumber");
        SearchLabel = _l10n.GetString("Org_Search");
        ExpirySectionLabel = _l10n.GetString("Org_Expiry_Section");
        VerificationLabel = _l10n.GetString("Org_Verification");
        WaitingLabel = _l10n.GetString("Org_Waiting");
        CancelLabel = _l10n.GetString("Org_Cancel");
        ContinueLabel = _l10n.GetString("Org_Continue");
        MembersSectionLabel = _l10n.GetString("Org_Members_Section");
        AddMemberLabel = _l10n.GetString("Org_AddMember");
        RoleSignerLabel = _l10n.GetString("Org_Role_Signer");
        RoleAdminLabel = _l10n.GetString("Org_Role_Admin");
        EnglishNameLabel = _l10n.GetString("Org_EnglishName");
        EnglishNameUnset = _l10n.GetString("Org_EnglishName_Unset");
        EditLabel = _l10n.GetString("Org_Edit");
        SaveLabel = _l10n.GetString("Org_Save");
    }
}

public sealed record ExpiryChoice(OrgExpiryClass Value, string Label);

public sealed record OrgRow(
    Guid Id,
    string RegistrationNumber,
    string Name,
    string? NameEn,
    string StatusLabel,
    string StatusVariant);

public sealed record MemberRow(
    Guid Id,
    Guid? UserId,
    string UserName,
    string NationalId,
    string RoleLabel,
    string StatusLabel,
    string StatusVariant,
    bool IsAdmin);
