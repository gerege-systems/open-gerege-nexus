using NexusGeregeDesktop.Client.Views.Pages;
using NexusGeregeDesktop.Presentation.Navigation;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Media.Animation;

namespace NexusGeregeDesktop.Client.Navigation;

public sealed class NavigationService : INavigationService
{
    private readonly Dictionary<string, Type> _routes = new(StringComparer.Ordinal)
    {
        [NavigationRoutes.Home]          = typeof(HomePage),
        [NavigationRoutes.Verify]        = typeof(VerifyPage),
        [NavigationRoutes.Sign]          = typeof(SignPage),
        [NavigationRoutes.Login]         = typeof(LoginPage),
        [NavigationRoutes.Dashboard]     = typeof(DashboardPage),
        [NavigationRoutes.Organizations] = typeof(OrganizationsPage),
        [NavigationRoutes.Children]      = typeof(ChildrenPage),
        [NavigationRoutes.Certificates]  = typeof(CertificatesPage),
        [NavigationRoutes.Settings]      = typeof(SettingsPage),
        [NavigationRoutes.Tokens]        = typeof(TokensPage),
        [NavigationRoutes.TokenDetail]   = typeof(TokenDetailPage),
        [NavigationRoutes.Platform]      = typeof(PlatformPage),
    };

    private Frame? _frame;
    private string? _currentRoute;

    public bool CanGoBack => _frame?.CanGoBack ?? false;

    public string? CurrentRoute => _currentRoute;

    public event EventHandler<string>? Navigated;

    public void AttachFrame(Frame frame)
    {
        ArgumentNullException.ThrowIfNull(frame);
        _frame = frame;
    }

    public bool NavigateTo(string routeKey, object? parameter = null)
    {
        if (_frame is null)
        {
            return false;
        }
        if (!_routes.TryGetValue(routeKey, out var pageType))
        {
            return false;
        }
        if (routeKey == _currentRoute)
        {
            return true;
        }

        var ok = _frame.Navigate(pageType, parameter, new EntranceNavigationTransitionInfo());
        if (ok)
        {
            _currentRoute = routeKey;
            Navigated?.Invoke(this, routeKey);
        }
        return ok;
    }

    public bool GoBack()
    {
        if (_frame is null || !_frame.CanGoBack)
        {
            return false;
        }
        _frame.GoBack();
        var route = _routes.FirstOrDefault(kvp => kvp.Value == _frame.SourcePageType).Key;
        if (route is not null)
        {
            _currentRoute = route;
            Navigated?.Invoke(this, route);
        }
        return true;
    }
}
