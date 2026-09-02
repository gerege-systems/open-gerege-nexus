using System;
using System.IO;
using System.Text.Json;

namespace GeregeNexusNativeWin;

public sealed class NativeSettings
{
    public int SchemaVersion { get; set; } = 3;
    public bool LaunchAtLogin { get; set; }
    public string Language { get; set; } = "mn";
    // Web ба API нэг host дээр байгаа нь санаатай: ажлын мужаас гарах дуудлага
    // same-origin болж, session cookie нь SameSite=Strict хэвээр ажиллана.
    public string WebEndpoint { get; set; } = ShellProfile.LineOrigin;
    public string ApiEndpoint { get; set; } = ShellProfile.LineOrigin;
    public string PrinterTransport { get; set; } = "USB";
    public string PrinterHost { get; set; } = "";
    public int PrinterPort { get; set; } = 9100;
    public string PaperWidth { get; set; } = "80 mm";
    public string ScannerMode { get; set; } = "Keyboard wedge";
    public string ScannerSuffix { get; set; } = "Enter";
    public string SerialPort { get; set; } = "";
    public int BaudRate { get; set; } = 9600;
    public bool BiometricLock { get; set; } = true;
    public int IdleLockMinutes { get; set; } = 5;
    public string UpdateChannel { get; set; } = "Stable";
    public bool Telemetry { get; set; } = true;
    public string DeviceName { get; set; } = Environment.MachineName;
    public string Site { get; set; } = "";
    public string DeviceId { get; set; } = "";
    public int DrawerPulseMs { get; set; } = 120;

    private static string FilePath => Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "GeregeNexus", "native-settings-v1.json");
    public static NativeSettings Load()
    {
        NativeSettings value;
        try { value = JsonSerializer.Deserialize<NativeSettings>(File.ReadAllText(FilePath)) ?? new(); }
        catch { return new(); }

        // v3: dev-ийн localhost анхдагчийг энэ build-ийн domain шугам руу
        // зөөнө. `https://nexus.gerege.mn` энд зориудаар байхгүй — тэр хаяг
        // ижил backend руу очдог тул ажилласаар байгаа бөгөөд түүнийг хүчээр
        // зөөвөл хөтчийн шугамыг санаатай сонгосон суулгацыг булааж авна.
        if (value.SchemaVersion < 3)
        {
            if (value.WebEndpoint is "http://localhost:3000") value.WebEndpoint = ShellProfile.LineOrigin;
            if (value.ApiEndpoint is "http://localhost:8080") value.ApiEndpoint = ShellProfile.LineOrigin;
            value.SchemaVersion = 3;
            value.Save();
        }
        return value;
    }

    public void Save()
    {
        Directory.CreateDirectory(Path.GetDirectoryName(FilePath)!);
        File.WriteAllText(FilePath, JsonSerializer.Serialize(this, new JsonSerializerOptions { WriteIndented = true }));
    }
}
