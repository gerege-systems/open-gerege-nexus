using System.Security.Cryptography;
using System.Security.Cryptography.X509Certificates;
using System.Text.Json.Nodes;
using NexusGeregeDesktop.Application.Abstractions;
using NexusGeregeDesktop.Domain.Tokens;
using Microsoft.Extensions.Logging;

namespace NexusGeregeDesktop.Infrastructure.Esign;

/// <summary>
/// ESIGN websocket хүсэлтийг боловсруулах гол логик.
///
/// Хоёр горим:
///  • <b>Физик токен</b> — залгаастай USB токен байвал түүн дээрх гэрчилгээгээр
///    (PIN асууж) гарын үсэг зурна.
///  • <b>Программ токен</b> — токен байхгүй бол апп-д нэвтэрсэн иргэний identity-ээр
///    (гар утасны threshold түлхүүр рүү push) гарын үсэг зуруулна.
/// Токен нь программ токеноос эрхэмлэгддэг (байвал түүнийг эхэлж хэрэглэнэ).
/// </summary>
public sealed class EsignBridge
{
    private readonly ITokenRegistry _tokens;
    private readonly IEsignInteraction _ui;
    private readonly IEsignSoftwareToken _soft;
    private readonly IEsignPreferences _prefs;
    private readonly ISensitiveActionGuard _consent;
    private readonly ILogger<EsignBridge> _log;
    private readonly string _expectedType;

    public EsignBridge(
        ITokenRegistry tokens,
        IEsignInteraction ui,
        IEsignSoftwareToken soft,
        IEsignPreferences prefs,
        ISensitiveActionGuard consent,
        ILogger<EsignBridge> log,
        string expectedType = "bb4702f31917793f")
    {
        _tokens = tokens;
        _ui = ui;
        _soft = soft;
        _prefs = prefs;
        _consent = consent;
        _log = log;
        _expectedType = expectedType;
    }

    private sealed record CertEntry(
        string ProviderId, string TokenId, string TokenLabel, string TokenSerial,
        string CertId, byte[] Der, EsignCertOption Option);

