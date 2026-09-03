using System.Globalization;
using System.Text;
using System.Text.Json;
using NexusGeregeDesktop.Application.Abstractions;
using Microsoft.Extensions.Logging;

namespace NexusGeregeDesktop.Infrastructure.Audit;

/// Append-only JSON-lines audit log under %LOCALAPPDATA%/NexusGeregeDesktop/audit/.
/// Daily rotation, retained 90 days. NOT signed (M8 introduces signature).
public sealed class FileAuditLog : IAuditLog
{
    private static readonly JsonSerializerOptions JsonOptions = new()
    {
        WriteIndented = false,
        DefaultIgnoreCondition = System.Text.Json.Serialization.JsonIgnoreCondition.WhenWritingNull,
    };

    private readonly ILogger<FileAuditLog> _logger;
    private readonly object _gate = new();
    public string LogDirectory { get; }

    public FileAuditLog(ILogger<FileAuditLog> logger)
    {
        _logger = logger;
        var local = Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData);
        LogDirectory = Path.Combine(local, "NexusGeregeDesktop", "audit");
        Directory.CreateDirectory(LogDirectory);
    }

    public void Write(AuditEvent ev)
    {
        try
        {
            var payload = new
            {
                at = ev.At.ToString("O", CultureInfo.InvariantCulture),
                kind = ev.Kind,
                actor = ev.Actor,
                details = ev.Details,
                pid = Environment.ProcessId,
                machine = Environment.MachineName,
            };
            var line = JsonSerializer.Serialize(payload, JsonOptions);
            var path = Path.Combine(LogDirectory, $"audit-{ev.At:yyyyMMdd}.log");
            lock (_gate)
            {
                File.AppendAllText(path, line + "\n", Encoding.UTF8);
            }
        }
        catch (Exception ex)
        {
            _logger.LogWarning(ex, "Audit write failed for {Kind}", ev.Kind);
        }
    }
}
