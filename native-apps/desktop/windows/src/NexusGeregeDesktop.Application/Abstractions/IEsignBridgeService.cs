namespace NexusGeregeDesktop.Application.Abstractions;

/// <summary>
/// Локал ESIGN гүүр (ws:59001 + wss:59005) — төрийн сайтын "Тоон гарын үсгээр нэвтрэх"
/// хуудас холбогдоно. Апп эхлэхэд <see cref="Start"/>-аар гараар асаана
/// (IHealthMonitor-той ижил загвар).
/// </summary>
public interface IEsignBridgeService
{
    /// <summary>Гүүр ажиллаж байгаа эсэх (Тохиргоо хуудсанд харуулна).</summary>
    bool IsRunning { get; }

    /// <summary>Сонссон ws порт (wss нь үүн дээр +4).</summary>
    int WsPort { get; }

    /// <summary>WS/WSS серверийг фоноор эхлүүлнэ (давхар дуудахад дахин асаахгүй).</summary>
    void Start();

    /// <summary>Серверийг зогсооно.</summary>
    void Stop();
}
