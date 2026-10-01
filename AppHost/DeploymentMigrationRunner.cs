using System.ComponentModel;
using System.Diagnostics;
using System.Text.Json;
using Aspire.Hosting.ApplicationModel;
using Aspire.Hosting.Pipelines;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Options;

internal static class DeploymentMigrationRunner
{
    private static readonly SemaphoreSlim MigrationLock = new(1, 1);
    private static readonly HashSet<string> CompletedMigrations = new(StringComparer.Ordinal);
    private static bool DatabasesEnsured;
    private static bool PostgresAliasEnsured;
    private static readonly IReadOnlyDictionary<string, MigrationDefinition> Definitions =
        new Dictionary<string, MigrationDefinition>(StringComparer.Ordinal)
        {
            ["contacts"] = new("contactsmigrate", UsersDatabase: true, TransactionsDatabase: false, []),
            ["users"] = new("usersmigrate", UsersDatabase: true, TransactionsDatabase: false, []),
            ["accounts"] = new(
                "accountsmigrate",
                UsersDatabase: true,
                TransactionsDatabase: false,
                ["users", "contacts"]),
            ["transactions"] = new(
                "transactionsmigrate",
                UsersDatabase: true,
                TransactionsDatabase: true,
                ["users"])
        };

#pragma warning disable ASPIREPIPELINES001
    public static async Task RunAsync(
        string serviceName,
        string kubernetesNamespace,
        ParameterResource postgresPassword,
        PipelineStepContext context)
    {
        await MigrationLock.WaitAsync(context.CancellationToken);
        try
        {
            var selectedStep = context.Services
                .GetRequiredService<IOptions<PipelineOptions>>()
                .Value.Step;
            var isFullDeployment = string.IsNullOrWhiteSpace(selectedStep) ||
                                   string.Equals(selectedStep, WellKnownPipelineSteps.Deploy, StringComparison.Ordinal);

            if (!HasPendingMigration(serviceName, includePrerequisites: isFullDeployment))
            {
                context.Logger.LogInformation(
                    "Deployment migration for '{ServiceName}' already completed in this pipeline run",
                    serviceName);
                return;
            }

            var password = await postgresPassword.GetValueAsync(context.CancellationToken)
                           ?? throw new InvalidOperationException("The PostgreSQL password is not configured.");

            var postgresServiceName = await FindPostgresServiceAsync(
                kubernetesNamespace,
                context.Logger,
                context.CancellationToken);
            await EnsurePostgresAliasServiceAsync(
                kubernetesNamespace,
                context.Logger,
                context.CancellationToken);

            await using var portForward = await KubernetesPortForward.StartAsync(
                kubernetesNamespace,
                postgresServiceName,
                5432,
                context.Logger,
                context.CancellationToken);

            await EnsureDatabasesAsync(
                kubernetesNamespace,
                postgresServiceName,
                password,
                context.Logger,
                context.CancellationToken);

            await RunOnceAsync(
                serviceName,
                includePrerequisites: isFullDeployment,
                password,
                portForward.LocalPort,
                context);
        }
        finally
        {
            MigrationLock.Release();
        }
    }
#pragma warning restore ASPIREPIPELINES001

    private static async Task EnsurePostgresAliasServiceAsync(
        string kubernetesNamespace,
        ILogger logger,
        CancellationToken cancellationToken)
    {
        if (PostgresAliasEnsured)
        {
            return;
        }

        var service = JsonSerializer.Serialize(new
        {
            apiVersion = "v1",
            kind = "Service",
            metadata = new { name = "postgres-postgres", @namespace = kubernetesNamespace },
            spec = new
            {
                selector = new { app = "postgresql", resource = "postgres" },
                ports = new[] { new { name = "postgres", port = 5432, targetPort = 5432 } }
            }
        });

        await RunKubectlAsync(
            ["apply", "--filename", "-"],
            cancellationToken,
            service);
        PostgresAliasEnsured = true;
        logger.LogInformation(
            "Ensured stable PostgreSQL service alias '{Namespace}/postgres-postgres'",
            kubernetesNamespace);
    }

