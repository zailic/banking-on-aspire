using Aspire.Hosting.ApplicationModel;
using Aspire.Hosting.Pipelines;

internal sealed record BankingMigrations(
    IResourceBuilder<ExecutableResource> Contacts,
    IResourceBuilder<ExecutableResource> Users,
    IResourceBuilder<ExecutableResource> Accounts,
    IResourceBuilder<ExecutableResource> Transactions);

internal static class Migrations
{
    public static BankingMigrations AddBankingMigrations(
        this IDistributedApplicationBuilder builder,
        BankingInfrastructure infrastructure)
    {
        var contacts = builder.AddExecutable(
                "contacts-migrations", "go", "services/contacts",
                "run", "-buildvcs=false", "./cmd/contactsmigrate")
            .WithReference(infrastructure.UsersDb)
            .WaitFor(infrastructure.UsersDb);

        var users = builder.AddExecutable(
                "users-migrations", "go", "services/users",
                "run", "-buildvcs=false", "./cmd/usersmigrate")
            .WithReference(infrastructure.UsersDb)
            .WaitFor(infrastructure.UsersDb);

        var accounts = builder.AddExecutable(
                "accounts-migrations", "go", "services/accounts",
                "run", "-buildvcs=false", "./cmd/accountsmigrate")
            .WithReference(infrastructure.UsersDb)
            .WaitFor(infrastructure.UsersDb)
            .WaitForCompletion(users)
            .WaitForCompletion(contacts);

        var transactions = builder.AddExecutable(
                "transactions-migrations", "go", "services/transactions",
                "run", "-buildvcs=false", "./cmd/transactionsmigrate")
            .WithReference(infrastructure.TransactionsDb)
            .WithReference(infrastructure.UsersDb)
            .WaitFor(infrastructure.TransactionsDb)
            .WaitForCompletion(users);

        if (builder.ExecutionContext.IsPublishMode)
        {
            AddDeploymentStep(builder, infrastructure, "contacts");
            AddDeploymentStep(builder, infrastructure, "users");
            AddDeploymentStep(builder, infrastructure, "accounts");
            AddDeploymentStep(builder, infrastructure, "transactions");
        }

        return new BankingMigrations(contacts, users, accounts, transactions);
    }

#pragma warning disable ASPIREPIPELINES001
    private static void AddDeploymentStep(
        IDistributedApplicationBuilder builder,
        BankingInfrastructure infrastructure,
        string serviceName)
    {
        builder.Pipeline.AddStep(
            $"migrate-radius-{serviceName}",
            context => DeploymentMigrationRunner.RunAsync(
                serviceName,
                infrastructure.Radius.Resource.Namespace,
                infrastructure.PostgresPassword.Resource,
                context),
            dependsOn: "configure-radius-keycloak",
            requiredBy: $"deploy-radius-{serviceName}");
    }
#pragma warning restore ASPIREPIPELINES001
}