    public async Task<string> HandleAsync(string requestJson, CancellationToken ct = default)
    {
        ITokenSession? tokenSession = null;
        IAsyncDisposable? pushUi = null;
        try
        {
            JsonNode? root = JsonNode.Parse(requestJson);
            if (root is null) return Err("Хоосон JSON.");
            string? type = root["type"]?.GetValue<string>();
            if (_expectedType is not null && type != _expectedType) return Err($"Танигдахгүй type: {type}");

            JsonNode? data = root["data"];
            string? ott = data?["_ott"]?.GetValue<string>();
            string? serverCertB64 = root["cert"]?.GetValue<string>();
            if (string.IsNullOrEmpty(ott)) return Err("_ott алга.");
            if (string.IsNullOrEmpty(serverCertB64)) return Err("cert алга.");

            _log.LogInformation("ESIGN хүсэлт: type={Type}", type);

            // Гарын үсэг зурах эх сурвалжийг сонгоно: физик токен эсвэл программ токен.
            byte[] certDer;
            string serial;
            byte[] keyIdRaw;
            string ownerForLog;
            Func<byte[], Task<byte[]>> signP;

            // Тохиргоонд "программ токен давуу" гэж сонгосон бөгөөд апп-д нэвтэрсэн бол
            // физик токеныг огт хайхгүй (токен уншуулах, PIN асуухыг алгасана).
            bool softFirst = _prefs.PreferSoftwareToken && _soft.IsAvailable;
            List<CertEntry> certs = softFirst
                ? new List<CertEntry>()
                : await CollectCertsAsync(ct).ConfigureAwait(false);

            if (certs.Count > 0)
            {
                // ─── Физик токен ───
                int idx = 0;
                if (certs.Count > 1)
                {
                    idx = await _ui.SelectCertificateAsync(certs.Select(c => c.Option).ToList(), ct).ConfigureAwait(false);
                    if (idx < 0) return Err("Хэрэглэгч гэрчилгээ сонгохоос татгалзлаа.");
                    if (idx >= certs.Count) idx = 0;
                }
                CertEntry sel = certs[idx];

                // Диагностик: физик токены АЖИЛЛАДАГ cert-ийн бүтэц (subject/issuer/
                // extensions). Программ токены cert-тэй харьцуулж, ялгааг олно.
                try
                {
                    using var xc = new X509Certificate2(sel.Der);
                    var exts = string.Join(",", xc.Extensions.Select(e => e.Oid?.Value));
                    _log.LogInformation("Физик токен cert: subject=[{Subj}] issuer=[{Iss}] key={Key} notAfter={NA} exts=[{Exts}]",
                        xc.Subject, xc.Issuer, xc.GetKeyAlgorithm(), xc.NotAfter, exts);
                }
                catch (Exception cex) { _log.LogWarning(cex, "Физик токен cert задлах алдаа"); }

                string? pin = await _ui.GetPinAsync(sel.TokenLabel, ct).ConfigureAwait(false);
                if (string.IsNullOrEmpty(pin)) return Err("PIN оруулаагүй.");

                // OpenAsync(pin) нь дотроо LoginAsync хийдэг тул дахин login дуудахгүй
                // (давхар login нь eps2003-д PIN алдаа өгдөг).
                var openRes = await _tokens.OpenAsync(sel.ProviderId, sel.TokenId, pin, ct).ConfigureAwait(false);
                if (!openRes.IsSuccess) return Err("PIN буруу эсвэл session нээгдсэнгүй: " + openRes.Error.Message);
                var session = openRes.Value;
                tokenSession = session;

                certDer = sel.Der;
                serial = sel.TokenSerial;
                keyIdRaw = HexToBytes(sel.CertId);
                ownerForLog = sel.Option.OwnerName;
                signP = async p =>
                {
                    var r = await session.SignAsync(sel.CertId, p, SigningAlg.Sha256WithRsa, ct).ConfigureAwait(false);
                    if (!r.IsSuccess) throw new InvalidOperationException("Гарын үсэг: " + r.Error.Message);
                    return r.Value;
                };
            }
            else if (_soft.IsAvailable)
            {
                // ─── Программ токен (токен алга, апп-д нэвтэрсэн) ───
                if (softFirst)
                {
                    _log.LogInformation("ESIGN: тохиргоогоор программ токен сонгосон — физик токен хайсангүй.");
                }
                else
                {
                    _log.LogInformation("ESIGN: физик токен алга — программ токеноор үргэлжлүүлнэ.");
                }
                // Утас руу PIN2 push илгээхийн өмнө эзнийг Windows Hello-оор батална —
                // түгжээгүй үлдээсэн компьютер дээрээс өөр хүн зурах эрсдэлийг хаана.
                var consent = await _consent
                    .RequireConsentAsync("Тоон гарын үсгээр нэвтрэхийг зөвшөөрөх", "sign.esign", ct)
                    .ConfigureAwait(false);
                if (consent.IsFailure)
                {
                    return Err("Windows Hello баталгаа өгөгдсөнгүй.");
                }

                var idRes = await _soft.GetIdentityAsync(ct).ConfigureAwait(false);
                if (!idRes.IsSuccess) return Err(SoftError(idRes.Error.Message));
                EsignSoftwareIdentity id = idRes.Value;

                // Диагностик: аль cert-ийг, ямар CA-гаас илгээж байгаа. sso.gov.mn
                // "тоон гэрчилгээ хүчин төгөлдөр бус" гэвэл энэ subject/issuer-ээс
                // Gerege CA танигдахгүй эсэх (эсвэл буруу cert) нь тодорхой болно.
                try
                {
                    using var xc = new X509Certificate2(id.CertDer);
                    var exts = string.Join(",", xc.Extensions.Select(e => e.Oid?.Value));
                    _log.LogInformation("Программ токен cert: subject=[{Subj}] issuer=[{Iss}] key={Key} notAfter={NA} exts=[{Exts}]",
                        xc.Subject, xc.Issuer, xc.GetKeyAlgorithm(), xc.NotAfter, exts);
                }
                catch (Exception cex) { _log.LogWarning(cex, "Программ токен cert задлах алдаа"); }

                certDer = id.CertDer;
                serial = id.Serial;
                keyIdRaw = id.KeyId;
                ownerForLog = id.Option.OwnerName;
                signP = async p =>
                {
                    byte[] digest = SHA256.HashData(p);
                    var r = await _soft.SignDigestAsync(
                        digest,
                        "Төрийн сайтад тоон гарын үсгээр нэвтрэх",
                        onVerificationCode: async vc => { pushUi = await _ui.BeginPushAsync(vc, ct).ConfigureAwait(false); },
                        ct).ConfigureAwait(false);
                    if (!r.IsSuccess) throw new InvalidOperationException(SoftError(r.Error.Message));
                    return r.Value;
                };
            }
            else
            {
                return Err("Токен олдсонгүй. Программ токеноор нэвтрэхийн тулд эхлээд апп-д нэвтэрсэн байх шаардлагатай.");
            }

            string dataJson = EsignCrypto.IndentedJson(data);
            byte[] serverCertDer = Convert.FromBase64String(serverCertB64);

            var outp = await EsignCrypto.BuildAsync(dataJson, certDer, serial, keyIdRaw, serverCertDer, signP).ConfigureAwait(false);
            _log.LogInformation("ESIGN гарын үсэг амжилттай ({Owner})", ownerForLog);

            var resp = new JsonObject
            {
                ["status"] = "success",
                ["signature"] = outp.Signature,
                ["cipher"] = outp.Cipher,
                ["meta"] = outp.Meta,
            };
            return resp.ToJsonString();
        }
        catch (Exception ex)
        {
            _log.LogWarning(ex, "ESIGN алдаа");
            return Err(ex.Message);
        }
        finally
        {
            if (pushUi is not null)
            {
                try { await pushUi.DisposeAsync().ConfigureAwait(false); } catch { /* цонх хаах алдаа non-fatal */ }
            }
            if (tokenSession is not null)
            {
                try { await tokenSession.DisposeAsync().ConfigureAwait(false); } catch { /* session хаах алдаа non-fatal */ }
            }
        }
    }

