using System.ComponentModel;
using NexusGeregeDesktop.Client.Navigation;
using NexusGeregeDesktop.Client.Services;
using NexusGeregeDesktop.Presentation.Navigation;
using NexusGeregeDesktop.Presentation.Services;
using NexusGeregeDesktop.Presentation.ViewModels;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Media;

namespace NexusGeregeDesktop.Client;

public sealed partial class MainWindow : Window
{
    private readonly ShellViewModel _shell;
    private readonly NavigationService _navigation;

    public MainWindow()
    {
        InitializeComponent();

        ExtendsContentIntoTitleBar = true;
        SetTitleBar(AppTitleBar);
        AppWindow.SetIcon("Assets\\AppIcon.ico");

        if (Content is FrameworkElement root)
        {
            var theme = App.Services.GetRequiredService<IThemeService>();
            if (theme is ThemeService impl)
            {
                impl.ApplyToRoot(root);
            }
        }

        _shell = App.Services.GetRequiredService<ShellViewModel>();
        _navigation = (NavigationService)App.Services.GetRequiredService<INavigationService>();
        _navigation.AttachFrame(ContentFrame);

        // Listen for menu / auth-state changes from the shell VM so we
        // can repaint the rail when the citizen logs in or out. Without
        // this, the rail you see at startup never updates.
        _shell.PropertyChanged += OnShellPropertyChanged;

        ApplyAuthGate();
        RebuildNav();
        NavView.SelectionChanged += OnNavSelectionChanged;
        _shell.NavigateToInitial();
    }

    private void OnShellPropertyChanged(object? sender, PropertyChangedEventArgs e)
    {
        // IWebSessionStore.Changed fires from inside SaveAsync / ClearAsync,
        // which complete on a thread-pool continuation (their internal
        // awaits use ConfigureAwait(false)). The ShellViewModel handler
        // mutates [ObservableProperty]-backed fields, which raises this
        // PropertyChanged event on the SAME thread pool thread. Touching
        // NavView from there throws RPC_E_WRONG_THREAD silently — the
        // rail just never appears. Marshal back to the dispatcher.
        var queue = DispatcherQueue;
        if (queue is null || queue.HasThreadAccess)
        {
            ApplyShellChange(e.PropertyName);
        }
        else
        {
            queue.TryEnqueue(() => ApplyShellChange(e.PropertyName));
        }
    }

    private void ApplyShellChange(string? propertyName)
    {
        switch (propertyName)
        {
            case nameof(ShellViewModel.MenuItems):
            case nameof(ShellViewModel.FooterItems):
                RebuildNav();
                break;
            case nameof(ShellViewModel.IsAuthenticated):
                ApplyAuthGate();
                break;
            case nameof(ShellViewModel.SelectedItem):
                SyncSelection();
                break;
        }
    }

    private void ApplyAuthGate()
    {
        // Pre-auth: collapse the whole rail so the only path forward is
        // through whatever Page the Frame is currently showing (the
        // navigation service pushed Login). Post-auth: rail comes back
        // in its normal LeftCompact form.
        NavView.IsPaneVisible = _shell.IsAuthenticated;
        NavView.IsPaneToggleButtonVisible = _shell.IsAuthenticated;
        NavView.PaneDisplayMode = _shell.IsAuthenticated
            ? NavigationViewPaneDisplayMode.LeftCompact
            : NavigationViewPaneDisplayMode.Top; // collapsed pane, content fills the window
    }

    private void RebuildNav()
    {
        NavView.MenuItems.Clear();
        NavView.FooterMenuItems.Clear();
        foreach (var item in _shell.MenuItems)
        {
            NavView.MenuItems.Add(BuildItem(item));
        }
        foreach (var item in _shell.FooterItems)
        {
            NavView.FooterMenuItems.Add(BuildItem(item));
        }
        SyncSelection();
    }

    private void SyncSelection()
    {
        var route = _shell.SelectedItem?.Route;
        if (string.IsNullOrEmpty(route)) return;
        foreach (var nvi in NavView.MenuItems.Concat(NavView.FooterMenuItems).OfType<NavigationViewItem>())
        {
            if (nvi.Tag is string r && r == route)
            {
                NavView.SelectedItem = nvi;
                return;
            }
        }
    }

    private static NavigationViewItem BuildItem(MenuItem item) => new()
    {
        Content = item.Label,
        Tag = item.Route,
        Icon = new FontIcon { Glyph = item.IconGlyph },
    };

    private void OnNavSelectionChanged(NavigationView sender, NavigationViewSelectionChangedEventArgs args)
    {
        if (args.SelectedItem is NavigationViewItem nvi && nvi.Tag is string route)
        {
            var match = _shell.MenuItems.Concat(_shell.FooterItems).FirstOrDefault(m => m.Route == route);
            if (match is not null)
            {
                _shell.SelectedItem = match;
            }
        }
    }
}
