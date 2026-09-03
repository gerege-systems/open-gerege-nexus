using NexusGeregeDesktop.Presentation.ViewModels.Pages;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Navigation;

namespace NexusGeregeDesktop.Client.Views.Pages;

public sealed partial class OrganizationsPage : Page
{
    public OrganizationsViewModel ViewModel { get; }

    public OrganizationsPage()
    {
        ViewModel = App.Services.GetRequiredService<OrganizationsViewModel>();
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

    private void OnOrgRowClick(object sender, RoutedEventArgs e)
    {
        if (sender is Button btn && btn.Tag is OrgRow row && ViewModel.OpenDetailCommand.CanExecute(row))
        {
            ViewModel.OpenDetailCommand.Execute(row);
        }
    }

    private void OnRemoveMemberClick(object sender, RoutedEventArgs e)
    {
        if (sender is Button btn && btn.Tag is MemberRow row && ViewModel.RemoveMemberCommand.CanExecute(row))
        {
            ViewModel.RemoveMemberCommand.Execute(row);
        }
    }
}
