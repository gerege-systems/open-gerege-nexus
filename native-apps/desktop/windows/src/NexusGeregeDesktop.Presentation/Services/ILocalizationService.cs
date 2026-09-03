using System.Globalization;

namespace NexusGeregeDesktop.Presentation.Services;

public interface ILocalizationService
{
    CultureInfo Current { get; }

    IReadOnlyList<CultureInfo> Supported { get; }

    event EventHandler<CultureInfo>? CultureChanged;

    string GetString(string key);

    void Apply(CultureInfo culture);
}
