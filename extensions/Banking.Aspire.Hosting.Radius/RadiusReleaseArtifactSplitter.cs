using System.Text;
using System.Text.RegularExpressions;

namespace Banking.Aspire.Hosting.Radius;

public sealed record RadiusReleaseArtifacts(
    string Infrastructure,
    IReadOnlyDictionary<string, string> Workloads);

public static partial class RadiusReleaseArtifactSplitter
{
    public static RadiusReleaseArtifacts Split(string source, IReadOnlyCollection<string> workloadNames)
    {
        ArgumentNullException.ThrowIfNull(source);
        ArgumentNullException.ThrowIfNull(workloadNames);

        var document = Parse(source);
        var workloadSet = workloadNames.ToHashSet(StringComparer.Ordinal);
        var workloadsByName = document.Resources
            .Where(resource => resource.Type.StartsWith("Radius.Compute/containers@", StringComparison.Ordinal))
            .ToDictionary(resource => resource.Name, StringComparer.Ordinal);

        foreach (var workloadName in workloadSet)
        {
            if (!workloadsByName.ContainsKey(workloadName))
            {
                throw new InvalidOperationException(
                    $"Could not find Radius.Compute/containers workload '{workloadName}' in the generated Bicep.");
            }
        }

        var infrastructureBody = RemoveRanges(
            source,
            document.Resources
                .Where(resource =>
                    workloadSet.Contains(resource.Name) &&
                    resource.Type.StartsWith("Radius.Compute/containers@", StringComparison.Ordinal))
                .Select(resource => (resource.Start, resource.End)));
        var infrastructure = KeepUsedParameters(infrastructureBody, document.Parameters);

        var workloads = new Dictionary<string, string>(StringComparer.Ordinal);
        foreach (var workloadName in workloadSet.Order(StringComparer.Ordinal))
        {
            var workload = workloadsByName[workloadName];
            workloads.Add(workloadName, CreateWorkloadArtifact(source, document, workload));
        }

        return new RadiusReleaseArtifacts(infrastructure, workloads);
    }

    private static string CreateWorkloadArtifact(string source, BicepDocument document, BicepResource workload)
    {
        var newline = source.Contains("\r\n", StringComparison.Ordinal) ? "\r\n" : "\n";
        var requiredSymbols = SymbolReference()
            .Matches(workload.Text)
            .Select(match => match.Groups[1].Value)
            .ToHashSet(StringComparer.Ordinal);
        var referencedResources = document.Resources
            .Where(resource => requiredSymbols.Contains(resource.Symbol))
            .OrderBy(resource => resource.Start)
            .ToArray();

        var contentForParameterAnalysis = string.Join(newline, referencedResources.Select(resource => resource.Text)) +
                                          newline + workload.Text;
        var parameters = document.Parameters
            .Where(parameter => ContainsSymbol(contentForParameterAnalysis, parameter.Symbol))
            .Select(parameter => parameter.Text.TrimEnd())
            .ToArray();

        var builder = new StringBuilder()
            .Append("extension radius").Append(newline).Append(newline);

        if (parameters.Length > 0)
        {
            builder.Append(string.Join(newline + newline, parameters)).Append(newline).Append(newline);
        }

        foreach (var dependency in referencedResources)
        {
            builder
                .Append("resource ").Append(dependency.Symbol)
                .Append(" '").Append(dependency.Type).Append("' existing = {").Append(newline)
                .Append("  name: '").Append(EscapeBicepString(dependency.Name)).Append('\'').Append(newline)
                .Append('}').Append(newline).Append(newline);
        }

        builder.Append(workload.Text.Trim()).Append(newline);
        return builder.ToString();
    }

    private static string KeepUsedParameters(string source, IReadOnlyList<BicepParameter> parameters)
    {
        var result = source;
        foreach (var parameter in parameters.OrderByDescending(parameter => parameter.Start))
        {
            var contentWithoutDeclaration = result.Remove(parameter.Start, parameter.End - parameter.Start);
            if (!ContainsSymbol(contentWithoutDeclaration, parameter.Symbol))
            {
                result = contentWithoutDeclaration;
            }
        }

        return NormalizeBlankLines(result);
    }

