package main

import (
	"github.com/RainnWorks/pulumi-porkbun/sdk/go/porkbun"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		domain := config.Require(ctx, "domain")

		apex, err := porkbun.NewDnsRecord(ctx, "apex", &porkbun.DnsRecordArgs{
			Domain:  pulumi.String(domain),
			Type:    pulumi.String("A"),
			Content: pulumi.String("203.0.113.10"),
		})
		if err != nil {
			return err
		}

		www, err := porkbun.NewDnsRecord(ctx, "www", &porkbun.DnsRecordArgs{
			Domain:  pulumi.String(domain),
			Name:    pulumi.String("www"),
			Type:    pulumi.String("CNAME"),
			Content: pulumi.String(domain),
		})
		if err != nil {
			return err
		}

		verification, err := porkbun.NewDnsRecord(ctx, "verification", &porkbun.DnsRecordArgs{
			Domain:  pulumi.String(domain),
			Name:    pulumi.String("_example-verification"),
			Type:    pulumi.String("TXT"),
			Content: pulumi.String("pulumi-porkbun-example"),
			Notes:   pulumi.String("Created by the pulumi-porkbun example"),
		})
		if err != nil {
			return err
		}

		nameservers := porkbun.GetNameserversOutput(ctx, porkbun.GetNameserversOutputArgs{
			Domain: pulumi.String(domain),
		})

		ctx.Export("names", pulumi.StringArray{apex.Fqdn, www.Fqdn, verification.Fqdn})
		ctx.Export("delegatedTo", nameservers.Nameservers())
		return nil
	})
}
