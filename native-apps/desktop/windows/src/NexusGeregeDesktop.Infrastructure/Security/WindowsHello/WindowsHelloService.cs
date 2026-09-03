using System.Runtime.Versioning;
using NexusGeregeDesktop.Application.Abstractions;
using Microsoft.Extensions.Logging;
using Windows.Security.Credentials.UI;

namespace NexusGeregeDesktop.Infrastructure.Security.WindowsHello;

[SupportedOSPlatform("windows10.0.17763.0")]
public sealed class WindowsHelloService : IWindowsHello
{
    private readonly IWindowHandleProvider _window;
    private readonly ILogger<WindowsHelloService> _logger;

    public WindowsHelloService(IWindowHandleProvider window, ILogger<WindowsHelloService> logger)
    {
        _window = window;
        _logger = logger;
    }

    public async ValueTask<bool> IsAvailableAsync(CancellationToken ct = default)
    {
        var availability = await UserConsentVerifier.CheckAvailabilityAsync().AsTask(ct).ConfigureAwait(false);
        return availability == UserConsentVerifierAvailability.Available;
    }

    public async ValueTask<HelloResult> RequestConsentAsync(string reasonLocalized, CancellationToken ct = default)
    {
        if (string.IsNullOrWhiteSpace(reasonLocalized))
        {
            throw new ArgumentException("Reason must be non-empty.", nameof(reasonLocalized));
        }

        var availability = await UserConsentVerifier.CheckAvailabilityAsync().AsTask(ct).ConfigureAwait(false);
        var preflight = MapAvailability(availability);
        if (preflight is not HelloResult.Verified)
        {
            _logger.LogWarning("Windows Hello unavailable: {Availability}", availability);
            return preflight;
        }

        try
        {
            // WinUI 3 нь Win32 desktop апп тул системийн Hello цонхыг эцэг цонхонд
            // холбох ёстой. Цонхны handle байхгүй үед (тест / эхлэлийн үе) л плайн
            // API руу унана — тэр нь desktop дээр ихэвчлэн алдаа өгдөг тул доор
            // барьж авч "боломжгүй" гэж үзнэ.
            var hwnd = _window.Handle;
            var result = hwnd != IntPtr.Zero
                ? await UserConsentVerifierInterop
                    .RequestVerificationForWindowAsync(hwnd, reasonLocalized)
                    .AsTask(ct)
                    .ConfigureAwait(false)
                : await UserConsentVerifier
                    .RequestVerificationAsync(reasonLocalized)
                    .AsTask(ct)
                    .ConfigureAwait(false);

            return MapResult(result);
        }
        catch (Exception ex) when (ex is System.Runtime.InteropServices.COMException
                                      or InvalidOperationException
                                      or NotSupportedException)
        {
            // Платформ дэмжихгүй / цонх холбогдоогүй — баталгаа авах боломжгүй.
            // ISensitiveActionGuard энэ үр дүнг "боломжгүй" гэж үзэж үйлдлийг
            // блоклохгүй (биометргүй компьютер дээр гарын үсгийг бүрмөсөн хаахгүй).
            _logger.LogWarning(ex, "Windows Hello баталгаа авах боломжгүй байна");
            return HelloResult.NotAvailable;
        }
    }

    private static HelloResult MapAvailability(UserConsentVerifierAvailability availability) => availability switch
    {
        UserConsentVerifierAvailability.Available => HelloResult.Verified,
        UserConsentVerifierAvailability.DeviceNotPresent => HelloResult.DeviceNotPresent,
        UserConsentVerifierAvailability.NotConfiguredForUser => HelloResult.NotAvailable,
        UserConsentVerifierAvailability.DisabledByPolicy => HelloResult.DisabledByPolicy,
        UserConsentVerifierAvailability.DeviceBusy => HelloResult.Failed,
        _ => HelloResult.Failed,
    };

    private static HelloResult MapResult(UserConsentVerificationResult result) => result switch
    {
        UserConsentVerificationResult.Verified => HelloResult.Verified,
        UserConsentVerificationResult.DeviceNotPresent => HelloResult.DeviceNotPresent,
        UserConsentVerificationResult.NotConfiguredForUser => HelloResult.NotAvailable,
        UserConsentVerificationResult.DisabledByPolicy => HelloResult.DisabledByPolicy,
        UserConsentVerificationResult.Canceled => HelloResult.Cancelled,
        UserConsentVerificationResult.RetriesExhausted => HelloResult.RetriesExhausted,
        _ => HelloResult.Failed,
    };
}
