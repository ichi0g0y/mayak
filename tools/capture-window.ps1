param(
    [string]$ProcessName = "MAYAK",
    [string]$OutputPath = ""
)

Add-Type -AssemblyName System.Drawing
Add-Type @"
using System;
using System.Runtime.InteropServices;
public static class MayakCapture {
    [StructLayout(LayoutKind.Sequential)] public struct Rect { public int Left, Top, Right, Bottom; }
    [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr hwnd, out Rect rect);
    [DllImport("user32.dll")] public static extern bool PrintWindow(IntPtr hwnd, IntPtr hdc, uint flags);
}
"@

$process = Get-Process -Name $ProcessName -ErrorAction Stop | Select-Object -First 1
$rect = New-Object MayakCapture+Rect
if (-not [MayakCapture]::GetWindowRect($process.MainWindowHandle, [ref]$rect)) { throw "GetWindowRect failed" }
$width = $rect.Right - $rect.Left
$height = $rect.Bottom - $rect.Top
$bitmap = New-Object System.Drawing.Bitmap($width, $height)
$graphics = [System.Drawing.Graphics]::FromImage($bitmap)
$dc = $graphics.GetHdc()
try {
    if (-not [MayakCapture]::PrintWindow($process.MainWindowHandle, $dc, 2)) { throw "PrintWindow failed" }
} finally {
    $graphics.ReleaseHdc($dc)
    $graphics.Dispose()
}
if ([string]::IsNullOrWhiteSpace($OutputPath)) { $OutputPath = Join-Path $PWD "build\mayak-ui-current.png" }
$bitmap.Save($OutputPath, [System.Drawing.Imaging.ImageFormat]::Png)
$bitmap.Dispose()
Write-Output $OutputPath
