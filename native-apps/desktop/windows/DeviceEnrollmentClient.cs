using System.Net.Http;
using System.Net.Http.Headers;
using System.Net.Http.Json;
using System.Text.Json.Serialization;
using System;
using System.Threading.Tasks;
using System.Collections.Generic;

namespace GeregeNexusNativeWin;

public sealed record EnrolledDevice([property: JsonPropertyName("device_id")] string DeviceId, [property: JsonPropertyName("tenant_id")] string TenantId, [property: JsonPropertyName("device_token")] string DeviceToken);
public sealed record DeviceIdentity(string Id, [property: JsonPropertyName("tenant_id")] string TenantId, string Name, string Platform, [property: JsonPropertyName("form_factor")] string FormFactor, string Status);

public sealed class DeviceEnrollmentClient
{
    private readonly HttpClient http = new();
    private static Uri Endpoint(string apiEndpoint, string path)
    {
        var root = apiEndpoint.TrimEnd('/');
        if (!root.EndsWith("/api/v1", StringComparison.OrdinalIgnoreCase)) root += "/api/v1";
        return new Uri($"{root}/{path}");
    }
    public async Task<EnrolledDevice> EnrollAsync(string endpoint, string code, string name, string site)
    {
        using var response = await http.PostAsJsonAsync(Endpoint(endpoint, "devices/enroll"), new { code, name, site, platform = "windows", form_factor = ShellProfile.FormFactor, app_version = "dev", os_version = Environment.OSVersion.ToString() });
        await EnsureSuccess(response);
        return (await response.Content.ReadFromJsonAsync<EnrolledDevice>())!;
    }
    public async Task<DeviceIdentity> IdentityAsync(string endpoint, string token)
    {
        using var request = new HttpRequestMessage(HttpMethod.Get, Endpoint(endpoint, "devices/me"));
        request.Headers.Authorization = new AuthenticationHeaderValue("Device", token);
        using var response = await http.SendAsync(request);
        await EnsureSuccess(response);
        return (await response.Content.ReadFromJsonAsync<DeviceIdentity>())!;
    }
    public async Task TelemetryAsync(string endpoint, string token, string level, string eventName, object? payload = null)
    {
        using var request = new HttpRequestMessage(HttpMethod.Post, Endpoint(endpoint, "devices/telemetry"));
        request.Headers.Authorization = new AuthenticationHeaderValue("Device", token);
        var item = new Dictionary<string, object>
        {
            ["level"] = level,
            ["event"] = eventName,
            ["payload"] = payload ?? new Dictionary<string, object>(),
            ["occurred_at"] = DateTimeOffset.UtcNow
        };
        request.Content = JsonContent.Create(new Dictionary<string, object> { ["events"] = new[] { item } });
        using var response = await http.SendAsync(request);
        await EnsureSuccess(response);
    }

    public async Task<string> RotateTokenAsync(string endpoint, string token)
    {
        using var request = new HttpRequestMessage(HttpMethod.Post, Endpoint(endpoint, "devices/token/rotate"));
        request.Headers.Authorization = new AuthenticationHeaderValue("Device", token);
        request.Content = JsonContent.Create(new Dictionary<string, object>());
        using var response = await http.SendAsync(request);
        await EnsureSuccess(response);
        var value = await response.Content.ReadFromJsonAsync<Dictionary<string, string>>();
        return value!["device_token"];
    }
    private static async Task EnsureSuccess(HttpResponseMessage response)
    {
        if (response.IsSuccessStatusCode) return;
        var body = await response.Content.ReadAsStringAsync();
        throw new HttpRequestException(string.IsNullOrWhiteSpace(body) ? $"HTTP {(int)response.StatusCode}" : body);
    }
}
