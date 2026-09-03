using System.Net;
using System.Net.Http.Json;
using System.Text.Json;
using System.Text.Json.Serialization;
using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Domain.Auth;
using NexusGeregeDesktop.Domain.Primitives;
using Microsoft.Extensions.Logging;

namespace NexusGeregeDesktop.Infrastructure.Auth;

/// Citizen-facing auth flow — **first-party** model (see BACKEND-INTEGRATION.md).
///
/// Talks to THIS PLATFORM's own public auth routes, exactly like the browser
/// does (no RP secret, no device HMAC, no bearer):
///   • national-id push  → POST /api/v1/auth/eid/start-id {national_id, callbackUrl}
///   • QR                → POST /api/v1/auth/eid/start     {callbackUrl}
///   • poll              → POST /api/v1/auth/eid/poll      {session_id}
///
/// It used to call `/api/start`, `/api/login-notify` and `/api/status`, which
/// nginx proxies to eidmongolia.mn — eID Mongolia's OWN web app. Sessions were
/// therefore opened under THEIR demo relying party, and the citizen read
/// "RP Demo Bank" on the phone while approving a sign-in to this product.
///
/// The poll returns the citizen block inline on completion (no bearer, no
/// `/me`), so on success we stash that identity in
/// <see cref="ISessionIdentityCache"/> for `UserProfileService.GetMeAsync`.
/// `callbackUrl` is empty on purpose: the platform only accepts its own
/// `${PUBLIC_ORIGIN}/auth/eid/callback`, and a desktop has nowhere to return to.
public sealed class CitizenAuthService : ICitizenAuthService
{
    /// The web session default TTL isn't echoed by `/api/*`; use a conservative
    /// client-side estimate purely to drive the "code expired" UI countdown.
    private static readonly TimeSpan SessionTtl = TimeSpan.FromMinutes(5);

    private static readonly JsonSerializerOptions JsonOptions = new(JsonSerializerDefaults.Web)
    {
        DefaultIgnoreCondition = JsonIgnoreCondition.WhenWritingNull,
    };

    private readonly HttpClient _http;
    private readonly IWebSessionStore _sessions;
    private readonly ISessionIdentityCache _identity;
    private readonly ILogger<CitizenAuthService> _logger;

    public CitizenAuthService(
        HttpClient http,
        IWebSessionStore sessions,
        ISessionIdentityCache identity,
        ILogger<CitizenAuthService> logger)
    {
        _http = http;
        _sessions = sessions;
        _identity = identity;
        _logger = logger;
    }

    public async Task<Result<WebAuthSession>> InitiateAsync(string nationalId, CancellationToken ct = default)
    {
        if (string.IsNullOrWhiteSpace(nationalId))
        {
            return Result<WebAuthSession>.Failure(ApiError.BadRequest("national_id is required."));
        }

        // POST /api/v1/auth/eid/start-id. `national_id` may be РД / civil-id /
        // PNOMN-… — the server resolves any of them. Rate-limited per target.
        var body = new StartByIdBody(nationalId.Trim(), string.Empty);
        try
        {
            using var resp = await _http
                .PostAsJsonAsync("/api/v1/auth/eid/start-id", body, JsonOptions, ct)
                .ConfigureAwait(false);

            if (!resp.IsSuccessStatusCode)
            {
                return Result<WebAuthSession>.Failure(await MapErrorAsync(resp, ct).ConfigureAwait(false));
            }

            var dto = await resp.Content
                .ReadFromJsonAsync<PlatformStartResponse>(JsonOptions, ct)
                .ConfigureAwait(false);

            return BuildSession(dto, deviceLinkFromQr: false);
        }
        catch (OperationCanceledException) when (ct.IsCancellationRequested)
        {
            return Result<WebAuthSession>.Failure(ApiError.Cancelled("Initiate cancelled."));
        }
        catch (HttpRequestException ex)
        {
            _logger.LogWarning(ex, "Web auth initiate network error");
            return Result<WebAuthSession>.Failure(ApiError.Network("Initiate network error.", ex.Message));
        }
    }

    public async Task<Result<WebAuthSession>> InitiateQrAsync(CancellationToken ct = default)
    {
        try
        {
            // POST /api/v1/auth/eid/start → {session_id, device_link_url,
            // verification_code}. What the QR carries is the plain session id:
            // eID's scanner reads the UUID and resolves it against its own server.
            using var resp = await _http
                .PostAsJsonAsync("/api/v1/auth/eid/start", new StartBody(string.Empty), JsonOptions, ct)
                .ConfigureAwait(false);

            if (!resp.IsSuccessStatusCode)
            {
                return Result<WebAuthSession>.Failure(await MapErrorAsync(resp, ct).ConfigureAwait(false));
            }

            var dto = await resp.Content
                .ReadFromJsonAsync<PlatformStartResponse>(JsonOptions, ct)
                .ConfigureAwait(false);

            return BuildSession(dto, deviceLinkFromQr: true);
        }
        catch (OperationCanceledException) when (ct.IsCancellationRequested)
        {
            return Result<WebAuthSession>.Failure(ApiError.Cancelled("QR init cancelled."));
        }
        catch (HttpRequestException ex)
        {
            _logger.LogWarning(ex, "Web auth QR init network error");
            return Result<WebAuthSession>.Failure(ApiError.Network("QR init network error.", ex.Message));
        }
    }

