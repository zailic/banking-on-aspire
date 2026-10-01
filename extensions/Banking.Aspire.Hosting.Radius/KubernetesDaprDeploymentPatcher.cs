using System.ComponentModel;
using System.Diagnostics;
using System.Text.Json;
using System.Text.Json.Nodes;
using Aspire.Hosting.Pipelines;
using Microsoft.Extensions.Logging;

namespace Banking.Aspire.Hosting.Radius;

#pragma warning disable ASPIREPIPELINES001
internal static class KubernetesDaprDeploymentPatcher
{
    // Keep the same internal port on every Dapr workload. 30002 is below Linux's
    // default ephemeral port range, avoiding a race where an outbound connection
    // claims Dapr's default 50002 before daprd can reserve it.
    private const string InternalGrpcPort = "30002";

    public static async Task PatchAsync(
        string deploymentName,
        string kubernetesNamespace,
        string? appProtocol,
        PipelineStepContext context)
    {
        var annotations = new Dictionary<string, string>(StringComparer.Ordinal)
        {
            ["dapr.io/internal-grpc-port"] = InternalGrpcPort
        };
        if (!string.IsNullOrWhiteSpace(appProtocol))
        {
            annotations["dapr.io/app-protocol"] = appProtocol.ToLowerInvariant();
        }

        var patch = JsonSerializer.Serialize(new
        {
            spec = new
            {
                template = new
                {
                    metadata = new { annotations }
                }
            }
        });

        await RunKubectlAsync(
            [
                "patch", "deployment", deploymentName,
                "--namespace", kubernetesNamespace,
                "--type", "merge",
                "--patch", patch
            ],
            standardInput: null,
            context);
        context.Logger.LogInformation(
            "Applied Kubernetes Dapr annotations to deployment '{Namespace}/{Deployment}'",
            kubernetesNamespace,
            deploymentName);

        await RunKubectlAsync(
            [
                "rollout", "status", $"deployment/{deploymentName}",
                "--namespace", kubernetesNamespace,
                "--timeout", "2m"
            ],
            standardInput: null,
            context);
    }

    public static async Task MirrorComponentAsync(
        string componentName,
        string kubernetesNamespace,
        PipelineStepContext context)
    {
        var componentsJson = await RunKubectlAsync(
            ["get", "components.dapr.io", "--all-namespaces", "--output", "json"],
            standardInput: null,
            context,
            logOutput: false);
        var document = JsonNode.Parse(componentsJson)?.AsObject()
            ?? throw new InvalidOperationException("kubectl returned an invalid Dapr component list.");
        var matches = document["items"]?.AsArray()
            .OfType<JsonObject>()
            .Where(component => string.Equals(
                component["metadata"]?["name"]?.GetValue<string>(),
                componentName,
                StringComparison.Ordinal))
            .ToArray() ?? [];
        var sourceMatches = matches
            .Where(component => !string.Equals(
                component["metadata"]?["namespace"]?.GetValue<string>(),
                kubernetesNamespace,
                StringComparison.Ordinal))
            .ToArray();

        if (sourceMatches.Length == 0 && matches.Length == 1)
        {
            context.Logger.LogInformation(
                "Dapr component '{Namespace}/{Component}' is already in the workload namespace",
                kubernetesNamespace,
                componentName);
            return;
        }
        if (sourceMatches.Length != 1)
        {
            throw new InvalidOperationException(
                $"Expected one Radius-managed source for Dapr component '{componentName}' outside " +
                $"namespace '{kubernetesNamespace}', but found {sourceMatches.Length}.");
        }

        var sourceNamespace = sourceMatches[0]["metadata"]?["namespace"]?.GetValue<string>()
            ?? throw new InvalidOperationException(
                $"Dapr component '{componentName}' has no source namespace.");
        var manifest = BuildMirroredComponentManifest(sourceMatches[0], kubernetesNamespace);
        await RunKubectlAsync(
            ["apply", "--filename", "-"],
            manifest,
            context);
        context.Logger.LogInformation(
            "Mirrored Radius Dapr component '{Component}' from namespace '{SourceNamespace}' to '{TargetNamespace}'",
            componentName,
            sourceNamespace,
            kubernetesNamespace);
    }

    internal static string BuildMirroredComponentManifest(
        JsonObject source,
        string kubernetesNamespace)
    {
        var name = source["metadata"]?["name"]?.GetValue<string>()
            ?? throw new InvalidOperationException("The source Dapr component has no name.");
        var spec = source["spec"]?.DeepClone()
            ?? throw new InvalidOperationException($"Dapr component '{name}' has no spec.");
        var manifest = new JsonObject
        {
            ["apiVersion"] = source["apiVersion"]?.DeepClone() ?? "dapr.io/v1alpha1",
            ["kind"] = source["kind"]?.DeepClone() ?? "Component",
            ["metadata"] = new JsonObject
            {
                ["name"] = name,
                ["namespace"] = kubernetesNamespace,
                ["labels"] = new JsonObject
                {
                    ["app.kubernetes.io/managed-by"] = "banking-aspire-radius-dapr"
                }
            },
            ["spec"] = spec
        };
        return manifest.ToJsonString();
    }

    private static async Task<string> RunKubectlAsync(
        IReadOnlyCollection<string> arguments,
        string? standardInput,
        PipelineStepContext context,
        bool logOutput = true)
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
                "The 'kubectl' CLI was not found on PATH. It is required to finalize Dapr deployments.",
                exception);
        }

        if (standardInput is not null)
        {
            await process.StandardInput.WriteAsync(standardInput.AsMemory(), context.CancellationToken);
            process.StandardInput.Close();
        }

        var outputTask = process.StandardOutput.ReadToEndAsync(context.CancellationToken);
        var errorTask = process.StandardError.ReadToEndAsync(context.CancellationToken);
        try
        {
            await process.WaitForExitAsync(context.CancellationToken);
        }
        catch (OperationCanceledException)
        {
            process.Kill(entireProcessTree: true);
            throw;
        }

        var output = await outputTask;
        var error = await errorTask;
        if (logOutput && !string.IsNullOrWhiteSpace(output))
        {
            context.Logger.LogInformation("{KubectlOutput}", output.Trim());
        }

        if (process.ExitCode != 0)
        {
            throw new InvalidOperationException(
                $"kubectl {arguments.First()} exited with code {process.ExitCode}: {error.Trim()}");
        }

        return output;
    }
}
#pragma warning restore ASPIREPIPELINES001
