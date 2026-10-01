using Banking.Aspire.Hosting.Radius;
using Xunit;

namespace Banking.Aspire.Hosting.Radius.Tests;

public sealed class RadiusReleaseArtifactSplitterTests
{
    private const string Source = """
        extension radius

        @secure()
        param postgres_password string

        @secure()
        param web_secret string

        resource radiusenv 'Radius.Core/environments@2025-08-01-preview' = {
          name: 'radius'
          properties: {}
        }

        resource radiusenv_legacy 'Applications.Core/environments@2023-10-01-preview' = {
          name: 'radius'
          properties: {}
        }

        resource app 'Radius.Core/applications@2025-08-01-preview' = {
          name: 'app'
          properties: {
            environment: radiusenv.id
          }
        }

        resource postgres 'Radius.Data/postgreSqlDatabases@2025-08-01-preview' = {
          name: 'postgres'
          properties: {
            password: postgres_password
            application: app.id
            environment: radiusenv.id
          }
        }

        resource accounts 'Radius.Compute/containers@2025-08-01-preview' = {
          name: 'accounts'
          properties: {
            containers: {
              accounts: {
                image: 'accounts:1.0.0'
                env: {
                  USERSDB_PASSWORD: {
                    value: postgres_password
                  }
                }
              }
            }
            application: app.id
            environment: radiusenv.id
            connections: {
              postgres: {
                source: postgres.id
              }
            }
          }
        }

        resource web 'Radius.Compute/containers@2025-08-01-preview' = {
          name: 'web'
          properties: {
            containers: {
              web: {
                image: 'web:1.0.0'
                env: {
                  SECRET: {
                    value: web_secret
                  }
                }
              }
            }
            application: app.id
            environment: radiusenv.id
          }
        }
        """;

    [Fact]
    public void SplitKeepsInfrastructureAndCreatesIndependentWorkloads()
    {
        var result = RadiusReleaseArtifactSplitter.Split(Source, ["accounts", "web"]);

        Assert.Contains("resource postgres", result.Infrastructure);
        Assert.DoesNotContain("resource accounts", result.Infrastructure);
        Assert.DoesNotContain("resource web", result.Infrastructure);
        Assert.Contains("param postgres_password", result.Infrastructure);
        Assert.DoesNotContain("param web_secret", result.Infrastructure);

        var accounts = result.Workloads["accounts"];
        Assert.Contains("resource app 'Radius.Core/applications@2025-08-01-preview' existing", accounts);
        Assert.Contains("resource radiusenv 'Radius.Core/environments@2025-08-01-preview' existing", accounts);
        Assert.Contains("resource postgres 'Radius.Data/postgreSqlDatabases@2025-08-01-preview' existing", accounts);
        Assert.Contains("param postgres_password", accounts);
        Assert.DoesNotContain("param web_secret", accounts);
        Assert.DoesNotContain("resource web", accounts);

        var web = result.Workloads["web"];
        Assert.Contains("param web_secret", web);
        Assert.DoesNotContain("param postgres_password", web);
        Assert.DoesNotContain("resource accounts", web);
    }

    [Fact]
    public void SplitRejectsAWorkloadMissingFromRadiusOutput()
    {
        var exception = Assert.Throws<InvalidOperationException>(
            () => RadiusReleaseArtifactSplitter.Split(Source, ["transactions"]));

        Assert.Contains("transactions", exception.Message);
    }
}
