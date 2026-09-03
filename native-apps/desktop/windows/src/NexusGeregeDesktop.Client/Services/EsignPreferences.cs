using NexusGeregeDesktop.Application.Abstractions;
using Windows.Storage;

namespace NexusGeregeDesktop.Client.Services;

/// ESIGN гүүрийн сонголтыг Windows-ийн LocalSettings-д хадгална
/// (<see cref="ThemeService"/>-тэй ижил загвар). Unpackaged горимд
/// ApplicationData байхгүй байж болзошгүй тул унших/бичихийг хамгаална —
/// тэр тохиолдолд сонголт зөвхөн session дотор амьдарна.
public sealed class EsignPreferences : IEsignPreferences
{
    private const string SettingsKey = "esign.preferSoftwareToken";

    private bool _preferSoftwareToken;

    public EsignPreferences()
    {
        _preferSoftwareToken = ReadPersisted() ?? false;
    }

    public bool PreferSoftwareToken => _preferSoftwareToken;

    public event EventHandler<bool>? PreferSoftwareTokenChanged;

    public void SetPreferSoftwareToken(bool value)
    {
        if (_preferSoftwareToken == value)
        {
            return;
        }
        _preferSoftwareToken = value;
        Persist(value);
        PreferSoftwareTokenChanged?.Invoke(this, value);
    }

    private static bool? ReadPersisted()
    {
        try
        {
            if (ApplicationData.Current.LocalSettings.Values.TryGetValue(SettingsKey, out var raw) && raw is bool b)
            {
                return b;
            }
        }
        catch (InvalidOperationException)
        {
            // Unpackaged: ApplicationData.Current хандах боломжгүй.
        }
        return null;
    }

    private static void Persist(bool value)
    {
        try
        {
            ApplicationData.Current.LocalSettings.Values[SettingsKey] = value;
        }
        catch (InvalidOperationException)
        {
            // Unpackaged: хадгалахгүй — сонголт зөвхөн энэ ажиллагааны турш үйлчилнэ.
        }
    }
}
