using System.Net;
using System.Net.Http.Json;
using System.Text.Json;
using System.Text.Json.Serialization;
using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Domain.Primitives;
using Microsoft.Extensions.Logging;

namespace NexusGeregeDesktop.Infrastructure.Dashboard;

/// First-party citizen dashboard.
///
/// Aggregates the WEB backend's public person-PKI `/api/*` routes for the
/// logged-in citizen. The DISPLAY identity comes from
/// <see cref="ISessionIdentityCache"/> (populated by CitizenAuthService at
/// login-complete); the BACKEND calls are session-bound — they carry the login
/// session's {sessionId, pollToken} and never a client-supplied personId
/// (broken-access-control fix, see BACKEND-INTEGRATION.md):
///   POST /api/dashboard {sessionId, pollToken} → certificate/activity counts
///   POST /api/devices   {sessionId, pollToken} → enrolled devices
///   POST /api/activity  {sessionId, pollToken} → RP-scoped session history
/// The web routes proxy to the Go RP-API with the first-party RP secret.
public sealed class DashboardService : IDashboardService
{
    private static readonly JsonSerializerOptions JsonOptions = new(JsonSerializerDefaults.Web);

    private readonly HttpClient _http;
    private readonly ISessionIdentityCache _identity;
    private readonly IWebSessionStore _sessions;
    private readonly ILogger<DashboardService> _logger;

    public DashboardService(HttpClient http, ISessionIdentityCache identity, IWebSessionStore sessions, ILogger<DashboardService> logger)
    {
        _http = http;
        _identity = identity;
        _sessions = sessions;
        _logger = logger;
    }

    public async Task<Result<DashboardSnapshot>> LoadAsync(CancellationToken ct = default)
    {
        var me = _identity.Current;
        if (me is null || string.IsNullOrWhiteSpace(me.NationalId))
        {
            return Result<DashboardSnapshot>.Failure(ApiError.Unauthorized("No active session."));
        }
        // Session-bound auth: identity display нь _identity-ээс, харин backend
        // хандалт нь login session-ий {sessionId, pollToken}-оор.
        var s = _sessions.Current;
        if (s is null || string.IsNullOrWhiteSpace(s.Token))
        {
            return Result<DashboardSnapshot>.Failure(ApiError.Unauthorized("No active session."));
        }
        var pid = new SessionAuthBody(s.Token, s.PollToken);

        SummaryDto? summary;
        try
        {
            using var resp = await _http.PostAsJsonAsync("/api/dashboard", pid, JsonOptions, ct).ConfigureAwait(false);
            if (!resp.IsSuccessStatusCode)
            {
                return Result<DashboardSnapshot>.Failure(await MapErrorAsync(resp, ct).ConfigureAwait(false));
            }
            summary = await resp.Content.ReadFromJsonAsync<SummaryDto>(JsonOptions, ct).ConfigureAwait(false);
        }
        catch (OperationCanceledException) when (ct.IsCancellationRequested)
        {
            return Result<DashboardSnapshot>.Failure(ApiError.Cancelled("Dashboard load cancelled."));
        }
        catch (HttpRequestException ex)
        {
            _logger.LogWarning(ex, "Dashboard network error");
            return Result<DashboardSnapshot>.Failure(ApiError.Network("Dashboard load failed.", ex.Message));
        }

        if (summary is null)
        {
            return Result<DashboardSnapshot>.Failure(ApiError.Internal("Empty dashboard body."));
        }

        // Devices + activity are best-effort — a failure there must not blank the whole dashboard.
        var devices = await TryDevicesAsync(pid, ct).ConfigureAwait(false);
        var sessions = await TryActivityAsync(pid, ct).ConfigureAwait(false);

        var user = new DashboardUser(
            me.NationalId, me.FullName, me.FullNameLatin, me.KycLevel, me.Status, me.CreatedAt);

        return Result<DashboardSnapshot>.Success(new DashboardSnapshot(
            user,
            devices,
            sessions,
            Certificates: summary.Certificates?.Total ?? 0,
            TotalLogins: summary.Activity?.Authentication ?? 0));
    }

