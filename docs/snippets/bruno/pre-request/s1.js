const EXPIRY_SKEW_SECONDS = 30;
const DEFAULT_EXPIRY_SECONDS = 300;

function requireEnv(name) {
  const value = bru.getEnvVar(name);

  if (value === undefined || value === null || String(value).trim() === "") {
    throw new Error(`Missing Bruno environment variable: ${name}`);
  }

  return String(value);
}

function formEncode(values) {
  return Object.entries(values)
    .filter(([, value]) => value !== undefined && value !== null)
    .map(
      ([key, value]) =>
        `${encodeURIComponent(key)}=${encodeURIComponent(String(value))}`
    )
    .join("&");
}

function getTokenUrl() {
  const baseUrl = requireEnv("kc_base_url").replace(/\/+$/, "");
  const realm = encodeURIComponent(requireEnv("kc_realm"));

  return `${baseUrl}/realms/${realm}/protocol/openid-connect/token`;
}

async function requestToken(grantType, refreshToken) {
  const payload = {
    client_id: requireEnv("kc_client_id"),
    client_secret: requireEnv("kc_client_secret"),
    grant_type: grantType
  };

  if (grantType === "refresh_token") {
    payload.refresh_token = refreshToken;
  } else if (grantType === "password") {
    payload.username = requireEnv("kc_username");
    payload.password = requireEnv("kc_password");
  } else {
    throw new Error(`Unsupported OAuth grant type: ${grantType}`);
  }

  const response = await bru.sendRequest({
    method: "POST",
    url: getTokenUrl(),
    headers: {
      "Content-Type": "application/x-www-form-urlencoded",
      Accept: "application/json"
    },
    data: formEncode(payload),
    validateStatus: () => true
  });

  const status = response.status ?? response.statusCode;
  const body = response.data ?? response.body ?? {};

  if (status < 200 || status >= 300) {
    const description =
      body.error_description || body.error || JSON.stringify(body);

    throw new Error(
      `${grantType} token request failed (${status}): ${description}`
    );
  }

  if (!body.access_token) {
    throw new Error(
      `${grantType} token response did not contain an access_token`
    );
  }

  return body;
}

function tokenNeedsRefresh() {
  const accessToken = bru.getEnvVar("kc_access_token");
  const expiresAt = Number(bru.getEnvVar("kc_expires_at") || 0);
  const now = Math.floor(Date.now() / 1000);

  return (
    !accessToken ||
    !Number.isFinite(expiresAt) ||
    now >= expiresAt - EXPIRY_SKEW_SECONDS
  );
}

async function acquireToken() {
  const refreshToken = bru.getEnvVar("kc_refresh_token");

  if (refreshToken) {
    try {
      return await requestToken("refresh_token", String(refreshToken));
    } catch (error) {
      console.warn(
        `Refresh token failed; falling back to password grant: ${error.message}`
      );

      // Prevent repeatedly retrying a refresh token known to be invalid.
      bru.setEnvVar("kc_refresh_token", "");
    }
  }

  return requestToken("password");
}

function saveToken(token) {
  const now = Math.floor(Date.now() / 1000);
  const expiresIn = Number(token.expires_in);

  bru.setEnvVar("kc_access_token", token.access_token);
  bru.setEnvVar(
    "kc_expires_at",
    String(
      now +
        (Number.isFinite(expiresIn) && expiresIn > 0
          ? expiresIn
          : DEFAULT_EXPIRY_SECONDS)
    )
  );

  // Keycloak commonly rotates refresh tokens, so always save the new one.
  if (token.refresh_token) {
    bru.setEnvVar("kc_refresh_token", token.refresh_token);
  }
}

(async () => {
  if (tokenNeedsRefresh()) {
    const token = await acquireToken();
    saveToken(token);
  }

  req.setHeader(
    "Authorization",
    `Bearer ${requireEnv("kc_access_token")}`
  );
})();