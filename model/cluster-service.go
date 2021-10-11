package model

import (
	"fmt"
	kindmeln5674v0 "github.com/meln5674/kind-operator.git/api/v0"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func ClusterHostname(cluster *kindmeln5674v0.Cluster) string {
	return fmt.Sprintf("%s.%s", ClusterServiceMeta(cluster).Name, cluster.Namespace)
}

func ClusterK8sAPIAddress(cluster *kindmeln5674v0.Cluster) string {
	return fmt.Sprintf("%s:%d", ClusterHostname(cluster), ClusterK8sAPIPort(cluster))
}

func ClusterK8sAPIURL(cluster *kindmeln5674v0.Cluster) string {
	return fmt.Sprintf("https://%s", ClusterK8sAPIAddress(cluster))
}

func ClusterDockerAddress(cluster *kindmeln5674v0.Cluster) string {
	return fmt.Sprintf("%s:%d", ClusterHostname(cluster), ClusterDockerPort(cluster))
}

func ClusterDockerRawAddress(cluster *kindmeln5674v0.Cluster, pod *corev1.Pod) string {
	return fmt.Sprintf("%s:%d", pod.Status.PodIPs[0].IP, ClusterDockerPort(cluster))
}

func ClusterDockerURL(cluster *kindmeln5674v0.Cluster) string {
	return fmt.Sprintf("tcp://%s", ClusterDockerAddress(cluster))
}

func ClusterDockerRawURL(cluster *kindmeln5674v0.Cluster, pod *corev1.Pod) string {
	return fmt.Sprintf("tcp://%s", ClusterDockerRawAddress(cluster, pod))
}

func ClusterServiceMeta(cluster *kindmeln5674v0.Cluster) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Namespace: cluster.Namespace,
		Name:      cluster.Name,
		Labels:    cluster.ObjectMeta.Labels,
	}
}

func ClusterService(cluster *kindmeln5674v0.Cluster) corev1.Service {
	return corev1.Service{
		ObjectMeta: ClusterServiceMeta(cluster),
		Spec: corev1.ServiceSpec{
			// TODO: Configurable?
			Type:     corev1.ServiceTypeClusterIP,
			Selector: ClusterPodLabelSelector(cluster),
			Ports: []corev1.ServicePort{
				{
					Name:       dockerPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       ClusterDockerPort(cluster),
					TargetPort: intstr.FromString(dockerPortName),
				},
				{
					Name:       k8sAPIPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       ClusterK8sAPIPort(cluster),
					TargetPort: intstr.FromString(k8sAPIPortName),
				},
			},
		},
	}
}
