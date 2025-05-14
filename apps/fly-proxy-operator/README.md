# Fly Proxy Operator

I want cluster ingress for my homelab/homeprod cluster to run on Fly.io.

To run in the cluster it would need to:

- Watch Ingress objects with the assigned ingress class name
- Watch the certificates and services associated with those ingress classes
- Build Nginx config, bundling certificates into the image or pushing them into Fly secrets
- Programatically push the change to Fly (and commit to git?)
- Update Ingress state with correct external IP so external-dns can pick it up

Maybe this is actually better than trying to run outside the cluster?
Don't have to expose k8s API outside the cluster, for one thing.

## Plan A: Inside Cluster

- shell-operator
- hook on schedule (daily?) and kubernetes events:
  - ingress (maybe with field selector but maybe not)
  - certificate secrets (for those ingresses?)
  - services
- generate oci image and push to fly:
  - nginx conf files
  - entrypoint scripts
- upload certificates to secrets
- `flyctl deploy`

### Fly config

- operator takes over fly config
- new entrypoint script that pulls certificates from env and writes to files
- generate a config file for every ingress

### Concerns
- restarting/redeploying fly too often?
- wait for kamiko to be done?
