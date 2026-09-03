using System.Net;
using System.Net.Security;
using System.Net.Sockets;
using System.Security.Cryptography;
using System.Security.Cryptography.X509Certificates;
using System.Text;
using Microsoft.Extensions.Logging;

namespace NexusGeregeDesktop.Infrastructure.Esign;

/// <summary>
/// Түүхий (RFC6455) WebSocket сервер — ws БОЛОН wss (SslStream). ESIGNClient-ийн
/// 59001(ws)/59005(wss) протоколын эквивалент. Нэг хүсэлт/хариугаар ажиллана.
/// </summary>
public sealed class EsignWebSocketServer
{
    private const string GUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11";
    private readonly int _port;
    private readonly X509Certificate2? _tls;
    private readonly Func<string, CancellationToken, Task<string>> _handler;
    private readonly ILogger _log;

    public EsignWebSocketServer(int port, X509Certificate2? tls, Func<string, CancellationToken, Task<string>> handler, ILogger log)
    {
        _port = port; _tls = tls; _handler = handler; _log = log;
    }

    public string Scheme => _tls is null ? "ws" : "wss";

    public async Task RunAsync(CancellationToken ct)
    {
        var listener = new TcpListener(IPAddress.Loopback, _port);
        listener.Start();
        _log.LogInformation("ESIGN сонсож байна: {Scheme}://127.0.0.1:{Port}/", Scheme, _port);
        using var reg = ct.Register(() => { try { listener.Stop(); } catch { } });
        try
        {
            while (!ct.IsCancellationRequested)
            {
                TcpClient client;
                try { client = await listener.AcceptTcpClientAsync(ct).ConfigureAwait(false); }
                catch (OperationCanceledException) { break; }
                catch (SocketException) { break; }
                _ = HandleAsync(client, ct);
            }
        }
        finally { listener.Stop(); }
    }

    private async Task HandleAsync(TcpClient client, CancellationToken ct)
    {
        using (client)
        {
            try
            {
                Stream stream = client.GetStream();
                if (_tls is not null)
                {
                    var ssl = new SslStream(stream, false);
                    await ssl.AuthenticateAsServerAsync(_tls, false, false).ConfigureAwait(false);
                    stream = ssl;
                }
                if (!await DoHandshakeAsync(stream, ct).ConfigureAwait(false)) return;
                string? request = await ReadTextAsync(stream, ct).ConfigureAwait(false);
                if (request is null) return;
                string response = await _handler(request, ct).ConfigureAwait(false);
                await WriteTextAsync(stream, response, ct).ConfigureAwait(false);
                try { await stream.WriteAsync(new byte[] { 0x88, 0x00 }, ct).ConfigureAwait(false); } catch { }
            }
            catch (Exception ex) { _log.LogDebug(ex, "ESIGN ws client алдаа"); }
        }
    }

    private static async Task<bool> DoHandshakeAsync(Stream s, CancellationToken ct)
    {
        var buf = new byte[8192]; int total = 0; string head = "";
        while (total < buf.Length)
        {
            int n = await s.ReadAsync(buf.AsMemory(total, buf.Length - total), ct).ConfigureAwait(false);
            if (n == 0) return false;
            total += n;
            head = Encoding.ASCII.GetString(buf, 0, total);
            if (head.Contains("\r\n\r\n")) break;
        }
        string? key = null;
        foreach (var line in head.Split("\r\n"))
        {
            int c = line.IndexOf(':');
            if (c > 0 && line[..c].Trim().Equals("Sec-WebSocket-Key", StringComparison.OrdinalIgnoreCase))
                key = line[(c + 1)..].Trim();
        }
        if (key is null) return false;
        string accept = Convert.ToBase64String(SHA1.HashData(Encoding.ASCII.GetBytes(key + GUID)));
        byte[] resp = Encoding.ASCII.GetBytes(
            "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n" +
            $"Sec-WebSocket-Accept: {accept}\r\n\r\n");
        await s.WriteAsync(resp, ct).ConfigureAwait(false);
        return true;
    }

    private static async Task<string?> ReadTextAsync(Stream s, CancellationToken ct)
    {
        var payload = new List<byte>();
        while (true)
        {
            byte[]? h2 = await ReadExact(s, 2, ct).ConfigureAwait(false);
            if (h2 is null) return null;
            bool fin = (h2[0] & 0x80) != 0;
            int opcode = h2[0] & 0x0f;
            bool masked = (h2[1] & 0x80) != 0;
            long len = h2[1] & 0x7f;
            if (len == 126) { var e = await ReadExact(s, 2, ct).ConfigureAwait(false); if (e is null) return null; len = (e[0] << 8) | e[1]; }
            else if (len == 127) { var e = await ReadExact(s, 8, ct).ConfigureAwait(false); if (e is null) return null; len = 0; for (int i = 0; i < 8; i++) len = (len << 8) | e[i]; }
            byte[] mask = Array.Empty<byte>();
            if (masked) { mask = await ReadExact(s, 4, ct).ConfigureAwait(false) ?? Array.Empty<byte>(); if (mask.Length < 4) return null; }
            byte[] data = Array.Empty<byte>();
            if (len > 0) { data = await ReadExact(s, (int)len, ct).ConfigureAwait(false) ?? Array.Empty<byte>(); if (data.Length < len) return null; }
            if (masked) for (int i = 0; i < data.Length; i++) data[i] ^= mask[i & 3];
            if (opcode == 0x8) return null;
            payload.AddRange(data);
            if (fin) break;
        }
        return Encoding.UTF8.GetString(payload.ToArray());
    }

    private static async Task<byte[]?> ReadExact(Stream s, int n, CancellationToken ct)
    {
        var b = new byte[n]; int off = 0;
        while (off < n) { int r = await s.ReadAsync(b.AsMemory(off, n - off), ct).ConfigureAwait(false); if (r == 0) return null; off += r; }
        return b;
    }

    private static async Task WriteTextAsync(Stream s, string text, CancellationToken ct)
    {
        byte[] payload = Encoding.UTF8.GetBytes(text);
        var header = new List<byte> { 0x81 };
        if (payload.Length < 126) header.Add((byte)payload.Length);
        else if (payload.Length < 65536) { header.Add(126); header.Add((byte)(payload.Length >> 8)); header.Add((byte)payload.Length); }
        else { header.Add(127); for (int i = 7; i >= 0; i--) header.Add((byte)(payload.Length >> (8 * i))); }
        await s.WriteAsync(header.ToArray(), ct).ConfigureAwait(false);
        await s.WriteAsync(payload, ct).ConfigureAwait(false);
    }

    /// <summary>wss-д зориулж 127.0.0.1 self-signed cert үүсгэх.</summary>
    public static X509Certificate2 CreateLocalhostCert()
    {
        using var rsa = RSA.Create(2048);
        var req = new CertificateRequest("CN=127.0.0.1", rsa, HashAlgorithmName.SHA256, RSASignaturePadding.Pkcs1);
        var san = new SubjectAlternativeNameBuilder();
        san.AddIpAddress(IPAddress.Loopback);
        san.AddDnsName("localhost");
        req.CertificateExtensions.Add(san.Build());
        var cert = req.CreateSelfSigned(DateTimeOffset.UtcNow.AddDays(-1), DateTimeOffset.UtcNow.AddYears(2));
        return new X509Certificate2(cert.Export(X509ContentType.Pfx));
    }
}
