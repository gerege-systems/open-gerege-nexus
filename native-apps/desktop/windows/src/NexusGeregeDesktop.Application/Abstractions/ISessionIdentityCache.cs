using NexusGeregeDesktop.Domain.Auth;

namespace NexusGeregeDesktop.Application.Abstractions;

/// First-party bridge for the missing bearer/`/me` step.
///
/// The gerege backend returned a bearer token on auth-complete and the desktop
/// then fetched the citizen's profile from `/web2app/v1/me`. This platform's
/// first-party `/api/status` route instead returns the citizen's `name` +
/// `idNumber` **inline** at completion, and there is no `/me` endpoint.
///
/// So `CitizenAuthService` stashes the identity it reads off `/api/status`
/// here, and `UserProfileService.GetMeAsync` serves it back — keeping the
/// login ViewModel's `Confirmed → GetMeAsync` flow working without change.
public interface ISessionIdentityCache
{
    /// Identity captured from the most recent completed auth session, if any.
    UserProfile? Current { get; }

    void Set(UserProfile profile);

    void Clear();
}
