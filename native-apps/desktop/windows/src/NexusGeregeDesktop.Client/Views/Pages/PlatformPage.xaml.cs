using System;
using NexusGeregeDesktop.Application.Configuration;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Options;
using Microsoft.UI.Xaml.Controls;

namespace NexusGeregeDesktop.Client.Views.Pages;

/// <summary>
/// Платформын ажлын муж, WebView2-оор.
/// </summary>
/// <remarks>
/// ViewModel-гүй нь санаатай: энэ хуудсанд төлөв байхгүй — нэг хаяг л ачаална.
/// ViewModel нэмэх нь нэг талбарыг зөөх давхарга нэмнэ.
///
/// <c>NewWindowRequested</c> нь хаягийг систем браузарт өгнө: SHELL_CONTRACT §1a
/// ёсоор ажлын муж хоёр дахь webview нээж болохгүй.
/// </remarks>
public sealed partial class PlatformPage : Page
{
    public PlatformPage()
    {
        InitializeComponent();

        var options = App.Services.GetRequiredService<IOptions<NexusGeregeDesktopOptions>>().Value;
        Work.Source = new Uri(options.Backend.BaseUrl, UriKind.Absolute);

        Work.CoreWebView2Initialized += (_, _) =>
        {
            Work.CoreWebView2.NewWindowRequested += (_, args) =>
            {
                args.Handled = true;
                Windows.System.Launcher.LaunchUriAsync(new Uri(args.Uri)).AsTask();
            };
        };
    }
}
