using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Application.Configuration;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Options;

namespace NexusGeregeDesktop.Infrastructure.Health;

public sealed class HealthMonitor : IHealthMonitor, IDisposable
{
    private readonly IBackendApi _api;
    private readonly IClock _clock;
    private readonly ILogger<HealthMonitor> _logger;
    private readonly TimeSpan _interval;
    private readonly object _gate = new();

    private CancellationTokenSource? _cts;
    private Task? _loopTask;
    private HealthSnapshot _current;

    public HealthMonitor(
        IBackendApi api,
        IClock clock,
        IOptions<NexusGeregeDesktopOptions> options,
        ILogger<HealthMonitor> logger)
    {
        _api = api;
        _clock = clock;
        _logger = logger;
        _interval = TimeSpan.FromSeconds(30);
        _current = new HealthSnapshot(HealthState.Unknown, null, clock.UtcNow, null);
        _ = options;
    }

    public HealthSnapshot Current => _current;

    public event EventHandler<HealthSnapshot>? Updated;

    public void Start()
    {
        lock (_gate)
        {
            if (_cts is not null)
            {
                return;
            }
            _cts = new CancellationTokenSource();
            _loopTask = Task.Run(() => RunLoopAsync(_cts.Token));
        }
    }

    public void Stop()
    {
        CancellationTokenSource? cts;
        lock (_gate)
        {
            cts = _cts;
            _cts = null;
        }
        cts?.Cancel();
        cts?.Dispose();
    }

    public async Task RefreshAsync(CancellationToken ct = default)
    {
        Publish(new HealthSnapshot(HealthState.Checking, _current.Version, _clock.UtcNow, null));

        var result = await _api.GetHealthAsync(ct).ConfigureAwait(false);
        HealthSnapshot snapshot;

        if (result.IsSuccess)
        {
            var h = result.Value;
            var state = (h.Live, h.Ready) switch
            {
                (true, true) => HealthState.Healthy,
                (true, false) => HealthState.Degraded,
                _ => HealthState.Unreachable,
            };
            snapshot = new HealthSnapshot(state, h.Version, _clock.UtcNow, null);
        }
        else
        {
            snapshot = new HealthSnapshot(
                HealthState.Unreachable,
                _current.Version,
                _clock.UtcNow,
                result.Error.Message);
        }

        Publish(snapshot);
    }

    private async Task RunLoopAsync(CancellationToken ct)
    {
        while (!ct.IsCancellationRequested)
        {
            try
            {
                await RefreshAsync(ct).ConfigureAwait(false);
            }
            catch (OperationCanceledException) when (ct.IsCancellationRequested)
            {
                return;
            }
            catch (Exception ex)
            {
                _logger.LogWarning(ex, "Health monitor loop error");
            }

            try
            {
                await Task.Delay(_interval, ct).ConfigureAwait(false);
            }
            catch (OperationCanceledException)
            {
                return;
            }
        }
    }

    private void Publish(HealthSnapshot snapshot)
    {
        _current = snapshot;
        Updated?.Invoke(this, snapshot);
    }

    public void Dispose() => Stop();
}
