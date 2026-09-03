using System.Globalization;
using NexusGeregeDesktop.Domain.Org;
using Microsoft.UI.Xaml.Data;

namespace NexusGeregeDesktop.Client.Converters;

/// Extracts the Nth character (0-indexed) from a string for verification-code box display.
/// Returns empty string when the string is shorter than the requested index.
public sealed class CharAtConverter : IValueConverter
{
    public object Convert(object value, Type targetType, object parameter, string language)
    {
        if (value is not string s || string.IsNullOrEmpty(s) || parameter is null)
        {
            return string.Empty;
        }
        if (!int.TryParse(parameter.ToString(), NumberStyles.Integer, CultureInfo.InvariantCulture, out var idx))
        {
            return string.Empty;
        }
        if (idx < 0 || idx >= s.Length)
        {
            return string.Empty;
        }
        return s[idx].ToString(CultureInfo.InvariantCulture);
    }

    public object ConvertBack(object value, Type targetType, object parameter, string language) =>
        throw new NotSupportedException();
}

/// Maps OrgRole enum to ComboBox SelectedIndex (Signer=0, Admin=1).
public sealed class OrgRoleIndexConverter : IValueConverter
{
    public object Convert(object value, Type targetType, object parameter, string language) =>
        value is OrgRole.Admin ? 1 : 0;

    public object ConvertBack(object value, Type targetType, object parameter, string language) =>
        value is int i && i == 1 ? OrgRole.Admin : OrgRole.Signer;
}