    private static async Task EnsureDatabasesAsync(
        string kubernetesNamespace,
        string postgresWorkloadName,
        string password,
        ILogger logger,
        CancellationToken cancellationToken)
    {
        if (DatabasesEnsured)
        {
            return;
        }

        var escapedPassword = password.Replace("'", "''", StringComparison.Ordinal);
        await RunKubectlAsync(
            [
                "exec", "--stdin", "--namespace", kubernetesNamespace,
                $"deployment/{postgresWorkloadName}", "--",
                "psql", "-U", "postgres", "-d", "postgres"
            ],
            cancellationToken,
            $"ALTER ROLE postgres WITH PASSWORD '{escapedPassword}';\n");
        logger.LogInformation("Synchronized the PostgreSQL deployment credential used by Aspire workloads");

        foreach (var databaseName in new[] { "usersdb", "transactionsdb" })
        {
            var exists = await RunKubectlAsync(
                [
                    "exec", "--namespace", kubernetesNamespace,
                    $"deployment/{postgresWorkloadName}", "--",
                    "psql", "-U", "postgres", "-d", "postgres", "-tAc",
                    $"SELECT 1 FROM pg_database WHERE datname = '{databaseName}'"
                ],
                cancellationToken);

            if (string.Equals(exists.Trim(), "1", StringComparison.Ordinal))
            {
                continue;
            }

            logger.LogInformation("Creating PostgreSQL database '{DatabaseName}'", databaseName);
            await RunKubectlAsync(
                [
                    "exec", "--namespace", kubernetesNamespace,
                    $"deployment/{postgresWorkloadName}", "--",
                    "createdb", "-U", "postgres", databaseName
                ],
                cancellationToken);
        }

        DatabasesEnsured = true;
    }

    private static async Task<string> RunKubectlAsync(
        IReadOnlyList<string> arguments,
        CancellationToken cancellationToken,
        string? standardInput = null)
    {
        var startInfo = new ProcessStartInfo("kubectl")
        {
            RedirectStandardOutput = true,
            RedirectStandardError = true,
            RedirectStandardInput = standardInput is not null,
            UseShellExecute = false,
            CreateNoWindow = true
        };
        foreach (var argument in arguments)
        {
            startInfo.ArgumentList.Add(argument);
        }

        using var process = new Process { StartInfo = startInfo };
        try
        {
            if (!process.Start())
            {
                throw new InvalidOperationException("Failed to start 'kubectl'.");
            }
        }
        catch (Win32Exception exception)
        {
            throw new InvalidOperationException(
                "The 'kubectl' CLI was not found on PATH. It is required for deployment migrations.",
                exception);
        }

        if (standardInput is not null)
        {
            await process.StandardInput.WriteAsync(standardInput.AsMemory(), cancellationToken);
            process.StandardInput.Close();
        }

        var standardOutput = process.StandardOutput.ReadToEndAsync(cancellationToken);
        var standardError = process.StandardError.ReadToEndAsync(cancellationToken);
        await process.WaitForExitAsync(cancellationToken);
        var output = await standardOutput;
        var error = await standardError;
        if (process.ExitCode != 0)
        {
            throw new InvalidOperationException(
                $"'kubectl {arguments[0]}' exited with code {process.ExitCode}: {error.Trim()}");
        }

        return output;
    }

