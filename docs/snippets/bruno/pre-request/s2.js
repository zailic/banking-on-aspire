const tokenUrl = `${bru.getEnvVar("kc_base_url")}/realms/${bru.getEnvVar("kc_realm")}/protocol/openid-connect/token`;

function requireEnv(name) {
  const v = bru.getEnvVar(name);
  if (!v) throw new Error(`Missing env var: ${name}`);
  return v;
}

function formEncode(obj) {
  return Object.keys(obj)
    .map(k => `${encodeURIComponent(k)}=${encodeURIComponent(obj[k] ?? "")}`)
    .join("&");
}

async function requestToken(grantType) {
  const payload = {
    client_id: requireEnv("kc_client_id"),
    client_secret: requireEnv("kc_client_secret"),
    grant_type: grantType
  };

  if (grantType === "password") {
    payload.username = requireEnv("kc_username");
    payload.password = requireEnv("kc_password");
  } else {
    payload.refresh_token = requireEnv("kc_refresh_token");
  }

  const res = await bru.sendRequest({
    method: "POST",
    url: tokenUrl,
    headers: {
      "Content-Type": "application/x-www-form-urlencoded"
    },
    data: formEncode(payload),
    validateStatus: () => true
  });

  const body = res.data || res.body || {};
  if (res.status !== 200) {
    throw new Error(`Token request failed (${res.status}): ${JSON.stringify(body)}`);
  }

  return body;
}

(async () => {
  const now = Math.floor(Date.now() / 1000);
  const exp = parseInt(bru.getEnvVar("kc_expires_at") || "0", 10);
  const needsRefresh = now >= (exp - 30);

  if (needsRefresh) {
    let token;
    try {
      token = bru.getEnvVar("kc_refresh_token")
        ? await requestToken("refresh_token")
        : await requestToken("password");
    } catch (e) {
      console.log(e);
      //token = await requestToken("password");
    }

    bru.setEnvVar("kc_access_token", token.access_token);
    if (token.refresh_token) {
      bru.setEnvVar("kc_refresh_token", token.refresh_token);
    }
    bru.setEnvVar("kc_expires_at", String(now + (token.expires_in || 300)));
  }

  req.setHeader("Authorization", `Bearer ${bru.getEnvVar("kc_access_token")}`);
})();