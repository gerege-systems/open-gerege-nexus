using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Application.Configuration;
using NexusGeregeDesktop.Presentation.Services;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Options;
using Windows.ApplicationModel.DataTransfer;

namespace NexusGeregeDesktop.Client.Services;

public sealed class ClipboardService : IClipboardService, IDisposable
{
    private readonly IUiDispatcher _ui;
    private readonly ILogger<ClipboardService> _logger;
    private readonly int _clearAfterSeconds;
    private CancellationTokenSource? _clearCts;
    private string? _lastCopied;

    public ClipboardService(IOptions<NexusGeregeDesktopOptions> options, IUiDispatcher ui, ILogger<ClipboardService> logger)
    {
        _ui = ui;
        _logger = logger;
        _clearAfterSeconds = options.Value.Security.ClipboardClearSeconds;
    }

    public void SetText(string text, bool sensitive = false)
    {
        if (string.IsNullOrEmpty(text))
        {
            return;
        }

        var dp = new DataPackage();
        dp.SetText(text);
        Clipboard.SetContent(dp);
        _lastCopied = text;
        // The strongest hygiene WinAppSDK gives us is the auto-clear below;
        // OS-level "exclude from history/roaming" is not exposed.

        if (!sensitive) return;

        _clearCts?.Cancel();
        _clearCts = new CancellationTokenSource();
        var token = _clearCts.Token;
        var delayMs = Math.Max(1, _clearAfterSeconds) * 1000;
        _ = Task.Delay(delayMs, token).ContinueWith(t =>
        {
            if (t.IsCanceled) return;
            _ui.Post(() =>
            {
                try
                {
                    // Only clear if the clipboard still contains OUR sensitive text;
                    // don't clobber whatever the user has copied since.
                    var content = Clipboard.GetContent();
                    if (content.Contains(StandardDataFormats.Text))
                    {
                        var current = content.GetTextAsync().AsTask().GetAwaiter().GetResult();
                        if (string.Equals(current, _lastCopied, StringComparison.Ordinal))
                        {
                            Clipboard.Clear();
                            _logger.LogDebug("Sensitive clipboard auto-cleared after {Seconds}s", _clearAfterSeconds);
                        }
                    }
                }
                catch (Exception ex)
                {
                    _logger.LogWarning(ex, "Clipboard auto-clear failed");
                }
                finally
                {
                    _lastCopied = null;
                }
            });
        }, TaskScheduler.Default);
    }

    public void Clear()
    {
        _clearCts?.Cancel();
        try { Clipboard.Clear(); } catch { /* ignore */ }
        _lastCopied = null;
    }

    public void Dispose()
    {
        _clearCts?.Cancel();
        _clearCts?.Dispose();
    }
}
