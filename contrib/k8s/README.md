# Example Kubernetes Manifests

This folder provides two example Kubernetes manifests:
* [shellyctl-prom-hostNetwork.yaml](shellyctl-prom-hostNetwork.yaml) - Configures a prometheus server with mDNS search enabled on all interfaces. To support mDNS discovery this pod MUST run with `hostNetwork: true` which enables additional network privileges. To avoid port-conficts during pod replacement, the server is run as a StatefulSet w/ 1 replica.
* [shellyctl-prom.yaml](shellyctl-prom.yaml) - Configures a prometheus server running within pod networking. This leverages a traditional Deployment model. Because no mDNS/BLEdiscovery is possible you MUST specify hosts in the config file or configure and MQTT server.

## Updating configuration
Both variants leverage a config-map for configuring shellyctl flags. Shellyctl does NOT support live config reloading so you must restar or replace the pod to reload configuration.

```
# edit the config file.
kubectl edit -n shellyctl configmap shellyctl-prom

# replace shellyctl pods to reload configuration
kubectl delete pod -n shellyctl --all
```

## Service Discovery
These examples use the `prometheus.io/scrape` and `prometheus.io/port` anotations for service discovery. Your prometheus service discovery may be configured differently. See:
* [Prometheus Service Discovery Configuration](https://prometheus.io/docs/prometheus/latest/configuration/configuration/)
* [PodMonitor/ServiceMonitor in Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator/blob/main/Documentation/user-guides/getting-started.md#deploying-a-sample-application)
* [Victoria Metrics Agent](https://docs.victoriametrics.com/vmagent/)


## shellyctl-prom-hostNetwork.yaml
```
# Create a namespace, shellyctl.
kubectl create namespace shellyctl

# Optionally add a password for authenticating with devices. This step may be skipped if you do not use auth.
kubectl create secret -n shellyctl generic shellyctl-auth --from-literal=SHELLYCTL_AUTH=password

# Apply the manifest.
kubectl apply -f https://raw.githubusercontent.com/jcodybaker/shellyctl/main/contrib/k8s/shellyctl-prom-hostNetwork.yaml
```

## shellyctl-prom.yaml
```
# Create a namespace, shellyctl.
kubectl create namespace shellyctl

# Optionally add a password for authenticating with devices. This step may be skipped if you do not use auth.
kubectl create secret -n shellyctl generic shellyctl-auth --from-literal=SHELLYCTL_AUTH=password

# Download the manifest for local editing.
curl -LfO https://raw.githubusercontent.com/jcodybaker/shellyctl/main/contrib/k8s/shellyctl-prom.yaml

# Edit the `host` list in configmap.
editor shellyctl-prom.yaml

# Apply the manifest.
kubectl apply -f shellyctl-prom.yaml
```