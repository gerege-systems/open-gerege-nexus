using System.Net.Http.Headers;
using NexusGeregeDesktop.Application.Abstractions;

namespace NexusGeregeDesktop.Infrastructure.Auth;

/// DelegatingHandler that injects "Authorization: Bearer <token>" for
/// citizen-authenticated /web2app/v1/* endpoints. Skips the unauthenticated
/// /web2app/v1/auth/init + /web2app/v1/auth/{id} (poll) paths.
public sealed class BearerAuthHandler : DelegatingHandler
{
    private readonly IWebSessionStore _sessions;

    public BearerAuthHandler(IWebSessionStore sessions) => _sessions = sessions;

    protected override async Task<HttpResponseMessage> SendAsync(
        HttpRequestMessage request,
        CancellationToken cancellationToken)
    {
        ArgumentNullException.ThrowIfNull(request);

        var path = request.RequestUri?.AbsolutePath ?? string.Empty;
        if (path.StartsWith("/web2app/v1/", StringComparison.Ordinal)
            && !IsPublicWebRoute(path)
            && request.Headers.Authorization is null)
        {
            await _sessions.LoadAsync(cancellationToken).ConfigureAwait(false);
            var session = _sessions.Current;
            if (session is not null)
            {
                request.Headers.Authorization = new AuthenticationHeaderValue("Bearer", session.Token);
            }
        }

        return await base.SendAsync(request, cancellationToken).ConfigureAwait(false);
    }

    private static bool IsPublicWebRoute(string path)
    {
        // Unauthenticated public routes per backend web_login_handler.go.
        if (path.Equals("/web2app/v1/auth/init", StringComparison.Ordinal)
            || path.Equals("/web2app/v1/auth/qr/init", StringComparison.Ordinal))
        {
            return true;
        }
        // /web2app/v1/auth/{id} poll — bound by X-Auth-Poll-Token, no bearer needed.
        // /web2app/v1/verify/{id} — public PDF signature verification.
        if (path.StartsWith("/web2app/v1/auth/", StringComparison.Ordinal)
            && !path.Equals("/web2app/v1/auth/logout", StringComparison.Ordinal))
        {
            return true;
        }
        if (path.StartsWith("/web2app/v1/verify/", StringComparison.Ordinal))
        {
            return true;
        }
        return false;
    }
}
