namespace GeregeShell.Core;

public static class ShellContract
{
    public const string Version = "1.4";
    public static readonly IReadOnlySet<string> LifecycleMethods = new HashSet<string>{"auth.reLogin","auth.lock","menu.changed"};
    public static readonly IReadOnlySet<string> Capabilities = new HashSet<string>{"biometric","external.open","print.system","fs.save","secure-store","escpos","scanner","serial","nfc","camera.scan","device.identity","kiosk.lockdown","telemetry","payments","shell.pane"};
}
