using System.Buffers;
using System.Collections.ObjectModel;
using System.Globalization;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Domain.Tokens;
using NexusGeregeDesktop.Presentation.Services;

namespace NexusGeregeDesktop.Presentation.ViewModels.Pages;

public partial class SettingsViewModel : ObservableObject
{
    private readonly IThemeService _theme;
    private readonly ILocalizationService _l10n;
    private readonly ITokenRegistry _tokens;
    private readonly IEsignPreferences _esign;
    private readonly IEsignBridgeService _bridge;
    private readonly IUiDispatcher _ui;

    public SettingsViewModel(
        IThemeService theme,
        ILocalizationService l10n,
        ITokenRegistry tokens,
        IEsignPreferences esign,
        IEsignBridgeService bridge,
        IUiDispatcher ui)
    {
        _theme = theme;
        _l10n = l10n;
        _tokens = tokens;
        _esign = esign;
        _bridge = bridge;
        _ui = ui;
        _preferSoftwareToken = esign.PreferSoftwareToken;

        // RefreshLabels нь ESIGN төлөвийн мөрийг ч шинэчилдэг (хэл сэлгэхэд дахин дуудагдана).
        RefreshLabels();

        AvailableThemes = new[]
        {
            new ThemeOption(AppTheme.System, _l10n.GetString("Settings_Theme_System")),
            new ThemeOption(AppTheme.Light,  _l10n.GetString("Settings_Theme_Light")),
            new ThemeOption(AppTheme.Dark,   _l10n.GetString("Settings_Theme_Dark")),
        };
        // GroupBy is defensive: if the configured SupportedCultures
        // pipeline ever surfaces the same culture twice (config-merge
        // glitch, DI re-init, etc.), the language ComboBox would
        // otherwise render duplicate "Монгол / English / Монгол /
        // English" rows. One row per culture key.
        AvailableLanguages = _l10n.Supported
            .GroupBy(c => c.Name, StringComparer.OrdinalIgnoreCase)
            .Select(g => g.First())
            .Select(c => new LanguageOption(c, NativeLanguageName(c)))
            .ToArray();

        _selectedTheme = AvailableThemes.First(t => t.Value == _theme.Current);
        _selectedLanguage = AvailableLanguages.First(l => l.Culture.Name == _l10n.Current.Name);

        ProviderSummary = string.Join(", ", _tokens.Providers.Select(p => p.DisplayName));
        if (string.IsNullOrEmpty(ProviderSummary))
        {
            ProviderSummary = _l10n.GetString("Settings_Token_NoProvider");
        }

        _theme.ThemeChanged += (_, t) => SelectedTheme = AvailableThemes.First(x => x.Value == t);
        _l10n.CultureChanged += (_, c) =>
        {
            SelectedLanguage = AvailableLanguages.First(x => x.Culture.Name == c.Name);
            RefreshLabels();
        };
    }

    #region Labels

    [ObservableProperty] private string _title = string.Empty;
    [ObservableProperty] private string _themeSectionTitle = string.Empty;
    [ObservableProperty] private string _languageSectionTitle = string.Empty;
    [ObservableProperty] private string _versionLabel = string.Empty;
    [ObservableProperty] private string _tokensSectionTitle = string.Empty;
    [ObservableProperty] private string _tokensRefreshLabel = string.Empty;
    [ObservableProperty] private string _tokensEmptyLabel = string.Empty;
    [ObservableProperty] private string _tokensScanningLabel = string.Empty;
    [ObservableProperty] private string _tokensProviderLabel = string.Empty;
    [ObservableProperty] private string _providerSummary = string.Empty;
    [ObservableProperty] private string _esignSectionTitle = string.Empty;
    [ObservableProperty] private string _esignPreferSoftwareLabel = string.Empty;
    [ObservableProperty] private string _esignPreferSoftwareHint = string.Empty;
    [ObservableProperty] private string _esignBridgeLabel = string.Empty;
    [ObservableProperty] private string _esignStatus = string.Empty;
    [ObservableProperty] private string _esignToggleBridgeLabel = string.Empty;

    #endregion

    #region ESIGN

    /// Физик токен залгаастай ч программ токеноор (утас руу PIN2 push) зурах эсэх.
    [ObservableProperty] private bool _preferSoftwareToken;

    partial void OnPreferSoftwareTokenChanged(bool value) => _esign.SetPreferSoftwareToken(value);

