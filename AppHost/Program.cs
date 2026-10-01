using Banking.Aspire.Hosting.Radius;
using Aspire.Hosting.Pipelines;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;

#pragma warning disable ASPIRECOMPUTE003, ASPIREPIPELINES001

var builder = DistributedApplication.CreateBuilder(args).AddDapr();
builder.Configuration.AddUserSecrets<Program>();
builder.Services.PostConfigure<PipelineOptions>(options =>
{
    if (string.IsNullOrWhiteSpace(options.OutputPath))
    {
        options.OutputPath = Path.Combine(builder.AppHostDirectory, "artifacts");
    }
});

var infrastructure = builder.AddBankingInfrastructure();
var migrations = builder.AddBankingMigrations(infrastructure);
var services = builder.AddBankingServices(infrastructure, migrations);
var bankingWeb = builder.AddBankingWeb(infrastructure, services);

builder.AddRadiusReleaseArtifacts(
    infrastructure.Radius,
    services.Accounts.Resource,
    services.Contacts.Resource,
    services.Users.Resource,
    services.Transactions.Resource,
    bankingWeb.Resource);

builder.Build().Run();
