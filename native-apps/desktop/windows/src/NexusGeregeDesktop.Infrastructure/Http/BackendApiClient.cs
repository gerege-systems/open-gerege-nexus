using System.Net;
using System.Net.Http.Json;
using System.Text.Json;
using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Domain.Primitives;
using Microsoft.Extensions.Logging;

namespace NexusGeregeDesktop.Infrastructure.Http;

public sealed class BackendApiClient : IBackendApi
{
    private static readonly JsonSerializerOptions JsonOptions = new(JsonSerializerDefaults.Web);

    private readonly HttpClient _http;
    private readonly ILogger<BackendApiClient> _logger;

    public BackendApiClient(HttpClient http, ILogger<BackendApiClient> logger)
    {
        _http = http;
        _logger = logger;
    }

    public async Task<Result<HealthStatus>> GetHealthAsync(CancellationToken ct = default)
    {
        try
        {
            // First-party model: the web backend exposes a single public
            // `GET /api/health` → {status:"ok", service:"…"}. There is no
            // separate live/ready split, so both mirror the one 200/!200.
            using var resp = await _http.GetAsync("/api/health", ct).ConfigureAwait(false);

            var ok = resp.IsSuccessStatusCode;

            string? version = null;
            if (ok)
            {
                try
                {
                    var payload = await resp.Content
                        .ReadFromJsonAsync<HealthPayload>(JsonOptions, ct)
                        .ConfigureAwait(false);
                    // /api/health has no version field; surface the service name.
                    version = payload?.Version ?? payload?.Service;
                }
                catch (JsonException)
                {
                    // Backend may return text/plain; version stays null.
                }
            }

            return Result<HealthStatus>.Success(new HealthStatus(ok, ok, version));
        }
        catch (OperationCanceledException) when (ct.IsCancellationRequested)
        {
            return Result<HealthStatus>.Failure(ApiError.Cancelled("Health check cancelled."));
        }
        catch (TaskCanceledException ex)
        {
            return Result<HealthStatus>.Failure(ApiError.Timeout("Health check timed out.", ex.Message));
        }
        catch (HttpRequestException ex) when (ex.StatusCode is HttpStatusCode.Unauthorized)
        {
            return Result<HealthStatus>.Failure(ApiError.Unauthorized("Health endpoint refused auth.", ex.Message));
        }
        catch (HttpRequestException ex)
        {
            _logger.LogWarning(ex, "Backend health request failed");
            return Result<HealthStatus>.Failure(ApiError.Network("Health check network error.", ex.Message));
        }
    }

    private sealed record HealthPayload(string? Status, string? Version, string? Service);
}
