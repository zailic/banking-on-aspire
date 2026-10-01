internal static class BankingWeb
{
    public static IResourceBuilder<ProjectResource> AddBankingWeb(
        this IDistributedApplicationBuilder builder,
        BankingInfrastructure infrastructure,
        BankingServiceResources services)
    {
        var bankingWeb = builder.AddProject<Projects.Banking_Web>("banking-web")
            .WithContainerRegistry(infrastructure.LocalRegistry)
            .PublishAsDockerFile(container => container
                .WithDockerfile(".", "frontend/Banking.Web/Dockerfile")
                .WithImageRegistry("localhost:5001"))
            .WithRemoteImageTag(infrastructure.ReleaseTag)
            .WithHttpsEndpoint(port: 7443, name: "https")
            .WithExternalHttpEndpoints()
            .WithEnvironment("Authentication__KeycloakBaseUrl", infrastructure.Keycloak.GetEndpoint("http"))
            .WithEnvironment("Authentication__ClientSecret", infrastructure.BankingWebClientSecret)
            .WithEnvironment("ASPNETCORE_FORWARDEDHEADERS_ENABLED", "true")
            .WithEnvironment("Backend__UsersUrl", services.Users.GetEndpoint("grpc"))
            .WithEnvironment("Backend__ContactsUrl", services.Contacts.GetEndpoint("grpc"))
            .WithEnvironment("Backend__AccountsUrl", services.Accounts.GetEndpoint("grpc"))
            .WithEnvironment("Backend__TransactionsUrl", services.Transactions.GetEndpoint("grpc"))
            .WithReference(infrastructure.Keycloak)
            .WaitFor(infrastructure.Keycloak)
            .WaitFor(services.Users)
            .WaitFor(services.Contacts)
            .WaitFor(services.Accounts)
            .WaitFor(services.Transactions);

        if (builder.ExecutionContext.IsPublishMode)
        {
            bankingWeb.WithEnvironment("Authentication__KeycloakIssuerBaseUrl", "http://localhost:8081");
        }
        else
        {
            bankingWeb.WithEnvironment(
                "Authentication__KeycloakIssuerBaseUrl",
                infrastructure.Keycloak.GetEndpoint("http"));
        }

        return bankingWeb;
    }
}
