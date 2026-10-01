using System.Diagnostics;
using System.Text.Json;
using System.Text.Json.Nodes;
using System.Text.RegularExpressions;
using Aspire.Hosting.ApplicationModel;
using Aspire.Hosting.Pipelines;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Logging;

namespace Banking.Aspire.Hosting.Radius;

#pragma warning disable ASPIREPIPELINES001, ASPIREPIPELINES004
internal static partial class RadiusArtifactDeployer
{
    public static async Task DeployAsync(string artifactRelativePath, PipelineStepContext context)
    {
        var outputDirectory = context.Services
            .GetRequiredService<IPipelineOutputService>()
            .GetOutputDirectory();
        var fullPath = Path.Combine(outputDirectory, artifactRelativePath);
        if (!File.Exists(fullPath))
        {
            throw new InvalidOperationException(
                $"Radius artifact '{fullPath}' was not generated. Run '{RadiusReleaseArtifactExtensions.PipelineStepName}' first.");
        }

        var parameterFile = await WriteParameterFileAsync(fullPath, context);
        try
        {
            var arguments = new List<string> { "deploy", fullPath };
            if (parameterFile is not null)
            {
                arguments.Add("--parameters");
                arguments.Add($"@{parameterFile}");
            }

            await RunRadAsync(arguments, Path.GetDirectoryName(fullPath)!, context);
        }
        finally
        {
            DeleteParameterFile(parameterFile, context.Logger);
        }
    }

    private static async Task<string?> WriteParameterFileAsync(
        string artifactPath,
        PipelineStepContext context)
    {
        var bicep = await File.ReadAllTextAsync(artifactPath, context.CancellationToken);
        var requiredParameters = ParameterDeclaration()
            .Matches(bicep)
            .Select(match => match.Groups[1].Value)
            .ToHashSet(StringComparer.Ordinal);

        if (requiredParameters.Count == 0)
        {
            return null;
        }

        var availableParameters = context.Model.Resources
            .OfType<ParameterResource>()
            .GroupBy(parameter => SanitizeIdentifier(parameter.Name), StringComparer.Ordinal)
            .ToDictionary(group => group.Key, group => group.ToArray(), StringComparer.Ordinal);
        var values = new JsonObject();

        foreach (var identifier in requiredParameters.Order(StringComparer.Ordinal))
        {
            if (!availableParameters.TryGetValue(identifier, out var matches) || matches.Length != 1)
            {
                throw new InvalidOperationException(
                    $"Bicep parameter '{identifier}' does not map to exactly one Aspire ParameterResource.");
            }

            var value = await matches[0].GetValueAsync(context.CancellationToken) ?? string.Empty;
            values[identifier] = new JsonObject { ["value"] = value };
        }

        var document = new JsonObject
        {
            ["$schema"] = "https://schema.management.azure.com/schemas/2019-04-01/deploymentParameters.json#",
            ["contentVersion"] = "1.0.0.0",
            ["parameters"] = values
        };

        var directory = Directory.CreateTempSubdirectory("radius-release-");
        var filePath = Path.Combine(directory.FullName, "parameters.json");
        try
        {
            await File.WriteAllTextAsync(
                filePath,
                document.ToJsonString(new JsonSerializerOptions { WriteIndented = true }),
                context.CancellationToken);

            if (!OperatingSystem.IsWindows())
            {
                File.SetUnixFileMode(filePath, UnixFileMode.UserRead | UnixFileMode.UserWrite);
            }

            return filePath;
        }
        catch
        {
            DeleteParameterFile(filePath, context.Logger);
            throw;
        }
    }

    private static async Task RunRadAsync(
        IReadOnlyCollection<string> arguments,
        string workingDirectory,
        PipelineStepContext context)
    {
        var startInfo = new ProcessStartInfo("rad")
        {
            WorkingDirectory = workingDirectory,
            RedirectStandardOutput = true,
            RedirectStandardError = true,
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
                throw new InvalidOperationException("Failed to start the 'rad' CLI.");
            }
        }
        catch (System.ComponentModel.Win32Exception exception)
        {
            throw new InvalidOperationException(
                "The 'rad' CLI was not found on PATH. Install Radius before deploying.",
                exception);
        }

        var standardOutput = process.StandardOutput.ReadToEndAsync(context.CancellationToken);
        var standardError = process.StandardError.ReadToEndAsync(context.CancellationToken);
        try
        {
            await process.WaitForExitAsync(context.CancellationToken);
        }
        catch (OperationCanceledException)
        {
            process.Kill(entireProcessTree: true);
            throw;
        }

        var output = await standardOutput;
        var error = await standardError;
        if (!string.IsNullOrWhiteSpace(output))
        {
            context.Logger.LogInformation("{RadiusOutput}", output.Trim());
        }

        if (process.ExitCode != 0)
        {
            throw new InvalidOperationException(
                $"rad {string.Join(' ', arguments.Take(2))} exited with code {process.ExitCode}: {error.Trim()}");
        }
    }

    private static void DeleteParameterFile(string? filePath, ILogger logger)
    {
        if (filePath is null)
        {
            return;
        }

        try
        {
            var directory = Path.GetDirectoryName(filePath);
            if (directory is not null && Directory.Exists(directory))
            {
                Directory.Delete(directory, recursive: true);
            }
        }
        catch (Exception exception) when (exception is IOException or UnauthorizedAccessException)
        {
            logger.LogWarning(exception, "Failed to delete temporary Radius parameter file '{Path}'", filePath);
        }
    }

    private static string SanitizeIdentifier(string value)
    {
        var identifier = InvalidIdentifierCharacters().Replace(value, "_");
        return identifier.Length > 0 && char.IsDigit(identifier[0]) ? $"_{identifier}" : identifier;
    }

    [GeneratedRegex("(?m)^param\\s+([A-Za-z_][A-Za-z0-9_]*)\\s+")]
    private static partial Regex ParameterDeclaration();

    [GeneratedRegex("[^A-Za-z0-9_]")]
    private static partial Regex InvalidIdentifierCharacters();
}
#pragma warning restore ASPIREPIPELINES001, ASPIREPIPELINES004
