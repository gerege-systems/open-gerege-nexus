using NexusGeregeDesktop.Domain.Auth;
using NexusGeregeDesktop.Domain.Primitives;

namespace NexusGeregeDesktop.Application.Abstractions;

/// M9.C — desktop "Setup Token" wizard. Requires the citizen to be
/// already logged in (BearerAuthHandler attaches the web-session bearer
/// to every /web2app/v1/cert/* call). The backend resolves the bearer to a
/// user_id, validates the CSR's proof-of-possession, mints a cert under
/// the e-ID Mongolia Issuing CA, and persists to the certificates table
/// — bound to that user_id. The desktop then writes the issued cert
/// back onto the token via ITokenSession.InstallIssuedCertificateAsync.
public interface ICertEnrollmentService
{
    Task<Result<CertEnrollmentResult>> EnrollAsync(
        string profile,
        string csrPem,
        CancellationToken ct = default);
}
