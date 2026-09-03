namespace NexusGeregeDesktop.Application.Abstractions;

public interface IIdleLockService
{
    void Start();

    void Stop();

    /// Resets the idle counter — called on user activity. Generally the
    /// concrete impl listens to OS input events; this method is for explicit
    /// "I am here" markers (e.g. after Hello verification).
    void NotifyActivity();

    event EventHandler? Locked;
}
