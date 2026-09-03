using System.Collections.ObjectModel;
using System.Globalization;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Domain.Primitives;
using NexusGeregeDesktop.Presentation.Navigation;
using NexusGeregeDesktop.Presentation.Services;

namespace NexusGeregeDesktop.Presentation.ViewModels.Pages;

public partial class DashboardViewModel : ObservableObject
{
    private readonly ILocalizationService _l10n;
    private readonly IDashboardService _dashboard;
    private readonly IWebSessionStore _sessions;
    private readonly INavigationService _navigation;
    private readonly IUiDispatcher _ui;

    public DashboardViewModel(
        ILocalizationService l10n,
        IDashboardService dashboard,
        IWebSessionStore sessions,
        INavigationService navigation,
        IUiDispatcher ui)
    {
        _l10n = l10n;
        _dashboard = dashboard;
        _sessions = sessions;
        _navigation = navigation;
        _ui = ui;
        RefreshLabels();
        _l10n.CultureChanged += (_, _) => RefreshLabels();
    }

    #region Labels

    [ObservableProperty] private string _title = string.Empty;
    [ObservableProperty] private string _subtitle = string.Empty;
    [ObservableProperty] private string _greeting = string.Empty;
    [ObservableProperty] private string _statusBadge = string.Empty;
    [ObservableProperty] private string _statsSecurityScoreLabel = string.Empty;
    [ObservableProperty] private string _statsCertificatesLabel = string.Empty;
    [ObservableProperty] private string _statsLoginsLabel = string.Empty;
    [ObservableProperty] private string _statsLastLoginLabel = string.Empty;
    [ObservableProperty] private string _devicesSectionTitle = string.Empty;
    [ObservableProperty] private string _devicesEmpty = string.Empty;
    [ObservableProperty] private string _activitySectionTitle = string.Empty;
    [ObservableProperty] private string _activityEmpty = string.Empty;
    [ObservableProperty] private string _refreshLabel = string.Empty;
    [ObservableProperty] private string _loadingLabel = string.Empty;
    [ObservableProperty] private string _errorMessage = string.Empty;

    #endregion

    [ObservableProperty]
    [NotifyPropertyChangedFor(nameof(IsLoading))]
    [NotifyPropertyChangedFor(nameof(IsLoaded))]
    [NotifyPropertyChangedFor(nameof(IsError))]
    private DashboardState _state = DashboardState.Initial;

    public bool IsLoading => State == DashboardState.Loading;
    public bool IsLoaded => State == DashboardState.Loaded;
    public bool IsError => State == DashboardState.Error;

    [ObservableProperty] private string _displayName = string.Empty;
    [ObservableProperty] private string _displayNationalId = string.Empty;
    [ObservableProperty] private string _displayKyc = string.Empty;
    [ObservableProperty] private string _displaySecurityScore = "98%";
    [ObservableProperty] private string _displayCertCount = "0";
    [ObservableProperty] private string _displayLoginCount = "0";
    [ObservableProperty] private string _displayLastLogin = string.Empty;

    public ObservableCollection<DeviceRow> Devices { get; } = new();
    public ObservableCollection<ActivityRow> Activity { get; } = new();

    [RelayCommand]
    private async Task LoadAsync(CancellationToken ct)
    {
        // Hydrate cached session info immediately so the hero card doesn't
        // flicker through "—" while we wait for the network call.
        if (_sessions.Current is { } cached)
        {
            DisplayName = cached.FullName;
            DisplayNationalId = cached.NationalId;
            DisplayKyc = cached.KycLevel;
        }

        State = DashboardState.Loading;
        var result = await _dashboard.LoadAsync(ct).ConfigureAwait(false);
        if (result.IsFailure)
        {
            _ui.Post(() =>
            {
                if (result.Error.Code == ApiErrorCode.Unauthorized)
                {
                    // QR/иргэний нэвтрэлт нь bearer-гүй (identity зөвхөн session cache-д) —
                    // backend dashboard дата 401 өгдөг ч session хүчинтэй. Тиймээс login руу
                    // буцалгүй, cached мэдээллийг (нэр/РД) харуулна. Session үнэхээр
                    // байхгүй (гарсан/дууссан) бол л login руу.
                    if (_sessions.Current is null)
                    {
                        _navigation.NavigateTo(NavigationRoutes.Login);
                        return;
                    }
                    State = DashboardState.Loaded;   // cached identity-тэй, backend датагүй
                    return;
                }
                ErrorMessage = string.IsNullOrWhiteSpace(result.Error.Message)
                    ? _l10n.GetString("Dashboard_Error_Generic")
                    : result.Error.Message;
                State = DashboardState.Error;
            });
            return;
        }

        var snap = result.Value;
        _ui.Post(() =>
        {
            DisplayName = snap.User.FullName;
            DisplayNationalId = snap.User.NationalId;
            DisplayKyc = snap.User.KycLevel;
            DisplayCertCount = snap.Certificates.ToString(CultureInfo.InvariantCulture);
            DisplayLoginCount = snap.TotalLogins.ToString(CultureInfo.InvariantCulture);
            DisplayLastLogin = snap.Sessions.Count > 0
                ? snap.Sessions[0].CreatedAt.LocalDateTime.ToString("yyyy-MM-dd HH:mm", CultureInfo.InvariantCulture)
                : string.Empty;

            Devices.Clear();
            foreach (var d in snap.Devices)
            {
                Devices.Add(new DeviceRow(
                    d.DeviceId.ToString(),
                    d.Platform,
                    PlatformDisplayName(d.Platform),
                    StatusLabel(d.Status),
                    d.Status.Equals("ACTIVE", StringComparison.OrdinalIgnoreCase) ? "OK" : "Idle",
                    d.LastUsedAt?.LocalDateTime.ToString("yyyy-MM-dd HH:mm", CultureInfo.InvariantCulture) ?? "—",
                    d.CreatedAt.LocalDateTime.ToString("yyyy-MM-dd", CultureInfo.InvariantCulture)));
            }

            Activity.Clear();
            foreach (var s in snap.Sessions)
            {
                // Дэд систем (жишээ: sso.dgov.mn) нь үндсэн гарчиг; байхгүй бол RP нэр рүү унана.
                var service = string.IsNullOrWhiteSpace(s.SubsystemName) ? s.RpName : s.SubsystemName;
                // Эцэг RP-г зөвхөн дэд системээс ялгаатай үед 2 дахь мөрөнд харуулна (давхардал хасна).
                var parentRp = !string.IsNullOrWhiteSpace(s.SubsystemName)
                    && !string.Equals(s.SubsystemName, s.RpName, StringComparison.OrdinalIgnoreCase)
                    ? s.RpName : string.Empty;
                Activity.Add(new ActivityRow(
                    s.Id.ToString(),
                    SessionTypeLabel(s.SessionType),
                    service ?? string.Empty,
                    parentRp,
                    s.PushText ?? string.Empty,
                    ActivityMeta(s),
                    ResultLabel(s.Result),
                    s.Result.Equals("OK", StringComparison.OrdinalIgnoreCase) ? "OK" : "Bad",
                    s.CreatedAt.LocalDateTime.ToString("yyyy-MM-dd HH:mm", CultureInfo.InvariantCulture)));
            }

            ErrorMessage = string.Empty;
            State = DashboardState.Loaded;
        });
    }

