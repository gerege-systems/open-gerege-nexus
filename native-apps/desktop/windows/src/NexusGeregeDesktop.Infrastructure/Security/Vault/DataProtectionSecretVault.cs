using System.Runtime.Versioning;
using System.Security.Cryptography;
using System.Text.Json;
using NexusGeregeDesktop.Application.Abstractions;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Options;

namespace NexusGeregeDesktop.Infrastructure.Security.Vault;

public sealed class SecretVaultOptions
{
    public const string SectionName = "NexusGeregeDesktop:SecretVault";

    /// Optional override. Defaults to %LOCALAPPDATA%/NexusGeregeDesktop/secrets.bin.
    public string? FilePath { get; init; }

    /// Optional purpose/entropy string mixed into DPAPI, providing a per-app
    /// scope so other apps signed as the same user cannot decrypt the blob.
    public string Entropy { get; init; } = "NexusGeregeDesktop.SecretVault.v1";
}

/// File-backed, DPAPI-encrypted (CurrentUser scope) JSON dictionary of
/// secrets. Replaces the WinRT PasswordVault, which is unreliable in
/// unpackaged Win32 / WindowsAppSDK desktop scenarios.
///
/// Each call performs a single-process file lock; cross-process locking is
/// not provided (the NexusGeregeDesktop desktop client is single-instance).
[SupportedOSPlatform("windows")]
public sealed class DataProtectionSecretVault : ISecretVault, IDisposable
{
    private static readonly JsonSerializerOptions JsonOptions = new()
    {
        WriteIndented = false,
        IncludeFields = false,
    };

    private readonly SemaphoreSlim _gate = new(1, 1);
    private readonly ILogger<DataProtectionSecretVault> _logger;
    private readonly string _filePath;
    private readonly byte[] _entropy;

    public DataProtectionSecretVault(
        IOptions<SecretVaultOptions> options,
        ILogger<DataProtectionSecretVault> logger)
    {
        ArgumentNullException.ThrowIfNull(options);
        var value = options.Value;
        _filePath = string.IsNullOrWhiteSpace(value.FilePath)
            ? DefaultPath()
            : Environment.ExpandEnvironmentVariables(value.FilePath);
        _entropy = System.Text.Encoding.UTF8.GetBytes(value.Entropy);
        _logger = logger;
    }

    public async ValueTask<byte[]?> ReadAsync(string key, CancellationToken ct = default)
    {
        ValidateKey(key);
        await _gate.WaitAsync(ct).ConfigureAwait(false);
        try
        {
            var store = await LoadAsync(ct).ConfigureAwait(false);
            return store.TryGetValue(key, out var v) ? v : null;
        }
        finally
        {
            _gate.Release();
        }
    }

    public async ValueTask WriteAsync(string key, byte[] value, CancellationToken ct = default)
    {
        ValidateKey(key);
        ArgumentNullException.ThrowIfNull(value);

        await _gate.WaitAsync(ct).ConfigureAwait(false);
        try
        {
            var store = await LoadAsync(ct).ConfigureAwait(false);
            store[key] = value;
            await SaveAsync(store, ct).ConfigureAwait(false);
        }
        finally
        {
            _gate.Release();
        }
    }

    public async ValueTask DeleteAsync(string key, CancellationToken ct = default)
    {
        ValidateKey(key);
        await _gate.WaitAsync(ct).ConfigureAwait(false);
        try
        {
            var store = await LoadAsync(ct).ConfigureAwait(false);
            if (store.Remove(key))
            {
                await SaveAsync(store, ct).ConfigureAwait(false);
            }
        }
        finally
        {
            _gate.Release();
        }
    }

    public async ValueTask<bool> ExistsAsync(string key, CancellationToken ct = default)
    {
        ValidateKey(key);
        await _gate.WaitAsync(ct).ConfigureAwait(false);
        try
        {
            var store = await LoadAsync(ct).ConfigureAwait(false);
            return store.ContainsKey(key);
        }
        finally
        {
            _gate.Release();
        }
    }

    private async Task<Dictionary<string, byte[]>> LoadAsync(CancellationToken ct)
    {
        if (!File.Exists(_filePath))
        {
            return new Dictionary<string, byte[]>(StringComparer.Ordinal);
        }

        var protectedBytes = await File.ReadAllBytesAsync(_filePath, ct).ConfigureAwait(false);
        if (protectedBytes.Length == 0)
        {
            return new Dictionary<string, byte[]>(StringComparer.Ordinal);
        }

        byte[] plaintext;
        try
        {
            plaintext = ProtectedData.Unprotect(
                protectedBytes,
                _entropy,
                DataProtectionScope.CurrentUser);
        }
        catch (CryptographicException ex)
        {
            _logger.LogError(ex, "Failed to decrypt secret vault at {Path}; treating as empty.", _filePath);
            return new Dictionary<string, byte[]>(StringComparer.Ordinal);
        }

        try
        {
            var dict = JsonSerializer.Deserialize<Dictionary<string, byte[]>>(plaintext, JsonOptions);
            return dict ?? new Dictionary<string, byte[]>(StringComparer.Ordinal);
        }
        finally
        {
            CryptographicOperations.ZeroMemory(plaintext);
        }
    }

    private async Task SaveAsync(Dictionary<string, byte[]> store, CancellationToken ct)
    {
        Directory.CreateDirectory(Path.GetDirectoryName(_filePath)!);

        var plaintext = JsonSerializer.SerializeToUtf8Bytes(store, JsonOptions);
        byte[] protectedBytes;
        try
        {
            protectedBytes = ProtectedData.Protect(
                plaintext,
                _entropy,
                DataProtectionScope.CurrentUser);
        }
        finally
        {
            CryptographicOperations.ZeroMemory(plaintext);
        }

        var temp = _filePath + ".tmp";
        await File.WriteAllBytesAsync(temp, protectedBytes, ct).ConfigureAwait(false);
        File.Move(temp, _filePath, overwrite: true);
    }

    private static void ValidateKey(string key)
    {
        if (string.IsNullOrWhiteSpace(key))
        {
            throw new ArgumentException("Secret key must be non-empty.", nameof(key));
        }
    }

    private static string DefaultPath()
    {
        var local = Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData);
        return Path.Combine(local, "NexusGeregeDesktop", "secrets.bin");
    }

    public void Dispose() => _gate.Dispose();
}
