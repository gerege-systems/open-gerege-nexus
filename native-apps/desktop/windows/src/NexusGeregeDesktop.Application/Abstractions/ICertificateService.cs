using NexusGeregeDesktop.Domain.Certificates;
using NexusGeregeDesktop.Domain.Primitives;

namespace NexusGeregeDesktop.Application.Abstractions;

public interface ICertificateService
{
    Result<CertificateInfo> ParsePem(string pemContents);

    Result<CertificateInfo> ParseFile(string filePath);
}
