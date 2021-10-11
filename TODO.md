* Helm Chart
* Better comparison for when to update stateful set
* Add webhook to reject changes which would cause a failure to update the statefulset (immutable containers, etc)
* Controller references for all child resources
* CRD for "docker image", will ensure a Cluster has an image present in the kind container loaded from the base docker daemon
* Field for custom docker config to be added to configmap and mounted as well as loaded from external tarball (e.g. ReadWriteMany extra mount)
* Watch for changes to statefulset to trigger reconcile of matching cluster
* Configurable requeue delay
* Conditions
* Events
* Figure out how to avoid privlledged containers
* Configurable kind exe path
* Add option to not export docker tls data in secret and port in service for potential security improvements
* Put labels on everything, update labels when they change
* Add option for annotations
* Add webhook to validate kind config, as it can't be included directly in the CRD as it doesn't have json tags
* Add DOCKER\_HOST to secret
* Remove k8sMasterPort from cluster CRD, as it can be inferred from the kind config
* Expose extra ports on the service based on kind config
* Set the k8s master port to a default if not set, as it chooses a random port otherwise
* Set the kind cluster name to the cluster resource name if not set
