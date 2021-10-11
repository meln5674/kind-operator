/*
Copyright © 2021 Andrew M Melnick

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	goflag "flag"

	"context"
	"github.com/spf13/cobra"
	"os"
	"os/exec"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	kindmeln5674v0 "github.com/meln5674/kind-operator.git/api/v0"
	"github.com/meln5674/kind-operator.git/controllers"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	k8srest "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	//+kubebuilder:scaffold:imports
)

type execWrapper struct {
	config    *k8srest.Config
	clientset *kubernetes.Clientset
	scheme    *runtime.Scheme
}

func (e *execWrapper) Exec(ctx context.Context, pod client.ObjectKey, opts corev1.PodExecOptions) (remotecommand.Executor, error) {
	req := e.clientset.
		CoreV1().
		RESTClient().
		Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("exec").
		VersionedParams(&opts, runtime.NewParameterCodec(scheme))
	return remotecommand.NewSPDYExecutor(e.config, "POST", req.URL())
}

var (
	metricsAddr          string
	enableLeaderElection bool
	probeAddr            string
	opts                 zap.Options = zap.Options{
		Development: true,
	}
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")

	// managerCmd represents the manager command
	managerCmd = &cobra.Command{
		Use:   "manager",
		Short: "The Kind Operator",
		Long:  `Manage Kubernetes-in-Kubernetes using Kind, controlled via the Kubernetes API`,
		Run: func(cmd *cobra.Command, args []string) {
			if dockerPath, err := exec.LookPath("docker"); err != nil {
				setupLog.Error(err, "Docker is not on $PATH, this is not supported", "PATH", os.Getenv("PATH"))
				os.Exit(1)
			} else {
				setupLog.Info("Found docker cli on $PATH", "docker", dockerPath)
			}

			config := ctrl.GetConfigOrDie()
			client := kubernetes.NewForConfigOrDie(config)

			ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

			mgr, err := ctrl.NewManager(config, ctrl.Options{
				Scheme:                 scheme,
				MetricsBindAddress:     metricsAddr,
				Port:                   9443,
				HealthProbeBindAddress: probeAddr,
				LeaderElection:         enableLeaderElection,
				LeaderElectionID:       "8e674aa7.kind-operator.meln5674",
			})
			if err != nil {
				setupLog.Error(err, "unable to start manager")
				os.Exit(1)
			}

			if err = (&controllers.ClusterReconciler{
				Client: mgr.GetClient(),
				Scheme: mgr.GetScheme(),
				PodExecutor: &execWrapper{
					clientset: client,
					config:    config,
					scheme:    scheme,
				},
				KindPath:     "kind",
				RequeueDelay: time.Second * 30,
			}).SetupWithManager(mgr); err != nil {
				setupLog.Error(err, "unable to create controller", "controller", "Fleet")
				os.Exit(1)
			}
			//+kubebuilder:scaffold:builder

			if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
				setupLog.Error(err, "unable to set up health check")
				os.Exit(1)
			}
			if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
				setupLog.Error(err, "unable to set up ready check")
				os.Exit(1)
			}

			setupLog.Info("starting manager")
			if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
				setupLog.Error(err, "problem running manager")
				os.Exit(1)
			}
		},
	}
)

func init() {
	rootCmd.AddCommand(managerCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// managerCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// managerCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(kindmeln5674v0.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme

	managerCmd.Flags().StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	managerCmd.Flags().StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	managerCmd.Flags().BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	tmpflags := goflag.NewFlagSet("", goflag.ContinueOnError)
	opts.BindFlags(tmpflags)
	managerCmd.Flags().AddGoFlagSet(tmpflags)

}
