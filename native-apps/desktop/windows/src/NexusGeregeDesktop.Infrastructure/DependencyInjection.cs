using System.Net;
using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Application.Configuration;
using NexusGeregeDesktop.Infrastructure.Audit;
using NexusGeregeDesktop.Infrastructure.Auth;
using NexusGeregeDesktop.Infrastructure.Certificates;
using NexusGeregeDesktop.Infrastructure.Dashboard;
using NexusGeregeDesktop.Infrastructure.Esign;
using NexusGeregeDesktop.Infrastructure.Guardian;
using NexusGeregeDesktop.Infrastructure.Health;
using NexusGeregeDesktop.Infrastructure.Http;
using NexusGeregeDesktop.Infrastructure.Org;
using NexusGeregeDesktop.Infrastructure.Security.Hmac;
using NexusGeregeDesktop.Infrastructure.Security.Identity;
using NexusGeregeDesktop.Infrastructure.Security.Idle;
using NexusGeregeDesktop.Infrastructure.Security.Rasp;
using NexusGeregeDesktop.Infrastructure.Security.Tls;
using NexusGeregeDesktop.Infrastructure.Security.Vault;
using NexusGeregeDesktop.Infrastructure.Security.WindowsHello;
using NexusGeregeDesktop.Infrastructure.Time;
using NexusGeregeDesktop.Infrastructure.Tokens;
using NexusGeregeDesktop.Infrastructure.Tokens.Cng;
using NexusGeregeDesktop.Infrastructure.Tokens.Pkcs11;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Http.Resilience;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Options;

namespace NexusGeregeDesktop.Infrastructure;

public static class DependencyInjection
{
    public const string BackendHttpClientName = "NexusGeregeDesktop.Backend";

