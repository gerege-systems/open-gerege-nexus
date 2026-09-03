using System.ComponentModel;
using NexusGeregeDesktop.Presentation.ViewModels.Pages;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Input;
using Microsoft.UI.Xaml.Media.Imaging;
using Microsoft.UI.Xaml.Navigation;
using Windows.Storage.Streams;
using Windows.System;

namespace NexusGeregeDesktop.Client.Views.Pages;

public sealed partial class LoginPage : Page
{
    public LoginViewModel ViewModel { get; }

    public LoginPage()
    {
        ViewModel = App.Services.GetRequiredService<LoginViewModel>();
        InitializeComponent();

        ViewModel.PropertyChanged += OnViewModelPropertyChanged;
        Unloaded += (_, _) => ViewModel.PropertyChanged -= OnViewModelPropertyChanged;

        // Initial paint in case the VM already has bytes (e.g. after re-navigation).
        _ = SyncQrImageAsync();
    }

    protected override void OnNavigatedTo(NavigationEventArgs e)
    {
        base.OnNavigatedTo(e);
        ViewModel.OnEnteredPage();
    }

    private async void OnViewModelPropertyChanged(object? sender, PropertyChangedEventArgs e)
    {
        if (e.PropertyName == nameof(LoginViewModel.QrPng))
        {
            await SyncQrImageAsync();
        }
    }

    private async Task SyncQrImageAsync()
    {
        var bytes = ViewModel.QrPng;
        if (bytes is null || bytes.Length == 0)
        {
            QrImage.Source = null;
            return;
        }
        try
        {
            var bitmap = new BitmapImage();
            using var stream = new InMemoryRandomAccessStream();
            using (var writer = new DataWriter(stream))
            {
                writer.WriteBytes(bytes);
                await writer.StoreAsync();
                await writer.FlushAsync();
                writer.DetachStream();
            }
            stream.Seek(0);
            await bitmap.SetSourceAsync(stream);
            QrImage.Source = bitmap;
        }
        catch
        {
            QrImage.Source = null;
        }
    }

    private void OnNationalIdKeyDown(object sender, KeyRoutedEventArgs e)
    {
        if (e.Key == VirtualKey.Enter && ViewModel.InitiateCommand.CanExecute(null))
        {
            ViewModel.InitiateCommand.Execute(null);
            e.Handled = true;
        }
    }
}