    private static string PlatformDisplayName(string platform) => platform.ToUpperInvariant() switch
    {
        "IOS"     => "iPhone / iPad",
        "ANDROID" => "Android",
        _ => platform,
    };

    private string StatusLabel(string status) => status.ToUpperInvariant() switch
    {
        "ACTIVE"   => _l10n.GetString("Dashboard_Device_Active"),
        "INACTIVE" => _l10n.GetString("Dashboard_Device_Inactive"),
        _ => status,
    };

    private string SessionTypeLabel(string kind) => kind.ToUpperInvariant() switch
    {
        "AUTH" => _l10n.GetString("Dashboard_Activity_Auth"),
        "SIGN" => _l10n.GetString("Dashboard_Activity_Sign"),
        _ => kind,
    };

    // ActivityMeta — картын жижиг мета мөр: IP · төхөөрөмж · interaction (байгаа хэсгийг нь л).
    private static string ActivityMeta(DashboardActivity s)
    {
        var parts = new List<string>(3);
        if (!string.IsNullOrWhiteSpace(s.ClientIp)) parts.Add(s.ClientIp);
        if (!string.IsNullOrWhiteSpace(s.DocumentNumber))
        {
            // documentNumber нь UUID — эхний 8 тэмдэгтээр товчилно (screenshot дахь session prefix-тэй ижил).
            var dev = s.DocumentNumber.Length > 8 ? s.DocumentNumber[..8] : s.DocumentNumber;
            parts.Add(dev);
        }
        if (!string.IsNullOrWhiteSpace(s.InteractionType)) parts.Add(s.InteractionType);
        return string.Join(" · ", parts);
    }

    private string ResultLabel(string result) => result.ToUpperInvariant() switch
    {
        "OK"   => _l10n.GetString("Dashboard_Activity_Success"),
        "FAIL" => _l10n.GetString("Dashboard_Activity_Failure"),
        _ => result,
    };

    private void RefreshLabels()
    {
        Title = _l10n.GetString("Dashboard_Title");
        Subtitle = _l10n.GetString("Dashboard_Subtitle");
        Greeting = _l10n.GetString("Dashboard_Greeting");
        StatusBadge = _l10n.GetString("Dashboard_StatusBadge");
        StatsSecurityScoreLabel = _l10n.GetString("Dashboard_Stats_SecurityScore");
        StatsCertificatesLabel = _l10n.GetString("Dashboard_Stats_Certificates");
        StatsLoginsLabel = _l10n.GetString("Dashboard_Stats_Logins");
        StatsLastLoginLabel = _l10n.GetString("Dashboard_Stats_LastLogin");
        DevicesSectionTitle = _l10n.GetString("Dashboard_Devices_Section");
        DevicesEmpty = _l10n.GetString("Dashboard_Devices_Empty");
        ActivitySectionTitle = _l10n.GetString("Dashboard_Activity_Section");
        ActivityEmpty = _l10n.GetString("Dashboard_Activity_Empty");
        RefreshLabel = _l10n.GetString("Dashboard_Refresh");
        LoadingLabel = _l10n.GetString("Dashboard_Loading");
    }
}

public enum DashboardState { Initial, Loading, Loaded, Error }

public sealed record DeviceRow(
    string DeviceId,
    string Platform,
    string PlatformLabel,
    string StatusLabel,
    string StatusVariant,
    string LastUsedAt,
    string CreatedAt);

public sealed record ActivityRow(
    string Id,
    string Kind,
    string Service,
    string RpName,
    string PushText,
    string Meta,
    string ResultLabel,
    string ResultVariant,
    string CreatedAt);
