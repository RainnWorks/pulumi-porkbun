# Examples

The same program in each language: three records on a domain you choose, and the
domain's current nameservers as an output.

```sh
cd nodejs            # or python, go, dotnet, yaml
pulumi stack init dev
pulumi config set domain example.com
pulumi config set porkbun:apiKey pk1_... --secret
pulumi config set porkbun:secretApiKey sk1_... --secret
pulumi up
```

**It changes real DNS.** Use a domain you don't mind adding records to, and run
`pulumi destroy` afterwards; that removes the three records again.
