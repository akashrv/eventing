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
	"errors"
	"fmt"

	eventingduck "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/pkg/apis"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
)

// ChannelValidatorImpl will be initialized by the WebHook
// It checks if the required fields have been set and that the Channel Provisioner specified is installed
// in the cluster (https://github.com/knative/eventing/issues/779)
// To check if Channel Provisioner is installed, and informer needs to be created on ChanneProvisioners and provided to the NewChannelValidator func.
// ChannelValidatorImpl will then use the Store (Or Indexer) inside this informer to check if the ChannelProvisioner exists or not and will
// not make a API Server call in the admission webhook. However the Informer will get Add/Delete/Update events on ChannelProvisioner and 
// the underlying Store/Indexer will be kept up to date.
type ChannelValidatorImpl struct {
	store  cache.Store
	logger *zap.Logger
}

var _ eventingv1alpha1.ChannelValidator = (*ChannelValidatorImpl)(nil)


func NewChannelValidator(informer cache.SharedIndexInformer, logger *zap.Logger) (*ChannelValidatorImpl, error) {
	if informer == nil {
		return nil, errors.New("Required argument is nil: informer")
	}
	if logger == nil {
		return nil, errors.New("Required argument is nil: logger")
	}
	return &ChannelValidatorImpl{
		store:  informer.GetStore(),
		logger: logger.With(zap.String("role", "ChannelValidator")),
	}, nil
}

func (cv *ChannelValidatorImpl) Validate(ch *eventingv1alpha1.Channel) *apis.FieldError {
	errs := cv.validateChannelSpec(&ch.Spec).ViaField("spec")
	return errs
}

func (cv *ChannelValidatorImpl) validateChannelSpec(cs *eventingv1alpha1.ChannelSpec) *apis.FieldError {
	var errs *apis.FieldError

	errs = errs.Also(cv.validateProvisioner(cs.Provisioner))
	errs = errs.Also(cv.validateSubscribable(cs.Subscribable))

	return errs
}

func (cv *ChannelValidatorImpl) validateProvisioner(provs *corev1.ObjectReference) *apis.FieldError {
	if provs == nil {
		return apis.ErrMissingField("provisioner")
	}
	if !cv.supportedProvisioners(provs) {
		return &apis.FieldError{
			Message: "Invalid",
			Details: "This kind of channel provisioner is not supported. Supported provisioners inclue: {apiVersion:eventing.knative.dev/v1alpha1, kind:ClusterChannelProvisioner}",
			Paths:   []string{"provisioner"},
		}
	}

	if !cv.provisionerInstalled(provs) {
		return &apis.FieldError{
			Message: "Not found",
			Details: fmt.Sprintf("Specified channel provisioner 'name:%v, kind:%v, apiVersion:%v ' not installed in cluster.", provs.Name, provs.Kind, provs.APIVersion),
			Paths:   []string{"provisioner"},
		}
	}
	return nil
}

func (cv *ChannelValidatorImpl) supportedProvisioners(obj *corev1.ObjectReference) bool {
	if obj != nil &&
		obj.APIVersion == "eventing.knative.dev/v1alpha1" &&
		obj.Kind == "ClusterChannelProvisioner" {
		return true
	}
	return false
}

func (cv *ChannelValidatorImpl) provisionerInstalled(obj *corev1.ObjectReference) bool {
	if obj != nil {
		_, exists, err := cv.store.GetByKey(obj.Name)

		if err != nil {
			cv.logger.Error("Error while retrieving ChannelProvisioner from the informer store.")
			// In this case log the error and return true so that operators trying to create the channel are not blocked.
			return true
		}
		return exists
	}
	return false
}

func (cv *ChannelValidatorImpl) validateSubscribable(s *eventingduck.Subscribable) *apis.FieldError {
	cv.logger.Info("Entering validateSubscribable")
	var errs *apis.FieldError
	if s != nil {
		for i, subscriber := range s.Subscribers {
			if subscriber.ReplyURI == "" && subscriber.SubscriberURI == "" {
				fe := apis.ErrMissingField("replyURI", "subscriberURI")
				fe.Details = "expected at least one of, got none"
				errs = errs.Also(fe.ViaField(fmt.Sprintf("subscriber[%d]", i)).ViaField("subscribable"))
			}
		}
	}

	cv.logger.Info("Exiting validateSubscribable")
	return errs
}
