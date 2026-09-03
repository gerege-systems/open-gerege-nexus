using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Domain.Auth;
using NexusGeregeDesktop.Domain.Primitives;

namespace NexusGeregeDesktop.Infrastructure.Auth;

/// First-party profile provider.
///
/// The gerege backend had `GET /web2app/v1/me`; this platform has no `/me`
/// endpoint — `/api/status` already delivered the citizen's `name` + `idNumber`
/// inline at auth-complete, which <see cref="CitizenAuthService"/> stashed in
/// <see cref="ISessionIdentityCache"/>. We serve it back from there so the login
/// ViewModel's `Confirmed → GetMeAsync` step keeps working unchanged.
public sealed class UserProfileService : IUserProfileService
{
    private readonly ISessionIdentityCache _identity;

    public UserProfileService(ISessionIdentityCache identity)
    {
        _identity = identity;
    }

    public Task<Result<UserProfile>> GetMeAsync(CancellationToken ct = default)
    {
        var profile = _identity.Current;
        var result = profile is null
            ? Result<UserProfile>.Failure(ApiError.Unauthorized("No active session."))
            : Result<UserProfile>.Success(profile);
        return Task.FromResult(result);
    }
}
