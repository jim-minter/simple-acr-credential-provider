# Simple ACR credential provider

## Introduction

This repo implements a simple [kubelet image credential
provider](https://kubernetes.io/docs/tasks/administer-cluster/kubelet-credential-provider/)
against [Azure Container
Registry](https://learn.microsoft.com/en-us/azure/container-registry/).

It is very similar to
[acr-credential-provider](https://github.com/kubernetes-sigs/cloud-provider-azure/tree/master/cmd/acr-credential-provider),
except that acr-credential-provider only really supports VM managed identity,
and not local credentials.

Note that hard-coding a client ID and client secret for ACR pull is also
possible via [Kubernetes image pull
secrets](https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/)
or, for example, (deprecated) [containerd registry
credentials](https://github.com/containerd/containerd/blob/main/docs/cri/registry.md#configure-registry-credentials).

## Use

You should have an Azure principal with AcrPull access or equivalent on the ACR
of your choice.

Note, regardless of the Azure authentication mechanism, it is always required to
set the AZURE_TENANT_ID environment variable in the CredentialProviderConfig
`providers[].env` section.

```shell
go install github.com/jim-minter/simple-acr-credential-provider@latest
sudo install "$(go env GOPATH)/bin/simple-acr-credential-provider" /usr/local/bin
```

Example 1: using a client secret:

```shell
sudo tee /var/lib/kubelet/credential-provider-config.yaml >/dev/null <<'EOF'
apiVersion: kubelet.config.k8s.io/v1
kind: CredentialProviderConfig
providers:
- name: simple-acr-credential-provider
  matchImages:
  - "*.azurecr.io"
  defaultCacheDuration: 10m
  apiVersion: credentialprovider.kubelet.k8s.io/v1
  env:
  - name: AZURE_TENANT_ID
    value: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
  - name: AZURE_CLIENT_ID
    value: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
  - name: AZURE_CLIENT_SECRET
    value: secret
EOF

sudo chown root:root /var/lib/kubelet/credential-provider-config.yaml
sudo chmod 0600 /var/lib/kubelet/credential-provider-config.yaml
```

Example 2: using a client certificate:

For example, `/path/to/certificate-and-key` could contain a certificate and key,
each in PEM format, concatenated.

```shell
sudo tee /var/lib/kubelet/credential-provider-config.yaml >/dev/null <<'EOF'
apiVersion: kubelet.config.k8s.io/v1
kind: CredentialProviderConfig
providers:
- name: simple-acr-credential-provider
  matchImages:
  - "*.azurecr.io"
  defaultCacheDuration: 10m
  apiVersion: credentialprovider.kubelet.k8s.io/v1
  env:
  - name: AZURE_TENANT_ID
    value: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
  - name: AZURE_CLIENT_ID
    value: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
  - name: AZURE_CLIENT_CERTIFICATE_PATH
    value: /path/to/certificate-and-key
EOF

sudo chown root:root /path/to/certificate-and-key
sudo chmod 0600 /path/to/certificate-and-key
```

Example 3: using IMDS (including ARC):

```shell
sudo tee /var/lib/kubelet/credential-provider-config.yaml >/dev/null <<'EOF'
apiVersion: kubelet.config.k8s.io/v1
kind: CredentialProviderConfig
providers:
- name: simple-acr-credential-provider
  matchImages:
  - "*.azurecr.io"
  defaultCacheDuration: 10m
  apiVersion: credentialprovider.kubelet.k8s.io/v1
  env:
  - name: AZURE_TENANT_ID
    value: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
EOF
```

Example 4: using alternative authentication mechanisms:

Any other mechanism supported by
[azidentity.DefaultAzureCredential](https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/azidentity#DefaultAzureCredential)
may be used by setting environment variables accordingly (see linked
documentation).

Finally, configure `/etc/default/kubelet` and restart the kubelet.

```shell
sudo tee /etc/default/kubelet >/dev/null <<'EOF'
KUBELET_EXTRA_ARGS="--image-credential-provider-config=/var/lib/kubelet/credential-provider-config.yaml --image-credential-provider-bin-dir=/usr/local/bin"
EOF

sudo systemctl restart kubelet.service
```