    public static IServiceCollection AddInfrastructure(this IServiceCollection services, IConfiguration configuration)
    {
        ArgumentNullException.ThrowIfNull(services);
        ArgumentNullException.ThrowIfNull(configuration);

        services.AddOptions<SecretVaultOptions>()
            .Bind(configuration.GetSection(SecretVaultOptions.SectionName));

        services.AddSingleton<IClock, SystemClock>();
        // First-party identity bridge (see BACKEND-INTEGRATION.md): /api/status
        // returns name+idNumber inline (no bearer / no /me), stashed here by
        // CitizenAuthService and served by UserProfileService.
        services.AddSingleton<ISessionIdentityCache, SessionIdentityCache>();
        services.AddSingleton<ISecretVault, DataProtectionSecretVault>();
        services.AddSingleton<IDeviceIdentity, DeviceIdentityProvider>();
        services.AddSingleton<IWindowsHello, WindowsHelloService>();
        // Гарын үсэг зурахын өмнөх Windows Hello баталгаа (Security.RequireWindowsHello).
        services.AddSingleton<ISensitiveActionGuard, SensitiveActionGuard>();
        services.AddSingleton<ICertificateService, CertificateService>();
        services.AddSingleton<IHealthMonitor, HealthMonitor>();
        services.AddSingleton<IWebSessionStore, WebSessionStore>();
        services.AddSingleton<IAuditLog, FileAuditLog>();
        services.AddSingleton<IIdleLockService, IdleLockService>();
        services.AddSingleton<IRaspService, RaspService>();

        // Crypto-token providers — registry filters by IsAvailable per scan.
        services.AddSingleton<ICryptoTokenProvider, WindowsCngTokenProvider>();

        // PKCS#11 — one provider PER discovered module DLL. Different
        // cards need different modules: opensc-pkcs11 covers a curated
        // list (PIV, OpenPGP, GIDS, certain Feitian variants), while
        // eps2003csp11 only knows Feitian ePass2003. Loading a single
        // module is fragile — a Feitian ePass2003 token that OpenSC
        // doesn't recognize would simply disappear from the UI. By
        // registering each candidate independently, the discovery list
        // contains tokens from whichever module recognizes the physical
        // card.
        var tokenOpts = configuration
            .GetSection($"{NexusGeregeDesktopOptions.SectionName}:{nameof(NexusGeregeDesktopOptions.Tokens)}")
            .Get<TokenOptions>() ?? new TokenOptions();
        var modulePaths = Pkcs11TokenProvider.ResolveAllModulePaths(tokenOpts);
        if (modulePaths.Count == 0)
        {
            // No module found — register one provider that reports
            // IsAvailable=false so the UI still shows a "PKCS#11" entry
            // (and surfaces the misconfiguration cleanly).
            services.AddSingleton<ICryptoTokenProvider, Pkcs11TokenProvider>();
        }
        else
        {
            foreach (var path in modulePaths)
            {
                var captured = path;
                services.AddSingleton<ICryptoTokenProvider>(sp =>
                    new Pkcs11TokenProvider(captured, sp.GetRequiredService<ILogger<Pkcs11TokenProvider>>()));
            }
        }

        services.AddSingleton<ITokenRegistry, TokenRegistry>();

        // First-party trust model (see desktop/windows-app/BACKEND-INTEGRATION.md).
        // This app is a first-party client of the e-ID Mongolia WEB backend and
        // talks to its public `/api/*` routes exactly like a browser — it holds
        // NO RP secret, NO device HMAC secret, and NO long-lived bearer token.
        // The HMAC device-nonce signer and the bearer-attach handler from the
        // original gerege `/web2app/v1/*` model are therefore NOT wired into the
        // pipeline. The handler classes are kept in the tree for reference / a
        // possible future direct-backend build, but nothing depends on them.
        services
            .AddHttpClient(BackendHttpClientName, (sp, client) =>
            {
                var opts = sp.GetRequiredService<IOptions<NexusGeregeDesktopOptions>>().Value;
                client.BaseAddress = new Uri(opts.Backend.BaseUrl, UriKind.Absolute);
                client.Timeout = TimeSpan.FromSeconds(opts.Backend.RequestTimeoutSeconds);
                client.DefaultRequestHeaders.UserAgent.ParseAdd($"NexusGeregeDesktop-Windows/0.1");
                client.DefaultRequestHeaders.AcceptLanguage.ParseAdd(opts.Localization.DefaultCulture);
                // Иргэн утсан дээрээ session-ийг "eID Desktop" гэж харна ("Demo Bank" биш).
                // X-Eid-Client header БАЙХГҮЙ (macOS/iOS/Android-той ижил): платформ дээр
                // RP НЭГ бөгөөд эдгээр апп бүгд түүний өөрийн клиент — web тал одоо
                // header-ээр RP сонгодоггүй (rpclient.ts § RP_SELF).
            })
            .ConfigurePrimaryHttpMessageHandler(BuildPrimaryHandler)
            .AddStandardResilienceHandler();

        services.AddSingleton<IBackendApi>(sp =>
        {
            var factory = sp.GetRequiredService<IHttpClientFactory>();
            var http = factory.CreateClient(BackendHttpClientName);
            return new BackendApiClient(http, sp.GetRequiredService<ILogger<BackendApiClient>>());
        });

        services.AddSingleton<ICitizenAuthService>(sp =>
        {
            var factory = sp.GetRequiredService<IHttpClientFactory>();
            var http = factory.CreateClient(BackendHttpClientName);
            return new CitizenAuthService(
                http,
                sp.GetRequiredService<IWebSessionStore>(),
                sp.GetRequiredService<ISessionIdentityCache>(),
                sp.GetRequiredService<ILogger<CitizenAuthService>>());
        });

        // First-party: profile is served from the identity cache populated by
        // CitizenAuthService (no `/me` endpoint), so no HttpClient is needed.
        services.AddSingleton<IUserProfileService>(sp =>
            new UserProfileService(sp.GetRequiredService<ISessionIdentityCache>()));

        // Cert-enroll has no first-party `/api/*` equivalent → honest stub.
        services.AddSingleton<ICertEnrollmentService, CertEnrollmentService>();

        // Dashboard + org-list are REAL: they consume the WEB backend's public
        // person-PKI routes (/api/dashboard, /api/devices, /api/activity,
        // /api/representations) for the logged-in citizen (identity from the
        // session identity cache). Org WRITE/member ops remain stubs inside OrgService.
        services.AddSingleton<IDashboardService>(sp =>
        {
            var http = sp.GetRequiredService<IHttpClientFactory>().CreateClient(BackendHttpClientName);
            return new DashboardService(
                http,
                sp.GetRequiredService<ISessionIdentityCache>(),
                sp.GetRequiredService<IWebSessionStore>(),
                sp.GetRequiredService<ILogger<DashboardService>>());
        });
        // Иргэний гэрчилгээний жагсаалт — /api/certificates (session-bound, dashboard-тай ижил).
        services.AddSingleton<ICitizenCertificateService>(sp =>
        {
            var http = sp.GetRequiredService<IHttpClientFactory>().CreateClient(BackendHttpClientName);
            return new CitizenCertificateService(
                http,
                sp.GetRequiredService<IWebSessionStore>(),
                sp.GetRequiredService<ILogger<CitizenCertificateService>>());
        });

        // Программ токен (ESIGN) — токен байхгүй үед нэвтэрсэн иргэний identity-ээр
        // гарын үсэг зурна. Dashboard-тай ижилээр backend `/api/*`-г ашиглана.
        services.AddSingleton<IEsignSoftwareToken>(sp =>
        {
            var http = sp.GetRequiredService<IHttpClientFactory>().CreateClient(BackendHttpClientName);
            return new EsignSoftwareToken(
                http,
                sp.GetRequiredService<IWebSessionStore>(),
                sp.GetRequiredService<ILogger<EsignSoftwareToken>>());
        });

        services.AddSingleton<IOrgService>(sp =>
        {
            var http = sp.GetRequiredService<IHttpClientFactory>().CreateClient(BackendHttpClientName);
            return new OrgService(
                http,
                sp.GetRequiredService<IWebSessionStore>(),
                sp.GetRequiredService<ILogger<OrgService>>());
        });

        // "Миний хүүхдүүд" (guardian) — OrgService-тэй ижил first-party загвар.
        services.AddSingleton<IGuardianService>(sp =>
        {
            var http = sp.GetRequiredService<IHttpClientFactory>().CreateClient(BackendHttpClientName);
            return new GuardianService(
                http,
                sp.GetRequiredService<IWebSessionStore>(),
                sp.GetRequiredService<ILogger<GuardianService>>());
        });

        services.AddSingleton<IRpAuthService>(sp =>
        {
            var factory = sp.GetRequiredService<IHttpClientFactory>();
            var http = factory.CreateClient(BackendHttpClientName);
            http.Timeout = TimeSpan.FromSeconds(60);
            return new RpAuthService(http, sp.GetRequiredService<ILogger<RpAuthService>>());
        });

        return services;
    }

