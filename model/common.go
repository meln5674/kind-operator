package model

import (
	kindmeln5674v0 "github.com/meln5674/kind-operator.git/api/v0"
	kindv1alpha4 "sigs.k8s.io/kind/pkg/apis/config/v1alpha4"
	"sigs.k8s.io/yaml"
)

const (
	defaultK8sAPIPort = int32(6443)
	defaultDockerPort = int32(2376)
	ClusterLabel      = "kind-operator/cluster"
)

func KindContainerName(cluster *kindmeln5674v0.Cluster) string {
	if cluster.Spec.KindContainerName == nil {
		return defaultKindContainerName
	}
	return *cluster.Spec.KindContainerName
}

func KindClusterName(cluster *kindmeln5674v0.Cluster) (string, error) {
	if cluster.Spec.KindConfig == nil {
		return cluster.Name, nil
	}
	kindConfig := kindv1alpha4.Cluster{}
	if err := yaml.Unmarshal([]byte(cluster.Spec.KindConfig), &kindConfig); err != nil {
		return "", err
	}
	clusterName := kindConfig.Name
	if clusterName == "" {
		return cluster.Name, nil
	}
	return clusterName, nil
}

func ClusterDockerPort(cluster *kindmeln5674v0.Cluster) int32 {
	// TODO: Configurable
	return defaultDockerPort
}

func ClusterK8sAPIPort(cluster *kindmeln5674v0.Cluster) int32 {
	if cluster.Spec.K8sAPIPort == nil {
		return defaultK8sAPIPort
	}
	return *cluster.Spec.K8sAPIPort
}

func ClusterPodLabelSelector(cluster *kindmeln5674v0.Cluster) map[string]string {
	selector := map[string]string{ClusterLabel: cluster.Name}
	for key, value := range cluster.Labels {
		selector[key] = value
	}
	return selector
}
