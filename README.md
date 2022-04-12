# distribution-oauth

Adapt (semi-)standard OAuth to Docker's token system.

Needs custom registry build because https://github.com/distribution/distribution/issues/2875
(See ghcr.io/beryju/distribution:jwt)

## Env variables

- `TOKEN_URL`: URL to send POST Token request to
- `CLIENT_ID`: OAuth Client ID
- `SCOPE`: Special scope to append to requests

## Todo

- Add support for `client_assertion_type` to allow auth via JWT (reserved username)
- Add anonymous support (either static credentials or machine JWT (k8s SA JWT))

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