    private static HttpMessageHandler BuildPrimaryHandler(IServiceProvider sp)
    {
        var opts = sp.GetRequiredService<IOptions<NexusGeregeDesktopOptions>>().Value;
        var validator = new SpkiPinValidator(opts.Backend.CertificateSpkiPins);

        var handler = new SocketsHttpHandler
        {
            AutomaticDecompression = DecompressionMethods.All,
            AllowAutoRedirect = false,
            UseCookies = false,
            PooledConnectionLifetime = TimeSpan.FromMinutes(5),
            ConnectTimeout = TimeSpan.FromSeconds(10),
            SslOptions = new System.Net.Security.SslClientAuthenticationOptions
            {
                EnabledSslProtocols = System.Security.Authentication.SslProtocols.Tls13,
            },
        };

        if (opts.Security.RequireCertificatePinning && validator.HasPins)
        {
            handler.SslOptions.RemoteCertificateValidationCallback =
                (_, certificate, chain, sslErrors) =>
                {
                    var leaf = certificate switch
                    {
                        System.Security.Cryptography.X509Certificates.X509Certificate2 x2 => x2,
                        not null => new System.Security.Cryptography.X509Certificates.X509Certificate2(certificate),
                        _ => null,
                    };
                    return validator.Validate(leaf, chain, sslErrors);
                };
        }
        else
        {
            handler.SslOptions.EnabledSslProtocols =
                System.Security.Authentication.SslProtocols.Tls12 | System.Security.Authentication.SslProtocols.Tls13;
        }

        return handler;
    }
}
