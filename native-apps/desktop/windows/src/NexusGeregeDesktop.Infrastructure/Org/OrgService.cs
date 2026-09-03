using System.Net;
using System.Net.Http.Json;
using System.Text.Json;
using System.Text.Json.Serialization;
using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Domain.Org;
using NexusGeregeDesktop.Domain.Primitives;
using Microsoft.Extensions.Logging;

namespace NexusGeregeDesktop.Infrastructure.Org;

/// First-party organization service.
///
/// `ListAsync` is **real**: it fetches the logged-in citizen's active
/// organization representations from the WEB backend's public route
/// `POST /api/representations {personId}` (identity from ISessionIdentityCache;
/// the route proxies to /v3/organization/representations with the RP secret).
///
/// The remaining operations (lookup / register / member management / name-en /
/// X-Road CSR) have **no** first-party `/api/*` equivalent — they were
/// bearer-authenticated org-WRITE calls on the gerege backend — so they stay
/// explicit stubs and the corresponding UI stays disabled.
public sealed class OrgService : IOrgService
{
    private static readonly JsonSerializerOptions JsonOptions = new(JsonSerializerDefaults.Web);

    private readonly HttpClient _http;
    private readonly IWebSessionStore _sessions;
    private readonly ILogger<OrgService> _logger;

    public OrgService(HttpClient http, IWebSessionStore sessions, ILogger<OrgService> logger)
    {
        _http = http;
        _sessions = sessions;
        _logger = logger;
    }

    public async Task<Result<IReadOnlyList<Organization>>> ListAsync(CancellationToken ct = default)
    {
        var me = _sessions.Current;
        if (me is null || string.IsNullOrWhiteSpace(me.Token))
        {
            return Result<IReadOnlyList<Organization>>.Failure(ApiError.Unauthorized("No active session."));
        }

        try
        {
            using var resp = await _http
                .PostAsJsonAsync("/api/representations", new SessionAuthBody(me.Token, me.PollToken), JsonOptions, ct)
                .ConfigureAwait(false);

            // Диагностик: pollToken байгаа эсэх + HTTP статус. 401 бол session-bound
            // auth амжилтгүй (ихэвчлэн pollToken хоосон = хуучин/сэргээгдсэн session).
            _logger.LogInformation("Org representations: sid={HasSid} pollToken={HasPoll} status={Status}",
                !string.IsNullOrEmpty(me.Token), !string.IsNullOrEmpty(me.PollToken), (int)resp.StatusCode);

            if (!resp.IsSuccessStatusCode)
            {
                return Result<IReadOnlyList<Organization>>.Failure(await MapErrorAsync(resp, ct).ConfigureAwait(false));
            }

            var dto = await resp.Content.ReadFromJsonAsync<RepsDto>(JsonOptions, ct).ConfigureAwait(false);
            var reps = dto?.Representations ?? new List<RepDto>();
            _logger.LogInformation("Org representations: {Count} байгууллага олдлоо", reps.Count);
            IReadOnlyList<Organization> orgs = reps.Select(r => new Organization(
                DeterministicGuid(!string.IsNullOrEmpty(r.OrgRegister) ? r.OrgRegister! : r.OrgEtsi ?? string.Empty),
                r.OrgRegister ?? string.Empty,
                r.OrgName ?? string.Empty,
                r.OrgNameEn,
                Phone: null,
                DanVerified: string.Equals(r.Source, "REGISTRY", StringComparison.OrdinalIgnoreCase),
                Status: OrgStatus.Active,
                CreatedAt: DateTimeOffset.UtcNow)).ToList();

            return Result<IReadOnlyList<Organization>>.Success(orgs);
        }
        catch (OperationCanceledException) when (ct.IsCancellationRequested)
        {
            return Result<IReadOnlyList<Organization>>.Failure(ApiError.Cancelled("Org list cancelled."));
        }
        catch (HttpRequestException ex)
        {
            _logger.LogWarning(ex, "Org list network error");
            return Result<IReadOnlyList<Organization>>.Failure(ApiError.Network("Org list failed.", ex.Message));
        }
    }

