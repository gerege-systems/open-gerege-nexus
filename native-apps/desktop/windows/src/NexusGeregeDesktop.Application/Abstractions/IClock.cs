namespace NexusGeregeDesktop.Application.Abstractions;

public interface IClock
{
    DateTimeOffset UtcNow { get; }

    long UnixSeconds => UtcNow.ToUnixTimeSeconds();
}
