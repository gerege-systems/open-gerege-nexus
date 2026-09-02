using System.Globalization;
using System.Resources;

namespace GeregeNexusNativeWin;

public static class NativeStrings
{
    private static readonly ResourceManager Login = new("GeregeNexusNativeWin.Resources.Login",typeof(NativeStrings).Assembly);
    public static string Get(string key,string fallback)=>Login.GetString(key,CultureInfo.CurrentUICulture)??fallback;
}
