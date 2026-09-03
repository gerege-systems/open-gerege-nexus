using NexusGeregeDesktop.Application.Abstractions;

namespace NexusGeregeDesktop.Client.Services;

/// Аппын үндсэн цонхны HWND-ийг Infrastructure давхаргад дамжуулна
/// (Windows Hello-ийн систем цонхыг эцэг цонхонд холбоход хэрэгтэй).
/// <see cref="App.WindowHandle"/> нь App.OnLaunched дээр тавигддаг тул
/// эхлэлийн үед <see cref="IntPtr.Zero"/> байж болно.
public sealed class WindowHandleProvider : IWindowHandleProvider
{
    public IntPtr Handle => App.WindowHandle;
}