    private static string RemoveRanges(string source, IEnumerable<(int Start, int End)> ranges)
    {
        var result = source;
        foreach (var (start, end) in ranges.OrderByDescending(range => range.Start))
        {
            result = result.Remove(start, end - start);
        }

        return NormalizeBlankLines(result);
    }

    private static BicepDocument Parse(string source)
    {
        var resources = new List<BicepResource>();
        foreach (Match match in ResourceDeclaration().Matches(source))
        {
            var openBrace = source.IndexOf('{', match.Index, match.Length);
            var closeBrace = FindMatchingBrace(source, openBrace);
            var end = ConsumeTrailingNewlines(source, closeBrace + 1);
            var text = source[match.Index..end];
            var nameMatch = ResourceNameProperty().Match(text);
            if (!nameMatch.Success)
            {
                throw new InvalidOperationException(
                    $"Radius resource '{match.Groups[1].Value}' does not have a literal name.");
            }

            resources.Add(new BicepResource(
                match.Groups[1].Value,
                match.Groups[2].Value,
                UnescapeBicepString(nameMatch.Groups[1].Value),
                match.Index,
                end,
                text));
        }

        var parameters = ParameterDeclaration().Matches(source)
            .Select(match => new BicepParameter(
                match.Groups[2].Value,
                match.Index,
                match.Index + match.Length,
                match.Value))
            .ToArray();

        return new BicepDocument(resources, parameters);
    }

    private static int FindMatchingBrace(string text, int openBrace)
    {
        var depth = 0;
        var inString = false;

        for (var index = openBrace; index < text.Length; index++)
        {
            var current = text[index];
            if (inString)
            {
                if (current == '\\')
                {
                    index++;
                }
                else if (current == '\'')
                {
                    inString = false;
                }

                continue;
            }

            if (current == '\'')
            {
                inString = true;
            }
            else if (current == '{')
            {
                depth++;
            }
            else if (current == '}' && --depth == 0)
            {
                return index;
            }
        }

        throw new InvalidOperationException("Generated Radius Bicep contains an unbalanced resource block.");
    }

    private static int ConsumeTrailingNewlines(string source, int index)
    {
        while (index < source.Length && (source[index] == '\r' || source[index] == '\n'))
        {
            index++;
        }

        return index;
    }

    private static bool ContainsSymbol(string text, string symbol) =>
        Regex.IsMatch(text, $@"(?<![A-Za-z0-9_]){Regex.Escape(symbol)}(?![A-Za-z0-9_])");

    private static string NormalizeBlankLines(string value) =>
        ExcessBlankLines().Replace(value.TrimEnd() + Environment.NewLine, Environment.NewLine + Environment.NewLine);

    private static string EscapeBicepString(string value) => value.Replace("'", "\\'", StringComparison.Ordinal);

    private static string UnescapeBicepString(string value) => value.Replace("\\'", "'", StringComparison.Ordinal);

    [GeneratedRegex("(?m)^resource\\s+([A-Za-z_][A-Za-z0-9_]*)\\s+'([^']+)'(?:\\s+existing)?\\s*=\\s*\\{")]
    private static partial Regex ResourceDeclaration();

    [GeneratedRegex("(?m)^\\s{2}name:\\s*'((?:\\\\.|[^'])*)'\\s*$")]
    private static partial Regex ResourceNameProperty();

    [GeneratedRegex("(?m)((?:^@[^\\r\\n]+\\r?\\n)*)^param\\s+([A-Za-z_][A-Za-z0-9_]*)[^\\r\\n]*(?:\\r?\\n){1,2}")]
    private static partial Regex ParameterDeclaration();

    [GeneratedRegex("(?<![A-Za-z0-9_])([A-Za-z_][A-Za-z0-9_]*)\\.id(?![A-Za-z0-9_])")]
    private static partial Regex SymbolReference();

    [GeneratedRegex("(?:\\r?\\n){3,}")]
    private static partial Regex ExcessBlankLines();

    private sealed record BicepDocument(
        IReadOnlyList<BicepResource> Resources,
        IReadOnlyList<BicepParameter> Parameters);

    private sealed record BicepResource(
        string Symbol,
        string Type,
        string Name,
        int Start,
        int End,
        string Text);

    private sealed record BicepParameter(string Symbol, int Start, int End, string Text);
}
