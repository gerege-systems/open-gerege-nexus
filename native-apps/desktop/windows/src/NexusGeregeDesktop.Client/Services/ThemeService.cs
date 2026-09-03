using NexusGeregeDesktop.Presentation.Services;
using Microsoft.UI.Xaml;
using Windows.Storage;

namespace NexusGeregeDesktop.Client.Services;

public sealed class ThemeService : IThemeService
{
    private const string SettingsKey = "ui.theme";

    private AppTheme _current;

    public ThemeService()
    {
        _current = ReadPersisted() ?? AppTheme.System;
    }

    public AppTheme Current => _current;

    public event EventHandler<AppTheme>? ThemeChanged;

    public void Apply(AppTheme theme)
    {
        _current = theme;
        Persist(theme);

        if (App.Window?.Content is FrameworkElement root)
        {
            root.RequestedTheme = MapToElementTheme(theme);
        }

        ThemeChanged?.Invoke(this, theme);
    }

    public void ApplyToRoot(FrameworkElement root)
    {
        ArgumentNullException.ThrowIfNull(root);
        root.RequestedTheme = MapToElementTheme(_current);
    }

    private static ElementTheme MapToElementTheme(AppTheme theme) => theme switch
    {
        AppTheme.Light => ElementTheme.Light,
        AppTheme.Dark => ElementTheme.Dark,
        _ => ElementTheme.Default,
    };

    private static AppTheme? ReadPersisted()
    {
        try
        {
            if (ApplicationData.Current.LocalSettings.Values.TryGetValue(SettingsKey, out var raw)
                && raw is int i
                && Enum.IsDefined(typeof(AppTheme), i))
            {
                return (AppTheme)i;
            }
        }
        catch
        {
        }
        return null;
    }

    private static void Persist(AppTheme theme)
    {
        try
        {
            ApplicationData.Current.LocalSettings.Values[SettingsKey] = (int)theme;
        }
        catch
        {
        }
    }
}
