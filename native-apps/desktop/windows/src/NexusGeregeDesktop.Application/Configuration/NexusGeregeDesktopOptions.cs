using System.ComponentModel.DataAnnotations;

namespace NexusGeregeDesktop.Application.Configuration;

public sealed class NexusGeregeDesktopOptions
{
    public const string SectionName = "NexusGeregeDesktop";

    [Required]
    public BackendOptions Backend { get; init; } = new();

    [Required]
    public SecurityOptions Security { get; init; } = new();

    [Required]
    public LocalizationOptions Localization { get; init; } = new();

    public TokenOptions Tokens { get; init; } = new();
}

public sealed class TokenOptions
{
    public bool EnableWindowsCng { get; init; } = true;

    public bool EnablePkcs11 { get; init; } = true;

    /// Candidate PKCS#11 modules to probe at startup. The first one that
    /// loads + reports >=1 slot wins. Resolved as: literal path → System32
    /// → ProgramFiles. Order matters: list preferred drivers first.
    public IReadOnlyList<string> Pkcs11ModuleCandidates { get; init; } = new[]
    {
        // OpenSC (community, broad token support, MIT)
        @"C:\Program Files\OpenSC Project\OpenSC\pkcs11\opensc-pkcs11.dll",
        @"C:\Program Files (x86)\OpenSC Project\OpenSC\pkcs11\opensc-pkcs11.dll",
        "opensc-pkcs11.dll",
        // Hypersecu HyperPKI HYP2003 (ePass2003-ийн шинэ middleware — x64 модуль
        // ЭНЭ нэрээр System32-д суудаг; eps2003csp11.dll биш).
        @"C:\Windows\System32\HyperPKICsp11_2003.dll",
        "HyperPKICsp11_2003.dll",
        // Feitian / EnterSafe ePass2003 (хуучин нэр)
        @"C:\Windows\System32\eps2003csp11.dll",
        @"C:\Windows\System32\eps2003csp11v2.dll",
        "eps2003csp11.dll",
        "eps2003csp11v2.dll",
    };

    /// Optional explicit override — when set, only this DLL is loaded.
    public string? Pkcs11ModulePath { get; init; }
}

public sealed class BackendOptions
{
    [Required, Url]
    public string BaseUrl { get; init; } = string.Empty;

    [Range(1, 600)]
    public int RequestTimeoutSeconds { get; init; } = 60;

    [Range(1000, 60000)]
    public int LongPollTimeoutMs { get; init; } = 25_000;

    public IReadOnlyList<string> CertificateSpkiPins { get; init; } = Array.Empty<string>();

    /// X-Road client identifier sent on every /rp/v1/* request.
    /// Format: MN/COM/{member_code}/{subsystem_code}. Required for staging+prod;
    /// optional in local dev when calling a backend without X-Road middleware.
    public string? XRoadClient { get; init; }

    /// Display text shown on the citizen's phone during authentication
    /// (e.g. "Sign in to NexusGeregeDesktop desktop"). Max 200 chars.
    public string AuthDisplayText { get; init; } = "e-ID Mongolia Desktop-руу нэвтрэх";
}

public sealed class SecurityOptions
{
    public bool RequireCertificatePinning { get; init; } = true;

    public bool RequireWindowsHello { get; init; } = true;

    [Range(60, 3600)]
    public int IdleTimeoutSeconds { get; init; } = 900;

    [Range(5, 60)]
    public int ClipboardClearSeconds { get; init; } = 30;
}

public sealed class LocalizationOptions
{
    public string DefaultCulture { get; init; } = "mn-MN";

    public IReadOnlyList<string> SupportedCultures { get; init; } = new[] { "mn-MN", "en-US", "ru-RU", "zh-CN", "fr-FR", "es-ES", "ar-SA" };
}
