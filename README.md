# secrets-sync
The `secrets sync` operator automatically copies existing secrets from other namespaces for current namespace.

## Description
This `secrets-sync` operator allows to automatically copy secrets from different namespaces in the namespace for 
which its `CustomResource` was created. It is also possible to override secrets names and keys when copying. 
Two types of keys data are supported: `data` and `stringData`.
The `secrets-sync` operator makes this possible via CR:
```yaml
spec:
  secrets: # List of secrets objects
    mongodb: # Src secret name, (required)
      srcNamespace: mongodb # Source secret namespace, (required)
      dstSecrets: # List of destination secrets objects, (option)
        - name: mongodb-1 # override dst secret name, (option)
          keys: # List of keys objects, (option)
            mongodb-replica-set-key: MONGODB_REPLICA_SET_KEY # key = src secret, val = dst secret, (option)
            mongodb-root-password: MONGODB_ROOT_PASSWORD     # key = src secret, val = dst secret, (option)
        - name: mongodb-2 # override dst secret name, (option)
    elastic-secret: # Src secret name, (required)
      srcNamespace: elastic # Source secret namespace, (required)
    redis: # Src secret name, (required)
      srcNamespace: # Source secret namespace, (required)
```

## Getting Started
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Running on the cluster
1. Install Instances of Custom Resources:

```sh
kubectl apply -f config/samples/internal_v1alpha1_secretssync.yaml
```

2. Build and push your image to the location specified by `IMG`:

```sh
make docker-build docker-push IMG=<some-registry>/core.secrets-sync.operators.infra:tag
```

3. Deploy the controller to the cluster with the image specified by `IMG`:

```sh
make deploy IMG=<some-registry>/core.secrets-sync.operators.infra:tag
```

### Uninstall CRDs
To delete the CRDs from the cluster:

```sh
make uninstall
```

### Undeploy controller
UnDeploy the controller from the cluster:

```sh
make undeploy
```

## Contributing

### How it works
This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/),
which provide a reconcile function responsible for synchronizing resources until the desired state is reached on the cluster.

### Test It Out
1. Install the CRDs into the cluster:

```sh
make install
```

2. Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```sh
make run
```

**NOTE:** You can also run this in one step by running: `make install run`

### Modifying the API definitions
If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)
