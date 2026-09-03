namespace NexusGeregeDesktop.Application.Abstractions;

public interface IAuditLog
{
    void Write(AuditEvent ev);

    string LogDirectory { get; }
}

public sealed record AuditEvent(
    DateTimeOffset At,
    string Kind,
    string? Actor,
    IReadOnlyDictionary<string, string?>? Details = null);

public static class AuditEvents
{
    public const string AppStarted = "app.started";
    public const string AppStopped = "app.stopped";
    public const string LoginInitiated = "login.initiated";
    public const string LoginSucceeded = "login.succeeded";
    public const string LoginFailed = "login.failed";
    public const string SignOut = "session.signed_out";
    public const string IdleLocked = "session.idle_locked";
    public const string DebuggerDetected = "rasp.debugger";
    public const string ExeHashLogged = "rasp.exe_hash";
    public const string OrgRegistered = "org.registered";
    public const string OrgMemberAdded = "org.member_added";
    public const string OrgMemberRemoved = "org.member_removed";
    public const string SensitiveCopied = "clipboard.sensitive_copied";
    public const string ConsentGranted = "consent.granted";
    public const string ConsentDenied = "consent.denied";
    public const string ConsentUnavailable = "consent.unavailable";
}
