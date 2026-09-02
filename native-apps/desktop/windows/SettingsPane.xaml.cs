using System;
using System.Linq;
using System.Windows;
using System.Windows.Controls;

namespace GeregeNexusNativeWin;

public partial class SettingsPane : UserControl
{
    private readonly NativeSettings settings = NativeSettings.Load();
    private readonly DeviceEnrollmentClient deviceClient = new();

    /// <summary>
    /// Web/API endpoint өөрчлөгдөж хадгалагдсаныг бүрхүүлд дуулгана — ажлын
    /// муж өөр origin дээр дахин ачаалагдах ёстой болно.
    /// </summary>
    public event Action? EndpointsChanged;

    public SettingsPane() { InitializeComponent(); LoadValues(); Loaded += async (_, _) => await RefreshDeviceAsync(); }

    private static string Selected(ComboBox box) => (box.SelectedItem as ComboBoxItem)?.Content?.ToString() ?? "";
    private static void Select(ComboBox box, string value) => box.SelectedItem = box.Items.Cast<ComboBoxItem>().FirstOrDefault(x => x.Content?.ToString() == value) ?? box.Items[0];
    private void LoadValues() {
        launchAtLogin.IsChecked = settings.LaunchAtLogin; Select(language, settings.Language); webEndpoint.Text = settings.WebEndpoint; apiEndpoint.Text = settings.ApiEndpoint;
        Select(printerTransport, settings.PrinterTransport); printerHost.Text = settings.PrinterHost; printerPort.Text = settings.PrinterPort.ToString(); Select(paperWidth, settings.PaperWidth);
        Select(scannerMode, settings.ScannerMode); Select(scannerSuffix, settings.ScannerSuffix); serialPort.Text = settings.SerialPort; Select(baudRate, settings.BaudRate.ToString());
        biometricLock.IsChecked = settings.BiometricLock; idleMinutes.Text = settings.IdleLockMinutes.ToString(); Select(updateChannel, settings.UpdateChannel); telemetry.IsChecked = settings.Telemetry;
        osVersion.Text = $"Windows             {Environment.OSVersion}";
        deviceName.Text = settings.DeviceName; deviceSite.Text = settings.Site; deviceId.Text = settings.DeviceId; deviceStatus.Text = string.IsNullOrWhiteSpace(settings.DeviceId) ? "Бүртгэгдээгүй" : "ACTIVE";
        drawerPulse.Text = settings.DrawerPulseMs.ToString(); drawerSection.Visibility = ShellProfile.FormFactor == "pos" ? Visibility.Visible : Visibility.Collapsed;
    }
    private void Section_Changed(object sender, SelectionChangedEventArgs e) {
        if (!IsLoaded || sections.SelectedItem is not ListBoxItem item) return;
        var panels = new[] { generalPanel, connectionPanel, printerPanel, scannerPanel, serialPanel, privacyPanel, devicePanel, drawerPanel, updatePanel, diagnosticsPanel };
        foreach (var panel in panels) panel.Visibility = Visibility.Collapsed;
        var key = item.Tag?.ToString(); sectionTitle.Text = item.Content?.ToString()?.Substring(3) ?? "Тохиргоо";
        (key switch { "connection" => connectionPanel, "printer" => printerPanel, "scanner" => scannerPanel, "serial" => serialPanel, "privacy" => privacyPanel, "device" => devicePanel, "drawer" => drawerPanel, "update" => updatePanel, "diagnostics" => diagnosticsPanel, _ => generalPanel }).Visibility = Visibility.Visible;
    }
    private void Save_Click(object sender, RoutedEventArgs e) {
        var endpointsMoved = settings.WebEndpoint != webEndpoint.Text.Trim() || settings.ApiEndpoint != apiEndpoint.Text.Trim();
        settings.LaunchAtLogin = launchAtLogin.IsChecked == true; settings.Language = Selected(language); settings.WebEndpoint = webEndpoint.Text.Trim(); settings.ApiEndpoint = apiEndpoint.Text.Trim();
        settings.PrinterTransport = Selected(printerTransport); settings.PrinterHost = printerHost.Text.Trim(); settings.PrinterPort = int.TryParse(printerPort.Text, out var port) ? port : 9100; settings.PaperWidth = Selected(paperWidth);
        settings.ScannerMode = Selected(scannerMode); settings.ScannerSuffix = Selected(scannerSuffix); settings.SerialPort = serialPort.Text.Trim(); settings.BaudRate = int.TryParse(Selected(baudRate), out var baud) ? baud : 9600;
        settings.BiometricLock = biometricLock.IsChecked == true; settings.IdleLockMinutes = int.TryParse(idleMinutes.Text, out var idle) ? Math.Max(1, idle) : 5; settings.UpdateChannel = Selected(updateChannel); settings.Telemetry = telemetry.IsChecked == true;
        settings.DeviceName = deviceName.Text.Trim(); settings.Site = deviceSite.Text.Trim(); settings.DeviceId = deviceId.Text.Trim();
        settings.DrawerPulseMs = int.TryParse(drawerPulse.Text,out var pulse) ? Math.Clamp(pulse,1,510) : 120;
        settings.Save(); status.Text = "Хадгаллаа";
        if (endpointsMoved) EndpointsChanged?.Invoke();
    }
    private void TestConnection_Click(object s, RoutedEventArgs e) => status.Text = "Web/API холболтыг шалгаж байна…";
    private async void TestPrinter_Click(object s, RoutedEventArgs e) { try { status.Text="Принтерт холбогдож байна…"; await PeripheralAdapters.PrintNetworkTestAsync(printerHost.Text.Trim(),int.TryParse(printerPort.Text,out var p)?p:9100); status.Text="Туршилтын баримт илгээгдлээ"; } catch(Exception ex){status.Text=ex.Message;} }
    private void TestScanner_Click(object s, RoutedEventArgs e) => status.Text = "Сканнерын input хүлээж байна…";
    private void TestSerial_Click(object s, RoutedEventArgs e) { try { PeripheralAdapters.TestSerial(serialPort.Text.Trim(),int.TryParse(Selected(baudRate),out var b)?b:9600); status.Text=$"{serialPort.Text} амжилттай нээгдлээ"; } catch(Exception ex){status.Text=ex.Message;} }
    private async void OpenDrawer_Click(object s,RoutedEventArgs e){try{await PeripheralAdapters.OpenDrawerAsync(printerHost.Text.Trim(),int.TryParse(printerPort.Text,out var p)?p:9100,int.TryParse(drawerPulse.Text,out var ms)?ms:120);status.Text="Шургуулгын pulse илгээгдлээ";}catch(Exception ex){status.Text=ex.Message;}}
    private void ClearCredentials_Click(object s, RoutedEventArgs e) { try { CredentialManagerTokenStore.Clear(); deviceId.Clear(); deviceStatus.Text = "Бүртгэгдээгүй"; status.Text = "Device token Credential Manager-аас цэвэрлэгдлээ"; } catch (Exception ex) { status.Text = ex.Message; } }
    private void CheckUpdates_Click(object s, RoutedEventArgs e) => status.Text = $"{Selected(updateChannel)} сувгийг шалгаж байна…";
    private void ExportLogs_Click(object s, RoutedEventArgs e) => status.Text = "Log exporter дараагийн device adapter-т холбогдоно";
    private async void EnrollDevice_Click(object s, RoutedEventArgs e) {
        status.Text = "Төхөөрөмжийг бүртгэж байна…";
        try { var enrolled = await deviceClient.EnrollAsync(apiEndpoint.Text.Trim(), enrollmentCode.Text.Trim(), deviceName.Text.Trim(), deviceSite.Text.Trim()); CredentialManagerTokenStore.Save(enrolled.DeviceToken); deviceId.Text = enrolled.DeviceId; deviceStatus.Text = "ACTIVE"; enrollmentCode.Clear(); settings.DeviceId = enrolled.DeviceId; settings.DeviceName = deviceName.Text.Trim(); settings.Site = deviceSite.Text.Trim(); settings.Save(); status.Text = "Төхөөрөмж амжилттай бүртгэгдлээ"; }
        catch (Exception ex) { status.Text = ex.Message; }
    }
    private async System.Threading.Tasks.Task RefreshDeviceAsync() {
        try { var token = CredentialManagerTokenStore.Load(); if (string.IsNullOrWhiteSpace(token)) return; var identity = await deviceClient.IdentityAsync(settings.ApiEndpoint, token); deviceId.Text = identity.Id; deviceName.Text = identity.Name; deviceStatus.Text = identity.Status; }
        catch { deviceStatus.Text = "Token хүчингүй эсвэл сервер холбогдохгүй байна"; }
    }
    private async void RotateToken_Click(object s,RoutedEventArgs e){try{var token=CredentialManagerTokenStore.Load();if(string.IsNullOrWhiteSpace(token))throw new InvalidOperationException("Төхөөрөмж бүртгэгдээгүй");CredentialManagerTokenStore.Save(await deviceClient.RotateTokenAsync(apiEndpoint.Text.Trim(),token));status.Text="Device token шинэчлэгдлээ";}catch(Exception ex){status.Text=ex.Message;}}
}
