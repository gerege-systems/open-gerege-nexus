using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Presentation.Navigation;
using NexusGeregeDesktop.Presentation.Services;

namespace NexusGeregeDesktop.Presentation.ViewModels;

public partial class ShellViewModel : ObservableObject
{
    // Segoe Fluent Icons codepoints (FontIcon.Glyph string).
    private const string GlyphHome = "";       // Home
    private const string GlyphVerify = "";     // CheckMark
    private const string GlyphLogin = "";      // Contact2 / sign-in
    private const string GlyphDashboard = "";  // ViewDashboard
    private const string GlyphSettings = "";   // Settings
    private const string GlyphOrgs = "";       // People
    private const string GlyphSign = "";       // Edit / signature

    // Sentinel "route" used only inside the nav — never navigated to as
    // a page. OnSelectedItemChanged intercepts it and runs the logout
    // flow instead of telling NavigationService to push a page.
    public const string LogoutSentinelRoute = "__logout__";

    private readonly INavigationService _navigation;
    private readonly ILocalizationService _l10n;
    private readonly ICitizenAuthService _auth;
    private readonly IWebSessionStore _sessions;
    private readonly IUiDispatcher _ui;

    public ShellViewModel(
        INavigationService navigation,
        ILocalizationService l10n,
        ICitizenAuthService auth,
        IWebSessionStore sessions,
        IUiDispatcher ui)
    {
        _navigation = navigation;
        _l10n = l10n;
        _auth = auth;
        _sessions = sessions;
        _ui = ui;

        // Auth state drives the visible menu set. Unauthenticated
        // citizens only see Login; the rest of the rail materializes
        // once a web-session bearer exists. IsAuthenticated is wired
        // to NavigationView.IsPaneVisible in MainWindow.xaml so the
        // entire rail collapses on the first paint until login.
        _isAuthenticated = _sessions.Current is not null;
        AppTitle = _l10n.GetString("App_Title");
        MenuItems = BuildMenu();
        FooterItems = BuildFooter();
        _selectedItem = MenuItems[0];

        _sessions.Changed += (_, session) =>
        {
            // WebSessionStore.SaveAsync / ClearAsync finish their
            // internal awaits with ConfigureAwait(false), so this
            // handler runs on a thread-pool continuation. Setting
            // SelectedItem fires OnSelectedItemChanged → _navigation
            // .NavigateTo, which calls Frame.Navigate — STA-bound to
            // the UI thread. Without this _ui.Post, NavigateTo
            // silently no-ops and the citizen stays stuck on the
            // Login page despite the rail correctly switching to
            // its post-auth menu.
            _ui.Post(() =>
            {
                var next = session is not null;
                if (next == IsAuthenticated) return;
                IsAuthenticated = next;
                MenuItems = BuildMenu();
                FooterItems = BuildFooter();
                var fallback = MenuItems.Count > 0 ? MenuItems[0] : null;
                if (fallback is not null) SelectedItem = fallback;
            });
        };

        _l10n.CultureChanged += (_, _) =>
        {
            AppTitle = _l10n.GetString("App_Title");
            MenuItems = BuildMenu();
            FooterItems = BuildFooter();
            var current = _navigation.CurrentRoute ?? NavigationRoutes.Home;
            // Selection may have rebuilt with a different item identity;
            // pick the new one matching the current route, or fall back
            // to the first menu entry. Without this we'd hold on to a
            // dead MenuItem reference and SelectedItem-bound bindings
            // would stale-render.
            var match = MenuItems.Concat(FooterItems).FirstOrDefault(m => m.Route == current)
                ?? (MenuItems.Count > 0 ? MenuItems[0] : null);
            if (match is not null) SelectedItem = match;
        };

        _navigation.Navigated += (_, route) =>
        {
            var match = MenuItems.Concat(FooterItems).FirstOrDefault(m => m.Route == route);
            if (match is not null && !ReferenceEquals(match, SelectedItem))
            {
                SelectedItem = match;
            }
        };
    }

    [ObservableProperty]
    private string _appTitle = string.Empty;

    [ObservableProperty]
    private IReadOnlyList<MenuItem> _menuItems = Array.Empty<MenuItem>();

    [ObservableProperty]
    private IReadOnlyList<MenuItem> _footerItems = Array.Empty<MenuItem>();

    [ObservableProperty]
    private MenuItem _selectedItem;

    [ObservableProperty]
    private bool _isAuthenticated;

