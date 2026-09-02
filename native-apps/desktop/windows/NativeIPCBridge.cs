using System;
using System.Text.Json;
using System.Windows;
using Microsoft.Web.WebView2.Wpf;
using Windows.Security.Credentials.UI;
using System.Text;
using System.Windows.Input;

namespace GeregeNexusNativeWin
{
    public class NativeIPCBridge
    {
        private readonly WebView2 _webView;
        private readonly MainWindow _mainWindow;
        private bool _scannerActive; private readonly StringBuilder _scanBuffer = new();

        public NativeIPCBridge(WebView2 webView, MainWindow mainWindow)
        {
            _webView = webView;
            _mainWindow = mainWindow;
            _webView.PreviewTextInput += (_,e) => { if(_scannerActive)_scanBuffer.Append(e.Text); };
            _webView.PreviewKeyDown += (_,e) => { if(_scannerActive&&e.Key==Key.Enter&&_scanBuffer.Length>0){Emit("shell:scan",new{value=_scanBuffer.ToString(),format="keyboard-wedge"});_scanBuffer.Clear();e.Handled=true;} };
        }

        public async void HandleWebMessage(string jsonMessage, string source)
        {
            try
            {
                if (!Uri.TryCreate(source, UriKind.Absolute, out var origin) ||
                    !string.Equals(origin.GetLeftPart(UriPartial.Authority), _mainWindow.WebOrigin, StringComparison.OrdinalIgnoreCase)) return;
                using var doc = JsonDocument.Parse(jsonMessage);
                var root = doc.RootElement;

                string command = root.GetProperty("method").GetString() ?? "";
                string requestId = root.TryGetProperty("id", out var reqEl) ? reqEl.GetString() ?? "" : "";

                switch (command)
                {
                    case "auth.reLogin":
                        _mainWindow.ShowNativeLogin();
                        Resolve(requestId, true, null);
                        break;
                    case "menu.changed":
                        Resolve(requestId, true, null);
                        break;
                    case "shell.openPane":
                        // Ажлын муж бүрхүүлийн эзэмшдэг дэлгэц рүү шилжихийг
                        // хүсэж байна. Шинэ цонх нээхгүй — ижил хүрээн доторх
                        // дэлгэц солигдоно.
                        var pane = root.TryGetProperty("params", out var paneParams) && paneParams.TryGetProperty("pane", out var paneValue)
                            ? paneValue.GetString() ?? "" : "";
                        var opened = _webView.Dispatcher.Invoke(() => _mainWindow.OpenPane(pane));
                        Resolve(requestId, opened, opened ? null : "Unknown pane");
                        break;
                    case "print.system":
                        await _webView.ExecuteScriptAsync("window.print()");
                        Resolve(requestId, true, null);
                        break;
                    case "external.open":
                        if (!root.TryGetProperty("params", out var parameters) ||
                            !parameters.TryGetProperty("url", out var urlElement) ||
                            !Uri.TryCreate(urlElement.GetString(), UriKind.Absolute, out var target) ||
                            target.Scheme is not ("http" or "https" or "mailto" or "tel"))
                        {
                            Resolve(requestId, false, "URL scheme not allowed"); break;
                        }
                        System.Diagnostics.Process.Start(new System.Diagnostics.ProcessStartInfo(target.AbsoluteUri) { UseShellExecute = true });
                        Resolve(requestId, true, null);
                        break;
                    case "auth.lock":
                    case "biometric.authenticate":
                        var available = await UserConsentVerifier.CheckAvailabilityAsync();
                        if (available != UserConsentVerifierAvailability.Available) { Resolve(requestId, false, "Windows Hello тохируулаагүй байна"); break; }
                        var verified = await UserConsentVerifier.RequestVerificationAsync("Gerege Nexus ажлын хэсгийг нээх");
                        Resolve(requestId, verified == UserConsentVerificationResult.Verified, verified == UserConsentVerificationResult.Verified ? new { authenticated = true } : "Windows Hello баталгаажуулалт цуцлагдсан");
                        break;
                    case "device.identity":
                        var settings = NativeSettings.Load();
                        Resolve(requestId, true, new { id=settings.DeviceId, name=settings.DeviceName, site=settings.Site, platform="windows", form_factor=ShellProfile.FormFactor });
                        break;
                    case "escpos.print":
                        if (!root.TryGetProperty("params",out var printParams)) { Resolve(requestId,false,"params дутуу");break; }
                        var configured=NativeSettings.Load(); byte[] bytes;
                        if(printParams.TryGetProperty("base64",out var b64)&&!string.IsNullOrWhiteSpace(b64.GetString())) bytes=Convert.FromBase64String(b64.GetString()!);
                        else bytes=Encoding.UTF8.GetBytes(printParams.TryGetProperty("text",out var text)?text.GetString()??"":"");
                        await PeripheralAdapters.PrintRawAsync(configured.PrinterHost,configured.PrinterPort,bytes,!printParams.TryGetProperty("cut",out var cut)||cut.GetBoolean());Resolve(requestId,true,null);break;
                    case "escpos.drawer":
                        var drawer=NativeSettings.Load(); await PeripheralAdapters.OpenDrawerAsync(drawer.PrinterHost,drawer.PrinterPort,drawer.DrawerPulseMs);Resolve(requestId,true,null);break;
                    case "kiosk.lockdown":
                        var enabled=root.TryGetProperty("params",out var lockParams)&&lockParams.TryGetProperty("enabled",out var enabledValue)&&enabledValue.GetBoolean();_mainWindow.SetKioskMode(enabled);Resolve(requestId,true,new{enabled});break;
                    case "scanner.start": _scannerActive=true;_scanBuffer.Clear();Resolve(requestId,true,null);break;
                    case "scanner.stop": _scannerActive=false;_scanBuffer.Clear();Resolve(requestId,true,null);break;
                    case "serial.transact":
                        var serialSettings=NativeSettings.Load();var serialParams=root.GetProperty("params");var requestBytes=Convert.FromBase64String(serialParams.GetProperty("base64").GetString()??"");var responseBytes=PeripheralAdapters.SerialTransact(serialParams.TryGetProperty("port",out var portValue)?portValue.GetString()??serialSettings.SerialPort:serialSettings.SerialPort,serialParams.TryGetProperty("baud",out var baudValue)?baudValue.GetInt32():serialSettings.BaudRate,requestBytes,serialParams.TryGetProperty("read_timeout_ms",out var timeoutValue)?timeoutValue.GetInt32():1500);Resolve(requestId,true,new{base64=Convert.ToBase64String(responseBytes)});break;
                    default:
                        Resolve(requestId, false, $"Unsupported method: {command}");
                        break;
                }
            }
            catch (Exception ex)
            {
                Console.WriteLine($"[NativeIPC Error] Failed to parse message: {ex.Message}");
                try { using var failed=JsonDocument.Parse(jsonMessage); if(failed.RootElement.TryGetProperty("id",out var failedId)) Resolve(failedId.GetString()??"",false,ex.Message); } catch { }
            }
        }

        private void Resolve(string id, bool ok, object? value)
        {
            var idJson = JsonSerializer.Serialize(id);
            var valueJson = JsonSerializer.Serialize(value);
            _webView.Dispatcher.Invoke(() =>
            {
                string js = $"window.__geregeShellResolve({idJson},{ok.ToString().ToLowerInvariant()},{valueJson});";
                _webView.ExecuteScriptAsync(js);
            });
        }
        private void Emit(string name,object value){var nameJson=JsonSerializer.Serialize(name);var valueJson=JsonSerializer.Serialize(value);_webView.Dispatcher.Invoke(()=>_webView.ExecuteScriptAsync($"window.__geregeShellEmit&&window.__geregeShellEmit({nameJson},{valueJson})"));}
    }
}