    /// Гүүрийг гараар асаах/унтраах — порт эзлэгдсэн үед дахин оролдоход хэрэгтэй.
    [RelayCommand]
    private void ToggleEsignBridge()
    {
        if (_bridge.IsRunning)
        {
            _bridge.Stop();
        }
        else
        {
            _bridge.Start();
        }
        RefreshEsignStatus();
    }

    private void RefreshEsignStatus()
    {
        EsignStatus = _bridge.IsRunning
            ? string.Format(
                CultureInfo.InvariantCulture,
                _l10n.GetString("Settings_Esign_Bridge_Running"),
                _bridge.WsPort,
                _bridge.WsPort + 4)
            : _l10n.GetString("Settings_Esign_Bridge_Stopped");
        EsignToggleBridgeLabel = _bridge.IsRunning
            ? _l10n.GetString("Settings_Esign_Bridge_Stop")
            : _l10n.GetString("Settings_Esign_Bridge_Start");
    }

    #endregion

    public IReadOnlyList<ThemeOption> AvailableThemes { get; }
    public IReadOnlyList<LanguageOption> AvailableLanguages { get; }

    [ObservableProperty] private ThemeOption _selectedTheme;
    [ObservableProperty] private LanguageOption _selectedLanguage;

    public ObservableCollection<DetectedTokenRow> Tokens { get; } = new();

    [ObservableProperty] private bool _isScanningTokens;

    partial void OnSelectedThemeChanged(ThemeOption value)
    {
        if (value is not null && value.Value != _theme.Current)
        {
            _theme.Apply(value.Value);
        }
    }

    partial void OnSelectedLanguageChanged(LanguageOption value)
    {
        if (value is not null && value.Culture.Name != _l10n.Current.Name)
        {
            _l10n.Apply(value.Culture);
        }
    }

    // NativeName carries the region too ("русский (Россия)", "中文(中华人民共和国)").
    // The picker only needs the language, so cut at the first space or bracket —
    // the separator differs per culture, hence both.
    // Тусгаарлагчийг НЭГ УДАА кэшлэнэ (CA1861/CA1870: дуудлага бүрд массив үүсгэхгүй,
    // SearchValues нь оновчилсон хайлт хийнэ). Хэлний тоо нэмэгдэхэд энэ функц
    // хэл сэлгүүрийн мөр бүрд дуудагдана.
    private static readonly SearchValues<char> NativeNameSeparators = SearchValues.Create(" (（");

    private static string NativeLanguageName(CultureInfo culture)
    {
        var name = culture.NativeName;
        var cut = name.AsSpan().IndexOfAny(NativeNameSeparators);
        return cut > 0 ? name[..cut].TrimEnd() : name;
    }

    [RelayCommand]
    private async Task ScanTokensAsync(CancellationToken ct)
    {
        IsScanningTokens = true;
        try
        {
            var result = await _tokens.DiscoverAllAsync(ct).ConfigureAwait(false);
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
        Title = _l10n.GetString("Settings_Title");
        ThemeSectionTitle = _l10n.GetString("Settings_Theme");
        LanguageSectionTitle = _l10n.GetString("Settings_Language");
        VersionLabel = _l10n.GetString("Settings_Version");
        TokensSectionTitle = _l10n.GetString("Settings_Tokens_Title");
        TokensRefreshLabel = _l10n.GetString("Settings_Tokens_Refresh");
        TokensEmptyLabel = _l10n.GetString("Settings_Tokens_Empty");
        TokensScanningLabel = _l10n.GetString("Settings_Tokens_Scanning");
        TokensProviderLabel = _l10n.GetString("Settings_Tokens_Provider");
        EsignSectionTitle = _l10n.GetString("Settings_Esign_Title");
        EsignPreferSoftwareLabel = _l10n.GetString("Settings_Esign_PreferSoftware");
        EsignPreferSoftwareHint = _l10n.GetString("Settings_Esign_PreferSoftware_Hint");
        EsignBridgeLabel = _l10n.GetString("Settings_Esign_Bridge");
        RefreshEsignStatus();
    }
}

public sealed record ThemeOption(AppTheme Value, string Label);

public sealed record LanguageOption(CultureInfo Culture, string Label);

public sealed record DetectedTokenRow(
    string ProviderId,
    string ProviderName,
    string Backend,
    string Label,
    string Manufacturer,
    string Model,
    string SerialNumber,
    string TokenId,
    TokenState State = TokenState.Unknown,
    TokenCapabilities Capabilities = default);
