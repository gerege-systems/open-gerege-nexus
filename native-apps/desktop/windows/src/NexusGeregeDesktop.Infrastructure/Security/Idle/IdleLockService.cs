using System.Runtime.InteropServices;
using System.Runtime.Versioning;
using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Application.Configuration;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Options;

namespace NexusGeregeDesktop.Infrastructure.Security.Idle;

/// Polls Windows for system-wide idle time (GetLastInputInfo). When the
/// configured threshold is exceeded AND a session is active, fires Locked
/// and clears the bearer; the shell handles UI navigation to /login.
[SupportedOSPlatform("windows")]
public sealed class IdleLockService : IIdleLockService, IDisposable
{
    private readonly IWebSessionStore _sessions;
    private readonly IClock _clock;
    private readonly ILogger<IdleLockService> _logger;
    private readonly TimeSpan _threshold;
    private readonly object _gate = new();

    private CancellationTokenSource? _cts;
    private Task? _loop;
    private DateTimeOffset _explicitActivityAt;

    public IdleLockService(
        IWebSessionStore sessions,
        IClock clock,
        IOptions<NexusGeregeDesktopOptions> options,
        ILogger<IdleLockService> logger)
    {
        _sessions = sessions;
        _clock = clock;
        _logger = logger;
        _threshold = TimeSpan.FromSeconds(options.Value.Security.IdleTimeoutSeconds);
        _explicitActivityAt = _clock.UtcNow;
    }

    public event EventHandler? Locked;

    public void Start()
    {
        lock (_gate)
        {
            if (_cts is not null)
            {
                return;
            }
            _cts = new CancellationTokenSource();
            _loop = Task.Run(() => RunAsync(_cts.Token));
        }
        _logger.LogInformation("Idle lock service started (threshold={Seconds}s)", _threshold.TotalSeconds);
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

    public void NotifyActivity() => _explicitActivityAt = _clock.UtcNow;

    private async Task RunAsync(CancellationToken ct)
    {
        try
        {
            using var timer = new PeriodicTimer(TimeSpan.FromSeconds(10));
            while (await timer.WaitForNextTickAsync(ct).ConfigureAwait(false))
            {
                if (_sessions.Current is null)
                {
                    continue;
                }
                var idle = ResolveIdle();
                if (idle < _threshold)
                {
                    continue;
                }
                _logger.LogWarning("Idle threshold exceeded ({Idle}); locking session.", idle);
                await _sessions.ClearAsync(ct).ConfigureAwait(false);
                Locked?.Invoke(this, EventArgs.Empty);
            }
        }
        catch (OperationCanceledException) { }
    }

    private TimeSpan ResolveIdle()
    {
        var os = GetSystemIdleTime();
        var explicit_ = _clock.UtcNow - _explicitActivityAt;
        return os < explicit_ ? os : explicit_;
    }

    [StructLayout(LayoutKind.Sequential)]
    private struct LASTINPUTINFO
    {
        public uint cbSize;
        public uint dwTime;
    }

    [DllImport("user32.dll", SetLastError = false)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool GetLastInputInfo(ref LASTINPUTINFO plii);

    [DllImport("kernel32.dll")]
    private static extern uint GetTickCount();

    private static TimeSpan GetSystemIdleTime()
    {
        var lii = new LASTINPUTINFO { cbSize = (uint)Marshal.SizeOf<LASTINPUTINFO>() };
        if (!GetLastInputInfo(ref lii))
        {
            return TimeSpan.Zero;
        }
        var ticks = GetTickCount();
        // tick rollover-safe: subtract as uint.
        var idleMs = unchecked(ticks - lii.dwTime);
        return TimeSpan.FromMilliseconds(idleMs);
    }

    public void Dispose() => Stop();
}
