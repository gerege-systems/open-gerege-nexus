using NexusGeregeDesktop.Domain.Auth;
using NexusGeregeDesktop.Domain.Primitives;

namespace NexusGeregeDesktop.Application.Abstractions;

public interface IRpAuthService
{
    Task<Result<AuthSession>> InitiateAsync(InitiateAuthRequest request, CancellationToken ct = default);

    Task<Result<AuthSessionStatus>> PollAsync(Guid sessionId, int timeoutMs, CancellationToken ct = default);

    /// After the citizen has approved the PIN2 push (poll returned COMPLETE/OK),
    /// upload the original PDF bytes so the backend can stamp them (PAdES/PKCS#7
    /// + verification page) and return the signed PDF. Sends a multipart POST to
    /// `/api/sign-pdf-download` (`file`, `sessionId`, `pollToken`). The service
    /// threads the per-session <c>pollToken</c> captured at
    /// <see cref="InitiateAsync"/>, so only the session id + original bytes are
    /// needed here.
    Task<Result<byte[]>> DownloadSignedPdfAsync(Guid sessionId, byte[] originalPdf, string fileName, CancellationToken ct = default);
}

/// First-party PDF-sign request. The signature is bound to the document digest
/// (not an RP challenge), so the caller supplies the raw PDF bytes; the service
/// computes the SHA-256 digest client-side and sends only that to the backend.
/// <see cref="Nonce"/>/<see cref="DisplayText"/> are retained for source
/// compatibility but are unused by the first-party `/api/sign-pdf-start` route.
public sealed record InitiateAuthRequest(
    string NationalId,
    string? DisplayText = null,
    string? CallbackUrl = null,
    string? Nonce = null,
    byte[]? PdfBytes = null,
    string? FileName = null);
