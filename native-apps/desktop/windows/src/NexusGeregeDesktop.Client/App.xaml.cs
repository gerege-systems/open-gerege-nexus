using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Client.Hosting;
using NexusGeregeDesktop.Infrastructure.Security.Rasp;
using NexusGeregeDesktop.Presentation.Navigation;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;
using Serilog;

namespace NexusGeregeDesktop.Client;

public partial class App : Microsoft.UI.Xaml.Application
{
    private readonly IHost _host;

    public static IServiceProvider Services => ((App)Current)._host.Services;

    public static Microsoft.UI.Xaml.Window Window { get; private set; } = null!;

    public static Microsoft.UI.Dispatching.DispatcherQueue DispatcherQueue { get; private set; } = null!;

    public static nint WindowHandle => WinRT.Interop.WindowNative.GetWindowHandle(Window);

    public App()
    {
        BootstrapTrace("App() ctor entered");
        try
        {
            DispatcherQueue = Microsoft.UI.Dispatching.DispatcherQueue.GetForCurrentThread()
                ?? throw new InvalidOperationException("DispatcherQueue not available in App() ctor.");

            _host = AppHost.Build(DispatcherQueue);
            BootstrapTrace("AppHost.Build() OK");
            InitializeComponent();
            BootstrapTrace("InitializeComponent() OK");

            UnhandledException += OnUnhandledException;
            AppDomain.CurrentDomain.UnhandledException += OnDomainUnhandledException;
            TaskScheduler.UnobservedTaskException += OnUnobservedTaskException;
            BootstrapTrace("App() ctor done");
        }
        catch (Exception ex)
        {
            BootstrapTrace($"App() ctor FAILED: {ex.GetType().FullName}: {ex.Message}\n{ex}");
            throw;
        }
    }

    protected override void OnLaunched(Microsoft.UI.Xaml.LaunchActivatedEventArgs args)
    {
        var logger = Services.GetRequiredService<ILogger<App>>();
        logger.LogInformation("NexusGeregeDesktop.Client starting (version {Version})", AppHost.AssemblyVersion);

        Window = new MainWindow();
        Window.Activate();

        Services.GetRequiredService<IHealthMonitor>().Start();

        // ESIGN тоон гарын үсгийн локал гүүрийг асаах (ws:59001 + wss:59005)
        Services.GetRequiredService<IEsignBridgeService>().Start();

        // RASP probe at boot — logs exe SHA-256 + debugger state to audit.
        var audit = Services.GetRequiredService<IAuditLog>();
        var rasp = Services.GetRequiredService<IRaspService>().Snapshot();
        audit.Write(new AuditEvent(
            DateTimeOffset.UtcNow,
            AuditEvents.AppStarted,
            Actor: null,
            Details: new Dictionary<string, string?>
            {
                ["version"] = AppHost.AssemblyVersion,
                ["exe_sha256"] = rasp.ExecutableSha256Hex,
                ["exe_size"] = rasp.ExecutableSizeBytes.ToString(System.Globalization.CultureInfo.InvariantCulture),
            }));
        if (rasp.DebuggerAttached || rasp.RemoteDebuggerAttached)
        {
            audit.Write(new AuditEvent(
                DateTimeOffset.UtcNow,
                AuditEvents.DebuggerDetected,
                Actor: null,
                Details: new Dictionary<string, string?>
                {
                    ["managed"] = rasp.DebuggerAttached.ToString(),
                    ["remote"] = rasp.RemoteDebuggerAttached.ToString(),
                }));
        }

        // Idle-lock: when triggered, clear session and bounce to /login.
        var idle = Services.GetRequiredService<IIdleLockService>();
        idle.Locked += (_, _) =>
        {
            audit.Write(new AuditEvent(DateTimeOffset.UtcNow, AuditEvents.IdleLocked, Actor: null));
            DispatcherQueue.TryEnqueue(() =>
            {
                Services.GetRequiredService<INavigationService>().NavigateTo(NavigationRoutes.Login);
            });
        };
        idle.Start();
    }

    private void OnUnhandledException(object sender, Microsoft.UI.Xaml.UnhandledExceptionEventArgs e)
    {
        Services.GetRequiredService<ILogger<App>>()
            .LogCritical(e.Exception, "Unhandled UI exception");
        Log.CloseAndFlush();
    }

    private static void OnDomainUnhandledException(object sender, System.UnhandledExceptionEventArgs e)
    {
        if (e.ExceptionObject is Exception ex)
        {
            Log.Logger.Fatal(ex, "Unhandled AppDomain exception");
        }
        Log.CloseAndFlush();
    }

    private static void OnUnobservedTaskException(object? sender, UnobservedTaskExceptionEventArgs e)
    {
        Log.Logger.Error(e.Exception, "Unobserved task exception");
        e.SetObserved();
    }

    private static void BootstrapTrace(string message)
    {
        try
        {
            var dir = Path.Combine(
                Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
                "NexusGeregeDesktop", "logs");
            Directory.CreateDirectory(dir);
            var path = Path.Combine(dir, "bootstrap.log");
            File.AppendAllText(path, $"{DateTimeOffset.UtcNow:O}  {message}\n");
        }
        catch
        {
            // Best-effort only.
        }
    }
}
