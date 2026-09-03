using NexusGeregeDesktop.Domain.Auth;

namespace NexusGeregeDesktop.Application.Abstractions;

public interface IWebSessionStore
{
    WebSession? Current { get; }

    bool IsAuthenticated { get; }

    event EventHandler<WebSession?>? Changed;

    ValueTask<WebSession?> LoadAsync(CancellationToken ct = default);

    ValueTask SaveAsync(WebSession session, CancellationToken ct = default);

    ValueTask ClearAsync(CancellationToken ct = default);
}
