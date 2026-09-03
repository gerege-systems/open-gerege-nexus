using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Domain.Auth;
using NexusGeregeDesktop.Domain.Primitives;

namespace NexusGeregeDesktop.Infrastructure.Auth;

/// STUB — cert enrollment is not available on the first-party backend.
///
/// The original gerege model minted a citizen cert via a bearer-authenticated
/// `cert/enroll` call. This platform's desktop is a first-party client of the
/// WEB backend's public `/api/*` routes, which expose no enrollment endpoint
/// (enrollment happens in-app on the phone during 2-cert onboarding). The
/// "Setup Token" wizard therefore degrades cleanly with an explicit error.
public sealed class CertEnrollmentService : ICertEnrollmentService
{
    public Task<Result<CertEnrollmentResult>> EnrollAsync(string profile, string csrPem, CancellationToken ct = default)
        => Task.FromResult(Result<CertEnrollmentResult>.Failure(
            ApiError.Internal("not_available_on_first_party_backend")));
}
