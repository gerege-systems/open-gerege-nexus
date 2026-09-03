using System.Net;
using System.Net.Http.Json;
using System.Text.Json;
using System.Text.Json.Serialization;
using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Domain.Auth;
using NexusGeregeDesktop.Domain.Guardian;
using NexusGeregeDesktop.Domain.Primitives;
using Microsoft.Extensions.Logging;

namespace NexusGeregeDesktop.Infrastructure.Guardian;

/// <summary>
/// First-party "Миний хүүхдүүд" үйлчилгээ.
///   • жагсаалт (session-bound): POST /api/children {sessionId, pollToken}
///        → { children: [...] }  (backend identity-г баталгаажсан login session-ээс гаргана)
///   • нэмэх/цуцлах: POST /api/child/{init|code|revoke} — эдгээр маршрут хараахан
///        байхгүй (404) тул "guardian_backend_not_available"-аар цэвэрхэн уналт өгнө.
/// </summary>
public sealed class GuardianService : IGuardianService
{
    private static readonly JsonSerializerOptions JsonOptions = new(JsonSerializerDefaults.Web)
    {
        DefaultIgnoreCondition = JsonIgnoreCondition.WhenWritingNull,
    };

    private readonly HttpClient _http;
    private readonly IWebSessionStore _sessions;
    private readonly ILogger<GuardianService> _log;

    public GuardianService(HttpClient http, IWebSessionStore sessions, ILogger<GuardianService> log)
    {
        _http = http;
        _sessions = sessions;
        _log = log;
    }

    public async Task<Result<IReadOnlyList<GuardianChild>>> ListChildrenAsync(CancellationToken ct = default)
    {
        if (!TryMe(out var me, out var meErr))
            return Result<IReadOnlyList<GuardianChild>>.Failure(meErr);

        try
        {
            var body = new SessionAuthBody(me.Token, me.PollToken);
            using var resp = await _http.PostAsJsonAsync("/api/children", body, JsonOptions, ct).ConfigureAwait(false);
            if (resp.StatusCode == HttpStatusCode.NotFound)
                return Result<IReadOnlyList<GuardianChild>>.Failure(ApiError.Internal("guardian_backend_not_available"));
            if (!resp.IsSuccessStatusCode)
                return Result<IReadOnlyList<GuardianChild>>.Failure(await MapErrorAsync(resp, ct).ConfigureAwait(false));

            var dto = await resp.Content.ReadFromJsonAsync<ChildrenDto>(JsonOptions, ct).ConfigureAwait(false);
            var children = (dto?.Children ?? new List<ChildDto>()).Select(c => new GuardianChild(
                c.Etsi ?? string.Empty,
                c.RegNo ?? string.Empty,
                c.Name ?? string.Empty,
                c.BirthDate,
                c.Registered,
                c.CertNotAfter)).ToList();
            return Result<IReadOnlyList<GuardianChild>>.Success(children);
        }
        catch (OperationCanceledException) when (ct.IsCancellationRequested)
        {
            return Result<IReadOnlyList<GuardianChild>>.Failure(ApiError.Cancelled("Цуцлагдав."));
        }
        catch (HttpRequestException ex)
        {
            _log.LogWarning(ex, "Хүүхдийн жагсаалт: сүлжээний алдаа");
            return Result<IReadOnlyList<GuardianChild>>.Failure(ApiError.Network("Хүүхдийн жагсаалт авахад алдаа.", ex.Message));
        }
    }

