using Microsoft.UI;
using Microsoft.UI.Xaml.Data;
using Microsoft.UI.Xaml.Media;

namespace NexusGeregeDesktop.Client.Converters;

public sealed class StatusVariantBrushConverter : IValueConverter
{
    public object Convert(object value, Type targetType, object parameter, string language)
    {
        var variant = value as string ?? "Idle";
        var resources = Microsoft.UI.Xaml.Application.Current.Resources;
        return variant switch
        {
            "OK"   => (Brush)(resources["EidSuccessBrush"]    ?? new SolidColorBrush(Colors.Green)),
            "Warn" => (Brush)(resources["EidWarningBrush"]    ?? new SolidColorBrush(Colors.Goldenrod)),
            "Bad"  => (Brush)(resources["EidDangerBrush"]     ?? new SolidColorBrush(Colors.Red)),
            _      => (Brush)(resources["EidMutedForeground"] ?? new SolidColorBrush(Colors.Gray)),
        };
    }

    public object ConvertBack(object value, Type targetType, object parameter, string language) =>
        throw new NotSupportedException();
}
