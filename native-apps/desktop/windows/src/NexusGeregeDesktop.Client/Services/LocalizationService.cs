using System.Globalization;
using NexusGeregeDesktop.Application.Configuration;
using NexusGeregeDesktop.Presentation.Services;
using Microsoft.Extensions.Options;
using Microsoft.Windows.ApplicationModel.Resources;
using Windows.Globalization;
using Windows.Storage;

namespace NexusGeregeDesktop.Client.Services;

public sealed class LocalizationService : ILocalizationService
{
    private const string SettingsKey = "ui.culture";

    // ResourceManager + ResourceContext let us pin a specific language
    // at lookup time. ResourceLoader caches a context internally and
    // does NOT re-read PrimaryLanguageOverride on the fly — fresh
    // GetString calls still return the bootstrap-time language. By
    // owning the context here we just update the "Language" qualifier
    // on Apply() and every subsequent lookup honors it immediately.
    private readonly ResourceManager _manager = new();
    private readonly ResourceMap _stringsMap;
    private readonly ResourceContext _context;

    private CultureInfo _current;

    public LocalizationService(IOptions<NexusGeregeDesktopOptions> options)
    {
        ArgumentNullException.ThrowIfNull(options);
        var loc = options.Value.Localization;
        Supported = loc.SupportedCultures
            .Select(static c => CultureInfo.GetCultureInfo(c))
            .ToArray();

        var persisted = ReadPersisted();
        _current = persisted is not null && Supported.Any(c => c.Name == persisted.Name)
            ? persisted
            : Supported.FirstOrDefault(c => c.Name == loc.DefaultCulture) ?? Supported[0];

        TrySetPrimaryLanguage(_current.Name);

        // Resource map subtree "Resources" matches the default .resw
        // file name — the same surface ResourceLoader exposes.
        _stringsMap = _manager.MainResourceMap.GetSubtree("Resources");
        _context = _manager.CreateResourceContext();
        _context.QualifierValues["Language"] = _current.Name;
    }

    /// Баруунаас зүүн бичиглэлтэй хэл (араб, еврей, перс…) дээр аппын үндсэн цонхны
    /// FlowDirection-ыг эргүүлнэ. Цонх хараахан үүсээгүй бол чимээгүй өнгөрнө —
    /// App.xaml.cs эхлэхдээ мөн энэ утгыг тавина.
    private static void TrySetFlowDirection(CultureInfo culture)
    {
        try
        {
            var content = App.Window?.Content as Microsoft.UI.Xaml.FrameworkElement;
            if (content is not null)
            {
                content.FlowDirection = culture.TextInfo.IsRightToLeft
                    ? Microsoft.UI.Xaml.FlowDirection.RightToLeft
                    : Microsoft.UI.Xaml.FlowDirection.LeftToRight;
            }
        }
        catch
        {
            // Цонхгүй (тест/эхлэлийн үе) — алгасана.
        }
    }

    private static void TrySetPrimaryLanguage(string cultureName)
    {
        try
        {
            ApplicationLanguages.PrimaryLanguageOverride = cultureName;
        }
        catch (InvalidOperationException)
        {
            // Unpackaged apps cannot set PrimaryLanguageOverride; the OS-derived
            // PrimaryLanguageOverride defaults to user system language. This is fine
            // for dev; in M8 (MSIX) the package context will make this succeed.
        }
        catch (NotImplementedException)
        {
        }
    }

    public CultureInfo Current => _current;

    public IReadOnlyList<CultureInfo> Supported { get; }

    public event EventHandler<CultureInfo>? CultureChanged;

    public string GetString(string key)
    {
        try
        {
            var candidate = _stringsMap.TryGetValue(key, _context);
            var value = candidate?.ValueAsString ?? string.Empty;
            return string.IsNullOrEmpty(value) ? key : value;
        }
        catch
        {
            return key;
        }
    }

    public void Apply(CultureInfo culture)
    {
        ArgumentNullException.ThrowIfNull(culture);
        if (culture.Name == _current.Name)
        {
            return;
        }
        if (!Supported.Any(c => c.Name == culture.Name))
        {
            throw new ArgumentException($"Culture '{culture.Name}' not supported.", nameof(culture));
        }
        _current = culture;
        TrySetPrimaryLanguage(culture.Name);
        // Араб зэрэг баруунаас зүүн хэлэнд цонхны урсгалын чиглэлийг эргүүлнэ
        // (docs/I18N.md — RTL). WinUI-ийн бүх байрлал үүнээс хамаарна.
        TrySetFlowDirection(culture);
        // Re-pin the resource context to the new language. Subsequent
        // GetString lookups go straight to the .resw file matching the
        // new culture's qualifier — no per-instance loader recreate
        // needed.
        _context.QualifierValues["Language"] = culture.Name;
        Persist(culture.Name);
        CultureChanged?.Invoke(this, culture);
    }

    private static CultureInfo? ReadPersisted()
    {
        try
        {
            if (ApplicationData.Current.LocalSettings.Values.TryGetValue(SettingsKey, out var raw)
                && raw is string s
                && !string.IsNullOrWhiteSpace(s))
            {
                return CultureInfo.GetCultureInfo(s);
            }
        }
        catch
        {
            // ApplicationData not available for unpackaged + unsupported scenarios.
        }
        return null;
    }

    private static void Persist(string cultureName)
    {
        try
        {
            ApplicationData.Current.LocalSettings.Values[SettingsKey] = cultureName;
        }
        catch
        {
            // Best-effort.
        }
    }
}
