import pulumi
import rainnworks_porkbun as porkbun

domain = pulumi.Config().require("domain")

apex = porkbun.DnsRecord("apex", domain=domain, type="A", content="203.0.113.10")

www = porkbun.DnsRecord("www", domain=domain, name="www", type="CNAME", content=domain)

verification = porkbun.DnsRecord(
    "verification",
    domain=domain,
    name="_example-verification",
    type="TXT",
    content="pulumi-porkbun-example",
    notes="Created by the pulumi-porkbun example",
)

nameservers = porkbun.get_nameservers_output(domain=domain)

pulumi.export("names", [apex.fqdn, www.fqdn, verification.fqdn])
pulumi.export("delegatedTo", nameservers.nameservers)
