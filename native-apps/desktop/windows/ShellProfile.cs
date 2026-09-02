namespace GeregeNexusNativeWin;

/// <summary>
/// Тухайн build-ийн хэлбэр хүчин зүйл ба түүний domain шугам.
///
/// Windows-ийн гурван target гурван ӨӨР шугам дээр ажиллана: Desktop нь
/// win., Kiosk нь kiosk., POS нь pos. Гурвуул НЭГ ижил backend руу очно —
/// шугам нь тусдаа origin өгөхийн тулд байгаа болохоос тусдаа сервис
/// өгөхийн тулд биш. Бүртгэл: <c>native-apps/shared/device_lines.json</c>.
///
/// Шугамууд 2026-08-12-нд асав: DNS (wildcard), nginx vhost, TLS гэрчилгээ,
/// API-гийн origin allowlist дөрвүүлэн бэлэн. Web ба API нэг host дээр очдог
/// тул webview доторх дуудлага same-origin болж, session cookie нь
/// SameSite=Strict хэвээр ажиллана.
/// </summary>
public static class ShellProfile
{
#if KIOSK
    public const string FormFactor = "kiosk";
    public const string StartRoute = "/";
    public const string LineOrigin = "https://kiosk.nexus.gerege.mn";
    public static readonly string[] Capabilities = ["escpos", "scanner", "serial", "device.identity", "kiosk.lockdown", "telemetry", "shell.pane"];
#elif POS
    public const string FormFactor = "pos";
    public const string StartRoute = "/";
    public const string LineOrigin = "https://pos.nexus.gerege.mn";
    public static readonly string[] Capabilities = ["escpos", "scanner", "serial", "device.identity", "secure-store", "telemetry", "biometric", "shell.pane"];
#else
    public const string FormFactor = "desktop";
    public const string StartRoute = "/";
    public const string LineOrigin = "https://desktop.nexus.gerege.mn";
    public static readonly string[] Capabilities = ["external.open", "print.system", "secure-store", "device.identity", "telemetry", "biometric", "shell.pane"];
#endif
}
