using System.Text;
using NexusGeregeDesktop.Client.Controls;
using NexusGeregeDesktop.Domain.Tokens;
using NexusGeregeDesktop.Presentation.ViewModels.Pages;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Input;
using Microsoft.UI.Xaml.Navigation;
using Windows.Storage;
using Windows.Storage.Pickers;
using Windows.System;
using WinRT.Interop;

namespace NexusGeregeDesktop.Client.Views.Pages;

public sealed partial class TokenDetailPage : Page
{
    public TokenDetailViewModel ViewModel { get; }

    public TokenDetailPage()
    {
        ViewModel = App.Services.GetRequiredService<TokenDetailViewModel>();
        InitializeComponent();
        Unloaded += (_, _) => ViewModel.Dispose();
    }

    protected override async void OnNavigatedTo(NavigationEventArgs e)
    {
        base.OnNavigatedTo(e);
        if (e.Parameter is TokenDetailParameter p)
        {
            await ViewModel.InitializeAsync(p.ProviderId, p.Token);
        }
    }

    private void OnPinKeyDown(object sender, KeyRoutedEventArgs e)
    {
        if (e.Key == VirtualKey.Enter && ViewModel.UnlockCommand.CanExecute(null))
        {
            ViewModel.UnlockCommand.Execute(null);
            e.Handled = true;
        }
    }

    private async void OnImportCertClick(object sender, RoutedEventArgs e)
    {
        try
        {
            var picker = new FileOpenPicker();
            picker.SuggestedStartLocation = PickerLocationId.DocumentsLibrary;
            picker.FileTypeFilter.Add(".cer");
            picker.FileTypeFilter.Add(".crt");
            picker.FileTypeFilter.Add(".der");
            picker.FileTypeFilter.Add(".pem");

            var hwnd = App.WindowHandle;
            if (hwnd != IntPtr.Zero)
            {
                InitializeWithWindow.Initialize(picker, hwnd);
            }

            var file = await picker.PickSingleFileAsync();
            if (file is null) return;

            var buffer = await FileIO.ReadBufferAsync(file);
            var bytes = new byte[buffer.Length];
            using (var reader = Windows.Storage.Streams.DataReader.FromBuffer(buffer))
            {
                reader.ReadBytes(bytes);
            }

            // Decide DER vs PEM by sniffing the first few bytes. PEM
            // starts with ASCII "-----BEGIN" / "-----CERT"; otherwise
            // we treat the buffer as DER directly.
            byte[] der = LooksLikePem(bytes)
                ? PemToDer(Encoding.ASCII.GetString(bytes)) ?? bytes
                : bytes;

            await ViewModel.ImportCertificateAsync(der).ConfigureAwait(true);
        }
        catch
        {
            // Picker / IO failures are surfaced through the view-model's
            // ImportCertStatus string in the happy path; user can re-try.
        }
    }

    private static bool LooksLikePem(byte[] bytes)
    {
        // PEM is ASCII text; DER is binary. Peek first 16 bytes for the
        // BEGIN marker.
        if (bytes.Length < 11) return false;
        var prefix = Encoding.ASCII.GetString(bytes, 0, Math.Min(16, bytes.Length));
        return prefix.Contains("-----BEGIN", StringComparison.Ordinal);
    }

    private static byte[]? PemToDer(string pem)
    {
        var begin = pem.IndexOf("-----BEGIN", StringComparison.Ordinal);
        var end = pem.IndexOf("-----END", StringComparison.Ordinal);
        if (begin < 0 || end < 0 || end <= begin) return null;
        var afterBeginHeader = pem.IndexOf('\n', begin);
        if (afterBeginHeader < 0) return null;
        var body = pem.Substring(afterBeginHeader + 1, end - afterBeginHeader - 1)
            .Replace("\r", string.Empty)
            .Replace("\n", string.Empty)
            .Trim();
        try { return Convert.FromBase64String(body); }
        catch { return null; }
    }

    private async void OnDeleteObjectClick(object sender, RoutedEventArgs e)
    {
        if (sender is not Button btn || btn.Tag is not TokenObjectRow row)
        {
            return;
        }
        // Confirm before destroying anything on the token — destructive
        // and irreversible. Use a ContentDialog with explicit copy
        // (object label + kind) so the citizen knows exactly what's
        // about to be erased.
        var confirm = new ContentDialog
        {
            XamlRoot = this.XamlRoot,
            Title = "Delete object?",
            Content = $"This will permanently delete '{row.Label}' ({row.Kind}) from the token. " +
                      $"This cannot be undone.\n\nObject id: {row.Id}",
            PrimaryButtonText = "Delete",
            CloseButtonText = "Cancel",
            DefaultButton = ContentDialogButton.Close,
        };
        var result = await confirm.ShowAsync();
        if (result != ContentDialogResult.Primary) return;

        await ViewModel.DeleteObjectAsync(row).ConfigureAwait(true);
    }

    private async void OnCertDetailClick(object sender, RoutedEventArgs e)
    {
        if (sender is not Button btn || btn.Tag is not TokenObjectRow row)
        {
            return;
        }
        var der = await ViewModel.ReadCertificateDerAsync(row).ConfigureAwait(true);
        if (der is null || der.Length == 0)
        {
            return;
        }
        var dialog = new CertDetailDialog
        {
            // XamlRoot is required for ContentDialog inside a WinUI 3
            // page — the dialog otherwise throws "must have a XamlRoot"
            // when ShowAsync runs.
            XamlRoot = this.XamlRoot,
        };
        dialog.LoadFromDer(der);
        await dialog.ShowAsync();
    }
}

public sealed record TokenDetailParameter(string ProviderId, TokenInfo Token);
