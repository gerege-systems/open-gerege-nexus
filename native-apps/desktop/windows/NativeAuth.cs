using System;
using System.Net;
using System.Net.Http;
using System.Net.Http.Json;
using System.Text.Json.Serialization;
using System.Threading;
using System.Threading.Tasks;

namespace GeregeNexusNativeWin;

public enum LoginPhase { Idle, Starting, Waiting, Success, Expired, Refused, Error }
public sealed record LoginStatus(LoginPhase Phase, string Message = "");
internal sealed record EIDStart(
    [property: JsonPropertyName("session_id")] string SessionId,
    [property: JsonPropertyName("verification_code")] string VerificationCode,
    [property: JsonPropertyName("expires_at")] DateTimeOffset? ExpiresAt,
    [property: JsonPropertyName("device_link_url")] string? DeviceLinkUrl);
internal sealed record EIDPoll([property: JsonPropertyName("state")] string State);

public sealed class NativeAuth : IDisposable
{
    private readonly Uri _apiBase;
    private readonly CookieContainer _cookies = new();
    private readonly HttpClient _http;
    private CancellationTokenSource? _attempt;
    private long _ticket;
    public event Action<LoginStatus>? StatusChanged;
    public string? LastDeviceLinkUrl { get; private set; }

    public NativeAuth(string apiEndpoint)
    {
        var root = apiEndpoint.TrimEnd('/'); if (!root.EndsWith("/api/v1", StringComparison.OrdinalIgnoreCase)) root += "/api/v1";
        _apiBase = new Uri(root + "/");
        _http = new HttpClient(new HttpClientHandler { CookieContainer = _cookies, UseCookies = true }) { BaseAddress = _apiBase };
        _http.DefaultRequestHeaders.AcceptLanguage.ParseAdd("mn");
        _http.DefaultRequestHeaders.Add("Origin", _apiBase.GetLeftPart(UriPartial.Authority));
    }

    public CookieCollection SessionCookies => _cookies.GetCookies(_apiBase);

    public void Cancel()
    {
        Interlocked.Increment(ref _ticket);
        _attempt?.Cancel(); _attempt?.Dispose(); _attempt = null;
        Publish(LoginPhase.Idle);
    }

    public Task PasswordAsync(string email, string password) => BeginAsync(async (ticket, token) =>
    {
        await PostAsync("auth/login", new { email, password }, token);
        if (ticket == _ticket) Publish(LoginPhase.Success);
    });

    public Task StaffPinAsync(string pin, string deviceToken) => BeginAsync(async (ticket, token) =>
    {
        using var request = new HttpRequestMessage(HttpMethod.Post, "devices/staff/pin") { Content = JsonContent.Create(new { pin }) };
        request.Headers.Authorization = new System.Net.Http.Headers.AuthenticationHeaderValue("Device", deviceToken);
        using var response = await _http.SendAsync(request, token);
        if (!response.IsSuccessStatusCode) throw new HttpRequestException($"PIN нэвтрэлт амжилтгүй: HTTP {(int)response.StatusCode}");
        if (ticket == _ticket) Publish(LoginPhase.Success);
    });

    public Task PushAsync(string nationalId) => BeginAsync(async (ticket, token) =>
    {
        var started = await PostAsync<EIDStart>("auth/eid/start-id", new {
            national_id = nationalId.Trim().ToUpperInvariant(), callbackUrl = ""
        }, token);
        if (ticket != _ticket) return;
        Publish(LoginPhase.Waiting, $"eID апп дээр зөвшөөрнө үү. Код: {started.VerificationCode}");
        await PollAsync(started, ticket, token);
    });

    public Task QrAsync() => BeginAsync(async (ticket, token) =>
    {
        var started = await PostAsync<EIDStart>("auth/eid/start", new { callbackUrl = "" }, token);
        LastDeviceLinkUrl = started.DeviceLinkUrl;
        if (ticket != _ticket) return;
        Publish(LoginPhase.Waiting, $"eID апп-аар QR уншуулна уу. Код: {started.VerificationCode}");
        await PollAsync(started, ticket, token);
    });

    private async Task BeginAsync(Func<long, CancellationToken, Task> operation)
    {
        Cancel();
        var ticket = Interlocked.Increment(ref _ticket);
        _attempt = new CancellationTokenSource();
        Publish(LoginPhase.Starting, "Хүсэлт эхлүүлж байна…");
        try { await operation(ticket, _attempt.Token); }
        catch (OperationCanceledException) { }
        catch (Exception error) when (ticket == _ticket) { Publish(LoginPhase.Error, error.Message); }
    }

    private async Task PollAsync(EIDStart start, long ticket, CancellationToken token)
    {
        var deadline = start.ExpiresAt ?? DateTimeOffset.UtcNow.AddMinutes(15);
        var failures = 0;
        while (ticket == _ticket && !token.IsCancellationRequested)
        {
            if (DateTimeOffset.UtcNow >= deadline) { Publish(LoginPhase.Expired, "Хугацаа дууслаа"); return; }
            try
            {
                var result = await PostAsync<EIDPoll>("auth/eid/poll", new { session_id = start.SessionId }, token);
                failures = 0;
                switch (result.State.ToUpperInvariant())
                {
                    case "COMPLETE": Publish(LoginPhase.Success); return;
                    case "EXPIRED": Publish(LoginPhase.Expired, "Хугацаа дууслаа"); return;
                    case "REFUSED": Publish(LoginPhase.Refused, "Хүсэлтээс татгалзлаа"); return;
                }
            }
            catch (OperationCanceledException) { throw; }
            catch when (++failures <= 3) { }
            await Task.Delay(400, token);
        }
    }

    private async Task PostAsync(string path, object body, CancellationToken token) =>
        _ = await PostAsync<object>(path, body, token);

    private async Task<T> PostAsync<T>(string path, object body, CancellationToken token)
    {
        using var response = await _http.PostAsJsonAsync(path, body, token);
        if (!response.IsSuccessStatusCode)
        {
            var error = await response.Content.ReadFromJsonAsync<ApiError>(cancellationToken: token);
            throw new HttpRequestException(error?.Error ?? $"HTTP {(int)response.StatusCode}");
        }
        return (await response.Content.ReadFromJsonAsync<T>(cancellationToken: token))!;
    }

    private void Publish(LoginPhase phase, string message = "") => StatusChanged?.Invoke(new LoginStatus(phase, message));
    public void Dispose() { Cancel(); _http.Dispose(); }
    private sealed record ApiError([property: JsonPropertyName("error")] string Error);
}
