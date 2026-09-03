using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Domain.Tokens;
using NexusGeregeDesktop.Presentation.Navigation;
using NexusGeregeDesktop.Presentation.ViewModels.Pages;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Navigation;

namespace NexusGeregeDesktop.Client.Views.Pages;

public sealed partial class TokensPage : Page
{
    public TokensViewModel ViewModel { get; }

    public TokensPage()
    {
        ViewModel = App.Services.GetRequiredService<TokensViewModel>();
        InitializeComponent();
    }

    protected override void OnNavigatedTo(NavigationEventArgs e)
    {
        base.OnNavigatedTo(e);
        if (ViewModel.ScanTokensCommand.CanExecute(null))
        {
            ViewModel.ScanTokensCommand.Execute(null);
        }
    }

    private void OnViewTokenClick(object sender, RoutedEventArgs e)
    {
        if (sender is not Button btn || btn.Tag is not DetectedTokenRow row)
        {
            return;
        }
        var navigation = App.Services.GetRequiredService<INavigationService>();
        var backend = Enum.TryParse<TokenBackend>(row.Backend, out var b) ? b : TokenBackend.WindowsCng;
        var info = new TokenInfo(
            Id: row.TokenId,
            Label: row.Label,
            Manufacturer: row.Manufacturer,
            SerialNumber: row.SerialNumber,
            Model: row.Model,
            Backend: backend,
            Removable: true,
            HardwareSlot: true,
            IsLoggedIn: false,
            State: row.State,
            Capabilities: row.Capabilities);
        navigation.NavigateTo(NavigationRoutes.TokenDetail, new TokenDetailParameter(row.ProviderId, info));
    }
}