    private async Task<List<CertEntry>> CollectCertsAsync(CancellationToken ct)
    {
        var result = new List<CertEntry>();
        var disc = await _tokens.DiscoverAllAsync(ct).ConfigureAwait(false);
        if (!disc.IsSuccess) return result;

        foreach (DiscoveredToken dt in disc.Value)
        {
            try
            {
                var openRes = await _tokens.OpenAsync(dt.Provider.Id, dt.Info.Id, null, ct).ConfigureAwait(false);
                if (!openRes.IsSuccess) continue;
                await using var session = openRes.Value;
                var objs = await session.ListObjectsAsync(ct).ConfigureAwait(false);
                if (!objs.IsSuccess) continue;

                foreach (TokenObject o in objs.Value)
                {
                    if (o.Kind != TokenObjectKind.Certificate) continue;
                    var certRes = await session.ReadCertificateAsync(o.Id, ct).ConfigureAwait(false);
                    if (!certRes.IsSuccess || certRes.Value is not { Length: > 0 }) continue;
                    byte[] der = certRes.Value;
                    if (result.Any(c => c.Der.AsSpan().SequenceEqual(der))) continue;
                    EsignCertOption opt = EsignCertParser.ParseOption(der);
                    result.Add(new CertEntry(dt.Provider.Id, dt.Info.Id, dt.Info.Label, dt.Info.SerialNumber, o.Id, der, opt));
                }
            }
            catch (Exception ex) { _log.LogDebug(ex, "Токен уншилт алдаа {Id}", dt.Info.Id); }
        }
        return result;
    }

    /// <summary>Программ токены дотоод алдааг хэрэглэгчид ойлгомжтой болгох.</summary>
    private static string SoftError(string raw) => raw switch
    {
        "esign_backend_not_available" =>
            "Программ токен дэмжлэг серверт хараахан идэвхжээгүй байна (endpoint дутуу).",
        _ => raw,
    };

    private static string Err(string message) =>
        new JsonObject { ["status"] = "error", ["message"] = message }.ToJsonString();

    private static byte[] HexToBytes(string hex)
    {
        if (string.IsNullOrEmpty(hex)) return Array.Empty<byte>();
        hex = hex.Trim();
        if (hex.Length % 2 != 0) hex = "0" + hex;
        var b = new byte[hex.Length / 2];
        for (int i = 0; i < b.Length; i++) b[i] = Convert.ToByte(hex.Substring(i * 2, 2), 16);
        return b;
    }
}
