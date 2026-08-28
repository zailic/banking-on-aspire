#:property AspireUseCliBundle=true
#:package Aspire.Hosting.Keycloak@13.5.3-preview.1.26425.3
#:sdk Aspire.AppHost.Sdk@13.5.3
#:package Aspire.Hosting.PostgreSQL@13.5.3
#:package CommunityToolkit.Aspire.Hosting.Dapr@13.2.1-beta.532

using System.Net.Http.Headers;
using System.Text;
using System.Text.Json;
using System.Text.Json.Nodes;
using Aspire.Hosting.ApplicationModel;

#pragma warning disable ASPIRECSHARPAPPS001

var builder = DistributedApplication.CreateBuilder(args);
var bankingWebClientSecret = builder.AddParameter("banking-web-client-secret", secret: true);
var realmDirectory = Path.GetFullPath("dev/keycloak");
const string developmentRealm = "banking-on-aspire";
var realmFile = Path.Combine(realmDirectory, $"{developmentRealm}-realm.json");

var keycloak = builder.AddKeycloak("keycloak", port: 8081)
    // Local services share the localhost cookie namespace even when their ports
    // differ. Allow Keycloak to accept and clean up legacy development cookies.
    .WithEnvironment("QUARKUS_HTTP_LIMITS_MAX_HEADER_SIZE", "64K")
    .WithEnvironment("QUARKUS_HTTP_LIMITS_MAX_HEADER_LIST_SIZE", "65536")
    .WithDataVolume()
    .WithRealmImport(realmDirectory);

var postgres = builder.AddPostgres("postgres")
    .WithDataVolume();
var usersDb = postgres.AddDatabase("usersdb");

var contactsMigrations = builder.AddExecutable(
        "contacts-migrations",
        "go",
        "services/contacts",
        "run",
        "-buildvcs=false",
        "./cmd/contactsmigrate")
    .WithReference(usersDb)
    .WaitFor(usersDb);

var usersMigrations = builder.AddExecutable(
        "users-migrations",
        "go",
        "services/users",
        "run",
        "-buildvcs=false",
        "./cmd/usersmigrate")
    .WithReference(usersDb)
    .WaitFor(usersDb);

var accountsMigrations = builder.AddExecutable(
        "accounts-migrations",
        "go",
        "services/accounts",
        "run",
        "-buildvcs=false",
        "./cmd/accountsmigrate")
    .WithReference(usersDb)
    .WaitFor(usersDb)
    .WaitForCompletion(usersMigrations);

keycloak
    .WithCommand(
        "export-realm",
        "Export realm",
        context => ExportRealmAsync(
            keycloak,
            developmentRealm,
            realmFile,
            context.CancellationToken),
        new CommandOptions
        {
            Description = $"Export '{developmentRealm}' to {realmFile}."
        })
    .WithCommand(
        "import-realm",
        "Import realm",
        context => ImportRealmAsync(
            keycloak,
            developmentRealm,
            realmFile,
            context.CancellationToken),
        new CommandOptions
        {
            Description = $"Import {realmFile} into the running '{developmentRealm}' realm.",
            ConfirmationMessage =
                $"Existing resources in '{developmentRealm}' may be overwritten. Continue?"
        });

builder.AddExecutable(
        "accounts-legacy",
        "go",
        "services/accounts-legacy",
        "run",
        "-buildvcs=false",
        "./cmd/accounts")
    .WithEndpoint(targetPort: 8080, scheme: "http", name: "http", isExternal: true)
    .WithDaprSidecar(sidecar => sidecar
        .WithOptions(new CommunityToolkit.Aspire.Hosting.Dapr.DaprSidecarOptions
        {
            AppId = "accounts-legacy",
            AppPort = 8080,
            AppProtocol = "http",
            DaprHttpPort = 3500
        })
    )
    .WithEndpoint(
        port: 8082,
        targetPort: 8080,
        scheme: "http",
        name: "http",
        isExternal: true
    )
    .WithReference(keycloak)
    .WaitFor(keycloak);

