using NexusGeregeDesktop.Presentation.ViewModels.Pages;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Windows.Storage;
using Windows.Storage.Pickers;

namespace NexusGeregeDesktop.Client.Views.Pages;

public sealed partial class SignPage : Page
{
    public SignViewModel ViewModel { get; }

    public SignPage()
    {
        ViewModel = App.Services.GetRequiredService<SignViewModel>();
        InitializeComponent();
    }

    private async void OnPickFileClick(object sender, RoutedEventArgs e)
    {
        var picker = new FileOpenPicker();
        // WinUI 3 desktop: pickers need the parent HWND or they throw.
        WinRT.Interop.InitializeWithWindow.Initialize(picker, App.WindowHandle);
        picker.SuggestedStartLocation = PickerLocationId.DocumentsLibrary;
        picker.FileTypeFilter.Add(".pdf");

        var file = await picker.PickSingleFileAsync();
        if (file is null)
        {
            return;
        }

        var bytes = await File.ReadAllBytesAsync(file.Path);
        ViewModel.Start(bytes, file.Name);
    }

    private async void OnSaveClick(object sender, RoutedEventArgs e)
    {
        var bytes = ViewModel.SignedPdfBytes;
        if (bytes is null || bytes.Length == 0)
        {
            return;
        }

        var picker = new FileSavePicker();
        WinRT.Interop.InitializeWithWindow.Initialize(picker, App.WindowHandle);
        picker.SuggestedStartLocation = PickerLocationId.DocumentsLibrary;
        picker.SuggestedFileName = ViewModel.SuggestedFileName;
        picker.FileTypeChoices.Add("PDF", new List<string> { ".pdf" });

        var file = await picker.PickSaveFileAsync();
        if (file is null)
        {
            return;
        }

        await FileIO.WriteBytesAsync(file, bytes);
        ViewModel.MarkSaved();
    }
}
