using System.Diagnostics;
using System.Runtime.InteropServices;
using System.Runtime.Versioning;
using System.Security.Cryptography;
using NexusGeregeDesktop.Application.Abstractions;
using Microsoft.Extensions.Logging;

namespace NexusGeregeDesktop.Infrastructure.Security.Rasp;

public interface IRaspService
{
    RaspProbe Snapshot();
}

public sealed record RaspProbe(
    bool DebuggerAttached,
    bool RemoteDebuggerAttached,
    string ExecutableSha256Hex,
    string ExecutablePath,
    long ExecutableSizeBytes);

[SupportedOSPlatform("windows")]
public sealed class RaspService : IRaspService
{
    private readonly ILogger<RaspService> _logger;
    private readonly Lazy<RaspProbe> _snapshot;

    public RaspService(ILogger<RaspService> logger)
    {
        _logger = logger;
        _snapshot = new Lazy<RaspProbe>(Probe, isThreadSafe: true);
    }

    public RaspProbe Snapshot() => _snapshot.Value;

    private RaspProbe Probe()
    {
        var managedDbg = Debugger.IsAttached;
        var remoteDbg = IsRemoteDebuggerAttached();
        var exePath = Process.GetCurrentProcess().MainModule?.FileName ?? string.Empty;
        var (hash, size) = ComputeExecutableHash(exePath);

        if (managedDbg || remoteDbg)
        {
            _logger.LogWarning(
                "RASP: debugger attached (managed={Managed}, remote={Remote}); proceeding but auditing.",
                managedDbg, remoteDbg);
        }
        _logger.LogInformation(
            "RASP: exe sha256={Hash} size={Size} path={Path}",
            hash, size, exePath);

        return new RaspProbe(managedDbg, remoteDbg, hash, exePath, size);
    }

    [DllImport("kernel32.dll")]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool CheckRemoteDebuggerPresent(IntPtr hProcess, [MarshalAs(UnmanagedType.Bool)] ref bool isDebuggerPresent);

    [DllImport("kernel32.dll")]
    private static extern IntPtr GetCurrentProcess();

    private static bool IsRemoteDebuggerAttached()
    {
        try
        {
            bool present = false;
            return CheckRemoteDebuggerPresent(GetCurrentProcess(), ref present) && present;
        }
        catch
        {
            return false;
        }
    }

    private static (string Hash, long Size) ComputeExecutableHash(string path)
    {
        if (string.IsNullOrEmpty(path) || !File.Exists(path))
        {
            return (string.Empty, 0);
        }
        try
        {
            using var fs = File.OpenRead(path);
            var size = fs.Length;
            using var sha = SHA256.Create();
            var hash = sha.ComputeHash(fs);
            return (Convert.ToHexString(hash).ToLowerInvariant(), size);
        }
        catch
        {
            return (string.Empty, 0);
        }
    }
}