var accounts = builder.AddExecutable(
        "accounts",
        "go",
        "services/accounts",
        "run",
        "-buildvcs=false",
        "./cmd/accounts")
    .WithEnvironment("ACCOUNTS_PORT", "8085")
    .WithEndpoint(
        targetPort: 8085,
        scheme: "grpc",
        name: "grpc",
        isExternal: true)
    .WithDaprSidecar(sidecar => sidecar
        .WithOptions(new CommunityToolkit.Aspire.Hosting.Dapr.DaprSidecarOptions
        {
            AppId = "accounts",
            AppPort = 8085,
            AppProtocol = "grpc"
        }))
    .WithReference(usersDb)
    .WaitFor(usersDb)
    .WaitForCompletion(accountsMigrations)
    .WithReference(keycloak)
    .WaitFor(keycloak);

var contacts = builder.AddExecutable(
        "contacts",
        "go",
        "services/contacts",
        "run",
        "-buildvcs=false",
        "./cmd/contacts")
    .WithEnvironment("CONTACTS_PORT", "8083")
    .WithEndpoint(
        targetPort: 8083,
        scheme: "grpc",
        name: "grpc",
        isExternal: true)
    .WithDaprSidecar(sidecar => sidecar
        .WithOptions(new CommunityToolkit.Aspire.Hosting.Dapr.DaprSidecarOptions
        {
            AppId = "contacts",
            AppPort = 8083,
            AppProtocol = "grpc"
        }))
    .WithReference(usersDb)
    .WaitFor(usersDb)
    .WaitForCompletion(contactsMigrations)
    .WithReference(keycloak)
    .WaitFor(keycloak);

var users = builder.AddExecutable(
        "users",
        "go",
        "services/users",
        "run",
        "-buildvcs=false",
        "./cmd/users")
    .WithEnvironment("USERS_PORT", "8084")
    .WithEndpoint(
        targetPort: 8084,
        scheme: "grpc",
        name: "grpc",
        isExternal: true)
    .WithDaprSidecar(sidecar => sidecar
        .WithOptions(new CommunityToolkit.Aspire.Hosting.Dapr.DaprSidecarOptions
        {
            AppId = "users",
            AppPort = 8084,
            AppProtocol = "grpc"
        }))
    .WithReference(usersDb)
    .WaitFor(usersDb)
    .WaitForCompletion(usersMigrations)
    .WithReference(keycloak)
    .WaitFor(keycloak);

builder.AddCSharpApp("banking-web", "frontend/Banking.Web/Banking.Web.csproj")
    .WithHttpsEndpoint(port: 7443, name: "https")
    .WithExternalHttpEndpoints()
    .WithEnvironment("Authentication__KeycloakBaseUrl", keycloak.GetEndpoint("http"))
    .WithEnvironment("Authentication__ClientSecret", bankingWebClientSecret)
    .WithEnvironment("ASPNETCORE_FORWARDEDHEADERS_ENABLED", "true")
    .WithEnvironment("Backend__UsersUrl", users.GetEndpoint("grpc"))
    .WithEnvironment("Backend__ContactsUrl", contacts.GetEndpoint("grpc"))
    .WithEnvironment("Backend__AccountsUrl", accounts.GetEndpoint("grpc"))
    .WithReference(keycloak)
    .WaitFor(keycloak)
    .WaitFor(users)
    .WaitFor(contacts)
    .WaitFor(accounts);

builder.Build().Run();

static async Task<ExecuteCommandResult> ExportRealmAsync(
    IResourceBuilder<KeycloakResource> keycloak,
    string realm,
    string realmFile,
    CancellationToken cancellationToken)
{
    try
    {
        using var client = await CreateKeycloakAdminClientAsync(keycloak, cancellationToken);
        var encodedRealm = Uri.EscapeDataString(realm);
        using var response = await client.PostAsync(
            $"admin/realms/{encodedRealm}/partial-export?exportClients=true&exportGroupsAndRoles=true",
            content: null,
            cancellationToken);

        var responseBody = await response.Content.ReadAsStringAsync(cancellationToken);
        if (!response.IsSuccessStatusCode)
        {
            return CommandResults.Failure(
                $"Keycloak export failed ({(int)response.StatusCode}): {responseBody}");
        }

        var realmJson = JsonNode.Parse(responseBody)
            ?? throw new InvalidOperationException("Keycloak returned an empty export.");

        Directory.CreateDirectory(Path.GetDirectoryName(realmFile)!);
        var temporaryFile = realmFile + ".tmp";
        await File.WriteAllTextAsync(
            temporaryFile,
            realmJson.ToJsonString(new JsonSerializerOptions { WriteIndented = true }) + Environment.NewLine,
            Encoding.UTF8,
            cancellationToken);
        File.Move(temporaryFile, realmFile, overwrite: true);

        return CommandResults.Success($"Realm exported to {realmFile}.");
    }
    catch (Exception exception) when (exception is not OperationCanceledException)
    {
        return CommandResults.Failure(exception);
    }
}

