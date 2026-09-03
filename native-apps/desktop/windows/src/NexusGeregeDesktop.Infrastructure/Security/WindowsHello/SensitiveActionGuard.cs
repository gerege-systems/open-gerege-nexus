using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Application.Configuration;
using NexusGeregeDesktop.Domain.Primitives;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Options;

namespace NexusGeregeDesktop.Infrastructure.Security.WindowsHello;

/// <summary>
/// <see cref="ISensitiveActionGuard"/>-ийн хэрэгжилт — Windows Hello + audit.
/// Бодлогыг интерфэйсийн тайлбарт бичсэн: Hello байхгүй төхөөрөмж дээр нэвтрүүлнэ,
/// Hello байгаа мөртлөө цуцалсан/амжилтгүй бол зогсооно.
/// </summary>
public sealed class SensitiveActionGuard : ISensitiveActionGuard
{
    private readonly IWindowsHello _hello;
    private readonly IAuditLog _audit;
    private readonly IClock _clock;
    private readonly bool _required;
    private readonly ILogger<SensitiveActionGuard> _log;

    public SensitiveActionGuard(
        IWindowsHello hello,
        IAuditLog audit,
        IClock clock,
        IOptions<NexusGeregeDesktopOptions> options,
        ILogger<SensitiveActionGuard> log)
    {
        ArgumentNullException.ThrowIfNull(options);
        _hello = hello;
        _audit = audit;
        _clock = clock;
        _required = options.Value.Security.RequireWindowsHello;
        _log = log;
    }

    public async Task<Result> RequireConsentAsync(string reasonLocalized, string operation, CancellationToken ct = default)
    {
        if (!_required)
        {
            return Result.Success();
        }

        HelloResult outcome;
        try
        {
            outcome = await _hello.RequestConsentAsync(reasonLocalized, ct).ConfigureAwait(false);
        }
        catch (OperationCanceledException) when (ct.IsCancellationRequested)
        {
            return Result.Failure(ApiError.Cancelled("hello_cancelled"));
        }

        switch (outcome)
        {
            case HelloResult.Verified:
                Audit(AuditEvents.ConsentGranted, operation, outcome);
                return Result.Success();

            // Төхөөрөмж дээр Hello тохируулагдаагүй / бодлогоор хаалттай — үйлдлийг
            // зогсоовол биометргүй компьютерээс гарын үсэг зурах боломжгүй болно.
            case HelloResult.NotAvailable:
            case HelloResult.DeviceNotPresent:
            case HelloResult.DisabledByPolicy:
                _log.LogWarning("Windows Hello боломжгүй ({Outcome}) — {Operation} баталгаагүй үргэлжиллээ.", outcome, operation);
                Audit(AuditEvents.ConsentUnavailable, operation, outcome);
                return Result.Success();

            case HelloResult.Cancelled:
                Audit(AuditEvents.ConsentDenied, operation, outcome);
                return Result.Failure(ApiError.Unauthorized("hello_cancelled"));

            default:
                Audit(AuditEvents.ConsentDenied, operation, outcome);
                return Result.Failure(ApiError.Unauthorized("hello_failed"));
        }
    }

    private void Audit(string kind, string operation, HelloResult outcome) =>
        _audit.Write(new AuditEvent(
            _clock.UtcNow,
            kind,
            Actor: null,
            new Dictionary<string, string?>(StringComparer.Ordinal)
            {
                ["operation"] = operation,
                ["outcome"] = outcome.ToString(),
            }));
}
