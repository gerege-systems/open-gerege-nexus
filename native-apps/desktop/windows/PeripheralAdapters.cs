using System;
using System.Collections.Generic;
using System.IO.Ports;
using System.Net.Sockets;
using System.Text;
using System.Threading;
using System.Threading.Tasks;

namespace GeregeNexusNativeWin;

public static class PeripheralAdapters
{
    public static async Task PrintNetworkTestAsync(string host, int port)
    {
        if (string.IsNullOrWhiteSpace(host)) throw new ArgumentException("Printer host оруулна уу");
        using var client = new TcpClient(); using var timeout = new CancellationTokenSource(TimeSpan.FromSeconds(5));
        await client.ConnectAsync(host, port, timeout.Token); await using var stream = client.GetStream();
        var payload = new List<byte>(); payload.AddRange([0x1b,0x40]); payload.AddRange(Encoding.UTF8.GetBytes("Gerege Nexus\nNative ESC/POS test\n\n\n")); payload.AddRange([0x1d,0x56,0x41,0x03]);
        await stream.WriteAsync(payload.ToArray(), timeout.Token); await stream.FlushAsync(timeout.Token);
    }
    public static async Task PrintRawAsync(string host,int port,byte[] content,bool cut){using var client=new TcpClient();using var timeout=new CancellationTokenSource(TimeSpan.FromSeconds(8));await client.ConnectAsync(host,port,timeout.Token);await using var stream=client.GetStream();await stream.WriteAsync(content,timeout.Token);if(cut)await stream.WriteAsync(new byte[]{0x1d,0x56,0x41,0x03},timeout.Token);await stream.FlushAsync(timeout.Token);}
    public static void TestSerial(string portName, int baudRate) { if (string.IsNullOrWhiteSpace(portName)) throw new ArgumentException("COM port сонгоно уу"); using var port = new SerialPort(portName, baudRate) { ReadTimeout=1500,WriteTimeout=1500 }; port.Open(); }
    public static byte[] SerialTransact(string portName,int baudRate,byte[] request,int timeoutMs){using var port=new SerialPort(portName,baudRate){ReadTimeout=Math.Clamp(timeoutMs,100,30000),WriteTimeout=Math.Clamp(timeoutMs,100,30000)};port.Open();port.Write(request,0,request.Length);var response=new List<byte>();var deadline=DateTime.UtcNow.AddMilliseconds(timeoutMs);while(DateTime.UtcNow<deadline){try{var value=port.ReadByte();if(value>=0)response.Add((byte)value);}catch(TimeoutException){break;}}return response.ToArray();}
    public static async Task OpenDrawerAsync(string host, int port, int pulseMs)
    {
        using var client=new TcpClient(); using var timeout=new CancellationTokenSource(TimeSpan.FromSeconds(5)); await client.ConnectAsync(host,port,timeout.Token); await using var stream=client.GetStream(); byte on=(byte)Math.Clamp(pulseMs/2,1,255); await stream.WriteAsync(new byte[]{0x1b,0x70,0x00,on,on},timeout.Token);
    }
}
