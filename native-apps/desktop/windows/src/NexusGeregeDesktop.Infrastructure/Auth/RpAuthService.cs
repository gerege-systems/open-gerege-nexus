using System.Collections.Concurrent;
using System.Globalization;
using System.Net;
using System.Net.Http.Headers;
using System.Net.Http.Json;
using System.Security.Cryptography;
using System.Text.Json;
using System.Text.Json.Serialization;
using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Domain.Auth;
using NexusGeregeDesktop.Domain.Primitives;
using Microsoft.Extensions.Logging;

namespace NexusGeregeDesktop.Infrastructure.Auth;

/// Document (PDF) signing flow — **first-party** model (see BACKEND-INTEGRATION.md).
///
/// Talks to the e-ID Mongolia WEB backend's public `/api/*` routes exactly like a
/// browser (no RP secret, no X-Road client, no bearer):
///   • initiate → POST /api/sign-pdf-start {etsi, digestB64, fileName, callbackUrl}
///   • poll     → GET  /api/status?sessionId=&pollToken=
///
/// The client computes the PDF's SHA-256 digest itself (the same bytes it will
/// later stamp) and hands only the digest to the backend. `/api/*` gates the
/// signature/PII behind a `pollToken` returned by `sign-pdf-start`; since the
/// <see cref="IRpAuthService"/> poll signature carries no token, we thread it
/// per-session in <see cref="_pollTokens"/> (the service is a DI singleton).
public sealed class RpAuthService : IRpAuthService
{
    /// The web session TTL isn't echoed by `/api/*`; conservative client-side
    /// estimate used only for the AuthSession expiry hint.
    private static readonly TimeSpan SessionTtl = TimeSpan.FromMinutes(5);

    private static readonly JsonSerializerOptions JsonOptions = new(JsonSerializerDefaults.Web)
    {
        DefaultIgnoreCondition = JsonIgnoreCondition.WhenWritingNull,
    };

    private readonly HttpClient _http;
    private readonly ILogger<RpAuthService> _logger;

    /// sessionId → pollToken, captured at sign-pdf-start so PollAsync can gate
    /// `/api/status` (which requires the token to release signature/PII).
    private readonly ConcurrentDictionary<Guid, string> _pollTokens = new();

    public RpAuthService(HttpClient http, ILogger<RpAuthService> logger)
    {
        _http = http;
        _logger = logger;
    }

    public async Task<Result<AuthSession>> InitiateAsync(InitiateAuthRequest request, CancellationToken ct = default)
    {
        ArgumentNullException.ThrowIfNull(request);
        if (string.IsNullOrWhiteSpace(request.NationalId))
        {
            return Result<AuthSession>.Failure(ApiError.BadRequest("national_id is required."));
        }
        if (request.PdfBytes is null || request.PdfBytes.Length == 0)
        {
            return Result<AuthSession>.Failure(ApiError.BadRequest("pdf_bytes is required to sign."));
        }

        // Client-side SHA-256 of the exact bytes we will stamp; server signs the
        // 32-byte digest with PIN2 (contentCommitment / non-repudiation).
        var digestB64 = Convert.ToBase64String(SHA256.HashData(request.PdfBytes));

        var body = new SignStartBody(
            request.NationalId.Trim().ToUpperInvariant(),
            digestB64,
            request.FileName,
            request.CallbackUrl);

        try
        {
            using var resp = await _http
                .PostAsJsonAsync("/api/sign-pdf-start", body, JsonOptions, ct)
                .ConfigureAwait(false);

            if (!resp.IsSuccessStatusCode)
            {
                return Result<AuthSession>.Failure(await MapErrorAsync(resp, ct).ConfigureAwait(false));
            }

            var dto = await resp.Content
                .ReadFromJsonAsync<SignStartResponse>(JsonOptions, ct)
                .ConfigureAwait(false);

            if (dto is null || string.IsNullOrWhiteSpace(dto.SessionId))
            {
                return Result<AuthSession>.Failure(ApiError.Internal("Backend returned empty session."));
            }
            if (!Guid.TryParse(dto.SessionId, out var sid))
            {
                return Result<AuthSession>.Failure(ApiError.Internal("Backend session id was not a UUID."));
            }

            // Thread the pollToken for the subsequent PollAsync calls.
            _pollTokens[sid] = dto.PollToken ?? string.Empty;

            // First-party `/api/*` has no session secret / auth-code hashable — the
            // signature is bound to the PDF digest, not an RP challenge. Left empty.
            return Result<AuthSession>.Success(new AuthSession(
                sid,
                dto.Vc ?? string.Empty,
                DateTimeOffset.UtcNow.Add(SessionTtl),
                SessionSecret: string.Empty,
                AuthCodeHashable: string.Empty));
        }
        catch (OperationCanceledException) when (ct.IsCancellationRequested)
        {
            return Result<AuthSession>.Failure(ApiError.Cancelled("Initiate cancelled."));
        }
        catch (HttpRequestException ex)
        {
            _logger.LogWarning(ex, "Sign initiate network error");
            return Result<AuthSession>.Failure(ApiError.Network("Initiate network error.", ex.Message));
        }
    }

