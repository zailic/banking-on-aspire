using Aspire.Hosting.ApplicationModel;
using CommunityToolkit.Aspire.Hosting.Dapr;

internal sealed record BankingServiceResources(
    IResourceBuilder<ExecutableResource> Accounts,
    IResourceBuilder<ExecutableResource> Transactions,
    IResourceBuilder<ExecutableResource> Contacts,
    IResourceBuilder<ExecutableResource> Users);

internal static class BankingServices
{
    public static BankingServiceResources AddBankingServices(
        this IDistributedApplicationBuilder builder,
        BankingInfrastructure infrastructure,
        BankingMigrations migrations)
    {
        var accounts = builder.AddExecutable(
                "accounts", "go", "services/accounts",
                "run", "-buildvcs=false", "./cmd/accounts")
            .WithContainerRegistry(infrastructure.LocalRegistry)
            .PublishAsDockerFile(container => container
                .WithDockerfile(".", "build/go-service.Dockerfile")
                .WithBuildArg("SERVICE", "accounts")
                .WithBuildArg("COMMAND", "accounts")
                .WithImageRegistry("localhost:5001"))
            .WithRemoteImageTag(infrastructure.ReleaseTag)
            .WithOtlpExporter(OtlpProtocol.Grpc)
            .WithEnvironment("ACCOUNTS_PORT", "8085")
            .WithEndpoint(targetPort: 8085, scheme: "grpc", name: "grpc", isExternal: true)
            .WithDaprSidecar(sidecar => sidecar
                .WithReference(infrastructure.PubSub)
                .WithOptions(new DaprSidecarOptions
                {
                    AppId = "accounts",
                    AppPort = 8085,
                    AppProtocol = "grpc"
                }))
            .WithReference(infrastructure.UsersDb)
            .WaitFor(infrastructure.UsersDb)
            .WaitForCompletion(migrations.Accounts)
            .WaitFor(infrastructure.Redis)
            .WithReference(infrastructure.Keycloak)
            .WaitFor(infrastructure.Keycloak);

        var transactions = builder.AddExecutable(
                "transactions", "go", "services/transactions",
                "run", "-buildvcs=false", "./cmd/transactions")
            .WithContainerRegistry(infrastructure.LocalRegistry)
            .PublishAsDockerFile(container => container
                .WithDockerfile(".", "build/go-service.Dockerfile")
                .WithBuildArg("SERVICE", "transactions")
                .WithBuildArg("COMMAND", "transactions")
                .WithImageRegistry("localhost:5001"))
            .WithRemoteImageTag(infrastructure.ReleaseTag)
            .WithOtlpExporter(OtlpProtocol.Grpc)
            .WithEnvironment("TRANSACTIONS_HTTP_PORT", "8086")
            .WithEnvironment("TRANSACTIONS_GRPC_PORT", "8087")
            .WithEndpoint(targetPort: 8087, scheme: "grpc", name: "grpc", isExternal: true)
            .WithDaprSidecar(sidecar => sidecar
                .WithReference(infrastructure.PubSub)
                .WithOptions(new DaprSidecarOptions
                {
                    AppId = "transactions",
                    AppPort = 8086,
                    AppProtocol = "http"
                }))
            .WithReference(infrastructure.TransactionsDb)
            .WithReference(infrastructure.UsersDb)
            .WaitForCompletion(migrations.Transactions)
            .WaitFor(infrastructure.Redis)
            .WithReference(infrastructure.Keycloak)
            .WaitFor(infrastructure.Keycloak);

        var contacts = builder.AddExecutable(
                "contacts", "go", "services/contacts",
                "run", "-buildvcs=false", "./cmd/contacts")
            .WithContainerRegistry(infrastructure.LocalRegistry)
            .PublishAsDockerFile(container => container
                .WithDockerfile(".", "build/go-service.Dockerfile")
                .WithBuildArg("SERVICE", "contacts")
                .WithBuildArg("COMMAND", "contacts")
                .WithImageRegistry("localhost:5001"))
            .WithRemoteImageTag(infrastructure.ReleaseTag)
            .WithEnvironment("CONTACTS_PORT", "8083")
            .WithEnvironment("ACCOUNTS_GRPC", accounts.GetEndpoint("grpc"))
            .WithEndpoint(targetPort: 8083, scheme: "grpc", name: "grpc", isExternal: true)
            .WithDaprSidecar(sidecar => sidecar.WithOptions(new DaprSidecarOptions
            {
                AppId = "contacts",
                AppPort = 8083,
                AppProtocol = "grpc"
            }))
            .WithReference(infrastructure.UsersDb)
            .WaitFor(infrastructure.UsersDb)
            .WaitForCompletion(migrations.Contacts)
            .WaitFor(accounts)
            .WithReference(infrastructure.Keycloak)
            .WaitFor(infrastructure.Keycloak);

        var users = builder.AddExecutable(
                "users", "go", "services/users",
                "run", "-buildvcs=false", "./cmd/users")
            .WithContainerRegistry(infrastructure.LocalRegistry)
            .PublishAsDockerFile(container => container
                .WithDockerfile(".", "build/go-service.Dockerfile")
                .WithBuildArg("SERVICE", "users")
                .WithBuildArg("COMMAND", "users")
                .WithImageRegistry("localhost:5001"))
            .WithRemoteImageTag(infrastructure.ReleaseTag)
            .WithEnvironment("USERS_PORT", "8084")
            .WithEndpoint(targetPort: 8084, scheme: "grpc", name: "grpc", isExternal: true)
            .WithDaprSidecar(sidecar => sidecar.WithOptions(new DaprSidecarOptions
            {
                AppId = "users",
                AppPort = 8084,
                AppProtocol = "grpc"
            }))
            .WithReference(infrastructure.UsersDb)
            .WaitFor(infrastructure.UsersDb)
            .WaitForCompletion(migrations.Users)
            .WithReference(infrastructure.Keycloak)
            .WaitFor(infrastructure.Keycloak);

        if (builder.ExecutionContext.IsPublishMode)
        {
            const string deployedIssuer = "http://localhost:8081/realms/banking-on-aspire";
            accounts.WithEnvironment("KEYCLOAK_ISSUER", deployedIssuer);
            transactions.WithEnvironment("KEYCLOAK_ISSUER", deployedIssuer);
            contacts.WithEnvironment("KEYCLOAK_ISSUER", deployedIssuer);
            users.WithEnvironment("KEYCLOAK_ISSUER", deployedIssuer);
        }

        return new BankingServiceResources(accounts, transactions, contacts, users);
    }
}
