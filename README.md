# Styx Proxy

## Purpose

Cloudflare workers will not send requests to servers that do not have
a valid certificate. Since the certificate presented by Styx is
issued by pantheon's CA, Cloudflare considers the certificate invalid.
This services accepts requests from a Cloudflare worker and proxies
them to Styx using mTLS. The client certificate for that request is
provided in the `CLIENT_CERT` environment variable.

## Building the docker image

The image can be built with this command, substituting the path to
the appropriate artifact registry repository.
```
docker buildx build \
  --platform linux/amd64 \
  --tag us-central1-docker.pkg.dev/rhamilton-001/styx-proxy/styx-proxy:0.0.4 \
  .

docker push us-central1-docker.pkg.dev/rhamilton-001/styx-proxy/styx-proxy:0.0.4
```

## Deploying to cloud run

First, upload a valid client certificate to GCP Secret Manager.

Next, create a cloud run instance using the built image with an
environment variable named `STYX_URL` with value set to a Styx
endpoint (
`https://styx-fe1-a.production.dmz-01.us-central1.internal.k8s.pantheon.io:9093`
) and a secret named `CLIENT_CERT` with the value set to the secret
stored in Secret Manager.

To avoid unnecessary costs, consider choosing manual scaling and
scaling down to zero when not in active use.
