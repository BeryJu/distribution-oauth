# distribution-oauth

Adapt (semi-)standard OAuth to Docker's token system.

Needs custom registry build because https://github.com/distribution/distribution/issues/2875
(See ghcr.io/beryju/distribution:jwt)

## Env variables

- `TOKEN_URL`: URL to send POST Token request to
- `CLIENT_ID`: OAuth Client ID
- `SCOPE`: Special scope to append to requests
- `LOG_LEVEL`: Log level, defaults to info, can be set to trace which will print out credentials for debugging

*Optional variables*

- `ANON_USERNAME`: Credentials to be used when the client doesn't send any (optional)
- `ANON_PASSWORD`: Credentials to be used when the client doesn't send any (optional)
- `ANON_KUBE_JWT`: (Requires kubernetes) If set, will use the current pod's service account as anonymous credentials
- `PASS_JWT_USERNAME`: Username clients can use to pass a JWT directly as password (optional)
- `SESSION_KEY`: Secret key used for sessions (only used when registry is used in a browser, for example https://github.com/Joxit/docker-registry-ui)

## authentik setup

Scope mapping with this code to allow everything:

```python
scopes = request.http_request.POST.get("scope", "").split()
access = []
for scope in scopes:
    if scope.count(":") < 2:
        continue
    type, name, actions = scope.split(":")
    access.append({
        "type": type,
        "name": name,
        "actions": actions.split(","),
    })
return {
    "access": access,
}
```

Create with unique scope name, add scope to provider and configure env correctly
