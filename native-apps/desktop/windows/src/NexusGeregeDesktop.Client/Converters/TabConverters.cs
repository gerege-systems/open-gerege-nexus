using Microsoft.UI;
using Microsoft.UI.Xaml.Data;
using Microsoft.UI.Xaml.Media;

namespace NexusGeregeDesktop.Client.Converters;

/// True (active tab) → accent brush. False → transparent underline.
public sealed class TabUnderlineBrushConverter : IValueConverter
{
    public object Convert(object value, Type targetType, object parameter, string language)
    {
        if (value is bool b && b)
        {
            var brush = Microsoft.UI.Xaml.Application.Current.Resources["EidAccentBrush"] as Brush;
            return brush ?? new SolidColorBrush(Colors.SteelBlue);
        }
        return new SolidColorBrush(Colors.Transparent);
    }

    public object ConvertBack(object value, Type targetType, object parameter, string language) =>
        throw new NotSupportedException();
}

/// True (active tab) → accent brush. False → muted foreground.
public sealed class TabForegroundBrushConverter : IValueConverter
{
    public object Convert(object value, Type targetType, object parameter, string language)
    {
        var resources = Microsoft.UI.Xaml.Application.Current.Resources;
        if (value is bool b && b)
        {
            return (resources["EidAccentBrush"] as Brush) ?? new SolidColorBrush(Colors.SteelBlue);
        }
        return (resources["EidMutedForeground"] as Brush) ?? new SolidColorBrush(Colors.Gray);
    }

    public object ConvertBack(object value, Type targetType, object parameter, string language) =>
        throw new NotSupportedException();
}
