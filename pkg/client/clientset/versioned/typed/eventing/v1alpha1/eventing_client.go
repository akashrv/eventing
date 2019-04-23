/*
Copyright 2019 The Knative Authors

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

// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/client/clientset/versioned/scheme"
	serializer "k8s.io/apimachinery/pkg/runtime/serializer"
	rest "k8s.io/client-go/rest"
)

type EventingV1alpha1Interface interface {
	RESTClient() rest.Interface
	BrokersGetter
	ChannelsGetter
	ClusterChannelProvisionersGetter
	SubscriptionsGetter
	TriggersGetter
}

// EventingV1alpha1Client is used to interact with features provided by the eventing.knative.dev group.
type EventingV1alpha1Client struct {
	restClient rest.Interface
}

func (c *EventingV1alpha1Client) Brokers(namespace string) BrokerInterface {
	return newBrokers(c, namespace)
}

func (c *EventingV1alpha1Client) Channels(namespace string) ChannelInterface {
	return newChannels(c, namespace)
}

func (c *EventingV1alpha1Client) ClusterChannelProvisioners() ClusterChannelProvisionerInterface {
	return newClusterChannelProvisioners(c)
}

func (c *EventingV1alpha1Client) Subscriptions(namespace string) SubscriptionInterface {
	return newSubscriptions(c, namespace)
}

func (c *EventingV1alpha1Client) Triggers(namespace string) TriggerInterface {
	return newTriggers(c, namespace)
}

// NewForConfig creates a new EventingV1alpha1Client for the given config.
func NewForConfig(c *rest.Config) (*EventingV1alpha1Client, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &EventingV1alpha1Client{client}, nil
}

// NewForConfigOrDie creates a new EventingV1alpha1Client for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *EventingV1alpha1Client {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new EventingV1alpha1Client for the given RESTClient.
func New(c rest.Interface) *EventingV1alpha1Client {
	return &EventingV1alpha1Client{c}
}

func setConfigDefaults(config *rest.Config) error {
	gv := v1alpha1.SchemeGroupVersion
	config.GroupVersion = &gv
	config.APIPath = "/apis"
	config.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: scheme.Codecs}

	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	return nil
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *EventingV1alpha1Client) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
