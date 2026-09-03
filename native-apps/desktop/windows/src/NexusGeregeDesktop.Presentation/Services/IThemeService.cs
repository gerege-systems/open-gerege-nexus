namespace NexusGeregeDesktop.Presentation.Services;

public enum AppTheme
{
    System = 0,
    Light = 1,
    Dark = 2,
}

public interface IThemeService
{
    AppTheme Current { get; }

    event EventHandler<AppTheme>? ThemeChanged;

    void Apply(AppTheme theme);
}
