using System.Diagnostics;
using Microsoft.Extensions.Logging;

internal static class ProcessRunner
{
    public static async Task RunAsync(
        string fileName,
        IEnumerable<string> arguments,
        string workingDirectory,
        IReadOnlyDictionary<string, string> environment,
        ILogger logger,
        CancellationToken cancellationToken)
    {
        var startInfo = new ProcessStartInfo(fileName)
        {
            WorkingDirectory = workingDirectory,
            RedirectStandardOutput = true,
            RedirectStandardError = true,
            UseShellExecute = false
        };

        foreach (var argument in arguments)
        {
            startInfo.ArgumentList.Add(argument);
        }

        foreach (var (name, value) in environment)
        {
            startInfo.Environment[name] = value;
        }

        using var process = new Process { StartInfo = startInfo };
        if (!process.Start())
        {
            throw new InvalidOperationException($"Failed to start '{fileName}'.");
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
        if (!string.IsNullOrWhiteSpace(output))
        {
            logger.LogInformation("{MigrationOutput}", output.Trim());
        }

        if (process.ExitCode != 0)
        {
            throw new InvalidOperationException(
                $"'{fileName}' exited with code {process.ExitCode}: {error.Trim()}");
        }
    }
}
