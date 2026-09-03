using NexusGeregeDesktop.Application.Abstractions;
using Microsoft.Extensions.Logging;

namespace NexusGeregeDesktop.Infrastructure.Esign;

/// <summary>Тохиргоо: локал ESIGN гүүрийн порт.</summary>
public sealed class EsignBridgeOptions
{
    public bool Enabled { get; set; } = true;
    public int WsPort { get; set; } = 59001;         // wss = WsPort + 4 (59005)
    public string ExpectedType { get; set; } = "bb4702f31917793f";
}

/// <summary>
/// Локал ESIGN WebSocket серверийг (ws:59001 + wss:59005) фоноор ажиллуулна. Апп нь
/// host.StartAsync дуудахгүй тул BackgroundService биш — App эхлэхэд Start()-аар асаана.
/// </summary>
public sealed class EsignBridgeService : IEsignBridgeService, IDisposable
{
    private readonly ITokenRegistry _tokens;
    private readonly IEsignInteraction _ui;
    private readonly IEsignSoftwareToken _soft;
    private readonly IEsignPreferences _prefs;
    private readonly ISensitiveActionGuard _consent;
    private readonly EsignBridgeOptions _opt;
    private readonly ILoggerFactory _lf;
    private readonly ILogger<EsignBridgeService> _log;
    private CancellationTokenSource? _cts;
    private readonly object _gate = new();

    public EsignBridgeService(
        ITokenRegistry tokens, IEsignInteraction ui, IEsignSoftwareToken soft, IEsignPreferences prefs,
        ISensitiveActionGuard consent, EsignBridgeOptions opt, ILoggerFactory lf, ILogger<EsignBridgeService> log)
    {
        _tokens = tokens; _ui = ui; _soft = soft; _prefs = prefs; _consent = consent; _opt = opt; _lf = lf; _log = log;
    }

    public bool IsRunning
    {
        get { lock (_gate) { return _cts is not null; } }
    }

    public int WsPort => _opt.WsPort;

    public void Start()
    {
        if (!_opt.Enabled) { _log.LogInformation("ESIGN гүүр идэвхгүй."); return; }
        lock (_gate)
        {
            if (_cts is not null) return;   // аль хэдийн ажиллаж байна
            _cts = new CancellationTokenSource();
        }
        var ct = _cts.Token;
        _ = Task.Run(() => RunAsync(ct), ct);
    }

    private async Task RunAsync(CancellationToken ct)
    {
        try
        {
            var bridge = new EsignBridge(_tokens, _ui, _soft, _prefs, _consent, _lf.CreateLogger<EsignBridge>(), _opt.ExpectedType);
            Func<string, CancellationToken, Task<string>> handler = bridge.HandleAsync;

            var ws = new EsignWebSocketServer(_opt.WsPort, null, handler, _log);
            var tls = EsignWebSocketServer.CreateLocalhostCert();
            var wss = new EsignWebSocketServer(_opt.WsPort + 4, tls, handler, _log);

            _log.LogInformation("ESIGN гүүр эхэллээ (ws:{P1}, wss:{P2}).", _opt.WsPort, _opt.WsPort + 4);
            await Task.WhenAll(ws.RunAsync(ct), wss.RunAsync(ct)).ConfigureAwait(false);
        }
        catch (OperationCanceledException) { }
        catch (Exception ex) { _log.LogError(ex, "ESIGN гүүр зогслоо (алдаа). Порт {Port} эзлэгдсэн байж болзошгүй.", _opt.WsPort); }
    }

    public void Stop()
    {
        lock (_gate)
        {
            try { _cts?.Cancel(); } catch { }
            _cts?.Dispose();
            _cts = null;
        }
    }

    public void Dispose() => Stop();
}
