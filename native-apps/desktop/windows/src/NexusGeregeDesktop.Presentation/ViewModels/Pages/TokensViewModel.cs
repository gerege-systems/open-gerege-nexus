using System.Collections.ObjectModel;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Domain.Tokens;
using NexusGeregeDesktop.Presentation.Services;

namespace NexusGeregeDesktop.Presentation.ViewModels.Pages;

/// Dedicated USB token / smart-card page. Pulled out of SettingsViewModel
/// in May 2026 so token administration (scan, enroll, change PIN, export
/// cert, etc.) has a dedicated top-level nav entry — Settings now stays
/// focused on theme + language + version metadata.
public partial class TokensViewModel : ObservableObject
{
    private readonly ITokenRegistry _registry;
    private readonly ILocalizationService _l10n;
    private readonly IUiDispatcher _ui;

    public TokensViewModel(ITokenRegistry registry, ILocalizationService l10n, IUiDispatcher ui)
    {
        _registry = registry;
        _l10n = l10n;
        _ui = ui;

        RefreshLabels();
        ProviderSummary = string.Join(", ", _registry.Providers.Select(p => p.DisplayName));
        if (string.IsNullOrEmpty(ProviderSummary))
        {
            ProviderSummary = _l10n.GetString("Settings_Token_NoProvider");
        }

        _l10n.CultureChanged += (_, _) => RefreshLabels();
    }

    [ObservableProperty] private string _title = string.Empty;
    [ObservableProperty] private string _subtitle = string.Empty;
    [ObservableProperty] private string _sectionTitle = string.Empty;
    [ObservableProperty] private string _refreshLabel = string.Empty;
    [ObservableProperty] private string _emptyLabel = string.Empty;
    [ObservableProperty] private string _scanningLabel = string.Empty;
    [ObservableProperty] private string _providerLabel = string.Empty;
    [ObservableProperty] private string _providerSummary = string.Empty;

    public ObservableCollection<DetectedTokenRow> Tokens { get; } = new();

    [ObservableProperty] private bool _isScanningTokens;

    [RelayCommand]
    private async Task ScanTokensAsync(CancellationToken ct)
    {
        IsScanningTokens = true;
        try
        {
            var result = await _registry.DiscoverAllAsync(ct).ConfigureAwait(false);
            _ui.Post(() =>
            {
                Tokens.Clear();
                if (result.IsSuccess)
                {
                    foreach (var dt in result.Value)
                    {
                        Tokens.Add(new DetectedTokenRow(
                            ProviderId: dt.Provider.Id,
                            ProviderName: dt.Provider.DisplayName,
                            Backend: dt.Info.Backend.ToString(),
                            Label: string.IsNullOrEmpty(dt.Info.Label) ? "(no label)" : dt.Info.Label,
                            Manufacturer: dt.Info.Manufacturer,
                            Model: dt.Info.Model,
                            SerialNumber: dt.Info.SerialNumber,
                            TokenId: dt.Info.Id,
                            State: dt.Info.State,
                            Capabilities: dt.Info.Capabilities));
                    }
                }
            });
        }
        finally
        {
            _ui.Post(() => IsScanningTokens = false);
        }
    }

    private void RefreshLabels()
    {
        Title = _l10n.GetString("Tokens_Title");
        Subtitle = _l10n.GetString("Tokens_Subtitle");
        SectionTitle = _l10n.GetString("Settings_Tokens_Title");
        RefreshLabel = _l10n.GetString("Settings_Tokens_Refresh");
        EmptyLabel = _l10n.GetString("Settings_Tokens_Empty");
        ScanningLabel = _l10n.GetString("Settings_Tokens_Scanning");
        ProviderLabel = _l10n.GetString("Settings_Tokens_Provider");
    }
}
