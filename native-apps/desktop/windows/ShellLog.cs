using System;
using System.IO;

namespace GeregeNexusNativeWin;

/// <summary>
/// Бүрхүүлийн энгийн лог.
///
/// macOS клиент нь голлох алхмуудаа stdout руу хэвлэдэг; Windows тал дээр
/// GUI апп консольгүй ажилладаг тул юу ч үлддэггүй байв — нэвтрэлт, session
/// дамжуулалт, шилжилт бүтэлгүйтэхэд оношлох мэдээлэл огт байхгүй.
///
/// Нууц утга (cookie-ийн value, токен, нууц үг) ХЭЗЭЭ Ч энд бичигдэхгүй —
/// зөвхөн нэр, домэйн, зам, төлөв.
/// </summary>
public static class ShellLog
{
    private static readonly object Gate = new();

    private static string Path => System.IO.Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
        "NexusGeregeDesktop", "shell.log");

    public static void Write(string message)
    {
        try
        {
            var file = Path;
            Directory.CreateDirectory(System.IO.Path.GetDirectoryName(file)!);
            var line = $"{DateTimeOffset.Now:yyyy-MM-dd HH:mm:ss.fff}  {message}{Environment.NewLine}";
            lock (Gate)
            {
                // Лог хязгааргүй өсөхөөс сэргийлж 512 КБ дээр эргэлдүүлнэ.
                if (File.Exists(file) && new FileInfo(file).Length > 512 * 1024)
                {
                    File.Move(file, file + ".1", overwrite: true);
                }
                File.AppendAllText(file, line);
            }
        }
        catch (IOException) { }
        catch (UnauthorizedAccessException) { }
    }
}