    public async Task<Result<ChildAddSession>> AddChildInitAsync(string childRegNo, CancellationToken ct = default)
    {
        if (!TryMe(out var me, out var meErr))
            return Result<ChildAddSession>.Failure(meErr);
        if (string.IsNullOrWhiteSpace(childRegNo))
            return Result<ChildAddSession>.Failure(ApiError.BadRequest("Хүүхдийн РД оруулна уу."));

        try
        {
            var body = new ChildInitBody(me.NationalId, childRegNo.Trim());
            using var resp = await _http.PostAsJsonAsync("/api/child/init", body, JsonOptions, ct).ConfigureAwait(false);
            if (resp.StatusCode == HttpStatusCode.NotFound)
                return Result<ChildAddSession>.Failure(ApiError.Internal("guardian_backend_not_available"));
            if (!resp.IsSuccessStatusCode)
                return Result<ChildAddSession>.Failure(await MapErrorAsync(resp, ct).ConfigureAwait(false));

            var dto = await resp.Content.ReadFromJsonAsync<ChildInitDto>(JsonOptions, ct).ConfigureAwait(false);
            if (dto is null || string.IsNullOrWhiteSpace(dto.SessionId))
                return Result<ChildAddSession>.Failure(ApiError.Internal("Backend session буцаасангүй."));
            return Result<ChildAddSession>.Success(new ChildAddSession(
                dto.SessionId!, dto.VerificationCode ?? string.Empty, dto.ChildName ?? string.Empty, dto.ChildRegNo ?? childRegNo));
        }
        catch (OperationCanceledException) when (ct.IsCancellationRequested)
        {
            return Result<ChildAddSession>.Failure(ApiError.Cancelled("Цуцлагдав."));
        }
        catch (HttpRequestException ex)
        {
            _log.LogWarning(ex, "Хүүхэд нэмэх (init): сүлжээний алдаа");
            return Result<ChildAddSession>.Failure(ApiError.Network("Хүүхэд нэмэхэд алдаа.", ex.Message));
        }
    }

    public async Task<Result<ChildEnrollCode>> AddChildCodeAsync(string sessionId, CancellationToken ct = default)
    {
        if (!TryMe(out _, out var meErr))
            return Result<ChildEnrollCode>.Failure(meErr);
        if (string.IsNullOrWhiteSpace(sessionId))
            return Result<ChildEnrollCode>.Failure(ApiError.BadRequest("sessionId алга."));

        try
        {
            var body = new ChildCodeBody(sessionId);
            using var resp = await _http.PostAsJsonAsync("/api/child/code", body, JsonOptions, ct).ConfigureAwait(false);
            if (resp.StatusCode == HttpStatusCode.NotFound)
                return Result<ChildEnrollCode>.Failure(ApiError.Internal("guardian_backend_not_available"));
            if (!resp.IsSuccessStatusCode)
                return Result<ChildEnrollCode>.Failure(await MapErrorAsync(resp, ct).ConfigureAwait(false));

            var dto = await resp.Content.ReadFromJsonAsync<ChildCodeDto>(JsonOptions, ct).ConfigureAwait(false);
            if (dto is null || string.IsNullOrWhiteSpace(dto.EnrollCode))
                return Result<ChildEnrollCode>.Failure(ApiError.Internal("Backend код буцаасангүй."));
            return Result<ChildEnrollCode>.Success(new ChildEnrollCode(dto.EnrollCode!, dto.ExpiresInSeconds));
        }
        catch (OperationCanceledException) when (ct.IsCancellationRequested)
        {
            return Result<ChildEnrollCode>.Failure(ApiError.Cancelled("Цуцлагдав."));
        }
        catch (HttpRequestException ex)
        {
            _log.LogWarning(ex, "Хүүхэд нэмэх (code): сүлжээний алдаа");
            return Result<ChildEnrollCode>.Failure(ApiError.Network("Бүртгэлийн код авахад алдаа.", ex.Message));
        }
    }