    public async Task<Result<WebAuthSessionStatus>> PollAsync(Guid sessionId, string pollToken, CancellationToken ct = default)
    {
        if (sessionId == Guid.Empty)
        {
            return Result<WebAuthSessionStatus>.Failure(ApiError.BadRequest("sessionId required."));
        }
        // The platform's poll needs no token: the session id is the handle, and
        // the citizen block is only released once eID says COMPLETE. The
        // parameter stays on the interface for the signing flow, which still
        // rides eID's own web app and does gate on a token there.
        _ = pollToken;

        // POST /api/v1/auth/eid/poll {session_id}. The server holds the request
        // for up to 25s (`eid.PollWindow`) and answers RUNNING when that window
        // fills; the ViewModel re-polls.
        try
        {
            using var resp = await _http
                .PostAsJsonAsync("/api/v1/auth/eid/poll", new PollBody(sessionId.ToString("D")), JsonOptions, ct)
                .ConfigureAwait(false);

            if (!resp.IsSuccessStatusCode)
            {
                return Result<WebAuthSessionStatus>.Failure(await MapErrorAsync(resp, ct).ConfigureAwait(false));
            }

            var dto = await resp.Content
                .ReadFromJsonAsync<PollResponse>(JsonOptions, ct)
                .ConfigureAwait(false);

            if (dto is null)
            {
                return Result<WebAuthSessionStatus>.Failure(ApiError.Internal("Backend returned empty body."));
            }

            return Result<WebAuthSessionStatus>.Success(MapStatus(sessionId, dto));
        }
        catch (OperationCanceledException) when (ct.IsCancellationRequested)
        {
            return Result<WebAuthSessionStatus>.Failure(ApiError.Cancelled("Poll cancelled."));
        }
        catch (HttpRequestException ex)
        {
            _logger.LogWarning(ex, "Web auth poll network error");
            return Result<WebAuthSessionStatus>.Failure(ApiError.Network("Poll network error.", ex.Message));
        }
    }

    // ── USB hardware-token web login (M9.B) ──────────────────────────────────
    // No first-party `/api/*` equivalent exists (there is no `/token/challenge`
    // or `/token/verify` on the web backend). Surfaced as an explicit, honest
    // failure so the "Токен" login tab degrades cleanly instead of 404-ing.

    public Task<Result<WebTokenChallenge>> InitiateTokenChallengeAsync(CancellationToken ct = default)
        => Task.FromResult(Result<WebTokenChallenge>.Failure(
            ApiError.Internal("token_login_not_available_on_first_party_backend")));

    public Task<Result<WebTokenLoginResult>> VerifyTokenAsync(
        Guid sessionId,
        string certPem,
        byte[] signature,
        WebTokenSignatureAlg signatureAlg,
        CancellationToken ct = default)
        => Task.FromResult(Result<WebTokenLoginResult>.Failure(
            ApiError.Internal("token_login_not_available_on_first_party_backend")));

    public async Task<Result> LogoutAsync(CancellationToken ct = default)
    {
        // First-party model has no server-side session/bearer to revoke — the
        // citizen's identity was only ever held client-side. Just wipe local state.
        _identity.Clear();
        await _sessions.ClearAsync(ct).ConfigureAwait(false);
        return Result.Success();
    }

    private static Result<WebAuthSession> BuildSession(PlatformStartResponse? dto, bool deviceLinkFromQr)
    {
        if (dto is null || string.IsNullOrWhiteSpace(dto.SessionId))
        {
            return Result<WebAuthSession>.Failure(ApiError.Internal("Backend returned empty session."));
        }
        if (!Guid.TryParse(dto.SessionId, out var sid))
        {
            return Result<WebAuthSession>.Failure(ApiError.Internal("Backend session id was not a UUID."));
        }

        // QR mode: the payload is the session id itself. Push mode: no QR.
        var deviceLink = deviceLinkFromQr ? dto.SessionId : null;

        return Result<WebAuthSession>.Success(new WebAuthSession(
            sid,
            dto.VerificationCode ?? string.Empty,
            // No poll token in the platform flow — see PollAsync.
            string.Empty,
            DateTimeOffset.UtcNow.Add(SessionTtl),
            deviceLink));
    }

