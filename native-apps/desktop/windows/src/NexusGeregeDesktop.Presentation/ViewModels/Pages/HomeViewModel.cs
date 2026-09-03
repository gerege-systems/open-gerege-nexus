using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Presentation.Services;

namespace NexusGeregeDesktop.Presentation.ViewModels.Pages;

public partial class HomeViewModel : ObservableObject
{
    private readonly ILocalizationService _l10n;
    private readonly IHealthMonitor _health;
    private readonly IUiDispatcher _ui;

    public HomeViewModel(ILocalizationService l10n, IHealthMonitor health, IUiDispatcher ui)
    {
        _l10n = l10n;
        _health = health;
        _ui = ui;

        RefreshLabels();
        ApplyHealth(_health.Current);

        _health.Updated += OnHealthUpdated;
        _l10n.CultureChanged += (_, _) =>
        {
            RefreshLabels();
            ApplyHealth(_health.Current);
        };
    }

    [ObservableProperty]
    private string _title = string.Empty;

    [ObservableProperty]
    private string _subtitle = string.Empty;

    [ObservableProperty]
    private string _backendSectionTitle = string.Empty;

    [ObservableProperty]
    private string _statusLabel = string.Empty;

    [ObservableProperty]
    private string _statusDetail = string.Empty;

    [ObservableProperty]
    private string _versionLabel = string.Empty;

    [ObservableProperty]
    private string _lastCheckedLabel = string.Empty;

    /// "OK" | "Warn" | "Bad" | "Idle" — driven by ValueConverter in XAML.
    [ObservableProperty]
    private string _statusVariant = "Idle";

    [RelayCommand]
    private async Task RefreshAsync(CancellationToken ct)
    {
        await _health.RefreshAsync(ct).ConfigureAwait(false);
    }

    private void OnHealthUpdated(object? sender, HealthSnapshot snapshot)
    {
        _ui.Post(() => ApplyHealth(snapshot));
    }

    private void RefreshLabels()
    {
        Title = _l10n.GetString("Home_Title");
        Subtitle = _l10n.GetString("Home_Subtitle");
        BackendSectionTitle = _l10n.GetString("Home_Backend_Title");
        VersionLabel = _l10n.GetString("Home_Backend_Version");
        LastCheckedLabel = _l10n.GetString("Home_Backend_LastChecked");
    }

    private void ApplyHealth(HealthSnapshot snapshot)
    {
        StatusLabel = snapshot.State switch
        {
            HealthState.Healthy     => _l10n.GetString("Home_Status_Healthy"),
            HealthState.Degraded    => _l10n.GetString("Home_Status_Degraded"),
            HealthState.Unreachable => _l10n.GetString("Home_Status_Unreachable"),
            HealthState.Checking    => _l10n.GetString("Home_Status_Checking"),
            _                       => _l10n.GetString("Home_Status_Unknown"),
        };
        StatusVariant = snapshot.State switch
        {
            HealthState.Healthy => "OK",
            HealthState.Degraded => "Warn",
            HealthState.Unreachable => "Bad",
            HealthState.Checking => "Idle",
            _ => "Idle",
        };
        StatusDetail = snapshot.Detail ?? string.Empty;
        VersionLabel = snapshot.Version is { Length: > 0 } v
            ? $"{_l10n.GetString("Home_Backend_Version")}: {v}"
            : _l10n.GetString("Home_Backend_Version");
        LastCheckedLabel = $"{_l10n.GetString("Home_Backend_LastChecked")}: {snapshot.CheckedAt.LocalDateTime:HH:mm:ss}";
    }
}