    private static async Task<string> FindPostgresServiceAsync(
        string kubernetesNamespace,
        ILogger logger,
        CancellationToken cancellationToken)
    {
        var deadline = DateTimeOffset.UtcNow.AddMinutes(2);
        string? lastError = null;
        var waitingLogged = false;

        while (DateTimeOffset.UtcNow < deadline)
        {
            var startInfo = new ProcessStartInfo("kubectl")
            {
                RedirectStandardOutput = true,
                RedirectStandardError = true,
                UseShellExecute = false,
                CreateNoWindow = true
            };
            foreach (var argument in new[]
                     {
                         "get", "services", "--namespace", kubernetesNamespace, "--output", "json"
                     })
            {
                startInfo.ArgumentList.Add(argument);
            }

            using var process = new Process { StartInfo = startInfo };
            try
            {
                if (!process.Start())
                {
                    throw new InvalidOperationException("Failed to start 'kubectl get services'.");
                }
            }
            catch (Win32Exception exception)
            {
                throw new InvalidOperationException(
                    "The 'kubectl' CLI was not found on PATH. It is required for deployment migrations.",
                    exception);
            }

            var standardOutput = process.StandardOutput.ReadToEndAsync(cancellationToken);
            var standardError = process.StandardError.ReadToEndAsync(cancellationToken);
            await process.WaitForExitAsync(cancellationToken);

            var output = await standardOutput;
            lastError = (await standardError).Trim();
            if (process.ExitCode == 0)
            {
                using var document = JsonDocument.Parse(output);
                foreach (var service in document.RootElement.GetProperty("items").EnumerateArray())
                {
                    if (!service.TryGetProperty("spec", out var spec) ||
                        !spec.TryGetProperty("selector", out var selector) ||
                        !selector.TryGetProperty("resource", out var resource) ||
                        !string.Equals(resource.GetString(), "postgres", StringComparison.Ordinal))
                    {
                        continue;
                    }

                    var exposesPostgres = spec.TryGetProperty("ports", out var ports) &&
                                          ports.EnumerateArray().Any(port =>
                                              port.TryGetProperty("port", out var value) &&
                                              value.GetInt32() == 5432);
                    if (exposesPostgres)
                    {
                        var serviceName = service.GetProperty("metadata").GetProperty("name").GetString();
                        if (!string.IsNullOrWhiteSpace(serviceName) &&
                            !string.Equals(serviceName, "postgres-postgres", StringComparison.Ordinal))
                        {
                            logger.LogInformation(
                                "Found PostgreSQL service '{Namespace}/{ServiceName}' generated by Radius",
                                kubernetesNamespace,
                                serviceName);
                            return serviceName;
                        }
                    }
                }
            }

            if (!waitingLogged)
            {
                logger.LogInformation(
                    "Waiting for the Radius PostgreSQL recipe to create its Kubernetes service in namespace '{Namespace}'",
                    kubernetesNamespace);
                waitingLogged = true;
            }

            await Task.Delay(TimeSpan.FromSeconds(2), cancellationToken);
        }

        var detail = string.IsNullOrWhiteSpace(lastError) ? string.Empty : $" Last kubectl error: {lastError}";
        throw new TimeoutException(
            $"The Radius PostgreSQL service did not become available in namespace '{kubernetesNamespace}' within 2 minutes.{detail}");
    }

    private static string CreatePostgresUri(string password, int port, string database) =>
        $"postgresql://postgres:{Uri.EscapeDataString(password)}@127.0.0.1:{port}/{database}?sslmode=disable";

    private static bool HasPendingMigration(string serviceName, bool includePrerequisites)
    {
        if (!Definitions.TryGetValue(serviceName, out var definition))
        {
            throw new InvalidOperationException($"No deployment migration is registered for '{serviceName}'.");
        }

        return !CompletedMigrations.Contains(serviceName) ||
               includePrerequisites && definition.Prerequisites.Any(prerequisite =>
                   HasPendingMigration(prerequisite, includePrerequisites: true));
    }

#pragma warning disable ASPIREPIPELINES001
    private static async Task RunOnceAsync(
        string serviceName,
        bool includePrerequisites,
        string password,
        int port,
        PipelineStepContext context)
    {
        if (!Definitions.TryGetValue(serviceName, out var definition))
        {
            throw new InvalidOperationException($"No deployment migration is registered for '{serviceName}'.");
        }

        if (includePrerequisites)
        {
            foreach (var prerequisite in definition.Prerequisites)
            {
                await RunOnceAsync(prerequisite, includePrerequisites: true, password, port, context);
            }
        }

        if (!CompletedMigrations.Add(serviceName))
        {
            context.Logger.LogInformation(
                "Deployment migration for '{ServiceName}' already completed in this pipeline run",
                serviceName);
            return;
        }

        var environment = new Dictionary<string, string>(StringComparer.Ordinal);
        if (definition.UsersDatabase)
        {
            environment["USERSDB_URI"] = CreatePostgresUri(password, port, "usersdb");
        }

        if (definition.TransactionsDatabase)
        {
            environment["TRANSACTIONSDB_URI"] = CreatePostgresUri(password, port, "transactionsdb");
        }

        try
        {
            context.Logger.LogInformation("Running deployment migration for '{ServiceName}'", serviceName);
            await ProcessRunner.RunAsync(
                "go",
                ["run", "-buildvcs=false", $"./cmd/{definition.CommandName}"],
                Path.GetFullPath($"services/{serviceName}"),
                environment,
                context.Logger,
                context.CancellationToken);
        }
        catch
        {
            CompletedMigrations.Remove(serviceName);
            throw;
        }
    }
#pragma warning restore ASPIREPIPELINES001

    private sealed record MigrationDefinition(
        string CommandName,
        bool UsersDatabase,
        bool TransactionsDatabase,
        IReadOnlyList<string> Prerequisites);

}
