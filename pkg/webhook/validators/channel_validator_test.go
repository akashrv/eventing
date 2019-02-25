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

package validators

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	eventingduck "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	eventingtesting "github.com/knative/eventing/pkg/apis/eventing/testing"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	evetingclient "github.com/knative/eventing/pkg/client/clientset/versioned/typed/eventing/v1alpha1"
	"github.com/knative/pkg/apis"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/tools/cache"
)

func TestNewChannelValidator(t *testing.T) {
	informer := newInformerForTest()
	logger := zap.NewNop()
	testCases := map[string]struct {
		informer          cache.SharedIndexInformer
		logger            *zap.Logger
		expectedValidator *ChannelValidatorImpl
		errorExpected     bool
	}{
		"nil informer": {
			informer:      nil,
			logger:        logger,
			errorExpected: true,
		},
		"valid test": {
			informer:      informer,
			logger:        logger,
			errorExpected: false,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			_, err := NewChannelValidator(tc.informer, tc.logger)
			if tc.errorExpected == (err == nil) {
				t.Fatalf("Unexpected error returned. ErrorExpected:%v, ErrorReturned:%v", tc.errorExpected, err)
			}

		})
	}
}

func TestValidate(t *testing.T) {
	validCCPName := "ValidCCPName"
	validAPIVersion := "eventing.knative.dev/v1alpha1"
	validKind := "ClusterChannelProvisioner"

	storemap := make(map[string]interface{})
	storemap[validCCPName] = eventingv1alpha1.ClusterChannelProvisioner{ObjectMeta: metav1.ObjectMeta{Name: validCCPName}}
	keyFunc := func(obj interface{}) (string, error) {
		return obj.(eventingv1alpha1.ClusterChannelProvisioner).Name, nil
	}
	s := eventingtesting.NewMockStore(storemap, keyFunc)

	validator := &ChannelValidatorImpl{
		logger: zap.NewNop(),
		store:  s,
	}

	testCases := []struct {
		name     string
		channel  *eventingv1alpha1.Channel
		expected *apis.FieldError
	}{
		{
			name: "Empty Channel Spec",
			channel: &eventingv1alpha1.Channel{
				Spec: eventingv1alpha1.ChannelSpec{},
			},
			expected: apis.ErrMissingField("spec.provisioner"),
		},
		{
			name: "Not supported Kind",
			channel: &eventingv1alpha1.Channel{
				Spec: eventingv1alpha1.ChannelSpec{
					Provisioner: &corev1.ObjectReference{
						Name:       validCCPName,
						APIVersion: validAPIVersion,
						Kind:       "Invalid",
					},
				},
			},
			expected: (&apis.FieldError{
				Message: "Invalid",
				Details: fmt.Sprintf(
					"This kind of channel provisioner is not supported. Supported provisioners inclue: {apiVersion:%v, kind:%v}",
					validAPIVersion,
					validKind),
				Paths: []string{"provisioner"},
			}).ViaField("spec"),
		},
		{
			name: "Not supported APIVersion",
			channel: &eventingv1alpha1.Channel{
				Spec: eventingv1alpha1.ChannelSpec{
					Provisioner: &corev1.ObjectReference{
						Name:       validCCPName,
						APIVersion: "Invalid",
						Kind:       validKind,
					},
				},
			},
			expected: (&apis.FieldError{
				Message: "Invalid",
				Details: fmt.Sprintf(
					"This kind of channel provisioner is not supported. Supported provisioners inclue: {apiVersion:%v, kind:%v}",
					validAPIVersion,
					validKind),
				Paths: []string{"provisioner"},
			}).ViaField("spec"),
		},
		{
			name: "Not installed provisioner",
			channel: &eventingv1alpha1.Channel{
				Spec: eventingv1alpha1.ChannelSpec{
					Provisioner: &corev1.ObjectReference{
						Name:       "NotInstalled",
						APIVersion: validAPIVersion,
						Kind:       validKind,
					},
				},
			},
			expected: (&apis.FieldError{
				Message: "Not found",
				Details: "Specified channel provisioner 'name:NotInstalled, kind:ClusterChannelProvisioner, apiVersion:eventing.knative.dev/v1alpha1 ' not installed in cluster.",
				Paths:   []string{"provisioner"},
			}).ViaField("spec"),
		},
		{
			name: "Valid Channel Spec",
			channel: &eventingv1alpha1.Channel{
				Spec: eventingv1alpha1.ChannelSpec{
					Provisioner: &corev1.ObjectReference{
						Name:       validCCPName,
						APIVersion: validAPIVersion,
						Kind:       validKind,
					},
					Subscribable: &eventingduck.Subscribable{
						Subscribers: []eventingduck.ChannelSubscriberSpec{{
							SubscriberURI: "subscriberendpoint",
							ReplyURI:      "resultendpoint",
						}},
					}},
			},
			expected: nil,
		},
		{
			name: "empty subscriber at index 1",
			channel: &eventingv1alpha1.Channel{
				Spec: eventingv1alpha1.ChannelSpec{
					Provisioner: &corev1.ObjectReference{
						Name:       validCCPName,
						APIVersion: validAPIVersion,
						Kind:       validKind,
					},
					Subscribable: &eventingduck.Subscribable{
						Subscribers: []eventingduck.ChannelSubscriberSpec{{
							SubscriberURI: "subscriberendpoint",
							ReplyURI:      "replyendpoint",
						}, {}},
					}},
			},
			expected: func() *apis.FieldError {
				fe := apis.ErrMissingField("spec.subscribable.subscriber[1].replyURI", "spec.subscribable.subscriber[1].subscriberURI")
				fe.Details = "expected at least one of, got none"
				return fe
			}(),
		},
		{
			name: "2 empty subscribers",
			channel: &eventingv1alpha1.Channel{
				Spec: eventingv1alpha1.ChannelSpec{
					Provisioner: &corev1.ObjectReference{
						Name:       validCCPName,
						APIVersion: validAPIVersion,
						Kind:       validKind,
					},
					Subscribable: &eventingduck.Subscribable{
						Subscribers: []eventingduck.ChannelSubscriberSpec{{}, {}},
					},
				},
			},
			expected: func() *apis.FieldError {
				var errs *apis.FieldError
				fe := apis.ErrMissingField("spec.subscribable.subscriber[0].replyURI", "spec.subscribable.subscriber[0].subscriberURI")
				fe.Details = "expected at least one of, got none"
				errs = errs.Also(fe)
				fe = apis.ErrMissingField("spec.subscribable.subscriber[1].replyURI", "spec.subscribable.subscriber[1].subscriberURI")
				fe.Details = "expected at least one of, got none"
				errs = errs.Also(fe)
				return errs
			}(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := validator.Validate(tc.channel)
			if diff := cmp.Diff(tc.expected.Error(), actual.Error()); diff != "" {
				t.Errorf("%s: validate (-want, +got) = %v", tc.name, diff)
			}
		})
	}

}

func newInformerForTest() cache.SharedIndexInformer {
	client := evetingclient.EventingV1alpha1Client{}
	watchlist := cache.NewListWatchFromClient(
		client.RESTClient(),
		"clusterchannelprovisioners",
		"",
		fields.Everything())

	informer := cache.NewSharedIndexInformer(
		watchlist,
		&eventingv1alpha1.ClusterChannelProvisioner{},
		time.Minute*5,
		cache.Indexers{},
	)

	return informer
}
