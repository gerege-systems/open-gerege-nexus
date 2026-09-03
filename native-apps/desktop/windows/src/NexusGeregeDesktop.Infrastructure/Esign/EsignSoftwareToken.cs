using System.Globalization;
using System.Net;
using System.Net.Http.Json;
using System.Text.Json;
using System.Text.Json.Serialization;
using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Domain.Primitives;
using Microsoft.Extensions.Logging;

namespace NexusGeregeDesktop.Infrastructure.Esign;

/// <summary>
/// Программ токен — физик токен байхгүй үед нэвтэрсэн иргэний identity-ээр ESIGN
/// гарын үсэг зурна. First-party `/api/*` загвартай нийцүүлэн:
///   • гэрчилгээ (session-bound): POST /api/certificates {sessionId, pollToken}
///        → { signing, auth } (b64 DER). Backend identity-г баталгаажсан login
///          session-ээс гаргана (клиентийн personId-д итгэхгүй).
///   • гарын үсэг: POST /api/esign-sign {personId, digestB64, key, displayText}
///        → { sessionId, pollToken, vc } → GET /api/status?sessionId=&amp;pollToken=
///          → { state, signatureValueB64 }  (sign-pdf-start-тай ижил posture).
/// Backend эдгээр endpoint-ийг хараахан хэрэгжүүлээгүй бол "esign_backend_not_available".
/// </summary>
public sealed class EsignSoftwareToken : IEsignSoftwareToken
{
    private static readonly JsonSerializerOptions JsonOptions = new(JsonSerializerDefaults.Web)
    {
        DefaultIgnoreCondition = JsonIgnoreCondition.WhenWritingNull,
    };

    private readonly HttpClient _http;
    private readonly IWebSessionStore _sessions;
    private readonly ILogger<EsignSoftwareToken> _log;

    public EsignSoftwareToken(HttpClient http, IWebSessionStore sessions, ILogger<EsignSoftwareToken> log)
    {
        _http = http;
        _sessions = sessions;
        _log = log;
    }

    public bool IsAvailable => _sessions.Current is { Token.Length: > 0 };

    public async Task<Result<EsignSoftwareIdentity>> GetIdentityAsync(CancellationToken ct = default)
    {
        var me = _sessions.Current;
        if (me is null || string.IsNullOrWhiteSpace(me.Token))
            return Result<EsignSoftwareIdentity>.Failure(ApiError.Unauthorized("Апп-д нэвтрээгүй байна."));

        CertsResponse? dto;
        try
        {
            // Session-bound: sessionId (=WebSession.Token) + pollToken. Backend
            // identity-г баталгаажсан session-ээс гаргана.
            var body = new CertsAuthBody(me.Token, me.PollToken);
            using var resp = await _http
                .PostAsJsonAsync("/api/certificates", body, JsonOptions, ct)
                .ConfigureAwait(false);
            if (resp.StatusCode == HttpStatusCode.NotFound)
                return Result<EsignSoftwareIdentity>.Failure(ApiError.Internal("esign_backend_not_available"));
            if (!resp.IsSuccessStatusCode)
                return Result<EsignSoftwareIdentity>.Failure(await MapErrorAsync(resp, ct).ConfigureAwait(false));
            dto = await resp.Content.ReadFromJsonAsync<CertsResponse>(JsonOptions, ct).ConfigureAwait(false);
        }
        catch (OperationCanceledException) when (ct.IsCancellationRequested)
        {
            return Result<EsignSoftwareIdentity>.Failure(ApiError.Cancelled("Цуцлагдав."));
        }
        catch (HttpRequestException ex)
        {
            _log.LogWarning(ex, "Программ токен: гэрчилгээ авах сүлжээний алдаа");
            return Result<EsignSoftwareIdentity>.Failure(ApiError.Network("Гэрчилгээ авахад алдаа.", ex.Message));
        }

        // Нэвтрэлтэд гарын үсгийн (signing) cert-ийг эхэлж, байхгүй бол auth-г ашиглана.
        string? b64 = !string.IsNullOrWhiteSpace(dto?.Signing) ? dto!.Signing : dto?.Auth;
        if (string.IsNullOrWhiteSpace(b64))
            return Result<EsignSoftwareIdentity>.Failure(ApiError.Internal("Backend гэрчилгээ буцаасангүй."));

        byte[] der;
        try { der = Convert.FromBase64String(b64); }
        catch { return Result<EsignSoftwareIdentity>.Failure(ApiError.Internal("Гэрчилгээ b64 биш байна.")); }

        return Result<EsignSoftwareIdentity>.Success(new EsignSoftwareIdentity(
            der,
            EsignCertParser.Serial(der),
            EsignCertParser.KeyId(der),
            EsignCertParser.ParseOption(der)));
    }

