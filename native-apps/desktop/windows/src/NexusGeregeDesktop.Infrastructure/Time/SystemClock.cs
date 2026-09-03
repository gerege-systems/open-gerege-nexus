using NexusGeregeDesktop.Application.Abstractions;

namespace NexusGeregeDesktop.Infrastructure.Time;

public sealed class SystemClock : IClock
{
    public DateTimeOffset UtcNow => DateTimeOffset.UtcNow;
}
