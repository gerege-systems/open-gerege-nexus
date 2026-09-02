using System;
using System.ComponentModel;
using System.Runtime.InteropServices;
using System.Text;

namespace GeregeNexusNativeWin;

public static class CredentialManagerTokenStore
{
    private const string Target = "GeregeNexus/device-token";
    private const uint Generic = 1, PersistLocalMachine = 2;
    [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)] private struct Credential { public uint Flags; public uint Type; public string TargetName; public string? Comment; public System.Runtime.InteropServices.ComTypes.FILETIME LastWritten; public uint CredentialBlobSize; public IntPtr CredentialBlob; public uint Persist; public uint AttributeCount; public IntPtr Attributes; public string? TargetAlias; public string UserName; }
    [DllImport("Advapi32.dll", EntryPoint = "CredWriteW", CharSet = CharSet.Unicode, SetLastError = true)] private static extern bool CredWrite(ref Credential credential, uint flags);
    [DllImport("Advapi32.dll", EntryPoint = "CredReadW", CharSet = CharSet.Unicode, SetLastError = true)] private static extern bool CredRead(string target, uint type, uint flags, out IntPtr credential);
    [DllImport("Advapi32.dll", EntryPoint = "CredDeleteW", CharSet = CharSet.Unicode, SetLastError = true)] private static extern bool CredDelete(string target, uint type, uint flags);
    [DllImport("Advapi32.dll")] private static extern void CredFree(IntPtr buffer);

    public static void Save(string token)
    {
        var bytes = Encoding.Unicode.GetBytes(token); var blob = Marshal.AllocCoTaskMem(bytes.Length);
        try { Marshal.Copy(bytes, 0, blob, bytes.Length); var credential = new Credential { Type = Generic, TargetName = Target, CredentialBlobSize = (uint)bytes.Length, CredentialBlob = blob, Persist = PersistLocalMachine, UserName = "device" }; if (!CredWrite(ref credential, 0)) throw new Win32Exception(Marshal.GetLastWin32Error()); }
        finally { for (var index = 0; index < bytes.Length; index++) Marshal.WriteByte(blob, index, 0); Marshal.FreeCoTaskMem(blob); }
    }
    public static string? Load()
    {
        if (!CredRead(Target, Generic, 0, out var pointer)) return Marshal.GetLastWin32Error() == 1168 ? null : throw new Win32Exception(Marshal.GetLastWin32Error());
        try { var credential = Marshal.PtrToStructure<Credential>(pointer); return credential.CredentialBlobSize == 0 ? null : Marshal.PtrToStringUni(credential.CredentialBlob, (int)credential.CredentialBlobSize / 2); }
        finally { CredFree(pointer); }
    }
    public static void Clear() { if (!CredDelete(Target, Generic, 0) && Marshal.GetLastWin32Error() != 1168) throw new Win32Exception(Marshal.GetLastWin32Error()); }
}
