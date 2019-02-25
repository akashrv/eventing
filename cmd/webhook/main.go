/*
Copyright 2018 The Knative Authors
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

package main

import (
	"flag"
	"log"
	"time"

	channeldefaulter "github.com/knative/eventing/pkg/webhook/defaulters"
	"github.com/knative/eventing/pkg/webhook/validators"

	"go.uber.org/zap"

	"github.com/knative/pkg/configmap"
	"github.com/knative/pkg/logging"
	"github.com/knative/pkg/logging/logkey"
	"github.com/knative/pkg/signals"
	"github.com/knative/pkg/webhook"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/logconfig"
	"github.com/knative/pkg/system"

	eventingclient "github.com/knative/eventing/pkg/client/clientset/versioned"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

func main() {
	flag.Parse()
	// Read the logging config and setup a logger.
	cm, err := configmap.Load("/etc/config-logging")
	if err != nil {
		log.Fatalf("Error loading logging configuration: %v", err)
	}
	config, err := logging.NewConfigFromMap(cm, logconfig.Webhook)
	if err != nil {
		log.Fatalf("Error parsing logging configuration: %v", err)
	}
	logger, atomicLevel := logging.NewLoggerFromConfig(config, logconfig.Webhook)
	defer logger.Sync()
	logger = logger.With(zap.String(logkey.ControllerType, logconfig.Webhook))

	logger.Info("Starting the Eventing Webhook")

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	clusterConfig, err := rest.InClusterConfig()
	if err != nil {
		logger.Fatal("Failed to get in cluster config", zap.Error(err))
	}

	kubeClient, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		logger.Fatal("Failed to get the client set", zap.Error(err))
	}

	// Watch the logging config map and dynamically update logging levels.
	configMapWatcher := configmap.NewInformedWatcher(kubeClient, system.Namespace())

	configMapWatcher.Watch(logconfig.ConfigName, logging.UpdateLevelFromConfigMap(logger, atomicLevel, logconfig.Webhook, logconfig.Webhook))

	// Watch the default-channel-webhook ConfigMap and dynamically update the default
	// ClusterChannelProvisioner.
	channelDefaulter := channeldefaulter.New(logger.Desugar())
	eventingv1alpha1.ChannelDefaulterSingleton = channelDefaulter
	configMapWatcher.Watch(channeldefaulter.ConfigMapName, channelDefaulter.UpdateConfigMap)

	if err = configMapWatcher.Start(stopCh); err != nil {
		logger.Fatalf("failed to start webhook configmap watcher: %v", err)
	}

	// Create an eventing client that will be used to create a shared informer
	evtClient, err := eventingclient.NewForConfig(clusterConfig)
	if err != nil {
		logger.Fatal("Failed to get the eventing client set", zap.Error(err))
	}

	sInformer := newClusterChannelProvisionerInformer(evtClient)
	go sInformer.Run(stopCh)
	logger.Info("Started shared informer for clusterchannnelprovisioners")

	eventingv1alpha1.GlobalChannelValidator, err = validators.NewChannelValidator(sInformer, logger.Desugar())
	if err != nil {
		logger.Fatalf("failed to setup Channel validator. %v", err)
	}
	logger.Info("Configured global channel validator")

	options := webhook.ControllerOptions{
		ServiceName:    "webhook",
		DeploymentName: "webhook",
		Namespace:      system.Namespace(),
		Port:           443,
		SecretName:     "webhook-certs",
		WebhookName:    "webhook.eventing.knative.dev",
	}
	controller := webhook.AdmissionController{
		Client:  kubeClient,
		Options: options,
		Handlers: map[schema.GroupVersionKind]webhook.GenericCRD{
			// For group eventing.knative.dev,
			eventingv1alpha1.SchemeGroupVersion.WithKind("Channel"):                   &eventingv1alpha1.Channel{},
			eventingv1alpha1.SchemeGroupVersion.WithKind("ClusterChannelProvisioner"): &eventingv1alpha1.ClusterChannelProvisioner{},
			eventingv1alpha1.SchemeGroupVersion.WithKind("Subscription"):              &eventingv1alpha1.Subscription{},
		},
		Logger: logger,
	}
	if err != nil {
		logger.Fatal("Failed to create the admission controller", zap.Error(err))
	}
	controller.Run(stopCh)
}

func newClusterChannelProvisionerInformer(client *eventingclient.Clientset) cache.SharedIndexInformer {

	watchlist := cache.NewListWatchFromClient(
		client.EventingV1alpha1().RESTClient(),
		"clusterchannelprovisioners",
		"",
		fields.Everything())

	informer := cache.NewSharedIndexInformer(
		watchlist,
		&eventingv1alpha1.ClusterChannelProvisioner{},
		time.Hour*8,
		cache.Indexers{},
	)

	return informer
}
