using NexusGeregeDesktop.Domain.Guardian;
using NexusGeregeDesktop.Domain.Primitives;

namespace NexusGeregeDesktop.Application.Abstractions;

/// <summary>
/// "Миний хүүхдүүд" — асран хамгаалагчийн урсгал (mobile app-ын адил). Нэвтэрсэн
/// иргэний зөвшөөрлөөр бүртгэгдсэн хүүхдүүдийг жагсаах, шинэ хүүхэд нэмэх
/// (PIN2 зөвшөөрөл → бүртгэлийн код), зөвшөөрөл цуцлах. First-party `/api/*`
/// маршрутуудыг ашиглана; backend эдгээрийг хараахан хэрэгжүүлээгүй бол
/// "guardian_backend_not_available" алдаагаар цэвэрхэн уналт хийнэ.
/// </summary>
public interface IGuardianService
{
    /// <summary>Миний зөвшөөрлөөр бүртгэгдсэн хүүхдүүд.</summary>
    Task<Result<IReadOnlyList<GuardianChild>>> ListChildrenAsync(CancellationToken ct = default);

    /// <summary>
    /// Хүүхэд нэмэх эхлүүлэх: РД-аар ХУР-аас холбоо шалгаж, асран хамгаалагчийн
    /// утас руу PIN2 зөвшөөрлийн push илгээнэ. VC + хүүхдийн нэр буцаана.
    /// </summary>
    Task<Result<ChildAddSession>> AddChildInitAsync(string childRegNo, CancellationToken ct = default);

    /// <summary>
    /// Зөвшөөрөл PIN2-оор баталгаажсаны дараа хүүхдийн бүртгэлийн код авах.
    /// (Зөвшөөрөл хараахан ирээгүй бол алдаа буцаах ба ViewModel дахин оролдоно.)
    /// </summary>
    Task<Result<ChildEnrollCode>> AddChildCodeAsync(string sessionId, CancellationToken ct = default);

    /// <summary>Хүүхдийн зөвшөөрлийг цуцлах.</summary>
    Task<Result> RevokeChildAsync(string childRegNo, CancellationToken ct = default);
}
