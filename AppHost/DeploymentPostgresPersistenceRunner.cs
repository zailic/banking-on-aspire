using System.ComponentModel;
using System.Diagnostics;
using System.Text.Json;
using Aspire.Hosting.Pipelines;
using Microsoft.Extensions.Logging;

internal static class DeploymentPostgresPersistenceRunner
{
#pragma warning disable ASPIREPIPELINES001
    public static async Task RunAsync(
        string kubernetesNamespace,
        PipelineStepContext context)
    {
        var deploymentNames = (await RunKubectlAsync(
            [
                "get", "deployments", "--namespace", kubernetesNamespace,
                "--output",
                "jsonpath={range .items[?(@.spec.template.metadata.labels.resource==\"postgres\")]}{.metadata.name}{\"\\n\"}{end}"
            ],
            standardInput: null,
            context.CancellationToken))
            .Split('\n', StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries);
        if (deploymentNames.Length != 1)
        {
            throw new InvalidOperationException(
                $"Expected one Radius PostgreSQL deployment in namespace '{kubernetesNamespace}', " +
                $"but found {deploymentNames.Length}.");
        }
        var deploymentName = deploymentNames[0];

        await EnsurePersistentStorageAsync(
            kubernetesNamespace,
            deploymentName,
            containerName: "postgres",
            claimName: "boa-postgres-data",
            volumeName: "postgres-data",
            mountPath: "/var/lib/postgresql/data",
            context);
    }

    public static Task RunKeycloakAsync(
        string kubernetesNamespace,
        PipelineStepContext context) =>
        EnsurePersistentStorageAsync(
            kubernetesNamespace,
            deploymentName: "keycloak",
            containerName: "keycloak",
            claimName: "boa-keycloak-data",
            volumeName: "keycloak-data",
            mountPath: "/opt/keycloak/data",
            context);

    private static async Task EnsurePersistentStorageAsync(
        string kubernetesNamespace,
        string deploymentName,
        string containerName,
        string claimName,
        string volumeName,
        string mountPath,
        PipelineStepContext context)
    {
        var claim = JsonSerializer.Serialize(new
        {
            apiVersion = "v1",
            kind = "PersistentVolumeClaim",
            metadata = new { name = claimName, @namespace = kubernetesNamespace },
            spec = new
            {
                accessModes = new[] { "ReadWriteOnce" },
                resources = new
                {
                    requests = new Dictionary<string, string> { ["storage"] = "5Gi" }
                }
            }
        });
        await RunKubectlAsync(
            ["apply", "--filename", "-"],
            claim,
            context.CancellationToken);

        var patch = JsonSerializer.Serialize(new
        {
            spec = new
            {
                template = new
                {
                    spec = new
                    {
                        containers = new[]
                        {
                            new
                            {
                                name = containerName,
                                volumeMounts = new[]
                                {
                                    new
                                    {
                                        name = volumeName,
                                        mountPath
                                    }
                                }
                            }
                        },
                        volumes = new[]
                        {
                            new
                            {
                                name = volumeName,
                                persistentVolumeClaim = new { claimName }
                            }
                        }
                    }
                }
            }
        });
        await RunKubectlAsync(
            [
                "patch", "deployment", deploymentName,
                "--namespace", kubernetesNamespace,
                "--type", "strategic",
                "--patch", patch
            ],
            standardInput: null,
            context.CancellationToken);
        await RunKubectlAsync(
            [
                "rollout", "status", $"deployment/{deploymentName}",
                "--namespace", kubernetesNamespace,
                "--timeout", "2m"
            ],
            standardInput: null,
            context.CancellationToken);

        context.Logger.LogInformation(
            "Ensured persistent storage '{Namespace}/{Claim}' on deployment '{Deployment}'",
            kubernetesNamespace,
            claimName,
            deploymentName);
    }
#pragma warning restore ASPIREPIPELINES001

    private static async Task<string> RunKubectlAsync(
        IReadOnlyCollection<string> arguments,
        string? standardInput,
        CancellationToken cancellationToken)
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
                "The 'kubectl' CLI was not found on PATH. It is required to configure PostgreSQL persistence.",
                exception);
        }

        if (standardInput is not null)
        {
            await process.StandardInput.WriteAsync(standardInput.AsMemory(), cancellationToken);
            process.StandardInput.Close();
        }

        var standardOutput = process.StandardOutput.ReadToEndAsync(cancellationToken);
        var standardError = process.StandardError.ReadToEndAsync(cancellationToken);
        try
        {
            await process.WaitForExitAsync(cancellationToken);
        }
        catch (OperationCanceledException)
        {
            process.Kill(entireProcessTree: true);
            throw;
        }

        var output = await standardOutput;
        var error = await standardError;
        if (process.ExitCode != 0)
        {
            throw new InvalidOperationException(
                $"'kubectl {arguments.First()}' exited with code {process.ExitCode}: {error.Trim()}");
        }

        return output;
    }
}
