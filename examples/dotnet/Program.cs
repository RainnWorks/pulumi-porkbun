using System.Collections.Generic;
using Pulumi;
using RainnWorks.Porkbun;

return await Deployment.RunAsync(() =>
{
    var domain = new Config().Require("domain");

    var apex = new DnsRecord("apex", new()
    {
        Domain = domain,
        Type = "A",
        Content = "203.0.113.10",
    });

    var www = new DnsRecord("www", new()
    {
        Domain = domain,
        Name = "www",
        Type = "CNAME",
        Content = domain,
    });

    var verification = new DnsRecord("verification", new()
    {
        Domain = domain,
        Name = "_example-verification",
        Type = "TXT",
        Content = "pulumi-porkbun-example",
        Notes = "Created by the pulumi-porkbun example",
    });

    var nameservers = GetNameservers.Invoke(new() { Domain = domain });

    return new Dictionary<string, object?>
    {
        ["names"] = Output.All(apex.Fqdn, www.Fqdn, verification.Fqdn),
        ["delegatedTo"] = nameservers.Apply(r => r.Nameservers),
    };
});