    public async Task<Result> RevokeChildAsync(string childRegNo, CancellationToken ct = default)
    {
        if (!TryMe(out var me, out var meErr))
            return Result.Failure(meErr);
        if (string.IsNullOrWhiteSpace(childRegNo))
            return Result.Failure(ApiError.BadRequest("Хүүхдийн РД алга."));

        try
        {
            var body = new ChildRevokeBody(me.NationalId, childRegNo.Trim());
            using var resp = await _http.PostAsJsonAsync("/api/child/revoke", body, JsonOptions, ct).ConfigureAwait(false);
            if (resp.StatusCode == HttpStatusCode.NotFound)
                return Result.Failure(ApiError.Internal("guardian_backend_not_available"));
            if (!resp.IsSuccessStatusCode)
                return Result.Failure(await MapErrorAsync(resp, ct).ConfigureAwait(false));
            return Result.Success();
        }
        catch (OperationCanceledException) when (ct.IsCancellationRequested)
        {
            return Result.Failure(ApiError.Cancelled("Цуцлагдав."));
        }
        catch (HttpRequestException ex)
        {
            _log.LogWarning(ex, "Хүүхэд цуцлах: сүлжээний алдаа");
            return Result.Failure(ApiError.Network("Цуцлахад алдаа.", ex.Message));
        }
    }

    private bool TryMe(out WebSession me, out ApiError error)
    {
        var s = _sessions.Current;
        if (s is null || string.IsNullOrWhiteSpace(s.Token))
        {
            me = null!;
            error = ApiError.Unauthorized("Апп-д нэвтрээгүй байна.");
            return false;
        }
        me = s;
        error = default!;
        return true;
    }

    private static async Task<ApiError> MapErrorAsync(HttpResponseMessage resp, CancellationToken ct)
    {
        string? body = null;
        try { body = await resp.Content.ReadAsStringAsync(ct).ConfigureAwait(false); } catch { }
        return resp.StatusCode switch
        {
            HttpStatusCode.NotFound      => ApiError.Internal("guardian_backend_not_available"),
            HttpStatusCode.Unauthorized  => ApiError.Unauthorized("unauthenticated", body),
            HttpStatusCode.BadRequest    => ApiError.BadRequest("invalid_body", body),
            _ when (int)resp.StatusCode >= 500 => ApiError.Server($"server_{(int)resp.StatusCode}", body),
            _ => ApiError.Internal($"unexpected_{(int)resp.StatusCode}", body),
        };
    }

    // ── Wire DTOs ──
    private sealed record SessionAuthBody(
        [property: JsonPropertyName("sessionId")] string SessionId,
        [property: JsonPropertyName("pollToken")] string PollToken);

    private sealed record ChildInitBody(
        [property: JsonPropertyName("personId")] string PersonId,
        [property: JsonPropertyName("childRegNo")] string ChildRegNo);

    private sealed record ChildCodeBody(
        [property: JsonPropertyName("sessionId")] string SessionId);

    private sealed record ChildRevokeBody(
        [property: JsonPropertyName("personId")] string PersonId,
        [property: JsonPropertyName("childRegNo")] string ChildRegNo);

    private sealed record ChildrenDto([property: JsonPropertyName("children")] List<ChildDto>? Children);

    private sealed record ChildDto(
        [property: JsonPropertyName("etsi")] string? Etsi,
        [property: JsonPropertyName("regNo")] string? RegNo,
        [property: JsonPropertyName("name")] string? Name,
        [property: JsonPropertyName("birthDate")] string? BirthDate,
        [property: JsonPropertyName("registered")] bool Registered,
        [property: JsonPropertyName("certNotAfter")] DateTimeOffset? CertNotAfter);

    private sealed record ChildInitDto(
        [property: JsonPropertyName("sessionId")] string? SessionId,
        [property: JsonPropertyName("verificationCode")] string? VerificationCode,
        [property: JsonPropertyName("childName")] string? ChildName,
        [property: JsonPropertyName("childRegNo")] string? ChildRegNo);

    private sealed record ChildCodeDto(
        [property: JsonPropertyName("enrollCode")] string? EnrollCode,
        [property: JsonPropertyName("expiresInSeconds")] int ExpiresInSeconds);
}