    public async Task<Result<AuthSessionStatus>> PollAsync(Guid sessionId, int timeoutMs, CancellationToken ct = default)
    {
        _ = timeoutMs; // `/api/status` long-holds ~1s server-side; caller re-polls.
        if (sessionId == Guid.Empty)
        {
            return Result<AuthSessionStatus>.Failure(ApiError.BadRequest("sessionId required."));
        }
        if (!_pollTokens.TryGetValue(sessionId, out var pollToken) || string.IsNullOrWhiteSpace(pollToken))
        {
            return Result<AuthSessionStatus>.Failure(ApiError.BadRequest("pollToken unknown — call InitiateAsync first."));
        }

        var url = string.Create(
            CultureInfo.InvariantCulture,
            $"/api/status?sessionId={Uri.EscapeDataString(sessionId.ToString("D"))}&pollToken={Uri.EscapeDataString(pollToken)}");

        try
        {
            using var resp = await _http.GetAsync(url, ct).ConfigureAwait(false);

            if (!resp.IsSuccessStatusCode)
            {
                return Result<AuthSessionStatus>.Failure(await MapErrorAsync(resp, ct).ConfigureAwait(false));
            }

            var dto = await resp.Content
                .ReadFromJsonAsync<StatusResponse>(JsonOptions, ct)
                .ConfigureAwait(false);

            if (dto is null)
            {
                return Result<AuthSessionStatus>.Failure(ApiError.Internal("Backend returned empty body."));
            }

            return Result<AuthSessionStatus>.Success(Map(dto));
        }
        catch (OperationCanceledException) when (ct.IsCancellationRequested)
        {
            return Result<AuthSessionStatus>.Failure(ApiError.Cancelled("Poll cancelled."));
        }
        catch (TaskCanceledException ex)
        {
            return Result<AuthSessionStatus>.Failure(ApiError.Timeout("Poll timed out.", ex.Message));
        }
        catch (HttpRequestException ex)
        {
            _logger.LogWarning(ex, "Sign poll network error");
            return Result<AuthSessionStatus>.Failure(ApiError.Network("Poll network error.", ex.Message));
        }
    }

    public async Task<Result<byte[]>> DownloadSignedPdfAsync(Guid sessionId, byte[] originalPdf, string fileName, CancellationToken ct = default)
    {
        ArgumentNullException.ThrowIfNull(originalPdf);
        if (sessionId == Guid.Empty)
        {
            return Result<byte[]>.Failure(ApiError.BadRequest("sessionId required."));
        }
        if (originalPdf.Length == 0)
        {
            return Result<byte[]>.Failure(ApiError.BadRequest("original PDF is required to download the signed copy."));
        }
        if (!_pollTokens.TryGetValue(sessionId, out var pollToken) || string.IsNullOrWhiteSpace(pollToken))
        {
            return Result<byte[]>.Failure(ApiError.BadRequest("pollToken unknown — call InitiateAsync first."));
        }

        try
        {
            // Multipart mirrors the web demo + macOS client: the backend re-hashes
            // the uploaded `file`, checks it against the digest it signed under this
            // session, then stamps (PAdES/PKCS#7 + verification page) and returns
            // the signed PDF. `sessionId` + `pollToken` gate the signature release.
            using var content = new MultipartFormDataContent();
            var fileContent = new ByteArrayContent(originalPdf);
            fileContent.Headers.ContentType = new MediaTypeHeaderValue("application/pdf");
            content.Add(fileContent, "file", string.IsNullOrWhiteSpace(fileName) ? "document.pdf" : fileName);
            content.Add(new StringContent(sessionId.ToString("D", CultureInfo.InvariantCulture)), "sessionId");
            content.Add(new StringContent(pollToken), "pollToken");

            using var resp = await _http
                .PostAsync("/api/sign-pdf-download", content, ct)
                .ConfigureAwait(false);

            if (!resp.IsSuccessStatusCode)
            {
                return Result<byte[]>.Failure(await MapErrorAsync(resp, ct).ConfigureAwait(false));
            }

            var bytes = await resp.Content.ReadAsByteArrayAsync(ct).ConfigureAwait(false);
            if (bytes.Length == 0)
            {
                return Result<byte[]>.Failure(ApiError.Internal("Backend returned an empty signed PDF."));
            }

            return Result<byte[]>.Success(bytes);
        }
        catch (OperationCanceledException) when (ct.IsCancellationRequested)
        {
            return Result<byte[]>.Failure(ApiError.Cancelled("Download cancelled."));
        }
        catch (HttpRequestException ex)
        {
            _logger.LogWarning(ex, "Sign download network error");
            return Result<byte[]>.Failure(ApiError.Network("Download network error.", ex.Message));
        }
    }