static async Task<ExecuteCommandResult> ImportRealmAsync(
    IResourceBuilder<KeycloakResource> keycloak,
    string realm,
    string realmFile,
    CancellationToken cancellationToken)
{
    try
    {
        if (!File.Exists(realmFile))
        {
            return CommandResults.Failure(
                $"Realm file does not exist: {realmFile}. Export the realm first.");
        }

        var realmJson = JsonNode.Parse(await File.ReadAllTextAsync(realmFile, cancellationToken))
            as JsonObject
            ?? throw new InvalidOperationException("The realm file must contain a JSON object.");

        // Keycloak's partial-import endpoint requires an explicit collision policy.
        realmJson["ifResourceExists"] = "OVERWRITE";

        using var client = await CreateKeycloakAdminClientAsync(keycloak, cancellationToken);
        var encodedRealm = Uri.EscapeDataString(realm);
        using var content = new StringContent(
            realmJson.ToJsonString(),
            Encoding.UTF8,
            "application/json");
        using var response = await client.PostAsync(
            $"admin/realms/{encodedRealm}/partialImport",
            content,
            cancellationToken);

        var responseBody = await response.Content.ReadAsStringAsync(cancellationToken);
        if (!response.IsSuccessStatusCode)
        {
            return CommandResults.Failure(
                $"Keycloak import failed ({(int)response.StatusCode}): {responseBody}");
        }

        return CommandResults.Success(
            string.IsNullOrWhiteSpace(responseBody)
                ? $"Realm imported from {realmFile}."
                : responseBody);
    }
    catch (Exception exception) when (exception is not OperationCanceledException)
    {
        return CommandResults.Failure(exception);
    }
}

static async Task<HttpClient> CreateKeycloakAdminClientAsync(
    IResourceBuilder<KeycloakResource> keycloak,
    CancellationToken cancellationToken)
{
    var baseAddress = await keycloak.GetEndpoint("http").GetValueAsync(cancellationToken)
        ?? throw new InvalidOperationException("The Keycloak HTTP endpoint is not available.");
    // AddKeycloak uses "admin" when no explicit username parameter is supplied.
    var username = keycloak.Resource.AdminUserNameParameter is { } usernameParameter
        ? await usernameParameter.GetValueAsync(cancellationToken)
        : "admin";
    if (string.IsNullOrWhiteSpace(username))
    {
        throw new InvalidOperationException("The Keycloak admin username is not available.");
    }
    var password = await keycloak.Resource.AdminPasswordParameter.GetValueAsync(cancellationToken)
        ?? throw new InvalidOperationException("The Keycloak admin password is not available.");

    var client = new HttpClient
    {
        BaseAddress = new Uri(baseAddress.TrimEnd('/') + "/")
    };

    try
    {
        using var tokenContent = new FormUrlEncodedContent(new Dictionary<string, string>
        {
            ["client_id"] = "admin-cli",
            ["grant_type"] = "password",
            ["username"] = username,
            ["password"] = password
        });
        using var tokenResponse = await client.PostAsync(
            "realms/master/protocol/openid-connect/token",
            tokenContent,
            cancellationToken);
        var tokenResponseBody = await tokenResponse.Content.ReadAsStringAsync(cancellationToken);

        if (!tokenResponse.IsSuccessStatusCode)
        {
            throw new InvalidOperationException(
                $"Keycloak admin authentication failed ({(int)tokenResponse.StatusCode}): {tokenResponseBody}");
        }

        using var tokenJson = JsonDocument.Parse(tokenResponseBody);
        var accessToken = tokenJson.RootElement.GetProperty("access_token").GetString()
            ?? throw new InvalidOperationException("Keycloak did not return an access token.");
        client.DefaultRequestHeaders.Authorization =
            new AuthenticationHeaderValue("Bearer", accessToken);

        return client;
    }
    catch
    {
        client.Dispose();
        throw;
    }
}
