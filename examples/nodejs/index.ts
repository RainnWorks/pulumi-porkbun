import * as pulumi from "@pulumi/pulumi";
import * as porkbun from "@rainnworks/porkbun";

const domain = new pulumi.Config().require("domain");

const apex = new porkbun.DnsRecord("apex", {
    domain,
    type: "A",
    content: "203.0.113.10",
});

const www = new porkbun.DnsRecord("www", {
    domain,
    name: "www",
    type: "CNAME",
    content: domain,
});

const verification = new porkbun.DnsRecord("verification", {
    domain,
    name: "_example-verification",
    type: "TXT",
    content: "pulumi-porkbun-example",
    notes: "Created by the pulumi-porkbun example",
});

const nameservers = porkbun.getNameserversOutput({ domain });

export const names = [apex.fqdn, www.fqdn, verification.fqdn];
export const delegatedTo = nameservers.nameservers;