    public async Task<Result<byte[]>> SignDigestAsync(
        byte[] sha256Digest, string displayText, Func<string, Task>? onVerificationCode = null, CancellationToken ct = default)
    {
        var me = _sessions.Current;
        if (me is null || string.IsNullOrWhiteSpace(me.NationalId))
            return Result<byte[]>.Failure(ApiError.Unauthorized("Апп-д нэвтрээгүй байна."));
        if (sha256Digest is not { Length: 32 })
            return Result<byte[]>.Failure(ApiError.BadRequest("digest 32 байт байх ёстой."));

        // 1) start → утас руу push (esign-sign нь personId-оор, sign-pdf-start-тай ижил)
        SignStartResponse? start;
        try
        {
            var body = new SignStartBody(me.NationalId, Convert.ToBase64String(sha256Digest), "signing", displayText);
            using var resp = await _http.PostAsJsonAsync("/api/esign-sign", body, JsonOptions, ct).ConfigureAwait(false);
            if (resp.StatusCode == HttpStatusCode.NotFound)
                return Result<byte[]>.Failure(ApiError.Internal("esign_backend_not_available"));
            if (!resp.IsSuccessStatusCode)
                return Result<byte[]>.Failure(await MapErrorAsync(resp, ct).ConfigureAwait(false));
            start = await resp.Content.ReadFromJsonAsync<SignStartResponse>(JsonOptions, ct).ConfigureAwait(false);
        }
        catch (OperationCanceledException) when (ct.IsCancellationRequested)
        {
            return Result<byte[]>.Failure(ApiError.Cancelled("Цуцлагдав."));
        }
        catch (HttpRequestException ex)
        {
            _log.LogWarning(ex, "Программ токен: гарын үсэг эхлүүлэх сүлжээний алдаа");
            return Result<byte[]>.Failure(ApiError.Network("Гарын үсэг эхлүүлэхэд алдаа.", ex.Message));
        }

        if (start is null || string.IsNullOrWhiteSpace(start.SessionId) || string.IsNullOrWhiteSpace(start.PollToken))
            return Result<byte[]>.Failure(ApiError.Internal("Backend session буцаасангүй."));

        if (onVerificationCode is not null && !string.IsNullOrWhiteSpace(start.Vc))
        {
            try { await onVerificationCode(start.Vc!).ConfigureAwait(false); } catch { /* UI алдаа non-fatal */ }
        }

        // 2) poll /api/status → signatureValueB64 (esign-sign-ий буцаасан sessionId+pollToken-оор)
        var url = string.Create(CultureInfo.InvariantCulture,
            $"/api/status?sessionId={Uri.EscapeDataString(start.SessionId!)}&pollToken={Uri.EscapeDataString(start.PollToken!)}");
        var deadline = DateTimeOffset.UtcNow.AddSeconds(120);
        while (DateTimeOffset.UtcNow < deadline)
        {
            ct.ThrowIfCancellationRequested();
            StatusResponse? st;
            try
            {
                using var resp = await _http.GetAsync(url, ct).ConfigureAwait(false);
                if (!resp.IsSuccessStatusCode)
                    return Result<byte[]>.Failure(await MapErrorAsync(resp, ct).ConfigureAwait(false));
                st = await resp.Content.ReadFromJsonAsync<StatusResponse>(JsonOptions, ct).ConfigureAwait(false);
            }
            catch (OperationCanceledException) when (ct.IsCancellationRequested)
            {
                return Result<byte[]>.Failure(ApiError.Cancelled("Цуцлагдав."));
            }
            catch (HttpRequestException ex)
            {
                _log.LogWarning(ex, "Программ токен: poll сүлжээний алдаа");
                return Result<byte[]>.Failure(ApiError.Network("Poll алдаа.", ex.Message));
            }

            var state = (st?.State ?? "").ToUpperInvariant();
            if (state == "COMPLETE")
            {
                var end = (st?.EndResult ?? "").ToUpperInvariant();
                if (end != "OK")
                    return Result<byte[]>.Failure(ApiError.Internal("Гарын үсэг амжилтгүй: " + (st?.Error ?? end)));
                if (string.IsNullOrWhiteSpace(st?.SignatureValueB64))
                    return Result<byte[]>.Failure(ApiError.Internal("Backend гарын үсэг буцаасангүй."));
                try { return Result<byte[]>.Success(Convert.FromBase64String(st!.SignatureValueB64!)); }
                catch { return Result<byte[]>.Failure(ApiError.Internal("Гарын үсэг b64 биш.")); }
            }
            // RUNNING → /api/status ~1сек server-side long-hold; шууд дахин poll.
        }
        return Result<byte[]>.Failure(ApiError.Timeout("Гар утаснаас баталгаажуулалт хугацаандаа ирсэнгүй."));
    }

