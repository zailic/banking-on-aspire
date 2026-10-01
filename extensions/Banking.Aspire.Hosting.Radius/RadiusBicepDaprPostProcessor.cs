using System.Text;
using System.Text.RegularExpressions;

namespace Banking.Aspire.Hosting.Radius;

public static partial class RadiusBicepDaprPostProcessor
{
    private const string InvalidPubSubRecipe =
        "ghcr.io/radius-project/recipes/local-dev/daprpubsubbrokers:latest";
    private const string PubSubRecipe =
        "ghcr.io/radius-project/recipes/local-dev/pubsubbrokers:latest";

    public static string Apply(
        string bicep,
        IReadOnlyDictionary<string, RadiusDaprSidecarSettings> sidecars)
    {
        ArgumentNullException.ThrowIfNull(bicep);
        ArgumentNullException.ThrowIfNull(sidecars);

        // Aspire.Hosting.Radius 13.5 preview points at an OCI artifact name that
        // does not exist. Radius' published local-dev recipe omits the "dapr"
        // prefix. Keep the workaround beside the CLR resource-name shim.
        var result = bicep.Replace(
            InvalidPubSubRecipe,
            PubSubRecipe,
            StringComparison.Ordinal);
        foreach (var (resourceName, sidecar) in sidecars)
        {
            result = ApplyToResource(result, resourceName, sidecar);
        }

        return result;
    }

    private static string ApplyToResource(
        string bicep,
        string resourceName,
        RadiusDaprSidecarSettings sidecar)
    {
        foreach (Match match in RadiusContainerDeclaration().Matches(bicep))
        {
            var resourceOpenBrace = bicep.IndexOf('{', match.Index, match.Length);
            var resourceCloseBrace = FindMatchingBrace(bicep, resourceOpenBrace);
            var resourceBlock = bicep[resourceOpenBrace..(resourceCloseBrace + 1)];

            if (!HasResourceName(resourceBlock, resourceName))
            {
                continue;
            }

            if (DaprSidecarProperty().IsMatch(resourceBlock))
            {
                return bicep;
            }

            if (ExtensionsProperty().IsMatch(resourceBlock))
            {
                throw new InvalidOperationException(
                    $"Radius workload '{resourceName}' already contains an extensions object. " +
                    "Update the Dapr adapter to merge with the publisher-owned object instead of emitting a duplicate property.");
            }

            var propertiesMatch = PropertiesObject().Match(resourceBlock);
            if (!propertiesMatch.Success)
            {
                throw new InvalidOperationException(
                    $"Radius workload '{resourceName}' does not contain a properties object.");
            }

            var propertiesOpenBrace = resourceOpenBrace + propertiesMatch.Index + propertiesMatch.Length - 1;
            var insertionIndex = propertiesOpenBrace + 1;
            var newline = bicep.Contains("\r\n", StringComparison.Ordinal) ? "\r\n" : "\n";
            var extension = RenderExtension(sidecar, newline);

            return bicep.Insert(insertionIndex, newline + extension);
        }

        throw new InvalidOperationException(
            $"Could not find Radius.Compute/containers workload '{resourceName}' in the generated Bicep.");
    }

    private static bool HasResourceName(string resourceBlock, string resourceName)
    {
        var match = ResourceNameProperty().Match(resourceBlock);
        return match.Success &&
               string.Equals(UnescapeBicepString(match.Groups[1].Value), resourceName, StringComparison.Ordinal);
    }

    private static string RenderExtension(RadiusDaprSidecarSettings sidecar, string newline)
    {
        var builder = new StringBuilder()
            .Append("    extensions: {").Append(newline)
            .Append("      daprSidecar: {").Append(newline)
            .Append("        appId: '").Append(EscapeBicepString(sidecar.AppId)).Append('\'').Append(newline);

        if (sidecar.AppPort is int appPort)
        {
            builder.Append("        appPort: ").Append(appPort).Append(newline);
        }

        if (!string.IsNullOrWhiteSpace(sidecar.Config))
        {
            builder.Append("        config: '")
                .Append(EscapeBicepString(sidecar.Config))
                .Append('\'').Append(newline);
        }

        return builder
            .Append("      }").Append(newline)
            .Append("    }")
            .ToString();
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

    private static string EscapeBicepString(string value) =>
        value.Replace("\\", "\\\\", StringComparison.Ordinal)
            .Replace("'", "\\'", StringComparison.Ordinal);

    private static string UnescapeBicepString(string value) =>
        value.Replace("\\'", "'", StringComparison.Ordinal)
            .Replace("\\\\", "\\", StringComparison.Ordinal);

    [GeneratedRegex("(?m)^resource\\s+[A-Za-z_][A-Za-z0-9_]*\\s+'Radius\\.Compute/containers@[^']+'\\s*=\\s*\\{")]
    private static partial Regex RadiusContainerDeclaration();

    [GeneratedRegex("(?m)^\\s{2}name:\\s*'((?:\\\\.|[^'])*)'\\s*$")]
    private static partial Regex ResourceNameProperty();

    [GeneratedRegex("(?m)^\\s{2}properties:\\s*\\{")]
    private static partial Regex PropertiesObject();

    [GeneratedRegex("(?m)^\\s+daprSidecar:\\s*\\{")]
    private static partial Regex DaprSidecarProperty();

    [GeneratedRegex("(?m)^\\s{4}extensions:\\s*\\{")]
    private static partial Regex ExtensionsProperty();
}