    partial void OnSelectedItemChanged(MenuItem value)
    {
        if (value is null)
        {
            return;
        }
        // Special-case the logout sentinel: clear the session via the
        // citizen auth service, drop the cached web session, then push
        // the user back to the Login page. We do NOT call NavigateTo on
        // the sentinel route — there's no Page for it.
        if (value.Route == LogoutSentinelRoute)
        {
            _ = LogoutAndReturnAsync();
            return;
        }
        if (value.Route != _navigation.CurrentRoute)
        {
            _navigation.NavigateTo(value.Route);
        }
    }

    private async Task LogoutAndReturnAsync()
    {
        try
        {
            await _auth.LogoutAsync(CancellationToken.None).ConfigureAwait(false);
            await _sessions.ClearAsync(CancellationToken.None).ConfigureAwait(false);
        }
        catch { /* logout is best-effort; we still want to return to Login */ }
        // Hop back to the Login menu item so the highlighted footer
        // "Logout" doesn't stay selected.
        _navigation.NavigateTo(NavigationRoutes.Login);
        var loginItem = MenuItems.Concat(FooterItems).FirstOrDefault(m => m.Route == NavigationRoutes.Login);
        if (loginItem is not null)
        {
            SelectedItem = loginItem;
        }
    }

    [RelayCommand(CanExecute = nameof(CanGoBack))]
    private void GoBack() => _navigation.GoBack();

    private bool CanGoBack() => _navigation.CanGoBack;

    public async void NavigateToInitial()
    {
        // IWebSessionStore lazy-loads from the DPAPI vault on first
        // access — without this prime call IsAuthenticated would
        // start false even when a valid session exists, and the user
        // would see a "Login" rail flash before LoadAsync completed.
        try { await _sessions.LoadAsync().ConfigureAwait(true); }
        catch { /* worst case the rail looks unauth until first action */ }

        if (_sessions.Current is not null && !IsAuthenticated)
        {
            IsAuthenticated = true;
            MenuItems = BuildMenu();
            FooterItems = BuildFooter();
            SelectedItem = MenuItems.Count > 0 ? MenuItems[0] : SelectedItem;
        }

        // First-paint route depends on auth state. Without a web
        // session bearer there's no Home / Dashboard data to render,
        // so dropping the citizen straight onto Login is the only
        // sensible affordance.
        _navigation.NavigateTo(IsAuthenticated ? NavigationRoutes.Home : NavigationRoutes.Login);
    }

    private IReadOnlyList<MenuItem> BuildMenu()
    {
        // Pre-auth: a single Login affordance so the citizen has
        // exactly one path forward. Home / Dashboard / Orgs / Verify
        // appear only after a web-session bearer is in IWebSessionStore.
        if (!IsAuthenticated)
        {
            return new[]
            {
                new MenuItem(NavigationRoutes.Login, _l10n.GetString("Nav_Login"), GlyphLogin),
            };
        }
        return new[]
        {
            new MenuItem(NavigationRoutes.Home,          _l10n.GetString("Nav_Home"),          GlyphHome),
            new MenuItem(NavigationRoutes.Dashboard,     _l10n.GetString("Nav_Dashboard"),     GlyphDashboard),
            new MenuItem(NavigationRoutes.Certificates,  _l10n.GetString("Nav_Certificates"),  ""),
            new MenuItem(NavigationRoutes.Organizations, _l10n.GetString("Nav_Organizations"), GlyphOrgs),
            new MenuItem(NavigationRoutes.Children,      _l10n.GetString("Nav_Children"),      ""),
            new MenuItem(NavigationRoutes.Tokens,        _l10n.GetString("Nav_Tokens"),        ""),
            new MenuItem(NavigationRoutes.Sign,          _l10n.GetString("Nav_Sign"),          GlyphSign),
            new MenuItem(NavigationRoutes.Verify,        _l10n.GetString("Nav_Verify"),        GlyphVerify),
            // Платформын ажлын муж нь сүүлд: энэ нь аппын өөрийн дэлгэц биш
            // тул native дэлгэцүүдийн дараа байх нь хүнд аль нь аль болохыг хэлнэ.
            new MenuItem(NavigationRoutes.Platform,      _l10n.GetString("Nav_Platform"),      ""),
        };
    }

    private IReadOnlyList<MenuItem> BuildFooter()
    {
        // Settings + Logout only make sense post-login. Hiding them
        // pre-auth keeps the rail empty of dead-end affordances.
        if (!IsAuthenticated)
        {
            return Array.Empty<MenuItem>();
        }
        return new[]
        {
            new MenuItem(NavigationRoutes.Settings, _l10n.GetString("Nav_Settings"), GlyphSettings),
            new MenuItem(LogoutSentinelRoute, _l10n.GetString("Nav_Logout"), ""),
        };
    }
}

public sealed record MenuItem(string Route, string Label, string IconGlyph);