    private async Task<IReadOnlyList<DashboardDevice>> TryDevicesAsync(SessionAuthBody pid, CancellationToken ct)
    {
        try
        {
            using var resp = await _http.PostAsJsonAsync("/api/devices", pid, JsonOptions, ct).ConfigureAwait(false);
            if (!resp.IsSuccessStatusCode) return Array.Empty<DashboardDevice>();
            var dto = await resp.Content.ReadFromJsonAsync<DevicesDto>(JsonOptions, ct).ConfigureAwait(false);
            var list = dto?.Devices ?? new List<DeviceDto>();
            return list.Select(d => new DashboardDevice(
                Guid.TryParse(d.DocumentNumber, out var g) ? g : Guid.Empty,
                d.Platform ?? string.Empty,
                d.Active ? "ACTIVE" : "INACTIVE",
                LastUsedAt: null,
                CreatedAt: d.EnrolledAt)).ToList();
        }
        catch (Exception ex) when (ex is HttpRequestException or OperationCanceledException or JsonException)
        {
            return Array.Empty<DashboardDevice>();
        }
    }

    private async Task<IReadOnlyList<DashboardActivity>> TryActivityAsync(SessionAuthBody pid, CancellationToken ct)
    {
        try
        {
            using var resp = await _http.PostAsJsonAsync("/api/activity", pid, JsonOptions, ct).ConfigureAwait(false);
            if (!resp.IsSuccessStatusCode) return Array.Empty<DashboardActivity>();
            var dto = await resp.Content.ReadFromJsonAsync<ActivityDto>(JsonOptions, ct).ConfigureAwait(false);
            var list = dto?.Sessions ?? new List<ActivityItemDto>();
            return list.Select(s => new DashboardActivity(
                Guid.TryParse(s.SessionId, out var g) ? g : Guid.Empty,
                s.Flow ?? string.Empty,
                "COMPLETE",
                s.Outcome ?? string.Empty,
                RpName: s.RpName ?? string.Empty,
                PushText: s.DocText,
                CreatedAt: s.Timestamp,
                SubsystemName: s.SubsystemName ?? string.Empty,
                InteractionType: s.InteractionType ?? string.Empty,
                DocumentNumber: s.DocumentNumber ?? string.Empty,
                ClientIp: s.ClientIp ?? string.Empty)).ToList();
        }
        catch (Exception ex) when (ex is HttpRequestException or OperationCanceledException or JsonException)
        {
            return Array.Empty<DashboardActivity>();
        }
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

    private sealed record SessionAuthBody([property: JsonPropertyName("sessionId")] string SessionId, [property: JsonPropertyName("pollToken")] string PollToken);

    private sealed record CountsDto([property: JsonPropertyName("total")] int Total);

    private sealed record ActivityCountsDto(
        [property: JsonPropertyName("authentication")] int Authentication,
        [property: JsonPropertyName("signature")] int Signature);

    private sealed record SummaryDto(
        [property: JsonPropertyName("certificates")] CountsDto? Certificates,
        [property: JsonPropertyName("activity")] ActivityCountsDto? Activity);

    private sealed record DevicesDto([property: JsonPropertyName("devices")] List<DeviceDto>? Devices);

    private sealed record DeviceDto(
        [property: JsonPropertyName("documentNumber")] string DocumentNumber,
        [property: JsonPropertyName("platform")] string? Platform,
        [property: JsonPropertyName("enrolledAt")] DateTimeOffset EnrolledAt,
        [property: JsonPropertyName("active")] bool Active);

    private sealed record ActivityDto([property: JsonPropertyName("sessions")] List<ActivityItemDto>? Sessions);

    private sealed record ActivityItemDto(
        [property: JsonPropertyName("sessionId")] string? SessionId,
        [property: JsonPropertyName("flow")] string? Flow,
        [property: JsonPropertyName("outcome")] string? Outcome,
        [property: JsonPropertyName("docText")] string? DocText,
        [property: JsonPropertyName("timestamp")] DateTimeOffset Timestamp,
        [property: JsonPropertyName("rpName")] string? RpName = null,
        [property: JsonPropertyName("subsystemName")] string? SubsystemName = null,
        [property: JsonPropertyName("interactionType")] string? InteractionType = null,
        [property: JsonPropertyName("documentNumber")] string? DocumentNumber = null,
        [property: JsonPropertyName("clientIp")] string? ClientIp = null);
}
