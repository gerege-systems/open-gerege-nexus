@echo off
echo ========================================================
echo  Building Native Windows App (C# / .NET 8 WPF / WebView2)
echo ========================================================

cd /d "%~dp0"

dotnet restore GeregeNexusWin.csproj
dotnet build GeregeNexusWin.csproj -c Release

echo.
echo ========================================================
echo  Build Completed Successfully!
echo  Binary output at: bin\Release\net8.0-windows\GeregeNexusNativeWin.exe
echo ========================================================
