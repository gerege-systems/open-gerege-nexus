namespace NexusGeregeDesktop.Application.Abstractions;

/// <summary>
/// Тоон гарын үсгийн (ESIGN) гүүрт хэрэглэгчтэй харилцах цонхнууд — UI давхарга
/// (WinUI) хэрэгжүүлнэ. Infrastructure давхарга UI-аас хараат бус байхын тулд.
/// </summary>
public interface IEsignInteraction
{
    /// <summary>
    /// Олон гэрчилгээнээс сонгуулна. Сонгосон индекс (0..N-1) буцаана; болих бол -1.
    /// Ганц гэрчилгээ бол дуудагдахгүй.
    /// </summary>
    Task<int> SelectCertificateAsync(IReadOnlyList<EsignCertOption> options, CancellationToken ct = default);

    /// <summary>Токены PIN асууна. Болих бол null.</summary>
    Task<string?> GetPinAsync(string tokenLabel, CancellationToken ct = default);

    /// <summary>
    /// Программ токен: гар утас руу баталгаажуулах хүсэлт илгээснийг харуулж,
    /// баталгаажих кодыг (VC) үзүүлнэ (хэрэглэгч утсан дээрээ энэ кодыг таарч
    /// байгааг шалгаж баталгаажуулна). Буцаах <see cref="IAsyncDisposable"/>-ийг
    /// dispose хийхэд цонх хаагдана (гарын үсэг зурагдаж дуусмагц).
    /// </summary>
    Task<IAsyncDisposable> BeginPushAsync(string verificationCode, CancellationToken ct = default);
}

/// <summary>Гэрчилгээ сонгох диалогт харуулах мэдээлэл.</summary>
public sealed record EsignCertOption(
    string OwnerName,       // эзэмшигчийн нэр (CN)
    string PersonRegno,     // хувь хүний РД (SERIALNUMBER)
    string OrgName,         // байгууллага (O)
    string OrgRegno,        // ААН-ийн РД
    string Usage,           // "Нэвтрэлт" / "Гарын үсэг"
    DateTime NotAfter);     // хүчинтэй хугацаа