    private static async Task<ApiError> MapErrorAsync(HttpResponseMessage resp, CancellationToken ct)
    {
        string? body = null;
        try { body = await resp.Content.ReadAsStringAsync(ct).ConfigureAwait(false); } catch { }
        return resp.StatusCode switch
        {
            HttpStatusCode.NotFound      => ApiError.Internal("esign_backend_not_available"),
            HttpStatusCode.Unauthorized  => ApiError.Unauthorized("unauthenticated", body),
            HttpStatusCode.BadRequest    => ApiError.BadRequest("invalid_body", body),
            _ when (int)resp.StatusCode >= 500 => ApiError.Server($"server_{(int)resp.StatusCode}", body),
            _ => ApiError.Internal($"unexpected_{(int)resp.StatusCode}", body),
        };
    }

    // ── Wire DTOs (first-party `/api/*` конвенц: camelCase) ──
    private sealed record CertsAuthBody(
        [property: JsonPropertyName("sessionId")] string SessionId,
        [property: JsonPropertyName("pollToken")] string PollToken);

    private sealed record CertsResponse(
        [property: JsonPropertyName("signing")] string? Signing,
        [property: JsonPropertyName("auth")] string? Auth,
        [property: JsonPropertyName("certificateLevel")] string? CertificateLevel);

    private sealed record SignStartBody(
        [property: JsonPropertyName("personId")] string PersonId,
        [property: JsonPropertyName("digestB64")] string DigestB64,
        [property: JsonPropertyName("key")] string Key,
        [property: JsonPropertyName("displayText")] string? DisplayText);

    private sealed record SignStartResponse(
        [property: JsonPropertyName("sessionId")] string? SessionId,
        [property: JsonPropertyName("pollToken")] string? PollToken,
        [property: JsonPropertyName("vc")] string? Vc);

    private sealed record StatusResponse(
        [property: JsonPropertyName("state")] string? State,
        [property: JsonPropertyName("endResult")] string? EndResult,
        [property: JsonPropertyName("signatureValueB64")] string? SignatureValueB64,
        [property: JsonPropertyName("signatureAlgorithm")] string? SignatureAlgorithm,
        [property: JsonPropertyName("error")] string? Error);
}
