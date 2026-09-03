using System.Reflection;
using NexusGeregeDesktop.Application;
using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Client.Navigation;
using NexusGeregeDesktop.Client.Services;
using NexusGeregeDesktop.Infrastructure;
using NexusGeregeDesktop.Infrastructure.Esign;
using NexusGeregeDesktop.Presentation;
using NexusGeregeDesktop.Presentation.Navigation;
using NexusGeregeDesktop.Presentation.Services;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Serilog;

namespace NexusGeregeDesktop.Client.Hosting;

internal static class AppHost
{
    public static IHost Build(Microsoft.UI.Dispatching.DispatcherQueue uiQueue)
    {
        ArgumentNullException.ThrowIfNull(uiQueue);
        var settings = new HostApplicationBuilderSettings
        {
            ContentRootPath = AppContext.BaseDirectory,
            EnvironmentName = Environment.GetEnvironmentVariable("DOTNET_ENVIRONMENT")
                ?? Environment.GetEnvironmentVariable("EIDMNG_ENVIRONMENT")
                ?? "Production",
        };

        var builder = Host.CreateApplicationBuilder(settings);

        builder.Configuration.SetBasePath(AppContext.BaseDirectory);

        builder.Configuration
            .AddJsonFile("appsettings.json", optional: false, reloadOnChange: true)
            .AddJsonFile($"appsettings.{builder.Environment.EnvironmentName}.json", optional: true, reloadOnChange: true)
            .AddEnvironmentVariables(prefix: "EIDMNG_");

        builder.Services
            .AddSerilog((services, lc) => lc
                .ReadFrom.Configuration(builder.Configuration)
                .ReadFrom.Services(services)
                .Enrich.FromLogContext()
                .Enrich.WithEnvironmentName()
                .Enrich.WithMachineName()
                .Enrich.WithThreadId())
            .AddApplication(builder.Configuration)
            .AddInfrastructure(builder.Configuration)
            .AddPresentation();

        builder.Services.AddSingleton<ILocalizationService, LocalizationService>();
        builder.Services.AddSingleton<IThemeService, ThemeService>();
        builder.Services.AddSingleton<INavigationService, NavigationService>();
        builder.Services.AddSingleton<IUiDispatcher>(new UiDispatcher(uiQueue));
        builder.Services.AddSingleton<IClipboardService, ClipboardService>();
        // Windows Hello-ийн систем цонхыг эцэг цонхонд холбоход HWND хэрэгтэй.
        builder.Services.AddSingleton<IWindowHandleProvider, WindowHandleProvider>();

        // ESIGN тоон гарын үсгийн локал гүүр (ws:59001 + wss:59005)
        builder.Services.AddSingleton<IEsignInteraction, EsignInteraction>();
        builder.Services.AddSingleton<IEsignPreferences, EsignPreferences>();
        builder.Services.AddEsignBridge();

        return builder.Build();
    }

    public static string AssemblyVersion =>
        Assembly.GetEntryAssembly()?.GetCustomAttribute<AssemblyInformationalVersionAttribute>()?.InformationalVersion
        ?? "0.0.0-dev";
}
