using NexusGeregeDesktop.Presentation.ViewModels.Pages;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Windows.ApplicationModel.DataTransfer;
using Windows.Storage.Pickers;

namespace NexusGeregeDesktop.Client.Views.Pages;

public sealed partial class VerifyPage : Page
{
    public VerifyViewModel ViewModel { get; }

    public VerifyPage()
    {
        ViewModel = App.Services.GetRequiredService<VerifyViewModel>();
        InitializeComponent();
    }

    private async void OnPickFileClick(object sender, RoutedEventArgs e)
    {
        var picker = new FileOpenPicker();
        WinRT.Interop.InitializeWithWindow.Initialize(picker, App.WindowHandle);
        picker.SuggestedStartLocation = PickerLocationId.DocumentsLibrary;
        picker.FileTypeFilter.Add(".pem");
        picker.FileTypeFilter.Add(".cer");
        picker.FileTypeFilter.Add(".crt");
        picker.FileTypeFilter.Add(".der");
        picker.FileTypeFilter.Add("*");

        var file = await picker.PickSingleFileAsync();
        if (file is not null)
        {
            ViewModel.ParseFromFilePath(file.Path);
        }
    }

    private void OnDragOver(object sender, DragEventArgs e)
    {
        if (e.DataView.Contains(StandardDataFormats.StorageItems))
        {
            e.AcceptedOperation = DataPackageOperation.Copy;
            e.DragUIOverride.Caption = "Open certificate";
            e.DragUIOverride.IsCaptionVisible = true;
            e.DragUIOverride.IsGlyphVisible = true;
            e.Handled = true;
        }
    }

    private async void OnDrop(object sender, DragEventArgs e)
    {
        if (!e.DataView.Contains(StandardDataFormats.StorageItems))
        {
            return;
        }
        var items = await e.DataView.GetStorageItemsAsync();
        if (items.Count > 0 && items[0] is Windows.Storage.StorageFile file)
        {
            ViewModel.ParseFromFilePath(file.Path);
        }
    }
}
