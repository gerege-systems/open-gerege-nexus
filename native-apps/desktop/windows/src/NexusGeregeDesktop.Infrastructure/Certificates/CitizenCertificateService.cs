using System.Globalization;
using System.Net;
using System.Net.Http.Json;
using System.Text.Json;
using System.Text.Json.Serialization;
using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Domain.Certificates;
using NexusGeregeDesktop.Domain.Primitives;
using Microsoft.Extensions.Logging;

namespace NexusGeregeDesktop.Infrastructure.Certificates;

/// <summary>
/// Иргэний гэрчилгээний жагсаалт — `POST /api/certificates {sessionId, pollToken}`.
/// Хариу нь жагсаалт + тоолол, дээр нь ESIGN программ токенд зориулсан
/// `signing`/`auth` (b64 DER) талбарууд (эдгээрийг <see cref="Esign.EsignSoftwareToken"/>
/// ашигладаг; энд зөвхөн жагсаалтын хэсгийг уншина).
/// Backend `notBefore`/`notAfter`-ыг МӨРӨӨР (хоосон байж болно) буцаадаг тул
/// DateTimeOffset рүү гараар хөрвүүлнэ — шууд bind хийвэл "" дээр JsonException болно.
/// </summary>
public sealed class CitizenCertificateService : ICitizenCertificateService
{
    private static readonly JsonSerializerOptions JsonOptions = new(JsonSerializerDefaults.Web)
    {
        DefaultIgnoreCondition = JsonIgnoreCondition.WhenWritingNull,
    };

    private readonly HttpClient _http;
    private readonly IWebSessionStore _sessions;
    private readonly ILogger<CitizenCertificateService> _log;

    public CitizenCertificateService(HttpClient http, IWebSessionStore sessions, ILogger<CitizenCertificateService> log)
    {
        _http = http;
        _sessions = sessions;
        _log = log;
    }

    public async Task<Result<CitizenCertificateList>> ListAsync(CancellationToken ct = default)
    {
        var me = _sessions.Current;
        if (me is null || string.IsNullOrWhiteSpace(me.Token))
        {
            return Result<CitizenCertificateList>.Failure(ApiError.Unauthorized("Апп-д нэвтрээгүй байна."));
        }

        CertsDto? dto;
        try
        {
            var body = new SessionAuthBody(me.Token, me.PollToken);
            using var resp = await _http.PostAsJsonAsync("/api/certificates", body, JsonOptions, ct).ConfigureAwait(false);
            if (!resp.IsSuccessStatusCode)
            {
                return Result<CitizenCertificateList>.Failure(await MapErrorAsync(resp, ct).ConfigureAwait(false));
            }
            dto = await resp.Content.ReadFromJsonAsync<CertsDto>(JsonOptions, ct).ConfigureAwait(false);
        }
        catch (OperationCanceledException) when (ct.IsCancellationRequested)
        {
            return Result<CitizenCertificateList>.Failure(ApiError.Cancelled("Цуцлагдав."));
        }
        catch (HttpRequestException ex)
        {
            _log.LogWarning(ex, "Гэрчилгээний жагсаалт: сүлжээний алдаа");
            return Result<CitizenCertificateList>.Failure(ApiError.Network("Гэрчилгээ авахад алдаа.", ex.Message));
        }
        catch (JsonException ex)
        {
            _log.LogWarning(ex, "Гэрчилгээний жагсаалт: хариуг задлах алдаа");
            return Result<CitizenCertificateList>.Failure(ApiError.Internal("Гэрчилгээний хариу буруу форматтай.", ex.Message));
        }

        if (dto is null)
        {
            return Result<CitizenCertificateList>.Failure(ApiError.Internal("Хоосон хариу."));
        }

        var items = (dto.Certificates ?? new List<CertItemDto>())
            .Select(static c => new CitizenCertificate(
                c.DocumentNumber ?? string.Empty,
                c.Type ?? string.Empty,
                c.SerialNumber ?? string.Empty,
                c.CertificateLevel ?? string.Empty,
                c.Status ?? string.Empty,
                ParseDate(c.NotBefore),
                ParseDate(c.NotAfter),
                c.IssuerDn ?? string.Empty,
                c.CertValue ?? string.Empty))
            // Хамгийн шинэ нь дээр — backend дараалалд найдахгүй (pickEsignCerts-тэй ижил зарчим).
            .OrderByDescending(static c => c.NotBefore ?? DateTimeOffset.MinValue)
            .ToList();

        var counts = dto.Counts is { } k
            ? new CitizenCertificateCounts(k.Valid, k.Revoked, k.Expired, k.Suspended, k.Total)
            : new CitizenCertificateCounts(0, 0, 0, 0, items.Count);

        return Result<CitizenCertificateList>.Success(
            new CitizenCertificateList(dto.PersonEtsi ?? string.Empty, counts, items));
    }

    private static DateTimeOffset? ParseDate(string? value) =>
        DateTimeOffset.TryParse(value, CultureInfo.InvariantCulture, DateTimeStyles.AssumeUniversal, out var d)
            ? d
            : null;

    private static async Task<ApiError> MapErrorAsync(HttpResponseMessage resp, CancellationToken ct)
    {
        string? body = null;
        try { body = await resp.Content.ReadAsStringAsync(ct).ConfigureAwait(false); }
        catch (HttpRequestException) { /* хариуны бие уншигдахгүй бол статус л хангалттай */ }
        return resp.StatusCode switch
        {
            HttpStatusCode.NotFound      => ApiError.Internal("certificates_backend_not_available"),
            HttpStatusCode.Unauthorized  => ApiError.Unauthorized("unauthenticated", body),
            (HttpStatusCode)429          => ApiError.BadRequest("rate_limited", body),
            _ when (int)resp.StatusCode >= 500 => ApiError.Server($"server_{(int)resp.StatusCode}", body),
            _ => ApiError.Internal($"unexpected_{(int)resp.StatusCode}", body),
        };
    }

    // ── Wire DTOs ──
    private sealed record SessionAuthBody(
        [property: JsonPropertyName("sessionId")] string SessionId,
        [property: JsonPropertyName("pollToken")] string PollToken);

    private sealed record CertsDto(
        [property: JsonPropertyName("personEtsi")] string? PersonEtsi,
        [property: JsonPropertyName("counts")] CountsDto? Counts,
        [property: JsonPropertyName("certificates")] List<CertItemDto>? Certificates);

    private sealed record CountsDto(
        [property: JsonPropertyName("valid")] int Valid,
        [property: JsonPropertyName("revoked")] int Revoked,
        [property: JsonPropertyName("expired")] int Expired,
        [property: JsonPropertyName("suspended")] int Suspended,
        [property: JsonPropertyName("total")] int Total);

    private sealed record CertItemDto(
        [property: JsonPropertyName("documentNumber")] string? DocumentNumber,
        [property: JsonPropertyName("type")] string? Type,
        [property: JsonPropertyName("serialNumber")] string? SerialNumber,
        [property: JsonPropertyName("certificateLevel")] string? CertificateLevel,
        [property: JsonPropertyName("status")] string? Status,
        [property: JsonPropertyName("notBefore")] string? NotBefore,
        [property: JsonPropertyName("notAfter")] string? NotAfter,
        [property: JsonPropertyName("issuerDn")] string? IssuerDn,
        [property: JsonPropertyName("certValue")] string? CertValue);
}
