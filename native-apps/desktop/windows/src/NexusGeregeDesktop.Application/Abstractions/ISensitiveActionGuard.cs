using NexusGeregeDesktop.Domain.Primitives;

namespace NexusGeregeDesktop.Application.Abstractions;

/// <summary>
/// Мэдрэмтгий үйлдлийн өмнөх Windows Hello баталгаа (`Security.RequireWindowsHello`).
/// Гарын үсэг зурах бүрд — PDF болон ESIGN программ токен — эзэн нь компьютерийнхээ
/// ард байгааг батална: түгжээгүй үлдээсэн машин дээр гуравдагч этгээд утас руу
/// PIN2 push илгээх боломжийг хаана.
///
/// Бодлого: Hello тухайн төхөөрөмж дээр ТОХИРУУЛАГДААГҮЙ (эсвэл бодлогоор хаалттай)
/// бол үйлдэл үргэлжилнэ — эс бөгөөс биометргүй компьютер дээр гарын үсэг бүрмөсөн
/// боломжгүй болно. Харин Hello БАЙГАА мөртлөө хэрэглэгч цуцалсан/амжилтгүй болсон
/// тохиолдолд үйлдлийг зогсооно. Аль ч тохиолдол audit log-д бичигдэнэ.
/// </summary>
public interface ISensitiveActionGuard
{
    /// <param name="reasonLocalized">Hello цонхонд харагдах шалтгаан (локалчлагдсан).</param>
    /// <param name="operation">Audit-д бичих богино таг, ж: "sign.pdf", "sign.esign".</param>
    Task<Result> RequireConsentAsync(string reasonLocalized, string operation, CancellationToken ct = default);
}