    private WebAuthSessionStatus MapStatus(Guid sessionId, PollResponse dto)
    {
        // Platform session states (backend/internal/workspace/identity/eid):
        //   RUNNING (keep polling) | COMPLETE | EXPIRED | REFUSED
        var state = (dto.State ?? string.Empty).ToUpperInvariant();

        if (state == "COMPLETE" && dto.Identity is { } person)
        {
            // First-party bridge: identity arrives inline; there is no bearer.
            // Synthesize a stable-per-session token + user id and cache the
            // profile so UserProfileService.GetMeAsync can serve it.
            //
            // The names arrive in Mongolian, so nothing has to un-transliterate
            // the Latin subject of the certificate any more.
            var userId = Guid.NewGuid();
            var mongolian = string.Join(' ', new[] { person.LastName, person.FirstName }
                .Where(part => !string.IsNullOrWhiteSpace(part)));
            var latin = string.Join(' ', new[] { person.LastNameEn, person.FirstNameEn }
                .Where(part => !string.IsNullOrWhiteSpace(part)));

            _identity.Set(new UserProfile(
                userId,
                person.CivilId ?? string.Empty,
                mongolian,
                FullNameLatin: latin,
                // The sign-in floor is ADVANCED (`EID_CERT_LEVEL`); claiming
                // QUALIFIED here would assert something nobody checked.
                KycLevel: "ADVANCED",
                Status: "active",
                CreatedAt: DateTimeOffset.UtcNow));

            return new WebAuthSessionStatus(
                WebAuthState.Confirmed,
                SessionToken: sessionId.ToString("D"),
                UserId: userId,
                FailureReason: null,
                ExpiresAt: DateTimeOffset.UtcNow.Add(SessionTtl));
        }

        if (state == "EXPIRED")
        {
            return new WebAuthSessionStatus(WebAuthState.Expired, null, null, "timeout", null);
        }

        if (state == "REFUSED")
        {
            return new WebAuthSessionStatus(WebAuthState.Failed, null, null, "user_refused", null);
        }

        if (state == "COMPLETE")
        {
            // COMPLETE with no citizen block: eID finished, verification did not.
            return new WebAuthSessionStatus(WebAuthState.Failed, null, null, "verification_failed", null);
        }

        // RUNNING / anything transient → keep polling.
        return new WebAuthSessionStatus(WebAuthState.Pushed, null, null, null, null);
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
        catch { }

        var msg = string.IsNullOrWhiteSpace(parsed?.Error) ? body : parsed!.Error;

        return resp.StatusCode switch
        {
            HttpStatusCode.NotFound          => ApiError.NotFound(msg ?? "not_found", body),
            HttpStatusCode.BadRequest        => ApiError.BadRequest(msg ?? "invalid_body", body),
            HttpStatusCode.Unauthorized      => ApiError.Unauthorized(msg ?? "unauthenticated", body),
            (HttpStatusCode)429              => ApiError.BadRequest(msg ?? "rate_limited", body),
            HttpStatusCode.RequestTimeout    => ApiError.Timeout(msg ?? "timeout", body),
            _ when (int)resp.StatusCode >= 500 => ApiError.Server(msg ?? $"server_{(int)resp.StatusCode}", body),
            _ => ApiError.Internal(msg ?? $"unexpected_{(int)resp.StatusCode}", body),
        };
    }

    // ── Wire DTOs (camelCase; JsonSerializerDefaults.Web matches case-insensitively) ──

    private sealed record StartBody(
        [property: JsonPropertyName("callbackUrl")] string CallbackUrl);

    private sealed record StartByIdBody(
        [property: JsonPropertyName("national_id")] string NationalId,
        [property: JsonPropertyName("callbackUrl")] string CallbackUrl);

    private sealed record PollBody(
        [property: JsonPropertyName("session_id")] string SessionId);

    /// Shape of /api/v1/auth/eid/start and …/start-id (Go core: eid.StartResult).
    private sealed record PlatformStartResponse(
        [property: JsonPropertyName("session_id")] string? SessionId,
        [property: JsonPropertyName("device_link_url")] string? DeviceLinkUrl,
        [property: JsonPropertyName("verification_code")] string? VerificationCode);

    private sealed record PollResponse(
        [property: JsonPropertyName("state")] string? State,
        [property: JsonPropertyName("identity")] PollIdentity? Identity);

    private sealed record PollIdentity(
        [property: JsonPropertyName("civil_id")] string? CivilId,
        [property: JsonPropertyName("reg_number")] string? RegNumber,
        [property: JsonPropertyName("first_name")] string? FirstName,
        [property: JsonPropertyName("last_name")] string? LastName,
        [property: JsonPropertyName("first_name_en")] string? FirstNameEn,
        [property: JsonPropertyName("last_name_en")] string? LastNameEn,
        [property: JsonPropertyName("certificate_serial")] string? CertificateSerial);

    private sealed record BackendError(
        [property: JsonPropertyName("error")] string? Error);
}
