using NexusGeregeDesktop.Application.Abstractions;
using Microsoft.Extensions.DependencyInjection;

namespace NexusGeregeDesktop.Infrastructure.Esign;

public static class EsignServiceCollectionExtensions
{
    /// <summary>
    /// Локал ESIGN гүүрийг бүртгэнэ (singleton + тохиргоо). App эхлэхэд
    /// <c>IEsignBridgeService.Start()</c> дуудаж асаана. IEsignInteraction-ийг
    /// UI давхарга (Client) тусад нь бүртгэх ёстой.
    /// </summary>
    public static IServiceCollection AddEsignBridge(this IServiceCollection services, Action<EsignBridgeOptions>? configure = null)
    {
        var opt = new EsignBridgeOptions();
        configure?.Invoke(opt);
        services.AddSingleton(opt);
        services.AddSingleton<IEsignBridgeService, EsignBridgeService>();
        return services;
    }
}
