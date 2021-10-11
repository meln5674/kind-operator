# KinD operator

This tool implements the operator pattern to manage Kubernetes-in-Kubernetes via Kubernetes-in-Docker (KinD).

This tool is intended to be used to create isolated k8s clusters for use in CI/CD, testing, and debugging.

**WARNING**: This tool is in alpha, it has not been tested thoroughly, use at your own risk.

**WARNING**: This tool ***creates privileged containers***, and there is currently no known way around this. It is the user's responsibility to ensure that these containers are secure against potential abuse. Users and sevice accounts with privileges to pods/exec to the pods created by this tool will effectively have root access to the node these containers run on. Pods which can mount the secrets created by this tool will likewise effectively have root access.

