using System.ComponentModel;
using System.Diagnostics;
using System.Net;
using System.Net.Sockets;
using System.Text;
using Microsoft.Extensions.Logging;

internal sealed class KubernetesPortForward : IAsyncDisposable
{
    private readonly Process _process;
    private readonly ILogger _logger;
    private readonly StringBuilder _output;

    private KubernetesPortForward(Process process, int localPort, ILogger logger, StringBuilder output)
    {
        _process = process;
        LocalPort = localPort;
        _logger = logger;
        _output = output;
    }

    public int LocalPort { get; }

    public static async Task<KubernetesPortForward> StartAsync(
        string kubernetesNamespace,
        string serviceName,
        int remotePort,
        ILogger logger,
        CancellationToken cancellationToken)
    {
        var localPort = AllocateLocalPort();
        var startInfo = new ProcessStartInfo("kubectl")
        {
            RedirectStandardOutput = true,
            RedirectStandardError = true,
            UseShellExecute = false,
            CreateNoWindow = true
        };
        startInfo.ArgumentList.Add("port-forward");
        startInfo.ArgumentList.Add("--namespace");
        startInfo.ArgumentList.Add(kubernetesNamespace);
        startInfo.ArgumentList.Add($"service/{serviceName}");
        startInfo.ArgumentList.Add($"{localPort}:{remotePort}");
        startInfo.ArgumentList.Add("--address");
        startInfo.ArgumentList.Add("127.0.0.1");

        var process = new Process { StartInfo = startInfo, EnableRaisingEvents = true };
        var ready = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        var exited = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        var output = new StringBuilder();

        void CaptureLine(string? line)
        {
            if (string.IsNullOrWhiteSpace(line))
            {
                return;
            }

            lock (output)
            {
                output.AppendLine(line);
            }

            if (line.Contains("Forwarding from", StringComparison.OrdinalIgnoreCase))
            {
                ready.TrySetResult();
            }
        }

        process.OutputDataReceived += (_, args) => CaptureLine(args.Data);
        process.ErrorDataReceived += (_, args) => CaptureLine(args.Data);
        process.Exited += (_, _) => exited.TrySetResult();

        try
        {
            if (!process.Start())
            {
                throw new InvalidOperationException("Failed to start 'kubectl port-forward'.");
            }
        }
        catch (Win32Exception exception)
        {
            process.Dispose();
            throw new InvalidOperationException(
                "The 'kubectl' CLI was not found on PATH. It is required for deployment tasks.",
                exception);
        }

        process.BeginOutputReadLine();
        process.BeginErrorReadLine();

        using var timeout = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
        timeout.CancelAfter(TimeSpan.FromSeconds(60));
        try
        {
            var completed = await Task.WhenAny(ready.Task, exited.Task).WaitAsync(timeout.Token);
            if (completed == exited.Task)
            {
                throw new InvalidOperationException(
                    $"kubectl port-forward exited before the service became reachable: {ReadOutput(output)}");
            }

            logger.LogInformation(
                "Forwarding Kubernetes service '{Namespace}/{ServiceName}' to 127.0.0.1:{Port}",
                kubernetesNamespace,
                serviceName,
                localPort);
            return new KubernetesPortForward(process, localPort, logger, output);
        }
        catch
        {
            Stop(process);
            process.Dispose();
            throw;
        }
    }

    public ValueTask DisposeAsync()
    {
        Stop(_process);
        var output = ReadOutput(_output);
        if (!string.IsNullOrWhiteSpace(output))
        {
            _logger.LogDebug("kubectl port-forward output: {Output}", output);
        }

        _process.Dispose();
        return ValueTask.CompletedTask;
    }

    private static int AllocateLocalPort()
    {
        var listener = new TcpListener(IPAddress.Loopback, 0);
        listener.Start();
        try
        {
            return ((IPEndPoint)listener.LocalEndpoint).Port;
        }
        finally
        {
            listener.Stop();
        }
    }

    private static void Stop(Process process)
    {
        if (!process.HasExited)
        {
            process.Kill(entireProcessTree: true);
            process.WaitForExit();
        }
    }

    private static string ReadOutput(StringBuilder output)
    {
        lock (output)
        {
            return output.ToString().Trim();
        }
    }
}
