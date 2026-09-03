using NexusGeregeDesktop.Client.Controls;
using NexusGeregeDesktop.Presentation.ViewModels.Pages;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Navigation;

namespace NexusGeregeDesktop.Client.Views.Pages;

/// "Миний гэрчилгээ" — /api/certificates-ийн жагсаалт. Мөрийн "харах" товч нь
/// base64 DER-ийг задалж CertDetailDialog-оор бүрэн X.509-ийг үзүүлнэ
/// (TokenDetailPage-ийн ашигладаг диалогтой ижил).
public sealed partial class CertificatesPage : Page
{
    public CertificatesViewModel ViewModel { get; }

    public CertificatesPage()
    {
        ViewModel = App.Services.GetRequiredService<CertificatesViewModel>();
        InitializeComponent();
    }

    protected override void OnNavigatedTo(NavigationEventArgs e)
    {
        base.OnNavigatedTo(e);
        if (ViewModel.LoadCommand.CanExecute(null))
        {
            ViewModel.LoadCommand.Execute(null);
        }
    }

    private async void OnViewClick(object sender, RoutedEventArgs e)
    {
        if (sender is not Button { Tag: CertificateRow row } || string.IsNullOrWhiteSpace(row.CertValueB64))
        {
            return;
        }

        byte[] der;
        try
        {
            der = Convert.FromBase64String(row.CertValueB64);
        }
        catch (FormatException)
        {
            // Backend-ээс ирсэн DER эвдэрсэн — жагсаалтыг хэвээр үлдээж чимээгүй өнгөрнө.
            return;
        }

        var dialog = new CertDetailDialog
        {
            // WinUI 3 дээр ContentDialog-д XamlRoot заавал хэрэгтэй.
            XamlRoot = this.XamlRoot,
        };
        dialog.LoadFromDer(der);
        await dialog.ShowAsync();
    }
}