    // ── No first-party `/api/*` equivalent → explicit stubs ──────────────────

    public Task<Result<OrgLookupResult>> LookupAsync(string regNumber, CancellationToken ct = default)
        => NotAvailable<OrgLookupResult>();

    public Task<Result<OrgRegisterSession>> RegisterAsync(string regNumber, OrgExpiryClass expiry, CancellationToken ct = default)
        => NotAvailable<OrgRegisterSession>();

    public Task<Result<Organization>> FinalizeRegisterAsync(Guid sessionId, CancellationToken ct = default)
        => NotAvailable<Organization>();

    public Task<Result<IReadOnlyList<OrgMember>>> ListMembersAsync(Guid orgId, CancellationToken ct = default)
        => NotAvailable<IReadOnlyList<OrgMember>>();

    public Task<Result<OrgMember>> AddMemberAsync(Guid orgId, string nationalId, OrgRole role, CancellationToken ct = default)
        => NotAvailable<OrgMember>();

    public Task<Result> RemoveMemberAsync(Guid orgId, Guid userId, CancellationToken ct = default)
        => NotAvailableVoid();

    public Task<Result> UpdateEnglishNameAsync(Guid orgId, string nameEn, CancellationToken ct = default)
        => NotAvailableVoid();

    public Task<Result<OrgXroadCert>> SignXroadCsrAsync(Guid orgId, string csrPem, string profile, CancellationToken ct = default)
        => NotAvailable<OrgXroadCert>();

    private static Task<Result<T>> NotAvailable<T>()
        => Task.FromResult(Result<T>.Failure(ApiError.Internal("not_available_on_first_party_backend")));

    private static Task<Result> NotAvailableVoid()
        => Task.FromResult(Result.Failure(ApiError.Internal("not_available_on_first_party_backend")));

    /// Representations carry no server-side Guid; synthesize a stable one from the
    /// registration number so list rows have consistent keys. (Member/detail ops
    /// that need a real org id are stubbed, so this id is display-only.) SHA-256
    /// (not MD5) purely to keep the crypto analyzer CA5351 happy — this is an
    /// identifier hash, not a security primitive.
    private static Guid DeterministicGuid(string s)
    {
        var hash = System.Security.Cryptography.SHA256.HashData(System.Text.Encoding.UTF8.GetBytes(s));
        return new Guid(hash.AsSpan(0, 16));
    }

    private static async Task<ApiError> MapErrorAsync(HttpResponseMessage resp, CancellationToken ct)
    {
        var body = await resp.Content.ReadAsStringAsync(ct).ConfigureAwait(false);
        return resp.StatusCode switch
        {
            HttpStatusCode.Unauthorized       => ApiError.Unauthorized("unauthenticated", body),
            (HttpStatusCode)429               => ApiError.BadRequest("rate_limited", body),
            _ when (int)resp.StatusCode >= 500 => ApiError.Server($"server_{(int)resp.StatusCode}", body),
            _ => ApiError.Internal($"unexpected_{(int)resp.StatusCode}", body),
        };
    }

    private sealed record SessionAuthBody(
        [property: JsonPropertyName("sessionId")] string SessionId,
        [property: JsonPropertyName("pollToken")] string PollToken);

    private sealed record RepsDto([property: JsonPropertyName("representations")] List<RepDto>? Representations);

    private sealed record RepDto(
        [property: JsonPropertyName("orgEtsi")] string? OrgEtsi,
        [property: JsonPropertyName("orgRegister")] string? OrgRegister,
        [property: JsonPropertyName("orgName")] string? OrgName,
        [property: JsonPropertyName("orgNameEn")] string? OrgNameEn,
        [property: JsonPropertyName("role")] string? Role,
        [property: JsonPropertyName("rightType")] string? RightType,
        [property: JsonPropertyName("source")] string? Source);
}
