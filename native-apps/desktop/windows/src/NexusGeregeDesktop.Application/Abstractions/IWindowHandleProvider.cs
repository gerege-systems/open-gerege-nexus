namespace NexusGeregeDesktop.Application.Abstractions;

/// <summary>
/// Аппын үндсэн цонхны HWND. Win32 (WinUI 3) апп-д Windows Hello-ийн систем цонхыг
/// эцэг цонхонд заавал холбох ёстой (<c>UserConsentVerifierInterop</c>) — цонхгүйгээр
/// API нь "Element not found" алдаа өгдөг. UI давхарга хэрэгжүүлнэ.
/// </summary>
public interface IWindowHandleProvider
{
    /// <summary>Цонх хараахан үүсээгүй бол <see cref="IntPtr.Zero"/>.</summary>
    IntPtr Handle { get; }
}