    private static AuthSessionStatus Map(StatusResponse dto)
    {
        // Local session state machine (server/internal/domain/enums.go):
        //   state:     RUNNING (keep polling) | COMPLETE (terminal)
        //   endResult: OK | TIMEOUT | USER_REFUSED* | WRONG_VC | FAILED | …
        var state = (dto.State ?? string.Empty).ToUpperInvariant();
        if (state != "COMPLETE")
        {
            return new AuthSessionStatus(AuthSessionState.Running, dto.State, null, null, null, null);
        }

        var end = (dto.EndResult ?? string.Empty).ToUpperInvariant();
        if (end == "OK")
        {
            // Identity + signature arrive inline on completion (no bearer, no cert PEM).
            var identity = new AuthIdentity(
                dto.IdNumber ?? string.Empty,
                CivilId: null,
                GivenName: null,
                Surname: null,
                FullName: dto.Name ?? string.Empty,
                KycLevel: dto.CertificateLevel ?? string.Empty);

            AuthSignature? signature = string.IsNullOrEmpty(dto.SignatureValueB64)
                ? null
                : new AuthSignature(dto.SignatureValueB64, dto.SignatureAlgorithm ?? string.Empty, Hashable: string.Empty);

            return new AuthSessionStatus(AuthSessionState.Complete, "OK", null, identity, null, signature);
        }

        if (end == "TIMEOUT")
        {
            return new AuthSessionStatus(AuthSessionState.Expired, "TIMEOUT", null, null, null, null);
        }

        // USER_REFUSED*, WRONG_VC, FAILED, DOCUMENT_UNUSABLE, … → refused.
        var reason = string.IsNullOrWhiteSpace(dto.Error) ? end : dto.Error;
        return new AuthSessionStatus(AuthSessionState.Refused, reason, null, null, null, null);
    }

    private static async Task<ApiError> MapErrorAsync(HttpResponseMessage resp, CancellationToken ct)
    {
        string? body = null;
        BackendError? parsed = null;
        try
        {
            body = await resp.Content.ReadAsStringAsync(ct).ConfigureAwait(false);
            if (body.Length > 0)
            {
                try { parsed = JsonSerializer.Deserialize<BackendError>(body, JsonOptions); }
                catch (JsonException) { }
            }
        }
        catch
        {
            // ignore
        }

        var msg = string.IsNullOrWhiteSpace(parsed?.Error) ? body : parsed!.Error;

        return resp.StatusCode switch
        {
            HttpStatusCode.NotFound            => ApiError.NotFound(msg ?? "not_found", body),
            HttpStatusCode.BadRequest          => ApiError.BadRequest(msg ?? "invalid_body", body),
            HttpStatusCode.Unauthorized        => ApiError.Unauthorized(msg ?? "unauthenticated", body),
            (HttpStatusCode)429                => ApiError.BadRequest(msg ?? "rate_limited", body),
            HttpStatusCode.RequestTimeout      => ApiError.Timeout(msg ?? "timeout", body),
            _ when (int)resp.StatusCode >= 500 => ApiError.Server(msg ?? $"server_{(int)resp.StatusCode}", body),
            _ => ApiError.Internal(msg ?? $"unexpected_{(int)resp.StatusCode}", body),
        };
    }

    // ── Wire DTOs (camelCase; JsonSerializerDefaults.Web matches case-insensitively) ──

    private sealed record SignStartBody(
        [property: JsonPropertyName("etsi")] string Etsi,
        [property: JsonPropertyName("digestB64")] string DigestB64,
        [property: JsonPropertyName("fileName")] string? FileName,
        [property: JsonPropertyName("callbackUrl")] string? CallbackUrl);

    private sealed record SignStartResponse(
        [property: JsonPropertyName("sessionId")] string? SessionId,
        [property: JsonPropertyName("vc")] string? Vc,
        [property: JsonPropertyName("pollToken")] string? PollToken);

    private sealed record StatusResponse(
        [property: JsonPropertyName("state")] string? State,
        [property: JsonPropertyName("endResult")] string? EndResult,
        [property: JsonPropertyName("name")] string? Name,
        [property: JsonPropertyName("idNumber")] string? IdNumber,
        [property: JsonPropertyName("documentNumber")] string? DocumentNumber,
        [property: JsonPropertyName("certificateLevel")] string? CertificateLevel,
        [property: JsonPropertyName("signatureValueB64")] string? SignatureValueB64,
        [property: JsonPropertyName("signatureAlgorithm")] string? SignatureAlgorithm,
        [property: JsonPropertyName("error")] string? Error);

    private sealed record BackendError(
        [property: JsonPropertyName("error")] string? Error);
}
