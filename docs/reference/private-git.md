# Private repositories, proxies, and TLS

skmgr never handles credentials itself. It shells out to `git`, so whatever
authentication already works for `git clone` works here, and nothing sensitive
is stored in `skmgr.yml` or `skmgr.lock`.

That means the answer to "how do I install from our private repo?" is always
"make `git clone <source>` work first". If that succeeds from the same shell,
skmgr will succeed too.

## SSH keys

The usual approach for a private repository. Use an `git@` source:

```yaml
skills:
  - source: git@github.com:acme/internal-skills.git
    path: skills/deploy
    ref: v2.1.0
```

To pick a specific key without editing `~/.ssh/config`:

```bash
GIT_SSH_COMMAND="ssh -i ~/.ssh/skmgr_deploy_key -o IdentitiesOnly=yes" skmgr install
```

In CI, load the key into an agent before running skmgr:

```yaml
- uses: webfactory/ssh-agent@v0.9.0
  with:
    ssh-private-key: ${{ secrets.SKMGR_DEPLOY_KEY }}
- run: skmgr install --frozen
```

## HTTPS with a token

Use a credential helper or a URL rewrite rather than putting a token in the
manifest, which would be committed:

```bash
git config --global url."https://${GITHUB_TOKEN}@github.com/".insteadOf "https://github.com/"
skmgr install --frozen
```

On GitHub Actions the checkout action already configures this for the repository
being built; for *other* private repositories you need the rewrite above with a
token that can read them.

Always set `GIT_TERMINAL_PROMPT=0` in automation. Without it, a missing
credential makes git wait for input that will never come, and the job hangs
until it times out rather than failing:

```bash
export GIT_TERMINAL_PROMPT=0
```

## Proxies

Git reads the standard proxy variables, and skmgr inherits them:

```bash
export HTTPS_PROXY=http://proxy.corp:3128
export HTTP_PROXY=http://proxy.corp:3128
export NO_PROXY=git.internal.corp,localhost
skmgr install
```

`NO_PROXY` matters when some sources are internal and some are not: without it,
requests to an internal git server are sent to a proxy that cannot reach it.

The equivalent git configuration also works, and survives across shells:

```bash
git config --global http.proxy http://proxy.corp:3128
```

## TLS with an internal certificate authority

An internal git server with a certificate signed by a private CA needs that CA
trusted. Point git at the bundle:

```bash
export GIT_SSL_CAINFO=/etc/ssl/certs/corp-ca.pem
skmgr install
```

or per host:

```bash
git config --global http."https://git.internal.corp/".sslCAInfo /etc/ssl/certs/corp-ca.pem
```

Do not disable certificate verification. `http.sslVerify=false` makes every
fetch trivially interceptable, and skmgr installs code that your agents then
execute.

## Air-gapped machines

Two options, depending on whether the machine can reach an internal mirror.

**Mirror the repository** and point at it with `git::`, which keeps refs and
version ranges working:

```yaml
skills:
  - source: git::/srv/mirrors/skills.git
    path: skills/deploy
    ref: ^1.2.0
```

**Warm the cache elsewhere** and copy it across, then install with no network
at all:

```bash
# On a connected machine
skmgr install
tar czf skmgr-cache.tar.gz -C ~/.skmgr cache

# On the air-gapped machine
tar xzf skmgr-cache.tar.gz -C ~/.skmgr
skmgr install --frozen --offline
```

`--offline` refuses any network access rather than falling back to it, so a
cache miss fails loudly instead of hanging on an unreachable host.

## Checking your setup

`skmgr doctor` confirms git is present and usable:

```
[ ok ] git        git version 2.49.0
```

If a fetch fails, reproduce it with git directly — the error will be clearer,
and it tells you whether the problem is skmgr or the credential:

```bash
git ls-remote git@github.com:acme/internal-skills.git
```
