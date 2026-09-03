using NexusGeregeDesktop.Presentation.ViewModels.Pages;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Navigation;

namespace NexusGeregeDesktop.Client.Views.Pages;

public sealed partial class ChildrenPage : Page
{
    public ChildrenViewModel ViewModel { get; }

    public ChildrenPage()
    {
        ViewModel = App.Services.GetRequiredService<ChildrenViewModel>();
        InitializeComponent();
    }

    protected override void OnNavigatedTo(NavigationEventArgs e)
    {
        base.OnNavigatedTo(e);
        if (ViewModel.LoadListCommand.CanExecute(null))
        {
            ViewModel.LoadListCommand.Execute(null);
        }
    }

    private void OnRevokeClick(object sender, RoutedEventArgs e)
    {
        if (sender is Button btn && btn.Tag is ChildRow row && ViewModel.RevokeCommand.CanExecute(row))
        {
            ViewModel.RevokeCommand.Execute(row);
        }
    }
}
