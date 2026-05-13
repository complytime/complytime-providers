# Tutorial: Your First OPA Provider Scan

This tutorial walks you through scanning configuration files against OPA
policies using the `complyctl-provider-opa` plugin. By the end, you will have
installed the provider, run a scan against local Kubernetes manifests, and
read the compliance results.

## Before You Start

You need the following installed on your machine:

- `complyctl` CLI ([installation guide](https://github.com/complytime/complyctl))
- `conftest` ([conftest.dev](https://www.conftest.dev/install/))
- `git`
- Access to an OCI registry hosting OPA policy bundles

Verify your tools are available:

```bash
complyctl version
conftest --version
git --version
```

## Step 1: Build the OPA provider

Clone the complytime-providers repository and build the OPA provider binary:

```bash
git clone https://github.com/complytime/complytime-providers.git
cd complytime-providers
make build-opa-provider
```

This produces `bin/complyctl-provider-opa`.

## Step 2: Install the provider

Copy the binary to the complyctl providers directory. The provider is
discovered automatically by the `complyctl-provider-` naming convention:

```bash
mkdir -p ~/.complytime/providers
cp bin/complyctl-provider-opa ~/.complytime/providers/
```

Verify complyctl discovers the provider:

```bash
complyctl provider list
```

You should see `opa` in the output.

## Step 3: Create a sample configuration file

Create a Kubernetes deployment manifest to scan:

```bash
mkdir -p /tmp/opa-tutorial
cat > /tmp/opa-tutorial/deployment.yaml << 'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web-app
spec:
  replicas: 1
  selector:
    matchLabels:
      app: web-app
  template:
    metadata:
      labels:
        app: web-app
    spec:
      containers:
      - name: web
        image: nginx:latest
        securityContext:
          runAsRoot: true
EOF
```

This deployment intentionally violates common security policies — running as
root and using the `latest` image tag.

## Step 4: Run a scan

Run complyctl with the OPA provider, pointing at your sample file and a policy
bundle:

```bash
complyctl scan \
  --provider opa \
  --target input_path=/tmp/opa-tutorial \
  --var opa_bundle_ref=ghcr.io/your-org/opa-policies:latest
```

Replace `ghcr.io/your-org/opa-policies:latest` with the OCI reference of your
actual policy bundle.

## Step 5: Read the results

The scan output shows compliance assessment results grouped by requirement ID.
Each requirement lists the targets that were evaluated and whether they passed
or failed.

The provider also writes per-target result files to the workspace directory.
Find them at:

```bash
ls ~/.complytime/workspace/opa/results/
cat ~/.complytime/workspace/opa/results/*.json
```

Each JSON file contains the individual findings, success counts, and timestamps
for a single target evaluation.

## What You Learned

You have:

1. Built and installed the OPA provider plugin
2. Scanned local configuration files against OPA policies from an OCI bundle
3. Read both the complyctl assessment output and the per-target result files

## Next Steps

- [How-to Guide](how-to.md) — Scan remote repositories, use access tokens,
  scan specific subdirectories
- [Reference](reference.md) — Complete list of target variables and
  configuration options
- [Explanation](explanation.md) — How the OPA provider fits into the complyctl
  architecture
